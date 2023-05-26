package main

import (
	"github.com/ntorresalberto/dogi/cmd"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	cmd.Execute()
}
