package main

import (
	"os"

	"github.com/kyleking/gh-star-search/cmd"
	"github.com/kyleking/gh-star-search/internal/logging"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// Ensure logger is closed on exit
		if logger := logging.GetLogger(); logger != nil {
			logger.Close()
		}

		os.Exit(1)
	}

	// Ensure logger is closed on normal exit
	if logger := logging.GetLogger(); logger != nil {
		logger.Close()
	}
}
