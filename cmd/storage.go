package cmd

import (
	"fmt"

	"github.com/kyleking/gh-star-search/internal/config"
	"github.com/kyleking/gh-star-search/internal/storage"
)

// initializeStorage creates and initializes a storage repository
func initializeStorage() (storage.Repository, error) {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Expand home directory in database path
	dbPath := expandPath(cfg.Database.Path)

	// Create DuckDB repository
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return repo, nil
}
