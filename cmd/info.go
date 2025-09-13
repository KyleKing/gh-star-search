package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <repository>",
	Short: "Display detailed information about a specific repository",
	Long:  `Show detailed information about a specific repository stored in the local database.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		repo := args[0]
		return runInfo(ctx, repo)
	},
}

func runInfo(_ context.Context, repo string) error {
	// TODO: Implement info functionality
	fmt.Printf("Info command not yet implemented for repository: %s\n", repo)
	return nil
}
