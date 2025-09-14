package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kyleking/gh-star-search/internal/processor"
	_ "github.com/marcboeker/go-duckdb" // DuckDB driver
)

// DuckDBRepository implements the Repository interface using DuckDB
type DuckDBRepository struct {
	db   *sql.DB
	path string
}

// NewDuckDBRepository creates a new DuckDB repository instance with connection pooling
func NewDuckDBRepository(dbPath string) (*DuckDBRepository, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for optimal performance
	db.SetMaxOpenConns(10)                  // Maximum number of open connections
	db.SetMaxIdleConns(5)                   // Maximum number of idle connections
	db.SetConnMaxLifetime(30 * time.Minute) // Maximum lifetime of a connection
	db.SetConnMaxIdleTime(5 * time.Minute)  // Maximum idle time for a connection

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &DuckDBRepository{
		db:   db,
		path: dbPath,
	}

	return repo, nil
}

// Initialize creates the database schema using migrations
func (r *DuckDBRepository) Initialize(ctx context.Context) error {
	migrationManager := NewMigrationManager(r.db)

	// Check if migration is needed
	needsMigration, currentVersion, latestVersion, err := migrationManager.NeedsMigration(ctx)
	if err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if needsMigration {
		fmt.Printf("Database schema update required (v%d -> v%d)\n", currentVersion, latestVersion)

		// For now, auto-migrate without prompting in Initialize
		// The CLI commands can handle user prompting if needed
		return migrationManager.MigrateUp(ctx)
	}

	return migrationManager.MigrateUp(ctx)
}

// InitializeWithPrompt creates the database schema with user confirmation for migrations
func (r *DuckDBRepository) InitializeWithPrompt(ctx context.Context, autoConfirm bool) error {
	migrationManager := NewMigrationManager(r.db)
	return migrationManager.MigrateToLatest(ctx, autoConfirm)
}

// StoreRepository stores a new repository in the database
func (r *DuckDBRepository) StoreRepository(ctx context.Context, repo processor.ProcessedRepo) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Convert arrays and objects to JSON
	technologiesJSON, _ := json.Marshal(repo.Summary.Technologies)
	useCasesJSON, _ := json.Marshal(repo.Summary.UseCases)
	featuresJSON, _ := json.Marshal(repo.Summary.Features)
	languagesJSON, _ := json.Marshal(map[string]int64{}) // Default empty, will be populated by sync
	contributorsJSON, _ := json.Marshal([]Contributor{}) // Default empty, will be populated by sync

	// Generate a new UUID for the repository
	repoID := uuid.New().String()

	// Convert topics to JSON for storage (DuckDB doesn't handle []string directly)
	topicsJSON, _ := json.Marshal(repo.Repository.Topics)

	// Insert repository with new schema
	insertRepoSQL := `
	INSERT INTO repositories (
		id, full_name, description, homepage, language, stargazers_count, forks_count, size_kb,
		created_at, updated_at, last_synced, 
		open_issues_open, open_issues_total, open_prs_open, open_prs_total,
		commits_30d, commits_1y, commits_total,
		topics_array, languages, contributors,
		license_name, license_spdx_id,
		purpose, technologies, use_cases, features, installation_instructions,
		usage_instructions, summary_generated_at, summary_version, summary_generator,
		content_hash
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var licenseName, licenseSPDXID string
	if repo.Repository.License != nil {
		licenseName = repo.Repository.License.Name
		licenseSPDXID = repo.Repository.License.SPDXID
	}

	var summaryGeneratedAt *time.Time
	if repo.Summary.GeneratedAt != nil && !repo.Summary.GeneratedAt.IsZero() {
		summaryGeneratedAt = repo.Summary.GeneratedAt
	}

	_, err = tx.ExecContext(ctx, insertRepoSQL,
		repoID,
		repo.Repository.FullName,
		repo.Repository.Description,
		repo.Repository.Homepage,
		repo.Repository.Language,
		repo.Repository.StargazersCount,
		repo.Repository.ForksCount,
		repo.Repository.Size,
		repo.Repository.CreatedAt,
		repo.Repository.UpdatedAt,
		repo.ProcessedAt,
		0, 0, 0, 0, // Default activity metrics, will be populated by sync
		0, 0, 0, // Default commit metrics, will be populated by sync
		string(topicsJSON),
		string(languagesJSON),
		string(contributorsJSON),
		licenseName,
		licenseSPDXID,
		repo.Summary.Purpose,
		string(technologiesJSON),
		string(useCasesJSON),
		string(featuresJSON),
		repo.Summary.Installation,
		repo.Summary.Usage,
		summaryGeneratedAt,
		repo.Summary.Version,
		repo.Summary.Generator,
		repo.ContentHash,
	)
	if err != nil {
		return fmt.Errorf("failed to insert repository: %w", err)
	}

	// Insert content chunks (deprecated but still supported for backward compatibility)
	if len(repo.Chunks) > 0 {
		// Check if content_chunks table exists
		var tableExists bool
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) > 0 
			FROM information_schema.tables 
			WHERE table_name = 'content_chunks'
		`).Scan(&tableExists)

		if err == nil && tableExists {
			insertChunkSQL := `
			INSERT INTO content_chunks (id, repository_id, source_path, chunk_type, content, tokens, priority)
			VALUES (?, ?, ?, ?, ?, ?, ?)`

			for _, chunk := range repo.Chunks {
				chunkID := uuid.New().String()

				_, err := tx.ExecContext(ctx, insertChunkSQL,
					chunkID,
					repoID,
					chunk.Source,
					chunk.Type,
					chunk.Content,
					chunk.Tokens,
					chunk.Priority,
				)
				if err != nil {
					return fmt.Errorf("failed to insert content chunk: %w", err)
				}
			}
		}
	}

	return tx.Commit()
}

