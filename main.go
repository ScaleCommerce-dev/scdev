package main

import (
	"os"

	"github.com/0ploy/zdev/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
