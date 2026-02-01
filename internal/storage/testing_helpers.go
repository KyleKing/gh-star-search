package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/gh-star-search/internal/processor"
)

// NewTestDB creates a temporary test database with auto-cleanup.
// Returns the repository and a cleanup function that should be deferred.
func NewTestDB(t *testing.T) (*DuckDBRepository, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "test_db_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	repo, err := NewDuckDBRepository(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create test repository: %v", err)
	}

	ctx := context.Background()
	if err := repo.Initialize(ctx); err != nil {
		repo.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("failed to initialize test repository: %v", err)
	}

	cleanup := func() {
		if err := repo.Close(); err != nil {
			t.Errorf("failed to close test repository: %v", err)
		}
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to remove temp dir: %v", err)
		}
	}

	return repo, cleanup
}

// NewTestDBWithData creates a temporary test database pre-seeded with test repositories.
// Returns the repository and a cleanup function that should be deferred.
func NewTestDBWithData(t *testing.T, repos []processor.ProcessedRepo) (*DuckDBRepository, func()) {
	t.Helper()

	repo, cleanup := NewTestDB(t)

	ctx := context.Background()
	for _, r := range repos {
		if err := repo.StoreRepository(ctx, r); err != nil {
			cleanup()
			t.Fatalf("failed to store test repository %s: %v", r.Repository.FullName, err)
		}
	}

	return repo, cleanup
}
