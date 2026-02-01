package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/kyleking/gh-star-search/internal/storage"
)

func ClearCommand() *cli.Command {
	return &cli.Command{
		Name:        "clear",
		Usage:       "Clear the local database",
		Description: `Remove all data and the database file. This action requires confirmation.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Skip confirmation prompt",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			force := cmd.Bool("force")
			return runClear(ctx, force)
		},
	}
}

func runClear(ctx context.Context, force bool) error {
	return RunClearWithStorage(ctx, force, nil)
}

func RunClearWithStorage(ctx context.Context, force bool, repo storage.Repository) error {
	// Initialize storage if not provided (for testing)
	if repo == nil {
		var err error

		cfg := getConfigFromContext(ctx)

		repo, err = initializeStorage(cfg)
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
			fmt.Println("Operation canceled.")
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
