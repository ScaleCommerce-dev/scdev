package main

import (
	"os"

	"github.com/ScaleCommerce-DEV/scdev/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
