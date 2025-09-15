package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kyleking/gh-star-search/internal/errors"
	"github.com/kyleking/gh-star-search/internal/related"
	"github.com/kyleking/gh-star-search/internal/storage"
)

var (
	relatedLimit int
)

var relatedCmd = &cobra.Command{
	Use:   "related <repository>",
	Short: "Find repositories related to the specified repository",
	Long: `Find repositories related to the specified repository based on:
- Same organization
- Shared GitHub topics  
- Shared contributors
- Vector similarity (if embeddings available)

The repository should be specified in owner/name format (e.g., "facebook/react").

Examples:
  gh star-search related facebook/react
  gh star-search related --limit 3 golang/go`,
	Args: cobra.ExactArgs(1),
	RunE: runRelated,
}

func init() {
	relatedCmd.Flags().IntVar(&relatedLimit, "limit", 5, "Maximum number of related repositories to show (1-20)")

	// Add to root command
	rootCmd.AddCommand(relatedCmd)
}

func runRelated(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Get configuration
	cfg, err := GetConfigFromContext(cmd)
	if err != nil {
		return err
	}

	// Validate repository name
	repoFullName := args[0]
	if err := validateRepositoryName(repoFullName); err != nil {
		return err
	}

	// Validate limit
	if relatedLimit < 1 || relatedLimit > 20 {
		return errors.New(errors.ErrTypeValidation, "limit must be between 1 and 20")
	}

	// Initialize repository
	repo, err := storage.NewDuckDBRepository(cfg.Database.Path)
	if err != nil {
		return errors.Wrap(err, errors.ErrTypeDatabase, "failed to initialize database")
	}
	defer repo.Close()

	// Check if database exists and is initialized
	if err := repo.Initialize(ctx); err != nil {
		return errors.Wrap(err, errors.ErrTypeDatabase, "failed to initialize database schema")
	}

	// Verify the target repository exists
	targetRepo, err := repo.GetRepository(ctx, repoFullName)
	if err != nil {
		return errors.Wrap(err, errors.ErrTypeValidation,
			fmt.Sprintf("repository '%s' not found in your starred repositories", repoFullName))
	}

	slog.Debug("Finding repositories related to",
		slog.String("repo", repoFullName),
		slog.Int("limit", relatedLimit))

	// Initialize related engine
	relatedEngine := related.NewEngine(repo)

	// Find related repositories
	relatedRepos, err := relatedEngine.FindRelated(ctx, repoFullName, relatedLimit)
	if err != nil {
		return errors.Wrap(err, errors.ErrTypeDatabase, "failed to find related repositories")
	}

	// Display results
	if len(relatedRepos) == 0 {
		fmt.Printf("No related repositories found for %s\n", repoFullName)
		return nil
	}

	// Display header
	fmt.Printf("Repositories related to %s:\n\n", repoFullName)

	// Display each related repository in short form with explanation
	for i, rel := range relatedRepos {
		displayRelatedRepository(i+1, rel, targetRepo)

		if i < len(relatedRepos)-1 {
			fmt.Println() // Add spacing between results
		}
	}

	return nil
}

// validateRepositoryName validates the repository name format
func validateRepositoryName(repoName string) error {
	if repoName == "" {
		return errors.New(errors.ErrTypeValidation, "repository name cannot be empty")
	}

	// Check for owner/name format
	parts := strings.Split(repoName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return errors.New(errors.ErrTypeValidation,
			"repository must be in 'owner/name' format (e.g., 'facebook/react')")
	}

	return nil
}

// displayRelatedRepository displays a related repository result
func displayRelatedRepository(rank int, rel related.Repository, _ *storage.StoredRepo) {
	repo := rel.Repository

	// First line: rank, name, stars, primary language, score
	primaryLang := repo.Language
	if primaryLang == "" {
		primaryLang = "Unknown"
	}

	fmt.Printf("%d. %s  ⭐ %d  %s  Score: %.2f\n",
		rank, repo.FullName, repo.StargazersCount, primaryLang, rel.Score)

	// Second line: truncated description
	description := repo.Description
	if len(description) > 80 {
		description = description[:77] + "..."
	}

	if description == "" {
		description = "-"
	}

	fmt.Printf("   %s\n", description)

	// Third line: relationship explanation
	fmt.Printf("   Related: %s\n", rel.Explanation)
}
