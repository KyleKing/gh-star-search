package main

import (
	"os"

	"github.com/username/gh-star-search/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}