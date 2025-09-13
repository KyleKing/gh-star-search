package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Display database statistics",
	Long:  `Show statistics about the local database including total repositories, last sync time, and database size.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		return runStats(ctx)
	},
}

func runStats(ctx context.Context) error {
	// TODO: Implement stats functionality
	fmt.Println("Stats command not yet implemented")
	return nil
}
