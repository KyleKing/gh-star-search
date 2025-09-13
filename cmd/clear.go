package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the local database",
	Long:  `Remove all data and the database file. This action requires confirmation.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		return runClear(ctx)
	},
}

func init() {
	clearCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

func runClear(_ context.Context) error {
	// TODO: Implement clear functionality
	fmt.Println("Clear command not yet implemented")
	return nil
}
