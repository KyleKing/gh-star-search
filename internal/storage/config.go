package storage

import (
	"fmt"
	"time"

	"github.com/KyleKing/gh-star-search/internal/config"
)

// NewDuckDBRepositoryFromConfig creates a new DuckDB repository with settings from config
func NewDuckDBRepositoryFromConfig(cfg *config.DatabaseConfig) (*DuckDBRepository, error) {
	queryTimeout, err := time.ParseDuration(cfg.QueryTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid query_timeout: %w", err)
	}

	return NewDuckDBRepositoryWithTimeout(cfg.Path, queryTimeout)
}
