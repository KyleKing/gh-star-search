package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/KyleKing/gh-star-search/internal/embedding"
	"github.com/KyleKing/gh-star-search/internal/errors"
	"github.com/KyleKing/gh-star-search/internal/python"
	"github.com/KyleKing/gh-star-search/internal/query"
	"github.com/KyleKing/gh-star-search/internal/related"
	"github.com/KyleKing/gh-star-search/internal/storage"
)

const (
	// MinQueryLimit is the minimum number of results that can be returned
	MinQueryLimit = 1
	// MaxQueryLimit is the maximum number of results that can be returned
	MaxQueryLimit = 50
	// DefaultQueryLimit is the default number of results to return
	DefaultQueryLimit = 10
	// MinQueryLength is the minimum length of a search query
	MinQueryLength = 2
)

func QueryCommand() *cli.Command {
	return &cli.Command{
		Name:  "query",
		Usage: "Search starred repositories using fuzzy or vector search",
		Description: `Search your starred repositories using a query string. Supports two search modes:
- fuzzy: Full-text search with BM25 scoring (default)
- vector: Semantic similarity search using embeddings

Examples:
  gh star-search query "web framework"
  gh star-search query --mode vector "machine learning"
  gh star-search query --limit 5 --long "golang http"
  gh star-search query --related "react components"`,
		ArgsUsage: "<search-string>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "mode",
				Aliases: []string{"m"},
				Value:   "fuzzy",
				Usage:   "Search mode: fuzzy or vector",
			},
			&cli.IntFlag{
				Name:    "limit",
				Aliases: []string{"l"},
				Value:   DefaultQueryLimit,
				Usage: fmt.Sprintf(
					"Maximum number of results (%d-%d)",
					MinQueryLimit,
					MaxQueryLimit,
				),
			},
			&cli.BoolFlag{
				Name:    "long",
				Aliases: []string{"L"},
				Usage:   "Use long-form output format",
			},
			&cli.BoolFlag{
				Name:    "short",
				Aliases: []string{"s"},
				Usage:   "Use short-form output format",
			},
			&cli.BoolFlag{
				Name:    "related",
				Aliases: []string{"r"},
				Usage:   "Include related repositories in results",
			},
		},
		Action: runQuery,
	}
}

func runQuery(ctx context.Context, cmd *cli.Command) error {
	// Get configuration
	configFromContext := getConfigFromContext(ctx)

	// Parse arguments
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return errors.New(errors.ErrTypeValidation, "expected exactly one search string argument")
	}

	// Validate query string
	queryString := strings.TrimSpace(args[0])
	if err := validateQuery(queryString); err != nil {
		return err
	}

	// Get flag values
	queryMode := cmd.String("mode")
	queryLimit := int(cmd.Int("limit"))
	queryLong := cmd.Bool("long")
	queryShort := cmd.Bool("short")
	queryRelated := cmd.Bool("related")

	// Validate and normalize flags
	if err := validateQueryFlags(queryMode, queryLimit, queryLong, queryShort); err != nil {
		return err
	}

	// Initialize repository
	repo, err := storage.NewDuckDBRepository(configFromContext.Database.Path)
	if err != nil {
		return errors.Wrap(err, errors.ErrTypeDatabase, "failed to initialize database")
	}
	defer repo.Close()

	// Check if database exists and is initialized
	if err := repo.Initialize(ctx); err != nil {
		return errors.Wrap(err, errors.ErrTypeDatabase, "failed to initialize database schema")
	}

	slog.Debug("Executing query",
		slog.String("query", queryString),
		slog.String("mode", queryMode),
		slog.Int("limit", queryLimit))

	// Initialize embedding manager (nil if not configured/enabled)
	embConfig := embedding.DefaultConfig()
	var uvPath, projectDir string
	if queryMode == "vector" {
		embConfig.Enabled = true
		uvPath, err = python.FindUV()
		if err != nil {
			return errors.Wrap(err, errors.ErrTypeValidation, "vector search requires uv")
		}
		cacheDir := expandPath(configFromContext.Cache.Directory)
		projectDir, err = python.EnsureEnvironment(ctx, uvPath, cacheDir)
		if err != nil {
			return errors.Wrap(err, errors.ErrTypeValidation, "failed to prepare Python environment")
		}
	}
	embManager, err := embedding.NewManager(embConfig, uvPath, projectDir)
	if err != nil {
		slog.Warn("Failed to initialize embedding manager", slog.String("error", err.Error()))
	}

	// Initialize search engine
	searchEngine := query.NewSearchEngine(repo, embManager)

	// Create query object
	searchQuery := query.Query{
		Raw:  queryString,
		Mode: query.Mode(queryMode),
	}

	// Set search options
	searchOpts := query.SearchOptions{
		Limit:    queryLimit,
		MinScore: 0.0, // No minimum score filter for now
	}

	// Execute search
	results, err := searchEngine.Search(ctx, searchQuery, searchOpts)
	if err != nil {
		return errors.Wrap(err, errors.ErrTypeDatabase, "search execution failed")
	}

	// Display results
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Determine output format
	longForm := queryLong ||
		(!queryShort && len(results) <= 3) // Default to long for small result sets

	// Populate related counts for long-form display
	if longForm {
		for i := range results {
			sameOrg, sharedContrib, countErr := repo.GetRelatedCounts(ctx, results[i].Repository.FullName)
			if countErr == nil {
				results[i].Repository.RelatedSameOrgCount = sameOrg
				results[i].Repository.RelatedSharedContribCount = sharedContrib
			}
		}
	}

	// Display results
	for i, result := range results {
		if longForm {
			displayLongFormResult(i+1, result, queryRelated)
		} else {
			displayShortFormResult(i+1, result)
		}

		if i < len(results)-1 {
			fmt.Println() // Add spacing between results
		}
	}

	// Display related repositories if requested
	if queryRelated && len(results) > 0 {
		fmt.Println("\n--- Related Repositories ---")

		// Initialize related engine
		relatedEngine := related.NewEngine(repo)

		// Show related repos for the top result
		topRepo := results[0].Repository.FullName

		relatedRepos, err := relatedEngine.FindRelated(
			ctx,
			topRepo,
			3,
		) // Limit to 3 for query output
		if err != nil {
			slog.Warn("Failed to find related repositories",
				slog.String("repo", topRepo),
				slog.String("error", err.Error()))
		} else if len(relatedRepos) > 0 {
			fmt.Printf("Repositories related to %s:\n", topRepo)

			for i, rel := range relatedRepos {
				fmt.Printf("  %d. %s (Score: %.2f) - %s\n",
					i+1, rel.Repository.FullName, rel.Score, rel.Explanation)
			}
		} else {
			fmt.Printf("No related repositories found for %s\n", topRepo)
		}
	}

	return nil
}

