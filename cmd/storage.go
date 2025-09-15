package cmd

import (
	"fmt"

	"github.com/kyleking/gh-star-search/internal/config"
	"github.com/kyleking/gh-star-search/internal/storage"
)

// initializeStorage creates and initializes a storage repository
func initializeStorage(cfg *config.Config) (storage.Repository, error) {
	// Expand home directory in database path
	dbPath := expandPath(cfg.Database.Path)

	// Create DuckDB repository
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return repo, nil
}
