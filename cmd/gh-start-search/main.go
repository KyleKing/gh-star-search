package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/kyleking/gh-star-search/cmd"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	app := &cli.Command{
		Name:    "gh-star-search",
		Usage:   "Search your starred GitHub repositories using natural language",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		Commands: []*cli.Command{
			cmd.SyncCommand(),
			cmd.ListCommand(),
			cmd.InfoCommand(),
			cmd.StatsCommand(),
			cmd.ClearCommand(),
			cmd.QueryCommand(),
			cmd.RelatedCommand(),
			cmd.ConfigCommand(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
