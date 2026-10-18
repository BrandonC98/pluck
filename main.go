package main

import (
	"os"

	"github.com/brandonc98/pluck/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
