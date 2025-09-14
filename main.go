package main

import (
	"os"

	"github.com/kyleking/gh-star-search/cmd"
	"github.com/kyleking/gh-star-search/internal/logging"
)

func main() {
	// Ensure logger is closed on exit
	defer func() {
		if logger := logging.GetLogger(); logger != nil {
			logger.Close()
		}
	}()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
