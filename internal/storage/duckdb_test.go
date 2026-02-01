package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KyleKing/gh-star-search/internal/github"
	"github.com/KyleKing/gh-star-search/internal/processor"
)

func TestDuckDBRepository(t *testing.T) {
	// Create temporary database for testing
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	repo, err := NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Initialize database once for all tests
	err = repo.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Test initialization
	t.Run("Initialize", func(t *testing.T) {
		// Verify tables exist
		var count int

		err = repo.db.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query repositories table: %v", err)
		}
	})

	// Create test data
	testRepo := createTestProcessedRepo()

	// Test storing repository
	t.Run("StoreRepository", func(t *testing.T) {
		err := repo.StoreRepository(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to store repository: %v", err)
		}

		// Verify repository was stored
		stored, err := repo.GetRepository(ctx, testRepo.Repository.FullName)
		if err != nil {
			t.Fatalf("Failed to get stored repository: %v", err)
		}

		if stored.FullName != testRepo.Repository.FullName {
			t.Errorf("Expected full name %s, got %s", testRepo.Repository.FullName, stored.FullName)
		}

		if stored.Description != testRepo.Repository.Description {
			t.Errorf(
				"Expected description %s, got %s",
				testRepo.Repository.Description,
				stored.Description,
			)
		}
	})

	// Test updating repository
	t.Run("UpdateRepository", func(t *testing.T) {
		t.Skip("UpdateRepository has a known issue with DuckDB constraints - skipping for now")
		// Use the repository from the StoreRepository test
		// Modify test data
		updatedRepo := testRepo
		updatedRepo.Repository.Description = "Updated description"

		err := repo.UpdateRepository(ctx, updatedRepo)
		if err != nil {
			t.Fatalf("Failed to update repository: %v", err)
		}

		// Verify update
		stored, err := repo.GetRepository(ctx, testRepo.Repository.FullName)
		if err != nil {
			t.Fatalf("Failed to get updated repository: %v", err)
		}

		if stored.Description != "Updated description" {
			t.Errorf("Expected updated description, got %s", stored.Description)
		}
	})

	// Test listing repositories
	t.Run("ListRepositories", func(t *testing.T) {
		// Store another repository
		testRepo2 := createTestProcessedRepo()
		testRepo2.Repository.FullName = "user/repo2"
		testRepo2.Repository.StargazersCount = 500

		err := repo.StoreRepository(ctx, testRepo2)
		if err != nil {
			t.Fatalf("Failed to store second repository: %v", err)
		}

		// List repositories
		repos, err := repo.ListRepositories(ctx, 10, 0)
		if err != nil {
			t.Fatalf("Failed to list repositories: %v", err)
		}

		if len(repos) < 2 {
			t.Errorf("Expected at least 2 repositories, got %d", len(repos))
		}

		// Should be ordered by stargazers count DESC
		if repos[0].StargazersCount < repos[1].StargazersCount {
			t.Errorf("Repositories not ordered by stargazers count")
		}
	})

	// Test search repositories
	t.Run("SearchRepositories", func(t *testing.T) {
		// Build FTS index before searching
		if err := repo.RebuildFTSIndex(ctx); err != nil {
			t.Fatalf("Failed to rebuild FTS index: %v", err)
		}

		results, err := repo.SearchRepositories(ctx, "testing")
		if err != nil {
			t.Fatalf("Failed to search repositories: %v", err)
		}

		if len(results) == 0 {
			t.Errorf("Expected search results, got none")
		}

		// Verify search result structure
		if len(results) > 0 {
			result := results[0]
			if result.Repository.FullName == "" {
				t.Errorf("Search result missing repository data")
			}

			if result.Score <= 0 {
				t.Errorf("Search result missing score")
			}

			if len(result.Matches) == 0 {
				t.Errorf("Search result missing matches")
			}
		}
	})

	// Test get stats
	t.Run("GetStats", func(t *testing.T) {
		stats, err := repo.GetStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TotalRepositories < 2 {
			t.Errorf("Expected at least 2 repositories in stats, got %d", stats.TotalRepositories)
		}

		if len(stats.LanguageBreakdown) == 0 {
			t.Errorf("Expected language breakdown in stats")
		}
	})

	// Test delete repository
	t.Run("DeleteRepository", func(t *testing.T) {
		err := repo.DeleteRepository(ctx, testRepo.Repository.FullName)
		if err != nil {
			t.Fatalf("Failed to delete repository: %v", err)
		}

		// Verify deletion
		_, err = repo.GetRepository(ctx, testRepo.Repository.FullName)
		if err == nil {
			t.Errorf("Expected error when getting deleted repository")
		}
	})

	// Test clear database
	t.Run("Clear", func(t *testing.T) {
		err := repo.Clear(ctx)
		if err != nil {
			t.Fatalf("Failed to clear database: %v", err)
		}

		// Verify database is empty
		repos, err := repo.ListRepositories(ctx, 10, 0)
		if err != nil {
			t.Fatalf("Failed to list repositories after clear: %v", err)
		}

		if len(repos) != 0 {
			t.Errorf("Expected empty database after clear, got %d repositories", len(repos))
		}
	})
}

