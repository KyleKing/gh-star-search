package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kyleking/gh-star-search/internal/storage"
	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the local database",
	Long:  `Remove all data and the database file. This action requires confirmation.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		force, _ := cmd.Flags().GetBool("force")
		return runClear(ctx, force)
	},
}

func init() {
	clearCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

func runClear(ctx context.Context, force bool) error {
	return runClearWithStorage(ctx, force, nil)
}

func runClearWithStorage(ctx context.Context, force bool, repo storage.Repository) error {
	// Initialize storage if not provided (for testing)
	if repo == nil {
		var err error
		repo, err = initializeStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		defer repo.Close()
	}

	// Get current stats to show what will be deleted
	stats, err := repo.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get statistics: %w", err)
	}

	if stats.TotalRepositories == 0 {
		fmt.Println("Database is already empty.")
		return nil
	}

	// Show what will be deleted
	fmt.Printf("This will delete:\n")
	fmt.Printf("  • %d repositories\n", stats.TotalRepositories)
	fmt.Printf("  • %d content chunks\n", stats.TotalContentChunks)
	fmt.Printf("  • %.2f MB of data\n", stats.DatabaseSizeMB)

	// Confirmation prompt (unless force flag is used)
	if !force {
		fmt.Printf("\nAre you sure you want to clear all data? This action cannot be undone.\n")
		fmt.Printf("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	// Clear the database
	if err := repo.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear database: %w", err)
	}

	fmt.Println("Database cleared successfully.")
	return nil
}
