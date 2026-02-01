package storage

import (
	"fmt"
	"time"

	"github.com/KyleKing/gh-star-search/internal/config"
)

// NewDuckDBRepositoryFromConfig creates a new DuckDB repository with settings from config
func NewDuckDBRepositoryFromConfig(cfg *config.DatabaseConfig) (*DuckDBRepository, error) {
	// Parse query timeout
	queryTimeout, err := time.ParseDuration(cfg.QueryTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid query_timeout: %w", err)
	}

	// Parse connection max lifetime
	connMaxLifetime, err := time.ParseDuration(cfg.ConnMaxLifetime)
	if err != nil {
		return nil, fmt.Errorf("invalid conn_max_lifetime: %w", err)
	}

	// Parse connection max idle time
	connMaxIdleTime, err := time.ParseDuration(cfg.ConnMaxIdleTime)
	if err != nil {
		return nil, fmt.Errorf("invalid conn_max_idle_time: %w", err)
	}

	// Create repository with custom settings
	repo, err := NewDuckDBRepositoryWithConfig(cfg.Path, RepositoryConfig{
		MaxOpenConns:    cfg.MaxConnections,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
		ConnMaxIdleTime: connMaxIdleTime,
		QueryTimeout:    queryTimeout,
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// RepositoryConfig holds connection pool configuration
type RepositoryConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	QueryTimeout    time.Duration
}

// NewDuckDBRepositoryWithConfig creates a repository with full configuration control
func NewDuckDBRepositoryWithConfig(dbPath string, cfg RepositoryConfig) (*DuckDBRepository, error) {
	repo, err := NewDuckDBRepositoryWithTimeout(dbPath, cfg.QueryTimeout)
	if err != nil {
		return nil, err
	}

	// Apply custom connection pool settings
	repo.db.SetMaxOpenConns(cfg.MaxOpenConns)
	repo.db.SetMaxIdleConns(cfg.MaxIdleConns)
	repo.db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	repo.db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return repo, nil
}