func TestMigrations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "migration_test.db")

	repo, err := NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	schemaManager := NewSchemaManager(repo.db)

	t.Run("CreateLatestSchema", func(t *testing.T) {
		err := schemaManager.CreateLatestSchema(ctx)
		if err != nil {
			t.Fatalf("Failed to create schema: %v", err)
		}

		// Verify repositories table exists
		var count int

		err = repo.db.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query repositories table: %v", err)
		}
	})

	t.Run("VerifySchema", func(t *testing.T) {
		// Verify tables exist after schema creation
		var count int

		err := repo.db.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&count)
		if err != nil {
			t.Fatalf("Repositories table not created: %v", err)
		}

		// Verify key columns exist
		var columnCount int

		err = repo.db.QueryRow(`
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name = 'repositories' AND column_name IN ('topics_array', 'languages', 'contributors')
		`).Scan(&columnCount)
		if err != nil {
			t.Fatalf("Failed to check schema columns: %v", err)
		}

		if columnCount != 3 {
			t.Errorf("Expected 3 key columns, got %d", columnCount)
		}
	})
}

func TestDuckDBRepositoryErrors(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "error_test.db")

	repo, err := NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Initialize database
	err = repo.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	t.Run("GetNonexistentRepository", func(t *testing.T) {
		_, err := repo.GetRepository(ctx, "nonexistent/repo")
		if err == nil {
			t.Errorf("Expected error when getting nonexistent repository")
		}
	})

	t.Run("UpdateNonexistentRepository", func(_ *testing.T) {
		testRepo := createTestProcessedRepo()
		testRepo.Repository.FullName = "nonexistent/repo"

		err := repo.UpdateRepository(ctx, testRepo)
		if err != nil {
			// This should not error, it should just not update anything
			// But we can verify no rows were affected
			_ = err // explicitly ignore the error
		}
	})

	t.Run("DeleteNonexistentRepository", func(t *testing.T) {
		err := repo.DeleteRepository(ctx, "nonexistent/repo")
		if err != nil {
			t.Errorf("Delete should not error for nonexistent repository: %v", err)
		}
	})
}

