package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

const (
	// maxRunTime is the maximum amount of time to run a benchmark
	maxRunTime = 10 * time.Second
)

// redisClient is for handling the communication and cacheing the results
var redisClient *redis.Client

// Program is the code and metadata and finished benchmarks for a program.
// This will be stored in the keystore using its hash as the key.
type Program struct {
	Created    time.Time       `json:"created,omitempty"`
	Code       string          `json:"code" binding:"required"`
	Hash       string          `json:"hash"`
	Benchmarks []BenchmarkCode `json:"benchmarks,omitempty"`
}

// BenchmarkCode contains the system info, the program hash, the meta data, and the
// benchmark results.
type BenchmarkCode struct {
	ProgramHash string    `json:"program_hash"`
	Created     time.Time `json:"created"`
	GoVersion   string    `json:"go_version"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	Cores       int       `json:"cores"`
	Stdout      string    `json:"stdout"`
	Stderr      string    `json:"stderr"`
	Error       error     `json:"error"`
}

func (bc BenchmarkCode) String() string {
	if bc.Error != nil {
		return "error: " + bc.Error.Error()
	}
	s := fmt.Sprintf(`cores: %d<br>`, bc.Cores)
	if bc.Stdout != "" {
		s += "stdout:<br>" + strings.Replace(bc.Stdout, "\n", "<br>", -1)
	}
	if bc.Stderr != "" {
		s += "stder:<br>" + strings.Replace(bc.Stderr, "\n", "<br>", -1)
	}
	return s
}

// NewBenchmark will do a new benchmark
func NewBenchmark(code string) (bc BenchmarkCode, err error) {
	stdout, stderr, err := DoBenchmark(code)
	bc = BenchmarkCode{
		Created:   time.Now(),
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Cores:     runtime.NumCPU(),
		Stdout:    stdout,
		Stderr:    stderr,
		Error:     err,
	}
	return
}

func init() {
	SetLogLevel("debug")
}

func main() {
	defer log.Flush()
	var isClient bool
	var redisServer string
	flag.StringVar(&redisServer, "redis", "localhost:6374", "address of redis")
	flag.BoolVar(&isClient, "client", false, "is client")
	flag.Parse()

	// initiate redis
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisServer,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	// make sure redis is up
	_, err := redisClient.Ping().Result()
	if err != nil {
		log.Errorf("can't start, bad connection to redis: ", err)
	}

	if isClient {
		startClient()
	} else {
		startServer()
	}
}

// startServer will start a server to listen for HTTP requests (for someone posting a new benchmark job)
// and will listen to the redis channel for finished jobs
func startServer() {
	// goroutien to listen for finished jobs
	go func() {
		pubsub := redisClient.Subscribe("finished")
		defer pubsub.Close()

		// Wait for subscription to be created before publishing message.
		subscr, err := pubsub.ReceiveTimeout(time.Second)
		if err != nil {
			log.Error(err)
			return
		}
		log.Debug(subscr)

		for {
			msg, err := pubsub.ReceiveMessage()
			if err != nil {
				log.Error(err)
				continue
			}

			// get the result
			var bc BenchmarkCode
			err = json.Unmarshal([]byte(msg.Payload), &bc)
			if err != nil {
				log.Error(err)
				continue
			}

			// get the program that created the result
			var p Program
			err = redisGet(bc.ProgramHash, &p)
			if err != nil {
				log.Error(err)
				continue
			}

			// add the result to the program cache
			p.Benchmarks = append(p.Benchmarks, bc)
			err = redisSet(bc.ProgramHash, p)
			if err != nil {
				log.Warn(err)
			}
			log.Debug(msg.Channel, msg.Payload)
		}
	}()

	// start the server for handling POST benchmarks
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(middleWareHandler(), gin.Recovery())
	router.MaxMultipartMemory = 8 << 20 // 8 MiB
	router.Static("/", "./client")
	// router.OPTIONS("/benchmark", func(c *gin.Context) { c.String(200, "ok") })
	// router.OPTIONS("/fmt", func(c *gin.Context) { c.String(200, "ok") })
	router.POST("/fmt", func(c *gin.Context) {
		err := func(c *gin.Context) (err error) {
			var p Program
			err = c.BindJSON(&p)
			if err != nil {
				log.Warn(err)
				return
			}
			code, err := goFmt(p.Code, true)
			if err != nil {
				return
			}
			c.JSON(200, gin.H{"success": true, "message": "reformatted code", "code": code})
			return
		}(c)
		if err != nil {
			log.Warn(err)
			c.JSON(200, gin.H{"success": false, "message": err.Error(), "code": ""})
		}
	})
	router.POST("/run", func(c *gin.Context) {
		// This route handles submitting new benchmarks and retrieving old results.
		// Basically, if there are no results then it will submit the benchmark as a new one.
		// Code is hashed so that the same code will always retrieve the same benchmarks.
		err := func(c *gin.Context) (err error) {
			var p Program
			err = c.BindJSON(&p)
			if err != nil {
				log.Warn(err)
				return
			}
			log.Debug(p)
			p.Code, err = goFmt(p.Code, false)

			hasher := md5.New()
			hasher.Write([]byte(p.Code))
			p.Hash = hex.EncodeToString(hasher.Sum(nil))

			// check to see if the benchmarks are done
			err = redisGet(p.Hash, &p)
			if err == nil {
				if len(p.Benchmarks) > 0 {
					message := ""
					for _, bc := range p.Benchmarks {
						message += bc.String()
					}
					c.JSON(200, gin.H{"success": true, "message": message})
					return
				}
			}

			// add the creation time
			p.Created = time.Now()
			bP, err := json.Marshal(p)
			if err != nil {
				return
			}

			// add to the results
			var foo Program
			err = redisGet(p.Hash, &foo)
			if err != nil {
				redisSet(p.Hash, p)
			}

			// publish code to new jobs
			err = redisClient.Publish("newjob", string(bP)).Err()
			if err == nil {
				log.Debugf("added job %s", p.Hash)
				c.JSON(200, gin.H{"success": true, "message": "submitted benchmarks"})
			}
			return
		}(c)
		if err != nil {
			log.Warn(err)
			c.JSON(200, gin.H{"success": false, "message": err.Error()})
		}
	})
	log.Info("Running on :8080")
	router.Run(":8080")
}

// startClient will simply listen for jobs and complete them.
func startClient() {
	pubsub := redisClient.Subscribe("newjob")
	defer pubsub.Close()

	// Wait for subscription to be created before publishing message.
	subscr, err := pubsub.ReceiveTimeout(time.Second)
	if err != nil {
		panic(err)
	}
	log.Debug(subscr)

	for {
		msg, err := pubsub.ReceiveMessage()
		if err != nil {
			log.Error(err)
			continue
		}

		log.Debug(msg.Channel, msg.Payload)

		// get the new program that needs benchmarking
		var p Program
		err = json.Unmarshal([]byte(msg.Payload), &p)
		if err != nil {
			log.Error(err)
			continue
		}

		// TODO: check to see if this particular machine has already benchmarked this program
		// TODO: check to see how old this job is (if a previous job was taking awhile, it could be tens of seconds
		// before getting to this one), and discard it if it is too old

		// benchmark the code
		bc, err := NewBenchmark(p.Code)
		if err != nil {
			log.Warn(err)
			continue
		}
		bc.ProgramHash = p.Hash

		// publish code to new jobs
		bcBytes, _ := json.Marshal(bc)
		err = redisClient.Publish("finished", string(bcBytes)).Err()
		if err != nil {
			log.Warn(err)
		}

		log.Debugf("finished job %s", bc.ProgramHash)
	}
}

// SetLogLevel determines the log level
func SetLogLevel(level string) (err error) {

	// https://en.wikipedia.org/wiki/ANSI_escape_code#3/4_bit
	// https://github.com/cihub/seelog/wiki/Log-levels
	appConfig := `
	<seelog minlevel="` + level + `">
	<outputs formatid="stdout">
	<filter levels="debug,trace">
		<console formatid="debug"/>
	</filter>
	<filter levels="info">
		<console formatid="info"/>
	</filter>
	<filter levels="critical,error">
		<console formatid="error"/>
	</filter>
	<filter levels="warn">
		<console formatid="warn"/>
	</filter>
	</outputs>
	<formats>
		<format id="stdout"   format="%Date %Time [%LEVEL] %File %FuncShort:%Line %Msg %n" />
		<format id="debug"   format="%Date %Time %EscM(37)[%LEVEL]%EscM(0) %File %FuncShort:%Line %Msg %n" />
		<format id="info"    format="%Date %Time %EscM(36)[%LEVEL]%EscM(0) %File %FuncShort:%Line %Msg %n" />
		<format id="warn"    format="%Date %Time %EscM(33)[%LEVEL]%EscM(0) %File %FuncShort:%Line %Msg %n" />
		<format id="error"   format="%Date %Time %EscM(31)[%LEVEL]%EscM(0) %File %FuncShort:%Line %Msg %n" />
	</formats>
	</seelog>
	`
	logger, err := log.LoggerFromConfigAsBytes([]byte(appConfig))
	if err != nil {
		return
	}
	log.ReplaceLogger(logger)
	return
}

// redisSet is helper for the redis key store
func redisSet(key string, value interface{}) (err error) {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}

	err = redisClient.Set(key, string(b), 0).Err()
	return
}

// redisGet is helper for the redis key store, returns an error
// if the value is not found.
func redisGet(key string, value interface{}) (err error) {
	valueString, err := redisClient.Get(key).Result()
	if err != nil {
		return
	}
	return json.Unmarshal([]byte(valueString), &value)
}

// DoBenchmark will create a temporary directory with the code
// and run the tests and return the output.
func DoBenchmark(code string) (stdoutString string, stderrString string, err error) {

	// TODO: before running benchmark, make sure to import third-party packages
	// here it would be useful to do a git clone --depth 1 on the imports so
	// to download them faster and use less space
	fset := token.NewFileSet() // positions are relative to fset
	// Parse src but stop after processing the imports.
	f, err := parser.ParseFile(fset, "", code, parser.ImportsOnly)
	if err != nil {
		log.Warn(err)
		return
	}
	// Print the imports from the file's AST.
	for _, s := range f.Imports {
		log.Debug(s.Path.Value)
	}

	content := []byte(code)
	dir, err := ioutil.TempDir("", "example")
	if err != nil {
		return
	}

	defer os.RemoveAll(dir) // clean up

	fname := "main_test.go"
	if strings.Contains(code, "func main()") {
		fname = "main.go"
	}
	// create the temp directory
	tmpfn := filepath.Join(dir, fname)
	if err = ioutil.WriteFile(tmpfn, content, 0666); err != nil {
		return
	}

	// enter the temp directory
	os.Chdir(dir)
	defer os.Chdir("..")

	ctx, cancel := context.WithTimeout(context.Background(), maxRunTime)
	defer cancel()
	var cmd *exec.Cmd
	if strings.Contains(code, "func main()") {
		log.Debug("go run")
		cmd = exec.CommandContext(ctx, "go", "run", "main.go")
	} else {
		log.Debug("go test")
		cmd = exec.CommandContext(ctx, "go", "test", "-bench=.")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("process took too long")
		}
		if _, ok := err.(*exec.ExitError); !ok {
			err = fmt.Errorf("error running sandbox: %v", err)
		}
	}
	if err != nil {
		return "", "", errors.Wrap(err, "problem running")
	}

	stderrString = string(stderr.Bytes())
	stdoutString = string(stdout.Bytes())
	return
}

func goFmt(s string, doimports bool) (formatted string, err error) {
	// create a temp file
	content := []byte(s)
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		return
	}

	defer os.Remove(tmpfile.Name()) // clean up

	if _, err = tmpfile.Write(content); err != nil {
		return
	}
	if err = tmpfile.Close(); err != nil {
		return
	}

	// run gofmt
	cmd := exec.Command("gofmt", "-e", "-s", "-w", tmpfile.Name())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	// check for errors from gofmt
	if strings.TrimSpace(string(stderr.Bytes())) != "" {
		err = fmt.Errorf(strings.TrimSpace(string(stderr.Bytes())))
		formatted = s
		return
	}

	if doimports {
		// run goimports
		cmd = exec.Command("goimports", "-w", tmpfile.Name())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		// check for errors from goimports
		if strings.TrimSpace(string(stderr.Bytes())) != "" {
			err = fmt.Errorf(strings.TrimSpace(string(stderr.Bytes())))
			formatted = s
			return
		}

	}

	bFormatted, err := ioutil.ReadFile(tmpfile.Name())
	if err != nil {
		return
	}

	formatted = string(bFormatted)
	return
}

func middleWareHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := time.Now()
		// Add base headers
		addCORS(c)
		// Run next function
		c.Next()
		// Log request
		log.Infof("%v %v %v %s", c.Request.RemoteAddr, c.Request.Method, c.Request.URL, time.Since(t))
	}
}

func addCORS(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Max-Age", "86400")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "GET")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Max")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
}
