package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/kyleking/gh-star-search/internal/storage"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all repositories in the local database",
	Long:  `Display all repositories in the local database with basic information.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		format, _ := cmd.Flags().GetString("format")

		return runList(ctx, limit, offset, format)
	},
}

func init() {
	listCmd.Flags().IntP("limit", "l", 50, "Maximum number of repositories to display")
	listCmd.Flags().IntP("offset", "o", 0, "Number of repositories to skip")
	listCmd.Flags().StringP("format", "f", "table", "Output format (table, json, csv)")
}

func runList(ctx context.Context, limit, offset int, format string) error {
	return RunListWithStorage(ctx, limit, offset, format, nil)
}

func RunListWithStorage(ctx context.Context, limit, offset int, format string, repo storage.Repository) error {
	// Initialize storage if not provided (for testing)
	if repo == nil {
		var err error

		repo, err = initializeStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		defer repo.Close()
	}

	// Get repositories
	repos, err := repo.ListRepositories(ctx, limit, offset)
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	if len(repos) == 0 {
		fmt.Println("No repositories found. Run 'gh star-search sync' to populate the database.")
		return nil
	}

	// Format output
	switch strings.ToLower(format) {
	case "json":
		return outputJSON(repos)
	case "csv":
		return outputCSV(repos)
	case "table":
		fallthrough
	default:
		return outputTable(repos)
	}
}

func outputTable(repos []storage.StoredRepo) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Header
	fmt.Fprintln(w, "NAME\tLANGUAGE\tSTARS\tFORKS\tUPDATED\tDESCRIPTION")
	fmt.Fprintln(w, "----\t--------\t-----\t-----\t-------\t-----------")

	// Rows
	for _, repo := range repos {
		description := repo.Description
		if len(description) > 60 {
			description = description[:57] + "..."
		}

		language := repo.Language
		if language == "" {
			language = "N/A"
		}

		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%s\n",
			repo.FullName,
			language,
			repo.StargazersCount,
			repo.ForksCount,
			repo.UpdatedAt.Format("2006-01-02"),
			description,
		)
	}

	return nil
}

func outputJSON(repos []storage.StoredRepo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	return encoder.Encode(repos)
}

func outputCSV(repos []storage.StoredRepo) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Header
	if err := writer.Write([]string{"Name", "Language", "Stars", "Forks", "Updated", "Description"}); err != nil {
		return err
	}

	// Rows
	for _, repo := range repos {
		language := repo.Language
		if language == "" {
			language = "N/A"
		}

		record := []string{
			repo.FullName,
			language,
			strconv.Itoa(repo.StargazersCount),
			strconv.Itoa(repo.ForksCount),
			repo.UpdatedAt.Format("2006-01-02"),
			repo.Description,
		}

		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}