// validateQuery validates the search query string
func validateQuery(query string) error {
	if len(query) < MinQueryLength {
		return errors.New(errors.ErrTypeValidation,
			fmt.Sprintf("query string must be at least %d characters long", MinQueryLength))
	}

	// Check for structured filter patterns and provide helpful message
	structuredPatterns := []string{
		"language:", "lang:", "stars:", "star:", "forks:", "fork:",
		"topic:", "topics:", "user:", "org:", "created:", "updated:",
	}

	queryLower := strings.ToLower(query)
	for _, pattern := range structuredPatterns {
		if strings.Contains(queryLower, pattern) {
			return errors.New(
				errors.ErrTypeValidation,
				fmt.Sprintf(
					"structured filters like '%s' are not yet supported. Use simple search terms instead.",
					pattern,
				),
			)
		}
	}

	return nil
}

// validateQueryFlags validates and normalizes command flags
func validateQueryFlags(queryMode string, queryLimit int, queryLong, queryShort bool) error {
	// Validate mode
	validModes := map[string]bool{"fuzzy": true, "vector": true}
	if !validModes[queryMode] {
		return errors.New(errors.ErrTypeValidation,
			fmt.Sprintf("invalid mode '%s'. Must be 'fuzzy' or 'vector'", queryMode))
	}

	// Validate limit
	if queryLimit < MinQueryLimit || queryLimit > MaxQueryLimit {
		return errors.New(errors.ErrTypeValidation,
			fmt.Sprintf("limit must be between %d and %d", MinQueryLimit, MaxQueryLimit))
	}

	// Validate format flags (can't have both)
	if queryLong && queryShort {
		return errors.New(errors.ErrTypeValidation,
			"cannot specify both --long and --short flags")
	}

	return nil
}

