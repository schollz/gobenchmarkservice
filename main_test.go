package main

import (
	"testing"

	log "github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

func TestGoGet(t *testing.T) {
	code := `package main

import (
	"fmt"
		_ "github.com/stretchr/testify/assert"
		)
func main() {
fmt.Println("hello, world")
}`
	err := ImportPackages(code)
	assert.Nil(t, err)
}

func TestGoFmt(t *testing.T) {
	defer log.Flush()
	correct := `package main

import "fmt"

func main() {
	fmt.Println("hello, world")
}
`
	formatted, err := goFmt(`package main

import "fmt"

func main() {
fmt.Println("hello, world")
}`, false)

	assert.Equal(t, correct, formatted)
	assert.Nil(t, err)

	formatted, err = goFmt(`package main

import (
		"github.com/stretchr/testify/assert"
		)
func main() {
fmt.Println("hello, world")
}`, true)

	assert.Equal(t, correct, formatted)
	assert.Nil(t, err)

	formatted, err = goFmt(`package main
fun main() {
fmt.Println("hello, world")
}`, false)
	assert.NotNil(t, err)

}
