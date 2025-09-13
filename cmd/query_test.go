package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/gh-star-search/internal/storage"
)

func TestQueryCommand(t *testing.T) {
	// Create a temporary database for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Initialize repository
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	if err := repo.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Test with empty database
	t.Run("EmptyDatabase", func(t *testing.T) {
		// This should handle the case where no repositories exist
		results, err := repo.SearchRepositories(ctx, "test query")
		if err != nil {
			t.Errorf("SearchRepositories failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results, got %d", len(results))
		}
	})

	// Test SQL query execution
	t.Run("SQLQuery", func(t *testing.T) {
		// Test with a simple SQL query
		sqlQuery := "SELECT 'test' as full_name, 'Test Repository' as description, 1.0 as score"
		results, err := repo.SearchRepositories(ctx, sqlQuery)
		if err != nil {
			t.Errorf("SQL query failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})

	// Test text search
	t.Run("TextSearch", func(t *testing.T) {
		// Test with simple text search
		results, err := repo.SearchRepositories(ctx, "nonexistent")
		if err != nil {
			t.Errorf("Text search failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for nonexistent query, got %d", len(results))
		}
	})
}

func TestDisplayResults(t *testing.T) {
	// Test result display functions
	results := []storage.SearchResult{
		{
			Repository: storage.StoredRepo{
				FullName:        "test/repo",
				Description:     "A test repository",
				Language:        "Go",
				StargazersCount: 100,
				ForksCount:      10,
				Purpose:         "Testing purposes",
				Technologies:    []string{"Go", "Testing"},
				Topics:          []string{"test", "example"},
			},
			Score: 0.95,
			Matches: []storage.Match{
				{
					Field:   "full_name",
					Content: "test/repo",
					Score:   1.0,
				},
			},
		},
	}

	t.Run("TableFormat", func(t *testing.T) {
		err := displayResultsTable(results)
		if err != nil {
			t.Errorf("displayResultsTable failed: %v", err)
		}
	})

	t.Run("JSONFormat", func(t *testing.T) {
		// Redirect stdout to capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayResultsJSON(results)

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Errorf("displayResultsJSON failed: %v", err)
		}

		// Read the output
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		if len(output) == 0 {
			t.Error("Expected JSON output, got empty string")
		}
	})

	t.Run("CSVFormat", func(t *testing.T) {
		err := displayResultsCSV(results)
		if err != nil {
			t.Errorf("displayResultsCSV failed: %v", err)
		}
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("TruncateString", func(t *testing.T) {
		tests := []struct {
			input    string
			maxLen   int
			expected string
		}{
			{"short", 10, "short"},
			{"this is a very long string that should be truncated", 20, "this is a very lo..."},
			{"", 10, ""},
		}

		for _, test := range tests {
			result := truncateString(test.input, test.maxLen)
			if result != test.expected {
				t.Errorf("truncateString(%q, %d) = %q, expected %q",
					test.input, test.maxLen, result, test.expected)
			}
		}
	})

	t.Run("EscapeCSV", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"simple", "simple"},
			{"with,comma", `"with,comma"`},
			{"with\"quote", `"with""quote"`},
			{"with\nnewline", "\"with\nnewline\""},
		}

		for _, test := range tests {
			result := escapeCSV(test.input)
			if result != test.expected {
				t.Errorf("escapeCSV(%q) = %q, expected %q",
					test.input, result, test.expected)
			}
		}
	})
}
