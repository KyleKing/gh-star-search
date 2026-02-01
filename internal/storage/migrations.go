package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// SchemaManager handles database schema creation and migrations
type SchemaManager struct {
	db *sql.DB
}

// NewSchemaManager creates a new schema manager
func NewSchemaManager(db *sql.DB) *SchemaManager {
	return &SchemaManager{db: db}
}

// Initialize runs all pending migrations to bring schema to latest version
func (m *SchemaManager) Initialize(ctx context.Context) error {
	// Create schema_version table if it doesn't exist
	if err := m.createVersionTable(ctx); err != nil {
		return fmt.Errorf("failed to create version table: %w", err)
	}

	// Get current version
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Get available migrations
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Run pending migrations
	for _, migration := range migrations {
		if migration.version > currentVersion {
			if err := m.runMigration(ctx, migration); err != nil {
				return fmt.Errorf("failed to run migration %d: %w", migration.version, err)
			}
		}
	}

	return nil
}

// CreateLatestSchema is kept for backward compatibility but now uses Initialize
func (m *SchemaManager) CreateLatestSchema(ctx context.Context) error {
	return m.Initialize(ctx)
}

// migration represents a single SQL migration file
type migration struct {
	version int
	name    string
	sql     string
}

// createVersionTable creates the schema_version tracking table
func (m *SchemaManager) createVersionTable(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			name VARCHAR NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// getCurrentVersion returns the current schema version (0 if no migrations applied)
func (m *SchemaManager) getCurrentVersion(ctx context.Context) (int, error) {
	var version int
	err := m.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM schema_version
	`).Scan(&version)

	if err != nil {
		return 0, fmt.Errorf("failed to query version: %w", err)
	}

	return version, nil
}

// loadMigrations reads and parses all migration files from embedded filesystem
func (m *SchemaManager) loadMigrations() ([]migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Parse version from filename (e.g., "001_initial_schema.sql" -> 1)
		version, name, err := parseMigrationFilename(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("invalid migration filename %s: %w", entry.Name(), err)
		}

		// Read SQL content
		content, err := migrationFiles.ReadFile(filepath.Join("migrations", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read migration %s: %w", entry.Name(), err)
		}

		migrations = append(migrations, migration{
			version: version,
			name:    name,
			sql:     string(content),
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

// parseMigrationFilename extracts version and name from filename
// Expected format: "001_description.sql" returns (1, "description")
func parseMigrationFilename(filename string) (int, string, error) {
	// Remove .sql extension
	base := strings.TrimSuffix(filename, ".sql")

	// Split on first underscore
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("filename must be in format NNN_description.sql")
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("version prefix must be numeric: %w", err)
	}

	return version, parts[1], nil
}

// runMigration executes a single migration and records it
func (m *SchemaManager) runMigration(ctx context.Context, mig migration) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Execute migration SQL
	if _, err := tx.ExecContext(ctx, mig.sql); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_version (version, name) VALUES (?, ?)
	`, mig.version, mig.name); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
