package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KyleKing/gh-star-search/internal/github"
	"github.com/KyleKing/gh-star-search/internal/processor"
	"github.com/KyleKing/gh-star-search/internal/storage"
)

func TestQueryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary database for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "integration_test.db")

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

	// Create test data
	testRepos := createTestRepositories()

	// Store test repositories
	for _, testRepo := range testRepos {
		if err := repo.StoreRepository(ctx, testRepo); err != nil {
			t.Fatalf("Failed to store test repository: %v", err)
		}
	}

	// Build FTS index after storing all repositories
	if err := repo.RebuildFTSIndex(ctx); err != nil {
		t.Fatalf("Failed to rebuild FTS index: %v", err)
	}

	// Test various query scenarios
	t.Run("LanguageFilter", func(t *testing.T) {
		// Search for "gin" which appears in the repo name and description
		// Note: "go" alone is an English stopword filtered by FTS
		results, err := repo.SearchRepositories(ctx, "gin")
		if err != nil {
			t.Errorf("Language filter query failed: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected results for gin search")
		}

		foundGo := false
		for _, result := range results {
			if result.Repository.Language == "Go" {
				foundGo = true
				break
			}
		}

		if !foundGo {
			t.Error("Expected to find Go repositories in results")
		}
	})

	t.Run("PurposeSearch", func(t *testing.T) {
		results, err := repo.SearchRepositories(ctx, "web framework")
		if err != nil {
			t.Errorf("Purpose search query failed: %v", err)
		}

		// Should find repositories with web framework in purpose
		foundMatch := false

		for _, result := range results {
			if len(result.Matches) > 0 {
				foundMatch = true
				break
			}
		}

		if !foundMatch {
			t.Error("Expected to find matches for web framework search")
		}
	})

	t.Run("SQLQuery", func(t *testing.T) {
		sqlQuery := `
		SELECT full_name, description, language, stargazers_count, 1.0 as score
		FROM repositories
		WHERE language = 'Go'
		ORDER BY stargazers_count DESC
		LIMIT 5`

		_, err := repo.SearchRepositories(ctx, sqlQuery)
		if err == nil {
			t.Error("Expected error for SQL query, got nil")
		}

		if err != nil && !strings.Contains(err.Error(), "SQL queries are not supported") {
			t.Errorf("Expected SQL rejection error, got: %v", err)
		}
	})

	t.Run("ComplexSearch", func(t *testing.T) {
		// Test search with multiple criteria
		results, err := repo.SearchRepositories(ctx, "javascript framework")
		if err != nil {
			t.Errorf("Complex search query failed: %v", err)
		}

		// Verify scoring and ranking
		for i := 1; i < len(results); i++ {
			if results[i-1].Score < results[i].Score {
				t.Error("Results should be sorted by score DESC")
			}
		}
	})

	t.Run("EmptyResults", func(t *testing.T) {
		results, err := repo.SearchRepositories(ctx, "nonexistent_technology_xyz")
		if err != nil {
			t.Errorf("Empty results query failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for nonexistent technology, got %d", len(results))
		}
	})
}

func createTestRepositories() []processor.ProcessedRepo {
	now := time.Now()

	return []processor.ProcessedRepo{
		{
			Repository: github.Repository{
				FullName:        "gin-gonic/gin",
				Description:     "Gin is a HTTP web framework written in Go",
				Language:        "Go",
				StargazersCount: 75000,
				ForksCount:      7800,
				Size:            1024,
				CreatedAt:       now.AddDate(-5, 0, 0),
				UpdatedAt:       now.AddDate(0, -1, 0),
				Topics:          []string{"go", "web", "framework", "http"},
				License: &github.License{
					Name:   "MIT License",
					SPDXID: "MIT",
				},
			},
			Chunks: []processor.ContentChunk{
				{
					Source:   "README.md",
					Type:     "readme",
					Content:  "Gin is a web framework written in Go. It features a martini-like API with performance that is up to 40 times faster thanks to httprouter.",
					Tokens:   50,
					Priority: 1,
				},
			},
			ProcessedAt: now,
			ContentHash: "gin-hash-123",
		},
		{
			Repository: github.Repository{
				FullName:        "facebook/react",
				Description:     "A declarative, efficient, and flexible JavaScript library for building user interfaces",
				Language:        "JavaScript",
				StargazersCount: 220000,
				ForksCount:      45000,
				Size:            2048,
				CreatedAt:       now.AddDate(-10, 0, 0),
				UpdatedAt:       now.AddDate(0, 0, -7),
				Topics:          []string{"javascript", "react", "frontend", "ui"},
				License: &github.License{
					Name:   "MIT License",
					SPDXID: "MIT",
				},
			},
			Chunks: []processor.ContentChunk{
				{
					Source:   "README.md",
					Type:     "readme",
					Content:  "React is a JavaScript library for building user interfaces. It lets you compose complex UIs from small and isolated pieces of code called components.",
					Tokens:   60,
					Priority: 1,
				},
			},
			ProcessedAt: now,
			ContentHash: "react-hash-456",
		},
		{
			Repository: github.Repository{
				FullName:        "golang/go",
				Description:     "The Go programming language",
				Language:        "Go",
				StargazersCount: 120000,
				ForksCount:      17000,
				Size:            50000,
				CreatedAt:       now.AddDate(-14, 0, 0),
				UpdatedAt:       now.AddDate(0, 0, -1),
				Topics:          []string{"go", "programming-language", "compiler"},
				License: &github.License{
					Name:   "BSD 3-Clause License",
					SPDXID: "BSD-3-Clause",
				},
			},
			Chunks: []processor.ContentChunk{
				{
					Source:   "README.md",
					Type:     "readme",
					Content:  "Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.",
					Tokens:   40,
					Priority: 1,
				},
			},
			ProcessedAt: now,
			ContentHash: "go-hash-789",
		},
	}
}
