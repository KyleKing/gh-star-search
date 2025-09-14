package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kyleking/gh-star-search/internal/errors"
	"github.com/kyleking/gh-star-search/internal/logging"
	"github.com/kyleking/gh-star-search/internal/query"
	"github.com/kyleking/gh-star-search/internal/related"
	"github.com/kyleking/gh-star-search/internal/storage"
)

var (
	queryMode    string
	queryLimit   int
	queryLong    bool
	queryShort   bool
	queryRelated bool
)

var queryCmd = &cobra.Command{
	Use:   "query <search-string>",
	Short: "Search starred repositories using fuzzy or vector search",
	Long: `Search your starred repositories using a query string. Supports two search modes:
- fuzzy: Full-text search with BM25 scoring (default)
- vector: Semantic similarity search using embeddings

Examples:
  gh star-search query "web framework"
  gh star-search query --mode vector "machine learning"
  gh star-search query --limit 5 --long "golang http"
  gh star-search query --related "react components"`,
	Args: cobra.ExactArgs(1),
	RunE: runQuery,
}

func init() {
	queryCmd.Flags().StringVar(&queryMode, "mode", "fuzzy", "Search mode: fuzzy or vector")
	queryCmd.Flags().IntVar(&queryLimit, "limit", 10, "Maximum number of results (1-50)")
	queryCmd.Flags().BoolVar(&queryLong, "long", false, "Use long-form output format")
	queryCmd.Flags().BoolVar(&queryShort, "short", false, "Use short-form output format")
	queryCmd.Flags().BoolVar(&queryRelated, "related", false, "Include related repositories in results")

	// Add to root command
	rootCmd.AddCommand(queryCmd)
}

func runQuery(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logger := logging.GetLogger()

	// Get configuration
	cfg, err := GetConfigFromContext(cmd)
	if err != nil {
		return err
	}

	// Validate query string
	queryString := strings.TrimSpace(args[0])
	if err := validateQuery(queryString); err != nil {
		return err
	}

	// Validate and normalize flags
	if err := validateQueryFlags(); err != nil {
		return err
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

	logger.Debugf("Executing query: %s (mode: %s, limit: %d)", queryString, queryMode, queryLimit)

	// Initialize search engine
	searchEngine := query.NewSearchEngine(repo)

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
	longForm := queryLong || (!queryShort && len(results) <= 3) // Default to long for small result sets

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
		relatedEngine := related.NewRelatedEngine(repo)

		// Show related repos for the top result
		topRepo := results[0].Repository.FullName

		relatedRepos, err := relatedEngine.FindRelated(ctx, topRepo, 3) // Limit to 3 for query output
		if err != nil {
			logger.Warnf("Failed to find related repositories: %v", err)
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
	if len(query) < 2 {
		return errors.New(errors.ErrTypeValidation,
			"query string must be at least 2 characters long")
	}

	// Check for structured filter patterns and provide helpful message
	structuredPatterns := []string{
		"language:", "lang:", "stars:", "star:", "forks:", "fork:",
		"topic:", "topics:", "user:", "org:", "created:", "updated:",
	}

	queryLower := strings.ToLower(query)
	for _, pattern := range structuredPatterns {
		if strings.Contains(queryLower, pattern) {
			return errors.New(errors.ErrTypeValidation,
				fmt.Sprintf("structured filters like '%s' are not yet supported. Use simple search terms instead.", pattern))
		}
	}

	return nil
}

// validateQueryFlags validates and normalizes command flags
func validateQueryFlags() error {
	// Validate mode
	validModes := map[string]bool{"fuzzy": true, "vector": true}
	if !validModes[queryMode] {
		return errors.New(errors.ErrTypeValidation,
			fmt.Sprintf("invalid mode '%s'. Must be 'fuzzy' or 'vector'", queryMode))
	}

	// Validate limit
	if queryLimit < 1 || queryLimit > 50 {
		return errors.New(errors.ErrTypeValidation,
			"limit must be between 1 and 50")
	}

	// Validate format flags (can't have both)
	if queryLong && queryShort {
		return errors.New(errors.ErrTypeValidation,
			"cannot specify both --long and --short flags")
	}

	return nil
}

// displayLongFormResult displays a search result in long format
func displayLongFormResult(rank int, result query.Result, showRelated bool) {
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

	// Summary (if available)
	if repo.Purpose != "" {
		fmt.Printf("Summary: %s\n", repo.Purpose)
	}

	// Score
	fmt.Printf("Score: %.2f\n", result.Score)

	// Planned placeholders
	fmt.Printf("(PLANNED: dependencies count)\n")
	fmt.Printf("(PLANNED: dependents count)\n")
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
	} else {
		years := days / 365
		if years == 1 {
			return "1 year ago"
		}

		return fmt.Sprintf("%d years ago", years)
	}
}

func formatContributors(contributors []storage.Contributor) string {
	if len(contributors) == 0 {
		return "-"
	}

	var parts []string

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
	// TODO: Implement actual related star counting
	// For now, return placeholder
	return "- in same org, - by top contributors"
}
