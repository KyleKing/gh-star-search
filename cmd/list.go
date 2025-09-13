package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all repositories in the local database",
	Long:  `Display all repositories in the local database with basic information.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		return runList(ctx)
	},
}

func init() {
	listCmd.Flags().IntP("limit", "l", 50, "Maximum number of repositories to display")
	listCmd.Flags().IntP("offset", "o", 0, "Number of repositories to skip")
	listCmd.Flags().StringP("format", "f", "table", "Output format (table, json, csv)")
}

func runList(_ context.Context) error {
	// TODO: Implement list functionality
	fmt.Println("List command not yet implemented")
	return nil
}
