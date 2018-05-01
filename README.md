# gobenchmarkservice

A simple POC for doing a benchmark service for Go.

## Get started

### Redis 

I'm using Redis pub/sub for easy relaying between clients/server. Why redis? Redis has pub/sub built-in with only a few lines of code, it serves as the keystore, it is readily accessed on a public server, it automatically handles disconnection/reconnection. 

On one machine, run redis:

```
docker run -p 6374:6379 redis
```

This will create a temporary instance of redis that doesn't write anything to disk. To create the persistant redis you could do 

```
$ docker run -d -v /place/to/store/data:/data -p 6374:6379 --name gobenchbackend redis
```

which will then save its data to disk and can be start/stopped with `docker start/stop gobenchbackend`.

To connect to redis you need your local ip of the machine running redis, (use `ifconfig`), e.g. 192.168.X.Y. 

### Server

The server has a route `POST /benchmark` where it will handle JSON data containing `{"code":"...benchmark code here"}` and send that out to clients.

```
$ go get github.com/schollz/gobenchmarkservice
$ cd $GOPATH/src/github.com/schollz/gobenchmarkservice
$ go build -v
$ ./gobenchmarkservice -redis 192.168.X.Y:6374
```

making sure to fill in the redis address.

### Client

The client is the one running the benchmark code, it can be run in or out of Docker.

```
$ go get github.com/schollz/gobenchmarkservice
$ cd $GOPATH/src/github.com/schollz/gobenchmarkservice
$ go build -v
$ ./gobenchmarkservice -redis 192.168.X.Y:6374 -client
```

You can startup clients on multiple machines using the same commands.

##  Try it 

Try submitting a job:

```
$ http POST SERVERADDRESS:8080/benchmark code="package main

import (
        \"fmt\"
        \"testing\"
)

func BenchmarkFoo(b *testing.B) {
        for i := 0; i < b.N; i++ {
                fmt.Sprintf(\"hello\")
        }
}
"
```

The first time you submit the job it will run. Then, submit the same job again and it will show the benchmarks (if they have completed).

## Todo

Lots of stuff, but first, is this a reasonable implementation?

# License

MIT