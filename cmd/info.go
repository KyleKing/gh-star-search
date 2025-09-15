package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyleking/gh-star-search/internal/storage"
	"github.com/urfave/cli/v3"
)

func InfoCommand() *cli.Command {
	return &cli.Command{
		Name:        "info",
		Usage:       "Display detailed information about a specific repository",
		Description: `Show detailed information about a specific repository stored in the local database.`,
		ArgsUsage:   " <repository>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args := cmd.Args()
			if args.Len() != 1 {
				return fmt.Errorf("expected exactly 1 argument, got %d", args.Len())
			}
			repo := args.First()
			return runInfo(ctx, repo)
		},
	}
}

func runInfo(ctx context.Context, repoName string) error {
	return RunInfoWithStorage(ctx, repoName, nil)
}

func RunInfoWithStorage(ctx context.Context, repoName string, repo storage.Repository) error {
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

	// Get repository
	storedRepo, err := repo.GetRepository(ctx, repoName)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Display detailed information
	fmt.Printf("Repository: %s\n", storedRepo.FullName)
	fmt.Printf("Description: %s\n", storedRepo.Description)
	fmt.Printf("Language: %s\n", getStringOrNA(storedRepo.Language))
	fmt.Printf("Stars: %d\n", storedRepo.StargazersCount)
	fmt.Printf("Forks: %d\n", storedRepo.ForksCount)
	fmt.Printf("Size: %d KB\n", storedRepo.SizeKB)
	fmt.Printf("Created: %s\n", storedRepo.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", storedRepo.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Last Synced: %s\n", storedRepo.LastSynced.Format("2006-01-02 15:04:05"))

	if storedRepo.LicenseName != "" {
		fmt.Printf("License: %s", storedRepo.LicenseName)

		if storedRepo.LicenseSPDXID != "" {
			fmt.Printf(" (%s)", storedRepo.LicenseSPDXID)
		}

		fmt.Println()
	}

	if len(storedRepo.Topics) > 0 {
		fmt.Printf("Topics: %s\n", strings.Join(storedRepo.Topics, ", "))
	}

	if len(storedRepo.Technologies) > 0 {
		fmt.Printf("Technologies: %s\n", strings.Join(storedRepo.Technologies, ", "))
	}

	if storedRepo.Purpose != "" {
		fmt.Printf("\nPurpose:\n%s\n", storedRepo.Purpose)
	}

	if len(storedRepo.UseCases) > 0 {
		fmt.Printf("\nUse Cases:\n")

		for _, useCase := range storedRepo.UseCases {
			fmt.Printf("  • %s\n", useCase)
		}
	}

	if len(storedRepo.Features) > 0 {
		fmt.Printf("\nFeatures:\n")

		for _, feature := range storedRepo.Features {
			fmt.Printf("  • %s\n", feature)
		}
	}

	if storedRepo.InstallationInstructions != "" {
		fmt.Printf("\nInstallation:\n%s\n", storedRepo.InstallationInstructions)
	}

	if storedRepo.UsageInstructions != "" {
		fmt.Printf("\nUsage:\n%s\n", storedRepo.UsageInstructions)
	}

	if len(storedRepo.Chunks) > 0 {
		fmt.Printf("\nContent Chunks: %d\n", len(storedRepo.Chunks))

		// Group chunks by type
		chunksByType := make(map[string]int)
		for _, chunk := range storedRepo.Chunks {
			chunksByType[chunk.Type]++
		}

		for chunkType, count := range chunksByType {
			fmt.Printf("  %s: %d chunks\n", chunkType, count)
		}
	}

	return nil
}

func getStringOrNA(s string) string {
	if s == "" {
		return "N/A"
	}

	return s
}
