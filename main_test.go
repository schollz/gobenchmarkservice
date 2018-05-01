package main

import (
	"testing"

	log "github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

func TestGoFmt(t *testing.T) {
	defer log.Flush()
	formatted, err := goFmt(`package main

import (
		"fmt"
		"github.com/stretchr/testify/assert"
		)
func main() {
fmt.Println("hello, world")
}`)

	assert.Equal(t, `package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
)

func main() {
	fmt.Println("hello, world")
}
`, formatted)
	assert.Nil(t, err)

	formatted, err = goFmt(`package main
fun main() {
fmt.Println("hello, world")
}`)
	assert.NotNil(t, err)

}
