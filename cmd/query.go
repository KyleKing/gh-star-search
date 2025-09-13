package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kyleking/gh-star-search/internal/config"
	"github.com/kyleking/gh-star-search/internal/llm"
	"github.com/kyleking/gh-star-search/internal/query"
	"github.com/kyleking/gh-star-search/internal/storage"
)

var queryCmd = &cobra.Command{
	Use:   "query [natural language query]",
	Short: "Search repositories using natural language",
	Long: `Parse natural language queries to search through your starred repositories.
The system will generate a DuckDB query that you can review and modify before execution.

Examples:
  gh star-search query "javascript formatter updated in last month"
  gh star-search query "go libraries for web development"
  gh star-search query "python machine learning projects with high stars"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		naturalQuery := strings.Join(args, " ")

		autoExecute, _ := cmd.Flags().GetBool("auto-execute")
		format, _ := cmd.Flags().GetString("format")

		return runQuery(ctx, naturalQuery, autoExecute, format)
	},
}

func init() {
	queryCmd.Flags().BoolP("auto-execute", "a", false, "Execute query without review")
	queryCmd.Flags().StringP("format", "f", "table", "Output format (table, json, csv)")
}

func runQuery(ctx context.Context, naturalQuery string, autoExecute bool, format string) error {
	// Initialize database
	dbPath := getDefaultDBPath()
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer repo.Close()

	// Check if database exists and has data
	stats, err := repo.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get database stats: %w", err)
	}

	if stats.TotalRepositories == 0 {
		fmt.Println("No repositories found in database. Please run 'gh star-search sync' first.")
		return nil
	}

	// Initialize LLM service
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	llmService, err := llm.NewManagerFromConfig(cfg.LLM)
	if err != nil {
		return fmt.Errorf("failed to initialize LLM service: %w", err)
	}

	// Initialize query parser
	parser := query.NewParser(llmService, nil) // We'll pass nil for now since we don't need DB connection for parsing

	fmt.Printf("Parsing natural language query: %s\n\n", naturalQuery)

	// Parse the natural language query
	parsedQuery, err := parser.Parse(ctx, naturalQuery)
	if err != nil {
		return fmt.Errorf("failed to parse query: %w", err)
	}

	// Display the generated SQL
	fmt.Printf("Generated SQL Query:\n")
	fmt.Printf("─────────────────────\n")
	fmt.Printf("%s\n\n", parsedQuery.SQL)

	if parsedQuery.Explanation != "" {
		fmt.Printf("Explanation: %s\n", parsedQuery.Explanation)
	}

	fmt.Printf("Confidence: %.2f\n", parsedQuery.Confidence)
	fmt.Printf("Query Type: %s\n\n", parsedQuery.Type)

	// Interactive review unless auto-execute is enabled
	finalSQL := parsedQuery.SQL
	if !autoExecute {
		finalSQL, err = reviewAndEditQuery(parsedQuery.SQL)
		if err != nil {
			return fmt.Errorf("query review failed: %w", err)
		}

		if finalSQL == "" {
			fmt.Println("Query execution cancelled.")
			return nil
		}
	}

	// Execute the query
	fmt.Println("Executing query...")
	results, err := repo.SearchRepositories(ctx, finalSQL)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Display results
	return displayResults(results, format)
}

func reviewAndEditQuery(sql string) (string, error) {
	fmt.Printf("Review the generated SQL query above.\n")
	fmt.Printf("Options:\n")
	fmt.Printf("  [Enter] - Execute the query as-is\n")
	fmt.Printf("  [e] - Edit the query\n")
	fmt.Printf("  [c] - Cancel\n")
	fmt.Printf("Choice: ")

	reader := bufio.NewReader(os.Stdin)
	choice, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	choice = strings.TrimSpace(strings.ToLower(choice))

	switch choice {
	case "", "y", "yes":
		return sql, nil
	case "e", "edit":
		return editQuery(sql)
	case "c", "cancel", "n", "no":
		return "", nil
	default:
		fmt.Printf("Invalid choice '%s'. Executing original query.\n", choice)
		return sql, nil
	}
}

func editQuery(originalSQL string) (string, error) {
	fmt.Printf("\nEnter your modified SQL query (press Ctrl+D when done):\n")
	fmt.Printf("Original: %s\n\n", originalSQL)

	reader := bufio.NewReader(os.Stdin)
	var lines []string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break // EOF (Ctrl+D)
		}
		lines = append(lines, line)
	}

	editedSQL := strings.TrimSpace(strings.Join(lines, ""))
	if editedSQL == "" {
		fmt.Println("No query entered. Using original query.")
		return originalSQL, nil
	}

	return editedSQL, nil
}

func displayResults(results []storage.SearchResult, format string) error {
	if len(results) == 0 {
		fmt.Println("No repositories found matching your query.")
		return nil
	}

	switch format {
	case "json":
		return displayResultsJSON(results)
	case "csv":
		return displayResultsCSV(results)
	default:
		return displayResultsTable(results)
	}
}

func displayResultsTable(results []storage.SearchResult) error {
	fmt.Printf("\nFound %d repositories:\n\n", len(results))

	for i, result := range results {
		repo := result.Repository

		fmt.Printf("%d. %s\n", i+1, repo.FullName)
		fmt.Printf("   Description: %s\n", truncateString(repo.Description, 80))
		fmt.Printf("   Language: %s | Stars: %d | Forks: %d\n",
			repo.Language, repo.StargazersCount, repo.ForksCount)

		if repo.Purpose != "" {
			fmt.Printf("   Purpose: %s\n", truncateString(repo.Purpose, 80))
		}

		if len(repo.Technologies) > 0 {
			fmt.Printf("   Technologies: %s\n", strings.Join(repo.Technologies, ", "))
		}

		if len(repo.Topics) > 0 {
			fmt.Printf("   Topics: %s\n", strings.Join(repo.Topics, ", "))
		}

		fmt.Printf("   Score: %.2f\n", result.Score)

		if len(result.Matches) > 0 {
			fmt.Printf("   Matches: ")
			for j, match := range result.Matches {
				if j > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", match.Field)
			}
			fmt.Printf("\n")
		}

		fmt.Printf("   Updated: %s\n", repo.UpdatedAt.Format("2006-01-02"))
		fmt.Println()
	}

	return nil
}

func displayResultsJSON(results []storage.SearchResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func displayResultsCSV(results []storage.SearchResult) error {
	// Print CSV header
	fmt.Println("full_name,description,language,stars,forks,purpose,technologies,topics,score,updated_at")

	for _, result := range results {
		repo := result.Repository

		// Escape CSV fields
		fullName := escapeCSV(repo.FullName)
		description := escapeCSV(repo.Description)
		language := escapeCSV(repo.Language)
		purpose := escapeCSV(repo.Purpose)
		technologies := escapeCSV(strings.Join(repo.Technologies, ";"))
		topics := escapeCSV(strings.Join(repo.Topics, ";"))
		updatedAt := repo.UpdatedAt.Format("2006-01-02")

		fmt.Printf("%s,%s,%s,%d,%d,%s,%s,%s,%.2f,%s\n",
			fullName, description, language, repo.StargazersCount, repo.ForksCount,
			purpose, technologies, topics, result.Score, updatedAt)
	}

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func escapeCSV(field string) string {
	// Simple CSV escaping - wrap in quotes if contains comma, quote, or newline
	if strings.ContainsAny(field, ",\"\n\r") {
		return `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}
	return field
}

func getDefaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./gh-star-search.db"
	}
	return filepath.Join(homeDir, ".config", "gh-star-search", "repositories.db")
}
