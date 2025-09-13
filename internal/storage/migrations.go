package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          string
	Down        string
}

// MigrationManager handles database schema migrations
type MigrationManager struct {
	db *sql.DB
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sql.DB) *MigrationManager {
	return &MigrationManager{db: db}
}

// GetMigrations returns all available migrations in order
func (m *MigrationManager) GetMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Initial schema creation",
			Up: `
				CREATE TABLE IF NOT EXISTS repositories (
					id VARCHAR PRIMARY KEY,
					full_name VARCHAR UNIQUE NOT NULL,
					description TEXT,
					language VARCHAR,
					stargazers_count INTEGER,
					forks_count INTEGER,
					size_kb INTEGER,
					created_at TIMESTAMP,
					updated_at TIMESTAMP,
					last_synced TIMESTAMP,
					topics VARCHAR,
					license_name VARCHAR,
					license_spdx_id VARCHAR,
					purpose TEXT,
					technologies VARCHAR,
					use_cases VARCHAR,
					features VARCHAR,
					installation_instructions TEXT,
					usage_instructions TEXT,
					content_hash VARCHAR
				);

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

				CREATE INDEX IF NOT EXISTS idx_repositories_language ON repositories(language);
				CREATE INDEX IF NOT EXISTS idx_repositories_updated_at ON repositories(updated_at);
				CREATE INDEX IF NOT EXISTS idx_repositories_stargazers ON repositories(stargazers_count);
				CREATE INDEX IF NOT EXISTS idx_repositories_full_name ON repositories(full_name);
				CREATE INDEX IF NOT EXISTS idx_content_chunks_repo_type ON content_chunks(repository_id, chunk_type);
				CREATE INDEX IF NOT EXISTS idx_content_chunks_repository_id ON content_chunks(repository_id);
			`,
			Down: `
				DROP INDEX IF EXISTS idx_content_chunks_repository_id;
				DROP INDEX IF EXISTS idx_content_chunks_repo_type;
				DROP INDEX IF EXISTS idx_repositories_full_name;
				DROP INDEX IF EXISTS idx_repositories_stargazers;
				DROP INDEX IF EXISTS idx_repositories_updated_at;
				DROP INDEX IF EXISTS idx_repositories_language;
				DROP TABLE IF EXISTS content_chunks;
				DROP TABLE IF EXISTS repositories;
			`,
		},

		// Future migrations can be added here
	}
}

// InitializeMigrationTable creates the migration tracking table
func (m *MigrationManager) InitializeMigrationTable(ctx context.Context) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		description VARCHAR NOT NULL,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := m.db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	return nil
}

// GetAppliedMigrations returns a list of applied migration versions
func (m *MigrationManager) GetAppliedMigrations(ctx context.Context) ([]int, error) {
	query := "SELECT version FROM schema_migrations ORDER BY version"

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}

	defer rows.Close()

	var versions []int

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}

		versions = append(versions, version)
	}

	return versions, rows.Err()
}

// IsMigrationApplied checks if a specific migration version has been applied
func (m *MigrationManager) IsMigrationApplied(ctx context.Context, version int) (bool, error) {
	query := "SELECT COUNT(*) FROM schema_migrations WHERE version = ?"

	var count int

	err := m.db.QueryRowContext(ctx, query, version).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check migration status: %w", err)
	}

	return count > 0, nil
}

// ApplyMigration applies a single migration
func (m *MigrationManager) ApplyMigration(ctx context.Context, migration Migration) error {
	// Check if already applied
	applied, err := m.IsMigrationApplied(ctx, migration.Version)
	if err != nil {
		return err
	}

	if applied {
		return fmt.Errorf("migration %d already applied", migration.Version)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Execute the migration
	_, err = tx.ExecContext(ctx, migration.Up)
	if err != nil {
		return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
	}

	// Record the migration as applied
	_, err = tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
		migration.Version, migration.Description)
	if err != nil {
		return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
	}

	return tx.Commit()
}

// RollbackMigration rolls back a single migration
func (m *MigrationManager) RollbackMigration(ctx context.Context, migration Migration) error {
	// Check if migration is applied
	applied, err := m.IsMigrationApplied(ctx, migration.Version)
	if err != nil {
		return err
	}

	if !applied {
		return fmt.Errorf("migration %d not applied", migration.Version)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Execute the rollback
	_, err = tx.ExecContext(ctx, migration.Down)
	if err != nil {
		return fmt.Errorf("failed to rollback migration %d: %w", migration.Version, err)
	}

	// Remove the migration record
	_, err = tx.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = ?", migration.Version)
	if err != nil {
		return fmt.Errorf("failed to remove migration record %d: %w", migration.Version, err)
	}

	return tx.Commit()
}

// MigrateUp applies all pending migrations
func (m *MigrationManager) MigrateUp(ctx context.Context) error {
	if err := m.InitializeMigrationTable(ctx); err != nil {
		return err
	}

	appliedVersions, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	appliedMap := make(map[int]bool)
	for _, version := range appliedVersions {
		appliedMap[version] = true
	}

	migrations := m.GetMigrations()
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	for _, migration := range migrations {
		if !appliedMap[migration.Version] {
			fmt.Printf("Applying migration %d: %s\n", migration.Version, migration.Description)

			if err := m.ApplyMigration(ctx, migration); err != nil {
				return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
			}
		}
	}

	return nil
}

// MigrateDown rolls back migrations to a specific version
func (m *MigrationManager) MigrateDown(ctx context.Context, targetVersion int) error {
	appliedVersions, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	migrations := m.GetMigrations()
	migrationMap := make(map[int]Migration)

	for _, migration := range migrations {
		migrationMap[migration.Version] = migration
	}

	// Sort applied versions in descending order for rollback
	sort.Sort(sort.Reverse(sort.IntSlice(appliedVersions)))

	for _, version := range appliedVersions {
		if version <= targetVersion {
			break
		}

		migration, exists := migrationMap[version]
		if !exists {
			return fmt.Errorf("migration %d not found", version)
		}

		fmt.Printf("Rolling back migration %d: %s\n", version, migration.Description)

		if err := m.RollbackMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to rollback migration %d: %w", version, err)
		}
	}

	return nil
}

// GetMigrationStatus returns the current migration status
func (m *MigrationManager) GetMigrationStatus(ctx context.Context) (map[int]MigrationStatus, error) {
	if err := m.InitializeMigrationTable(ctx); err != nil {
		return nil, err
	}

	appliedVersions, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[int]bool)
	for _, version := range appliedVersions {
		appliedMap[version] = true
	}

	migrations := m.GetMigrations()
	status := make(map[int]MigrationStatus)

	for _, migration := range migrations {
		status[migration.Version] = MigrationStatus{
			Version:     migration.Version,
			Description: migration.Description,
			Applied:     appliedMap[migration.Version],
		}
	}

	return status, nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Version     int       `json:"version"`
	Description string    `json:"description"`
	Applied     bool      `json:"applied"`
	AppliedAt   time.Time `json:"applied_at,omitempty"`
}