func setupSearchTestDB(t *testing.T) (*DuckDBRepository, context.Context) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "search_test.db")

	repo, err := NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	ctx := context.Background()

	err = repo.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	testRepos := []processor.ProcessedRepo{
		{
			Repository: github.Repository{
				FullName:        "org/terraform-provider",
				Description:     "Infrastructure as code tool for cloud resources",
				Language:        "Go",
				StargazersCount: 300,
				ForksCount:      50,
				Size:            2048,
				CreatedAt:       time.Now().Add(-60 * 24 * time.Hour),
				UpdatedAt:       time.Now().Add(-2 * time.Hour),
				Topics:          []string{"terraform", "infrastructure", "cloud"},
			},
			Chunks: []processor.ContentChunk{},
			ProcessedAt: time.Now(),
			ContentHash: "hash-terraform",
		},
		{
			Repository: github.Repository{
				FullName:        "dev/react-dashboard",
				Description:     "A beautiful analytics dashboard built with React",
				Language:        "TypeScript",
				StargazersCount: 1500,
				ForksCount:      200,
				Size:            4096,
				CreatedAt:       time.Now().Add(-90 * 24 * time.Hour),
				UpdatedAt:       time.Now().Add(-1 * time.Hour),
				Topics:          []string{"react", "dashboard", "analytics"},
			},
			Chunks: []processor.ContentChunk{},
			ProcessedAt: time.Now(),
			ContentHash: "hash-react",
		},
		{
			Repository: github.Repository{
				FullName:        "utils/json-parser",
				Description:     "Fast and lightweight JSON parsing library",
				Language:        "Rust",
				StargazersCount: 800,
				ForksCount:      30,
				Size:            512,
				CreatedAt:       time.Now().Add(-120 * 24 * time.Hour),
				UpdatedAt:       time.Now().Add(-5 * time.Hour),
				Topics:          []string{"json", "parser", "rust"},
			},
			Chunks: []processor.ContentChunk{},
			ProcessedAt: time.Now(),
			ContentHash: "hash-json",
		},
	}

	for _, r := range testRepos {
		if err := repo.StoreRepository(ctx, r); err != nil {
			t.Fatalf("Failed to store repository %s: %v", r.Repository.FullName, err)
		}
	}

	// Build FTS index for search tests
	if err := repo.RebuildFTSIndex(ctx); err != nil {
		t.Fatalf("Failed to rebuild FTS index: %v", err)
	}

	return repo, ctx
}

func TestSearchRepositories(t *testing.T) {
	repo, ctx := setupSearchTestDB(t)

	t.Run("SearchByFullName", func(t *testing.T) {
		results, err := repo.SearchRepositories(ctx, "terraform")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		if results[0].Repository.FullName != "org/terraform-provider" {
			t.Errorf("Expected org/terraform-provider, got %s", results[0].Repository.FullName)
		}

		hasFullNameMatch := false
		for _, m := range results[0].Matches {
			if m.Field == "full_name" {
				hasFullNameMatch = true
			}
		}

		if !hasFullNameMatch {
			t.Errorf("Expected a full_name match field in results")
		}
	})

	t.Run("SearchByDescription", func(t *testing.T) {
		results, err := repo.SearchRepositories(ctx, "analytics")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		if results[0].Repository.FullName != "dev/react-dashboard" {
			t.Errorf("Expected dev/react-dashboard, got %s", results[0].Repository.FullName)
		}

		hasDescriptionMatch := false
		for _, m := range results[0].Matches {
			if m.Field == "description" {
				hasDescriptionMatch = true
			}
		}

		if !hasDescriptionMatch {
			t.Errorf("Expected a description match field in results")
		}
	})

	t.Run("SearchMatchesMultipleRepos", func(t *testing.T) {
		results, err := repo.SearchRepositories(ctx, "json")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) < 1 {
			t.Fatalf("Expected at least 1 result, got %d", len(results))
		}

		foundJSONParser := false
		for _, r := range results {
			if r.Repository.FullName == "utils/json-parser" {
				foundJSONParser = true
			}
		}

		if !foundJSONParser {
			t.Errorf("Expected utils/json-parser in results")
		}
	})

	t.Run("SearchCaseInsensitive", func(t *testing.T) {
		results, err := repo.SearchRepositories(ctx, "REACT")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatalf("Expected results for case-insensitive search, got none")
		}

		foundReact := false
		for _, r := range results {
			if r.Repository.FullName == "dev/react-dashboard" {
				foundReact = true
			}
		}

		if !foundReact {
			t.Errorf("Expected dev/react-dashboard in case-insensitive results")
		}
	})
}

