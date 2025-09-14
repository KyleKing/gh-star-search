package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// SchemaManager handles database schema creation
type SchemaManager struct {
	db *sql.DB
}

// NewSchemaManager creates a new schema manager
func NewSchemaManager(db *sql.DB) *SchemaManager {
	return &SchemaManager{db: db}
}

// CreateLatestSchema creates the latest database schema
func (m *SchemaManager) CreateLatestSchema(ctx context.Context) error {
	schemaSQL := `
		-- Create repositories table with latest schema
		CREATE TABLE IF NOT EXISTS repositories (
			id VARCHAR PRIMARY KEY,
			full_name VARCHAR UNIQUE NOT NULL,
			description TEXT,
			homepage TEXT,
			language VARCHAR,
			stargazers_count INTEGER,
			forks_count INTEGER,
			size_kb INTEGER,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			last_synced TIMESTAMP,

			-- Activity & Metrics
			open_issues_open INTEGER DEFAULT 0,
			open_issues_total INTEGER DEFAULT 0,
			open_prs_open INTEGER DEFAULT 0,
			open_prs_total INTEGER DEFAULT 0,
			commits_30d INTEGER DEFAULT 0,
			commits_1y INTEGER DEFAULT 0,
			commits_total INTEGER DEFAULT 0,

			-- Metadata arrays and objects
			topics_array VARCHAR[] DEFAULT [],
			languages JSON DEFAULT '{}',
			contributors JSON DEFAULT '[]',

			-- License
			license_name VARCHAR,
			license_spdx_id VARCHAR,

			-- Summary fields
			purpose TEXT,
			technologies VARCHAR,
			use_cases VARCHAR,
			features VARCHAR,
			installation_instructions TEXT,
			usage_instructions TEXT,
			summary_generated_at TIMESTAMP,
			summary_version INTEGER DEFAULT 1,
			summary_generator VARCHAR DEFAULT 'heuristic',

			-- Embedding
			repo_embedding FLOAT[384],

			-- Content tracking
			content_hash VARCHAR
		);

		-- Create content_chunks table (deprecated but kept for compatibility)
		CREATE TABLE IF NOT EXISTS content_chunks (
			id VARCHAR PRIMARY KEY,
			repository_id VARCHAR,
			source_path VARCHAR NOT NULL,
			chunk_type VARCHAR NOT NULL,
			content TEXT NOT NULL,
			tokens INTEGER,
			priority INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (repository_id) REFERENCES repositories(id)
		);

		-- Create indexes
		CREATE INDEX IF NOT EXISTS idx_repositories_language ON repositories(language);
		CREATE INDEX IF NOT EXISTS idx_repositories_updated_at ON repositories(updated_at);
		CREATE INDEX IF NOT EXISTS idx_repositories_stargazers ON repositories(stargazers_count);
		CREATE INDEX IF NOT EXISTS idx_repositories_full_name ON repositories(full_name);
		CREATE INDEX IF NOT EXISTS idx_repositories_stars ON repositories(stargazers_count);
		CREATE INDEX IF NOT EXISTS idx_repositories_commits_total ON repositories(commits_total);
		CREATE INDEX IF NOT EXISTS idx_repositories_summary_version ON repositories(summary_version);
		CREATE INDEX IF NOT EXISTS idx_content_chunks_repo_type ON content_chunks(repository_id, chunk_type);
		CREATE INDEX IF NOT EXISTS idx_content_chunks_repository_id ON content_chunks(repository_id);
	`

	_, err := m.db.ExecContext(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}

	return nil
}