// displayLongFormResult displays a search result in long format
func displayLongFormResult(rank int, result query.Result, _ bool) {
	repo := result.Repository

	// Header line with link
	fmt.Printf("%d. %s  (https://github.com/%s)\n", rank, repo.FullName, repo.FullName)

	// GitHub Description
	description := repo.Description
	if description == "" {
		description = "-"
	}

	fmt.Printf("GitHub Description: %s\n", description)

	// External link (homepage)
	homepage := repo.Homepage
	if homepage == "" {
		homepage = "-"
	}

	fmt.Printf("GitHub External Description Link: %s\n", homepage)

	// Numbers: issues, PRs, stars, forks
	fmt.Printf("Numbers: %d/%d open issues, %d/%d open PRs, %d stars, %d forks\n",
		repo.OpenIssuesOpen, repo.OpenIssuesTotal,
		repo.OpenPRsOpen, repo.OpenPRsTotal,
		repo.StargazersCount, repo.ForksCount)

	// Commits
	commits30d := repo.Commits30d
	commits1y := repo.Commits1y
	commitsTotal := repo.CommitsTotal

	commits30dStr := formatCommitCount(commits30d)
	commits1yStr := formatCommitCount(commits1y)
	commitsTotalStr := formatCommitCount(commitsTotal)

	fmt.Printf("Commits: %s in last 30 days, %s in last year, %s total\n",
		commits30dStr, commits1yStr, commitsTotalStr)

	// Age
	age := formatAge(repo.CreatedAt)
	fmt.Printf("Age: %s\n", age)

	// License
	license := repo.LicenseSPDXID
	if license == "" {
		license = repo.LicenseName
	}

	if license == "" {
		license = "-"
	}

	fmt.Printf("License: %s\n", license)

	// Top 10 Contributors
	contributors := formatContributors(repo.Contributors)
	fmt.Printf("Top 10 Contributors: %s\n", contributors)

	// GitHub Topics
	topics := formatTopics(repo.Topics)
	fmt.Printf("GitHub Topics: %s\n", topics)

	// Languages
	languages := formatLanguages(repo.Languages)
	fmt.Printf("Languages: %s\n", languages)

	// Related Stars (computed counts)
	relatedStars := formatRelatedStars(repo)
	fmt.Printf("Related Stars: %s\n", relatedStars)

	// Last synced
	lastSynced := formatAge(repo.LastSynced)
	fmt.Printf("Last synced: %s\n", lastSynced)

	// Score
	fmt.Printf("Score: %.2f\n", result.Score)
}

// displayShortFormResult displays a search result in short format
func displayShortFormResult(rank int, result query.Result) {
	repo := result.Repository

	// First line: rank, name, stars, primary language, updated, score
	primaryLang := repo.Language
	if primaryLang == "" {
		primaryLang = "Unknown"
	}

	updated := formatAge(repo.UpdatedAt)

	fmt.Printf("%d. %s  ⭐ %d  %s  Updated %s  Score: %.2f\n",
		rank, repo.FullName, repo.StargazersCount, primaryLang, updated, result.Score)

	// Second line: truncated description
	description := repo.Description
	if len(description) > 80 {
		description = description[:77] + "..."
	}

	if description == "" {
		description = "-"
	}

	fmt.Printf("   %s\n", description)
}

// Helper functions for formatting

func formatCommitCount(count int) string {
	if count < 0 {
		return "?"
	}

	return strconv.Itoa(count)
}

func formatAge(timestamp time.Time) string {
	if timestamp.IsZero() {
		return "unknown"
	}

	now := time.Now()
	duration := now.Sub(timestamp)

	days := int(duration.Hours() / 24)

	if days == 0 {
		return "today"
	} else if days == 1 {
		return "1 day ago"
	} else if days < 7 {
		return fmt.Sprintf("%d days ago", days)
	} else if days < 30 {
		weeks := days / 7
		if weeks == 1 {
			return "1 week ago"
		}

		return fmt.Sprintf("%d weeks ago", weeks)
	} else if days < 365 {
		months := days / 30
		if months == 1 {
			return "1 month ago"
		}

		return fmt.Sprintf("%d months ago", months)
	}

	years := days / 365
	if years == 1 {
		return "1 year ago"
	}

	return fmt.Sprintf("%d years ago", years)
}

func formatContributors(contributors []storage.Contributor) string {
	if len(contributors) == 0 {
		return "-"
	}

	parts := make([]string, 0, 10)

	for i, contrib := range contributors {
		if i >= 10 { // Limit to top 10
			break
		}

		parts = append(parts, fmt.Sprintf("%s (%d)", contrib.Login, contrib.Contributions))
	}

	return strings.Join(parts, ", ")
}

func formatTopics(topics []string) string {
	if len(topics) == 0 {
		return "-"
	}

	return strings.Join(topics, ", ")
}

func formatLanguages(languages map[string]int64) string {
	if len(languages) == 0 {
		return "-"
	}

	var parts []string

	for lang, bytes := range languages {
		// Approximate LOC (bytes / 60 average bytes per line)
		loc := bytes / 60
		parts = append(parts, fmt.Sprintf("%s (%d LOC)", lang, loc))
	}

	return strings.Join(parts, ", ")
}

func formatRelatedStars(repo storage.StoredRepo) string {
	orgName := strings.Split(repo.FullName, "/")[0]
	return fmt.Sprintf("%d in %s, %d by top contributors",
		repo.RelatedSameOrgCount, orgName, repo.RelatedSharedContribCount)
}