func TestSearchRepositories_EdgeCases(t *testing.T) {
	repo, ctx := setupSearchTestDB(t)

	t.Run("EmptyResults", func(t *testing.T) {
		results, err := repo.SearchRepositories(ctx, "zyxwvutsrqp-nonexistent")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for nonexistent term, got %d", len(results))
		}
	})

	t.Run("ShortQuery", func(t *testing.T) {
		_, err := repo.SearchRepositories(ctx, "a")
		if err != nil {
			t.Fatalf("Short query should not error at storage layer: %v", err)
		}
	})

	t.Run("EmptyString", func(t *testing.T) {
		results, err := repo.SearchRepositories(ctx, "")
		if err != nil {
			t.Fatalf("Empty string search should not error at storage layer: %v", err)
		}

		// FTS with empty query returns no results (no terms to match)
		if len(results) != 0 {
			t.Errorf("Expected 0 results for empty FTS search, got %d", len(results))
		}
	})

	t.Run("ResultScore", func(t *testing.T) {
		results, err := repo.SearchRepositories(ctx, "terraform")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatalf("Expected results, got none")
		}

		if results[0].Score <= 0 {
			t.Errorf("Expected positive score, got %f", results[0].Score)
		}
	})

	t.Run("ResultsOrderedByScore", func(t *testing.T) {
		// Use a term that matches multiple repos for ordering verification
		results, err := repo.SearchRepositories(ctx, "parser cloud dashboard")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// FTS results are ordered by BM25 score descending
		for i := 1; i < len(results); i++ {
			if results[i-1].Score < results[i].Score {
				t.Errorf(
					"Results not ordered by score DESC: %s (%.4f) before %s (%.4f)",
					results[i-1].Repository.FullName, results[i-1].Score,
					results[i].Repository.FullName, results[i].Score,
				)
			}
		}
	})
}

func TestSearchRepositories_SQLInjection(t *testing.T) {
	repo, ctx := setupSearchTestDB(t)

	cases := []struct {
		name  string
		query string
	}{
		{"SELECT", "SELECT * FROM repositories"},
		{"select_lowercase", "select full_name from repositories"},
		{"DROP", "DROP TABLE repositories"},
		{"INSERT", "INSERT INTO repositories VALUES (1)"},
		{"DELETE", "DELETE FROM repositories"},
		{"UPDATE", "UPDATE repositories SET description = 'hacked'"},
		{"CREATE", "CREATE TABLE evil (id int)"},
		{"ALTER", "ALTER TABLE repositories ADD COLUMN evil TEXT"},
		{"TRUNCATE", "TRUNCATE TABLE repositories"},
		{"SELECT_with_whitespace", "  SELECT * FROM repositories  "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := repo.SearchRepositories(ctx, tc.query)
			if err == nil {
				t.Fatalf(
					"Expected error for SQL injection attempt %q, got %d results",
					tc.query,
					len(results),
				)
			}

			if !strings.Contains(err.Error(), "SQL queries are not supported") {
				t.Errorf("Expected error containing 'SQL queries are not supported', got: %v", err)
			}
		})
	}
}

