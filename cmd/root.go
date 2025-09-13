package cmd

import (
	"context"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gh-star-search",
	Short: "Search your starred GitHub repositories using natural language",
	Long: `gh-star-search is a GitHub CLI extension that ingests and indexes all repositories
starred by the currently logged-in user. It enables natural language search queries
against a local DuckDB database containing both structured metadata and unstructured
content from your starred repositories.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	ctx := context.Background()
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(clearCmd)
}