// UpdateRepository updates an existing repository in the database
func (r *DuckDBRepository) UpdateRepository(ctx context.Context, repo processor.ProcessedRepo) error {
	// For now, let's use a simple approach: delete the old repository and insert the new one
	// This avoids the complex update logic that's causing issues
	// Get the repository ID first
	var repoID string

	err := r.db.QueryRowContext(ctx, "SELECT id FROM repositories WHERE full_name = ?", repo.Repository.FullName).Scan(&repoID)
	if err != nil {
		return fmt.Errorf("failed to get repository ID: %w", err)
	}

	// Delete existing chunks (if table exists)
	var tableExists bool
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) > 0 
		FROM information_schema.tables 
		WHERE table_name = 'content_chunks'
	`).Scan(&tableExists)

	if err == nil && tableExists {
		_, err = r.db.ExecContext(ctx, "DELETE FROM content_chunks WHERE repository_id = ?", repoID)
		if err != nil {
			return fmt.Errorf("failed to delete existing chunks: %w", err)
		}
	}

	// Delete the repository
	_, err = r.db.ExecContext(ctx, "DELETE FROM repositories WHERE id = ?", repoID)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	// Convert arrays and objects to JSON
	topicsJSON, _ := json.Marshal(repo.Repository.Topics)
	technologiesJSON, _ := json.Marshal(repo.Summary.Technologies)
	useCasesJSON, _ := json.Marshal(repo.Summary.UseCases)
	featuresJSON, _ := json.Marshal(repo.Summary.Features)
	languagesJSON, _ := json.Marshal(map[string]int64{}) // Default empty, will be populated by sync
	contributorsJSON, _ := json.Marshal([]Contributor{}) // Default empty, will be populated by sync

	// Insert the repository with the same ID using new schema
	insertRepoSQL := `
	INSERT INTO repositories (
		id, full_name, description, homepage, language, stargazers_count, forks_count, size_kb,
		created_at, updated_at, last_synced, 
		open_issues_open, open_issues_total, open_prs_open, open_prs_total,
		commits_30d, commits_1y, commits_total,
		topics_array, languages, contributors,
		license_name, license_spdx_id,
		purpose, technologies, use_cases, features, installation_instructions,
		usage_instructions, summary_generated_at, summary_version, summary_generator,
		content_hash
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var licenseName, licenseSPDXID string
	if repo.Repository.License != nil {
		licenseName = repo.Repository.License.Name
		licenseSPDXID = repo.Repository.License.SPDXID
	}

	var summaryGeneratedAt *time.Time
	if repo.Summary.GeneratedAt != nil && !repo.Summary.GeneratedAt.IsZero() {
		summaryGeneratedAt = repo.Summary.GeneratedAt
	}

	_, err = r.db.ExecContext(ctx, insertRepoSQL,
		repoID, // Use the same ID
		repo.Repository.FullName,
		repo.Repository.Description,
		repo.Repository.Homepage,
		repo.Repository.Language,
		repo.Repository.StargazersCount,
		repo.Repository.ForksCount,
		repo.Repository.Size,
		repo.Repository.CreatedAt,
		repo.Repository.UpdatedAt,
		repo.ProcessedAt,
		0, 0, 0, 0, // Default activity metrics, will be populated by sync
		0, 0, 0, // Default commit metrics, will be populated by sync
		string(topicsJSON),
		string(languagesJSON),
		string(contributorsJSON),
		licenseName,
		licenseSPDXID,
		repo.Summary.Purpose,
		string(technologiesJSON),
		string(useCasesJSON),
		string(featuresJSON),
		repo.Summary.Installation,
		repo.Summary.Usage,
		summaryGeneratedAt,
		repo.Summary.Version,
		repo.Summary.Generator,
		repo.ContentHash,
	)
	if err != nil {
		return fmt.Errorf("failed to insert updated repository: %w", err)
	}

	// Insert content chunks if any (deprecated but still supported for backward compatibility)
	if len(repo.Chunks) > 0 && tableExists {
		insertChunkSQL := `
		INSERT INTO content_chunks (id, repository_id, source_path, chunk_type, content, tokens, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

		for _, chunk := range repo.Chunks {
			chunkID := uuid.New().String()

			_, err := r.db.ExecContext(ctx, insertChunkSQL,
				chunkID,
				repoID,
				chunk.Source,
				chunk.Type,
				chunk.Content,
				chunk.Tokens,
				chunk.Priority,
			)
			if err != nil {
				return fmt.Errorf("failed to insert content chunk: %w", err)
			}
		}
	}

	return nil
}

// DeleteRepository removes a repository and its chunks from the database
func (r *DuckDBRepository) DeleteRepository(ctx context.Context, fullName string) error {
	// Get repository ID first
	var repoID string

	err := r.db.QueryRowContext(ctx, "SELECT id FROM repositories WHERE full_name = ?", fullName).Scan(&repoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // Repository doesn't exist, nothing to delete
		}

		return fmt.Errorf("failed to get repository ID: %w", err)
	}

	// Delete chunks first
	_, err = r.db.ExecContext(ctx, "DELETE FROM content_chunks WHERE repository_id = ?", repoID)
	if err != nil {
		return fmt.Errorf("failed to delete content chunks: %w", err)
	}

	// Delete repository
	_, err = r.db.ExecContext(ctx, "DELETE FROM repositories WHERE id = ?", repoID)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	return nil
}

// GetRepository retrieves a specific repository by full name
func (r *DuckDBRepository) GetRepository(ctx context.Context, fullName string) (*StoredRepo, error) {
	// Try new schema first, fall back to old schema for backward compatibility
	query := `
	SELECT id, full_name, description, 
		   COALESCE(homepage, '') as homepage,
		   language, stargazers_count, forks_count, size_kb,
		   created_at, updated_at, last_synced,
		   COALESCE(open_issues_open, 0) as open_issues_open,
		   COALESCE(open_issues_total, 0) as open_issues_total,
		   COALESCE(open_prs_open, 0) as open_prs_open,
		   COALESCE(open_prs_total, 0) as open_prs_total,
		   COALESCE(commits_30d, 0) as commits_30d,
		   COALESCE(commits_1y, 0) as commits_1y,
		   COALESCE(commits_total, 0) as commits_total,
		   COALESCE(topics_array, '[]') as topics_data,
		   COALESCE(languages, '{}') as languages,
		   COALESCE(contributors, '[]') as contributors,
		   license_name, license_spdx_id,
		   purpose, technologies, use_cases, features, 
		   installation_instructions, usage_instructions,
		   summary_generated_at, 
		   COALESCE(summary_version, 1) as summary_version,
		   COALESCE(summary_generator, 'heuristic') as summary_generator,
		   content_hash
	FROM repositories WHERE full_name = ?`

	row := r.db.QueryRowContext(ctx, query, fullName)

	var repo StoredRepo

	var technologiesJSON, useCasesJSON, featuresJSON string

	var languagesJSON, contributorsJSON string

	var topicsData interface{} // Can be array or string

	err := row.Scan(
		&repo.ID, &repo.FullName, &repo.Description, &repo.Homepage,
		&repo.Language, &repo.StargazersCount, &repo.ForksCount, &repo.SizeKB,
		&repo.CreatedAt, &repo.UpdatedAt, &repo.LastSynced,
		&repo.OpenIssuesOpen, &repo.OpenIssuesTotal, &repo.OpenPRsOpen, &repo.OpenPRsTotal,
		&repo.Commits30d, &repo.Commits1y, &repo.CommitsTotal,
		&topicsData, &languagesJSON, &contributorsJSON,
		&repo.LicenseName, &repo.LicenseSPDXID,
		&repo.Purpose, &technologiesJSON, &useCasesJSON, &featuresJSON,
		&repo.InstallationInstructions, &repo.UsageInstructions,
		&repo.SummaryGeneratedAt, &repo.SummaryVersion, &repo.SummaryGenerator,
		&repo.ContentHash,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("repository not found: %s", fullName)
		}

		return nil, fmt.Errorf("failed to scan repository: %w", err)
	}

	// Parse topics (now always JSON string format)
	if topicsData != nil {
		if topicsStr, ok := topicsData.(string); ok && topicsStr != "" && topicsStr != "[]" {
			if err := json.Unmarshal([]byte(topicsStr), &repo.Topics); err != nil {
				// Fallback: try to parse as comma-separated string
				if topicsStr != "[]" {
					repo.Topics = strings.Split(topicsStr, ",")
				}
			}
		}
	}

	// Parse JSON fields
	if technologiesJSON != "" {
		_ = json.Unmarshal([]byte(technologiesJSON), &repo.Technologies)
	}

	if useCasesJSON != "" {
		_ = json.Unmarshal([]byte(useCasesJSON), &repo.UseCases)
	}

	if featuresJSON != "" {
		_ = json.Unmarshal([]byte(featuresJSON), &repo.Features)
	}

	if languagesJSON != "" {
		_ = json.Unmarshal([]byte(languagesJSON), &repo.Languages)
	}

	if contributorsJSON != "" {
		_ = json.Unmarshal([]byte(contributorsJSON), &repo.Contributors)
	}

	// Load chunks (deprecated but still supported)
	chunks, err := r.getRepositoryChunks(ctx, repo.ID)
	if err != nil {
		// Don't fail if chunks table doesn't exist
		repo.Chunks = []processor.ContentChunk{}
	} else {
		repo.Chunks = chunks
	}

	return &repo, nil
}

// getRepositoryChunks retrieves all chunks for a repository
func (r *DuckDBRepository) getRepositoryChunks(ctx context.Context, repoID string) ([]processor.ContentChunk, error) {
	// Check if content_chunks table exists
	var tableExists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) > 0 
		FROM information_schema.tables 
		WHERE table_name = 'content_chunks'
	`).Scan(&tableExists)

	if err != nil || !tableExists {
		return []processor.ContentChunk{}, nil
	}

	query := `
	SELECT source_path, chunk_type, content, tokens, priority
	FROM content_chunks WHERE repository_id = ?
	ORDER BY priority, source_path`

	rows, err := r.db.QueryContext(ctx, query, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []processor.ContentChunk

	for rows.Next() {
		var chunk processor.ContentChunk

		err := rows.Scan(&chunk.Source, &chunk.Type, &chunk.Content, &chunk.Tokens, &chunk.Priority)
		if err != nil {
			return nil, err
		}

		chunks = append(chunks, chunk)
	}

	return chunks, rows.Err()
}

// ListRepositories retrieves a paginated list of repositories
func (r *DuckDBRepository) ListRepositories(ctx context.Context, limit, offset int) ([]StoredRepo, error) {
	query := `
	SELECT id, full_name, description, 
		   COALESCE(homepage, '') as homepage,
		   language, stargazers_count, forks_count, size_kb,
		   created_at, updated_at, last_synced,
		   COALESCE(open_issues_open, 0) as open_issues_open,
		   COALESCE(open_issues_total, 0) as open_issues_total,
		   COALESCE(open_prs_open, 0) as open_prs_open,
		   COALESCE(open_prs_total, 0) as open_prs_total,
		   COALESCE(commits_30d, 0) as commits_30d,
		   COALESCE(commits_1y, 0) as commits_1y,
		   COALESCE(commits_total, 0) as commits_total,
		   COALESCE(topics_array, '[]') as topics_data,
		   COALESCE(languages, '{}') as languages,
		   COALESCE(contributors, '[]') as contributors,
		   license_name, license_spdx_id,
		   purpose, technologies, use_cases, features, 
		   installation_instructions, usage_instructions,
		   summary_generated_at, 
		   COALESCE(summary_version, 1) as summary_version,
		   COALESCE(summary_generator, 'heuristic') as summary_generator,
		   content_hash
	FROM repositories
	ORDER BY stargazers_count DESC, full_name
	LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query repositories: %w", err)
	}
	defer rows.Close()

	var repos []StoredRepo

	for rows.Next() {
		var repo StoredRepo

		var technologiesJSON, useCasesJSON, featuresJSON string

		var languagesJSON, contributorsJSON string

		var topicsData interface{} // Can be array or string

		err := rows.Scan(
			&repo.ID, &repo.FullName, &repo.Description, &repo.Homepage,
			&repo.Language, &repo.StargazersCount, &repo.ForksCount, &repo.SizeKB,
			&repo.CreatedAt, &repo.UpdatedAt, &repo.LastSynced,
			&repo.OpenIssuesOpen, &repo.OpenIssuesTotal, &repo.OpenPRsOpen, &repo.OpenPRsTotal,
			&repo.Commits30d, &repo.Commits1y, &repo.CommitsTotal,
			&topicsData, &languagesJSON, &contributorsJSON,
			&repo.LicenseName, &repo.LicenseSPDXID,
			&repo.Purpose, &technologiesJSON, &useCasesJSON, &featuresJSON,
			&repo.InstallationInstructions, &repo.UsageInstructions,
			&repo.SummaryGeneratedAt, &repo.SummaryVersion, &repo.SummaryGenerator,
			&repo.ContentHash,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}

		// Parse topics (now always JSON string format)
		if topicsData != nil {
			if topicsStr, ok := topicsData.(string); ok && topicsStr != "" && topicsStr != "[]" {
				if err := json.Unmarshal([]byte(topicsStr), &repo.Topics); err != nil {
					// Fallback: try to parse as comma-separated string
					if topicsStr != "[]" {
						repo.Topics = strings.Split(topicsStr, ",")
					}
				}
			}
		}

		// Parse JSON fields
		if technologiesJSON != "" {
			_ = json.Unmarshal([]byte(technologiesJSON), &repo.Technologies)
		}

		if useCasesJSON != "" {
			_ = json.Unmarshal([]byte(useCasesJSON), &repo.UseCases)
		}

		if featuresJSON != "" {
			_ = json.Unmarshal([]byte(featuresJSON), &repo.Features)
		}

		if languagesJSON != "" {
			_ = json.Unmarshal([]byte(languagesJSON), &repo.Languages)
		}

		if contributorsJSON != "" {
			_ = json.Unmarshal([]byte(contributorsJSON), &repo.Contributors)
		}

		repos = append(repos, repo)
	}

	return repos, rows.Err()
}

// SearchRepositories executes a SQL query or performs text search across repositories
func (r *DuckDBRepository) SearchRepositories(ctx context.Context, query string) ([]SearchResult, error) {
	// Check if the query looks like SQL (starts with SELECT)
	trimmedQuery := strings.TrimSpace(strings.ToUpper(query))

	if strings.HasPrefix(trimmedQuery, "SELECT") {
		return r.executeCustomSQL(ctx, query)
	}

	// Fall back to simple text search
	return r.executeTextSearch(ctx, query)
}

// executeCustomSQL executes a custom SQL query
func (r *DuckDBRepository) executeCustomSQL(ctx context.Context, sqlQuery string) ([]SearchResult, error) {
	rows, err := r.db.QueryContext(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to execute SQL query: %w", err)
	}
	defer rows.Close()

	// Get column information
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []SearchResult

	for rows.Next() {
		// Create a slice to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Try to map the results to a repository structure
		repo, score, matches := r.mapRowToRepository(columns, values)
		if repo != nil {
			results = append(results, SearchResult{
				Repository: *repo,
				Score:      score,
				Matches:    matches,
			})
		}
	}

	return results, rows.Err()
}

// executeTextSearch performs simple text search
func (r *DuckDBRepository) executeTextSearch(ctx context.Context, query string) ([]SearchResult, error) {
	searchQuery := `
	SELECT r.id, r.full_name, r.description, r.language, r.stargazers_count, r.forks_count, r.size_kb,
		   r.created_at, r.updated_at, r.last_synced, r.topics, r.license_name, r.license_spdx_id,
		   r.purpose, r.technologies, r.use_cases, r.features, r.installation_instructions,
		   r.usage_instructions, r.content_hash,
		   CAST(1.0 AS DOUBLE) as score
	FROM repositories r
	WHERE r.full_name ILIKE ?
		OR r.description ILIKE ?
		OR r.purpose ILIKE ?
		OR r.installation_instructions ILIKE ?
		OR r.usage_instructions ILIKE ?
		OR EXISTS (
			SELECT 1 FROM content_chunks c
			WHERE c.repository_id = r.id AND c.content ILIKE ?
		)
	ORDER BY r.stargazers_count DESC
	LIMIT 50`

	searchTerm := "%" + query + "%"

	rows, err := r.db.QueryContext(ctx, searchQuery, searchTerm, searchTerm, searchTerm, searchTerm, searchTerm, searchTerm)
	if err != nil {
		return nil, fmt.Errorf("failed to search repositories: %w", err)
	}

	defer rows.Close()

	var results []SearchResult

	for rows.Next() {
		var repo StoredRepo

		var score float64

		var topicsJSON, technologiesJSON, useCasesJSON, featuresJSON string

		err := rows.Scan(
			&repo.ID, &repo.FullName, &repo.Description, &repo.Language,
			&repo.StargazersCount, &repo.ForksCount, &repo.SizeKB,
			&repo.CreatedAt, &repo.UpdatedAt, &repo.LastSynced,
			&topicsJSON, &repo.LicenseName, &repo.LicenseSPDXID,
			&repo.Purpose, &technologiesJSON, &useCasesJSON, &featuresJSON,
			&repo.InstallationInstructions, &repo.UsageInstructions, &repo.ContentHash,
			&score,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		// Parse JSON arrays
		if topicsJSON != "" {
			_ = json.Unmarshal([]byte(topicsJSON), &repo.Topics)
		}

		if technologiesJSON != "" {
			json.Unmarshal([]byte(technologiesJSON), &repo.Technologies)
		}

		if useCasesJSON != "" {
			json.Unmarshal([]byte(useCasesJSON), &repo.UseCases)
		}

		if featuresJSON != "" {
			json.Unmarshal([]byte(featuresJSON), &repo.Features)
		}

		// Create matches based on which fields matched
		matches := r.findMatches(repo, query)

		results = append(results, SearchResult{
			Repository: repo,
			Score:      score,
			Matches:    matches,
		})
	}

	return results, rows.Err()
}

// mapRowToRepository attempts to map query results to a repository structure
func (r *DuckDBRepository) mapRowToRepository(columns []string, values []interface{}) (*StoredRepo, float64, []Match) {
	repo := &StoredRepo{}

	var score = 1.0

	var matches []Match

	// Create a map for easier lookup
	columnMap := make(map[string]interface{})
	for i, col := range columns {
		columnMap[strings.ToLower(col)] = values[i]
	}

	// Map common fields
	if val, ok := columnMap["id"]; ok && val != nil {
		if s, ok := val.(string); ok {
			repo.ID = s
		}
	}

	if val, ok := columnMap["full_name"]; ok && val != nil {
		if s, ok := val.(string); ok {
			repo.FullName = s
		}
	}

	if val, ok := columnMap["description"]; ok && val != nil {
		if s, ok := val.(string); ok {
			repo.Description = s
		}
	}

	if val, ok := columnMap["language"]; ok && val != nil {
		if s, ok := val.(string); ok {
			repo.Language = s
		}
	}

	if val, ok := columnMap["stargazers_count"]; ok && val != nil {
		if i, ok := val.(int64); ok {
			repo.StargazersCount = int(i)
		}
	}

	if val, ok := columnMap["forks_count"]; ok && val != nil {
		if i, ok := val.(int64); ok {
			repo.ForksCount = int(i)
		}
	}

	if val, ok := columnMap["purpose"]; ok && val != nil {
		if s, ok := val.(string); ok {
			repo.Purpose = s
		}
	}

	// Handle score if present
	if val, ok := columnMap["score"]; ok && val != nil {
		if f, ok := val.(float64); ok {
			score = f
		}
	}

	// Create basic matches
	if repo.FullName != "" {
		matches = append(matches, Match{
			Field:   "full_name",
			Content: repo.FullName,
			Score:   score,
		})
	}

	return repo, score, matches
}

// findMatches identifies which fields matched the search query
func (r *DuckDBRepository) findMatches(repo StoredRepo, query string) []Match {
	var matches []Match

	queryLower := strings.ToLower(query)

	// Check various fields for matches
	if strings.Contains(strings.ToLower(repo.FullName), queryLower) {
		matches = append(matches, Match{
			Field:   "full_name",
			Content: repo.FullName,
			Score:   1.0,
		})
	}

	if strings.Contains(strings.ToLower(repo.Description), queryLower) {
		matches = append(matches, Match{
			Field:   "description",
			Content: truncateForMatch(repo.Description, queryLower),
			Score:   0.8,
		})
	}

	if strings.Contains(strings.ToLower(repo.Purpose), queryLower) {
		matches = append(matches, Match{
			Field:   "purpose",
			Content: truncateForMatch(repo.Purpose, queryLower),
			Score:   0.9,
		})
	}

	if strings.Contains(strings.ToLower(repo.Language), queryLower) {
		matches = append(matches, Match{
			Field:   "language",
			Content: repo.Language,
			Score:   0.7,
		})
	}

	// Check technologies
	for _, tech := range repo.Technologies {
		if strings.Contains(strings.ToLower(tech), queryLower) {
			matches = append(matches, Match{
				Field:   "technologies",
				Content: tech,
				Score:   0.8,
			})
		}
	}

	// Check topics
	for _, topic := range repo.Topics {
		if strings.Contains(strings.ToLower(topic), queryLower) {
			matches = append(matches, Match{
				Field:   "topics",
				Content: topic,
				Score:   0.6,
			})
		}
	}

	return matches
}

// truncateForMatch truncates text around the matching term
func truncateForMatch(text, query string) string {
	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)

	index := strings.Index(textLower, queryLower)
	if index == -1 {
		// Fallback to simple truncation
		if len(text) > 100 {
			return text[:100] + "..."
		}

		return text
	}

	// Show context around the match
	start := index - 30
	if start < 0 {
		start = 0
	}

	end := index + len(query) + 30
	if end > len(text) {
		end = len(text)
	}

	result := text[start:end]
	if start > 0 {
		result = "..." + result
	}

	if end < len(text) {
		result += "..."
	}

	return result
}

// GetStats returns database statistics
func (r *DuckDBRepository) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{
		LanguageBreakdown: make(map[string]int),
		TopicBreakdown:    make(map[string]int),
	}

	// Get total repositories
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM repositories").Scan(&stats.TotalRepositories)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository count: %w", err)
	}

	// Get total content chunks
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM content_chunks").Scan(&stats.TotalContentChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk count: %w", err)
	}

	// Get last sync time
	var lastSyncTime *time.Time

	err = r.db.QueryRowContext(ctx, "SELECT MAX(last_synced) FROM repositories").Scan(&lastSyncTime)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get last sync time: %w", err)
	}

	if lastSyncTime != nil {
		stats.LastSyncTime = *lastSyncTime
	}

	// Get database size (approximate)
	if info, err := os.Stat(r.path); err == nil {
		stats.DatabaseSizeMB = float64(info.Size()) / (1024 * 1024)
	}

	// Get language breakdown
	langRows, err := r.db.QueryContext(ctx, "SELECT language, COUNT(*) FROM repositories WHERE language IS NOT NULL GROUP BY language ORDER BY COUNT(*) DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to get language breakdown: %w", err)
	}
	defer langRows.Close()

	for langRows.Next() {
		var language string

		var count int
		if err := langRows.Scan(&language, &count); err != nil {
			return nil, err
		}

		stats.LanguageBreakdown[language] = count
	}

	return stats, nil
}

// Clear removes all data from the database
func (r *DuckDBRepository) Clear(ctx context.Context) error {
	// Delete all content chunks first (due to foreign key constraint)
	_, err := r.db.ExecContext(ctx, "DELETE FROM content_chunks")
	if err != nil {
		return fmt.Errorf("failed to clear content chunks: %w", err)
	}

	// Delete all repositories
	_, err = r.db.ExecContext(ctx, "DELETE FROM repositories")
	if err != nil {
		return fmt.Errorf("failed to clear repositories: %w", err)
	}

	return nil
}

// UpdateRepositoryMetrics updates activity and metrics data for a repository
func (r *DuckDBRepository) UpdateRepositoryMetrics(ctx context.Context, fullName string, metrics RepositoryMetrics) error {
	languagesJSON, err := json.Marshal(metrics.Languages)
	if err != nil {
		return fmt.Errorf("failed to marshal languages: %w", err)
	}

	contributorsJSON, err := json.Marshal(metrics.Contributors)
	if err != nil {
		return fmt.Errorf("failed to marshal contributors: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	updateSQL := `
	UPDATE repositories SET 
		homepage = ?,
		open_issues_open = ?, open_issues_total = ?,
		open_prs_open = ?, open_prs_total = ?,
		commits_30d = ?, commits_1y = ?, commits_total = ?,
		languages = ?, contributors = ?,
		last_synced = CURRENT_TIMESTAMP
	WHERE full_name = ?`

	result, err := tx.ExecContext(ctx, updateSQL,
		metrics.Homepage,
		metrics.OpenIssuesOpen, metrics.OpenIssuesTotal,
		metrics.OpenPRsOpen, metrics.OpenPRsTotal,
		metrics.Commits30d, metrics.Commits1y, metrics.CommitsTotal,
		string(languagesJSON), string(contributorsJSON),
		fullName,
	)

	if err != nil {
		return fmt.Errorf("failed to update repository metrics: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no repository found with full_name: %s", fullName)
	}

	return tx.Commit()
}

// UpdateRepositoryEmbedding updates the embedding for a repository
func (r *DuckDBRepository) UpdateRepositoryEmbedding(ctx context.Context, fullName string, embedding []float32) error {
	// Convert []float32 to the format DuckDB expects for FLOAT arrays
	embeddingJSON, _ := json.Marshal(embedding)

	updateSQL := `UPDATE repositories SET repo_embedding = ? WHERE full_name = ?`

	_, err := r.db.ExecContext(ctx, updateSQL, string(embeddingJSON), fullName)
	if err != nil {
		return fmt.Errorf("failed to update repository embedding: %w", err)
	}

	return nil
}

// GetRepositoriesNeedingMetricsUpdate returns repositories that need metrics updates
func (r *DuckDBRepository) GetRepositoriesNeedingMetricsUpdate(ctx context.Context, staleDays int) ([]string, error) {
	// Use date arithmetic that DuckDB supports
	cutoffTime := time.Now().AddDate(0, 0, -staleDays)

	query := `
	SELECT full_name 
	FROM repositories 
	WHERE last_synced IS NULL 
		OR last_synced < ?
	ORDER BY last_synced ASC NULLS FIRST`

	rows, err := r.db.QueryContext(ctx, query, cutoffTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query repositories needing metrics update: %w", err)
	}
	defer rows.Close()

	var fullNames []string

	for rows.Next() {
		var fullName string
		if err := rows.Scan(&fullName); err != nil {
			return nil, err
		}

		fullNames = append(fullNames, fullName)
	}

	return fullNames, rows.Err()
}

// GetRepositoriesNeedingSummaryUpdate returns repositories that need summary updates
func (r *DuckDBRepository) GetRepositoriesNeedingSummaryUpdate(ctx context.Context, forceUpdate bool) ([]string, error) {
	var query string
	if forceUpdate {
		query = `SELECT full_name FROM repositories ORDER BY full_name`
	} else {
		query = `
		SELECT full_name 
		FROM repositories 
		WHERE purpose IS NULL OR purpose = ''
			OR summary_generated_at IS NULL
			OR summary_version < 1
		ORDER BY full_name`
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query repositories needing summary update: %w", err)
	}
	defer rows.Close()

	var fullNames []string

	for rows.Next() {
		var fullName string
		if err := rows.Scan(&fullName); err != nil {
			return nil, err
		}

		fullNames = append(fullNames, fullName)
	}

	return fullNames, rows.Err()
}

// Close closes the database connection
func (r *DuckDBRepository) Close() error {
	if r.db != nil {
		return r.db.Close()
	}

	return nil
}
