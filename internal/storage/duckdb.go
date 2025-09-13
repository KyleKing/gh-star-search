package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// NewDuckDBRepository creates a new DuckDB repository instance
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

	repo := &DuckDBRepository{
		db:   db,
		path: dbPath,
	}

	return repo, nil
}

// Initialize creates the database schema using migrations
func (r *DuckDBRepository) Initialize(ctx context.Context) error {
	migrationManager := NewMigrationManager(r.db)
	return migrationManager.MigrateUp(ctx)
}

// StoreRepository stores a new repository in the database
func (r *DuckDBRepository) StoreRepository(ctx context.Context, repo processor.ProcessedRepo) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Convert arrays to JSON
	topicsJSON, _ := json.Marshal(repo.Repository.Topics)
	technologiesJSON, _ := json.Marshal(repo.Summary.Technologies)
	useCasesJSON, _ := json.Marshal(repo.Summary.UseCases)
	featuresJSON, _ := json.Marshal(repo.Summary.Features)

	// Generate a new UUID for the repository
	repoID := uuid.New().String()

	// Insert repository
	insertRepoSQL := `
	INSERT INTO repositories (
		id, full_name, description, language, stargazers_count, forks_count, size_kb,
		created_at, updated_at, last_synced, topics, license_name, license_spdx_id,
		purpose, technologies, use_cases, features, installation_instructions,
		usage_instructions, content_hash
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var licenseName, licenseSPDXID string
	if repo.Repository.License != nil {
		licenseName = repo.Repository.License.Name
		licenseSPDXID = repo.Repository.License.SPDXID
	}

	_, err = tx.ExecContext(ctx, insertRepoSQL,
		repoID,
		repo.Repository.FullName,
		repo.Repository.Description,
		repo.Repository.Language,
		repo.Repository.StargazersCount,
		repo.Repository.ForksCount,
		repo.Repository.Size,
		repo.Repository.CreatedAt,
		repo.Repository.UpdatedAt,
		repo.ProcessedAt,
		string(topicsJSON),
		licenseName,
		licenseSPDXID,
		repo.Summary.Purpose,
		string(technologiesJSON),
		string(useCasesJSON),
		string(featuresJSON),
		repo.Summary.Installation,
		repo.Summary.Usage,
		repo.ContentHash,
	)
	if err != nil {
		return fmt.Errorf("failed to insert repository: %w", err)
	}

	// Insert content chunks
	if len(repo.Chunks) > 0 {
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

	// Delete existing chunks
	_, err = r.db.ExecContext(ctx, "DELETE FROM content_chunks WHERE repository_id = ?", repoID)
	if err != nil {
		return fmt.Errorf("failed to delete existing chunks: %w", err)
	}

	// Delete the repository
	_, err = r.db.ExecContext(ctx, "DELETE FROM repositories WHERE id = ?", repoID)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	// Convert arrays to JSON
	topicsJSON, _ := json.Marshal(repo.Repository.Topics)
	technologiesJSON, _ := json.Marshal(repo.Summary.Technologies)
	useCasesJSON, _ := json.Marshal(repo.Summary.UseCases)
	featuresJSON, _ := json.Marshal(repo.Summary.Features)

	// Insert the repository with the same ID
	insertRepoSQL := `
	INSERT INTO repositories (
		id, full_name, description, language, stargazers_count, forks_count, size_kb,
		created_at, updated_at, last_synced, topics, license_name, license_spdx_id,
		purpose, technologies, use_cases, features, installation_instructions,
		usage_instructions, content_hash
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var licenseName, licenseSPDXID string
	if repo.Repository.License != nil {
		licenseName = repo.Repository.License.Name
		licenseSPDXID = repo.Repository.License.SPDXID
	}

	_, err = r.db.ExecContext(ctx, insertRepoSQL,
		repoID, // Use the same ID
		repo.Repository.FullName,
		repo.Repository.Description,
		repo.Repository.Language,
		repo.Repository.StargazersCount,
		repo.Repository.ForksCount,
		repo.Repository.Size,
		repo.Repository.CreatedAt,
		repo.Repository.UpdatedAt,
		repo.ProcessedAt,
		string(topicsJSON),
		licenseName,
		licenseSPDXID,
		repo.Summary.Purpose,
		string(technologiesJSON),
		string(useCasesJSON),
		string(featuresJSON),
		repo.Summary.Installation,
		repo.Summary.Usage,
		repo.ContentHash,
	)
	if err != nil {
		return fmt.Errorf("failed to insert updated repository: %w", err)
	}

	// Insert content chunks if any
	if len(repo.Chunks) > 0 {
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
		if err == sql.ErrNoRows {
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
	query := `
	SELECT id, full_name, description, language, stargazers_count, forks_count, size_kb,
		   created_at, updated_at, last_synced, topics, license_name, license_spdx_id,
		   purpose, technologies, use_cases, features, installation_instructions,
		   usage_instructions, content_hash
	FROM repositories WHERE full_name = ?`

	row := r.db.QueryRowContext(ctx, query, fullName)

	var repo StoredRepo

	var topicsJSON, technologiesJSON, useCasesJSON, featuresJSON string

	err := row.Scan(
		&repo.ID, &repo.FullName, &repo.Description, &repo.Language,
		&repo.StargazersCount, &repo.ForksCount, &repo.SizeKB,
		&repo.CreatedAt, &repo.UpdatedAt, &repo.LastSynced,
		&topicsJSON, &repo.LicenseName, &repo.LicenseSPDXID,
		&repo.Purpose, &technologiesJSON, &useCasesJSON, &featuresJSON,
		&repo.InstallationInstructions, &repo.UsageInstructions, &repo.ContentHash,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found: %s", fullName)
		}

		return nil, fmt.Errorf("failed to scan repository: %w", err)
	}

	// Parse JSON arrays
	if topicsJSON != "" {
		if err := json.Unmarshal([]byte(topicsJSON), &repo.Topics); err != nil {
			// Log error or handle, but for now ignore since data should be valid JSON
			_ = err // explicitly ignore the error
		}
	}

	if technologiesJSON != "" {
		if err := json.Unmarshal([]byte(technologiesJSON), &repo.Technologies); err != nil {
			// Log error or handle, but for now ignore since data should be valid JSON
			_ = err // explicitly ignore the error
		}
	}

	if useCasesJSON != "" {
		if err := json.Unmarshal([]byte(useCasesJSON), &repo.UseCases); err != nil {
			// Log error or handle, but for now ignore since data should be valid JSON
			_ = err // explicitly ignore the error
		}
	}

	if featuresJSON != "" {
		if err := json.Unmarshal([]byte(featuresJSON), &repo.Features); err != nil {
			// Log error or handle, but for now ignore since data should be valid JSON
			_ = err // explicitly ignore the error
		}
	}

	// Load chunks
	chunks, err := r.getRepositoryChunks(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load chunks: %w", err)
	}

	repo.Chunks = chunks

	return &repo, nil
}

// getRepositoryChunks retrieves all chunks for a repository
func (r *DuckDBRepository) getRepositoryChunks(ctx context.Context, repoID string) ([]processor.ContentChunk, error) {
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
	SELECT id, full_name, description, language, stargazers_count, forks_count, size_kb,
		   created_at, updated_at, last_synced, topics, license_name, license_spdx_id,
		   purpose, technologies, use_cases, features, installation_instructions,
		   usage_instructions, content_hash
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

		var topicsJSON, technologiesJSON, useCasesJSON, featuresJSON string

		err := rows.Scan(
			&repo.ID, &repo.FullName, &repo.Description, &repo.Language,
			&repo.StargazersCount, &repo.ForksCount, &repo.SizeKB,
			&repo.CreatedAt, &repo.UpdatedAt, &repo.LastSynced,
			&topicsJSON, &repo.LicenseName, &repo.LicenseSPDXID,
			&repo.Purpose, &technologiesJSON, &useCasesJSON, &featuresJSON,
			&repo.InstallationInstructions, &repo.UsageInstructions, &repo.ContentHash,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}

		// Parse JSON arrays
		if topicsJSON != "" {
			if err := json.Unmarshal([]byte(topicsJSON), &repo.Topics); err != nil {
				// Log error or handle, but for now ignore since data should be valid JSON
			}
		}

		if technologiesJSON != "" {
			if err := json.Unmarshal([]byte(technologiesJSON), &repo.Technologies); err != nil {
				// Log error or handle, but for now ignore since data should be valid JSON
			}
		}

		if useCasesJSON != "" {
			if err := json.Unmarshal([]byte(useCasesJSON), &repo.UseCases); err != nil {
				// Log error or handle, but for now ignore since data should be valid JSON
			}
		}

		if featuresJSON != "" {
			if err := json.Unmarshal([]byte(featuresJSON), &repo.Features); err != nil {
				// Log error or handle, but for now ignore since data should be valid JSON
			}
		}

		repos = append(repos, repo)
	}

	return repos, rows.Err()
}

// SearchRepositories performs a full-text search across repositories
func (r *DuckDBRepository) SearchRepositories(ctx context.Context, query string) ([]SearchResult, error) {
	// Simple text search implementation - can be enhanced with more sophisticated search
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
			if err := json.Unmarshal([]byte(topicsJSON), &repo.Topics); err != nil {
				// Log error or handle, but for now ignore since data should be valid JSON
			}
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

		// Create matches (simplified - could be enhanced to show actual matching text)
		matches := []Match{
			{
				Field:   "repository",
				Content: repo.FullName,
				Score:   score,
			},
		}

		results = append(results, SearchResult{
			Repository: repo,
			Score:      score,
			Matches:    matches,
		})
	}

	return results, rows.Err()
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
	if err != nil && err != sql.ErrNoRows {
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

// Close closes the database connection
func (r *DuckDBRepository) Close() error {
	if r.db != nil {
		return r.db.Close()
	}

	return nil
}