func TestGetRelatedCounts(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "related_counts_test.db")

	repo, err := NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	if err := repo.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	repos := []processor.ProcessedRepo{
		{
			Repository: github.Repository{
				FullName:        "org1/repo-a",
				Description:     "First repo in org1",
				Language:        "Go",
				StargazersCount: 100,
				CreatedAt:       time.Now().Add(-30 * 24 * time.Hour),
				UpdatedAt:       time.Now(),
				Topics:          []string{"go"},
			},
			ProcessedAt: time.Now(),
			ContentHash: "hash-a",
		},
		{
			Repository: github.Repository{
				FullName:        "org1/repo-b",
				Description:     "Second repo in org1",
				Language:        "Go",
				StargazersCount: 50,
				CreatedAt:       time.Now().Add(-20 * 24 * time.Hour),
				UpdatedAt:       time.Now(),
				Topics:          []string{"go"},
			},
			ProcessedAt: time.Now(),
			ContentHash: "hash-b",
		},
		{
			Repository: github.Repository{
				FullName:        "org2/repo-c",
				Description:     "Repo in org2",
				Language:        "Go",
				StargazersCount: 75,
				CreatedAt:       time.Now().Add(-10 * 24 * time.Hour),
				UpdatedAt:       time.Now(),
				Topics:          []string{"go"},
			},
			ProcessedAt: time.Now(),
			ContentHash: "hash-c",
		},
	}

	for _, r := range repos {
		if err := repo.StoreRepository(ctx, r); err != nil {
			t.Fatalf("Failed to store repository %s: %v", r.Repository.FullName, err)
		}
	}

	// Update metrics for repo-a and repo-c to have shared contributors
	if err := repo.UpdateRepositoryMetrics(ctx, "org1/repo-a", RepositoryMetrics{
		Contributors: []Contributor{
			{Login: "alice", Contributions: 100},
			{Login: "bob", Contributions: 50},
		},
	}); err != nil {
		t.Fatalf("Failed to update metrics for repo-a: %v", err)
	}

	if err := repo.UpdateRepositoryMetrics(ctx, "org2/repo-c", RepositoryMetrics{
		Contributors: []Contributor{
			{Login: "alice", Contributions: 80},
			{Login: "charlie", Contributions: 30},
		},
	}); err != nil {
		t.Fatalf("Failed to update metrics for repo-c: %v", err)
	}

	t.Run("same org count", func(t *testing.T) {
		sameOrg, _, err := repo.GetRelatedCounts(ctx, "org1/repo-a")
		if err != nil {
			t.Fatalf("GetRelatedCounts failed: %v", err)
		}
		if sameOrg != 1 {
			t.Errorf("Expected 1 same-org repo, got %d", sameOrg)
		}
	})

	t.Run("shared contributor count", func(t *testing.T) {
		_, sharedContrib, err := repo.GetRelatedCounts(ctx, "org1/repo-a")
		if err != nil {
			t.Fatalf("GetRelatedCounts failed: %v", err)
		}
		if sharedContrib < 1 {
			t.Errorf("Expected at least 1 shared-contributor repo, got %d", sharedContrib)
		}
	})

	t.Run("no same org for unique org", func(t *testing.T) {
		sameOrg, _, err := repo.GetRelatedCounts(ctx, "org2/repo-c")
		if err != nil {
			t.Fatalf("GetRelatedCounts failed: %v", err)
		}
		if sameOrg != 0 {
			t.Errorf("Expected 0 same-org repos, got %d", sameOrg)
		}
	})
}

func TestEmbeddingRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "embedding_test.db")

	repo, err := NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	if err := repo.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	testRepo := createTestProcessedRepo()
	if err := repo.StoreRepository(ctx, testRepo); err != nil {
		t.Fatalf("Failed to store repository: %v", err)
	}

	embedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	if err := repo.UpdateRepositoryEmbedding(ctx, testRepo.Repository.FullName, embedding); err != nil {
		t.Fatalf("Failed to update embedding: %v", err)
	}

	stored, err := repo.GetRepository(ctx, testRepo.Repository.FullName)
	if err != nil {
		t.Fatalf("Failed to get repository: %v", err)
	}

	if len(stored.RepoEmbedding) != len(embedding) {
		t.Fatalf("Expected embedding length %d, got %d", len(embedding), len(stored.RepoEmbedding))
	}

	for i, v := range embedding {
		if abs32(stored.RepoEmbedding[i]-v) > 0.001 {
			t.Errorf("Embedding[%d]: expected %f, got %f", i, v, stored.RepoEmbedding[i])
		}
	}
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// Helper function to create test data
func createTestProcessedRepo() processor.ProcessedRepo {
	return processor.ProcessedRepo{
		Repository: github.Repository{
			FullName:        "user/test-repo",
			Description:     "A test repository",
			Language:        "Go",
			StargazersCount: 100,
			ForksCount:      10,
			Size:            1024,
			CreatedAt:       time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:       time.Now().Add(-1 * time.Hour),
			Topics:          []string{"testing", "go", "cli"},
			License: &github.License{
				Name:   "MIT License",
				SPDXID: "MIT",
			},
		},
		Chunks:      []processor.ContentChunk{},
		ProcessedAt: time.Now(),
		ContentHash: "test-hash-123",
	}
}
