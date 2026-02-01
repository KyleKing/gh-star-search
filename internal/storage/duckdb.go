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
	_ "github.com/marcboeker/go-duckdb" // DuckDB driver

	"github.com/KyleKing/gh-star-search/internal/processor"
)

const (
	// DefaultQueryTimeout is the default timeout for database queries
	DefaultQueryTimeout = 30 * time.Second
)

// DuckDBRepository implements the Repository interface using DuckDB
type DuckDBRepository struct {
	db           *sql.DB
	path         string
	queryTimeout time.Duration
}

// NewDuckDBRepository creates a new DuckDB repository instance with connection pooling
func NewDuckDBRepository(dbPath string) (*DuckDBRepository, error) {
	return NewDuckDBRepositoryWithTimeout(dbPath, DefaultQueryTimeout)
}

// NewDuckDBRepositoryWithTimeout creates a new DuckDB repository instance with custom timeout
func NewDuckDBRepositoryWithTimeout(
	dbPath string,
	queryTimeout time.Duration,
) (*DuckDBRepository, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &DuckDBRepository{
		db:           db,
		path:         dbPath,
		queryTimeout: queryTimeout,
	}

	return repo, nil
}

// withQueryTimeout creates a new context with the configured query timeout
// If the parent context already has a deadline, it keeps the earlier deadline
func (r *DuckDBRepository) withQueryTimeout(
	parent context.Context,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, r.queryTimeout)
}

// Initialize creates the database schema
func (r *DuckDBRepository) Initialize(ctx context.Context) error {
	schemaManager := NewSchemaManager(r.db)
	return schemaManager.CreateLatestSchema(ctx)
}

// StoreRepository stores a new repository in the database
func (r *DuckDBRepository) StoreRepository(
	ctx context.Context,
	repo processor.ProcessedRepo,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Convert arrays and objects to JSON
	languagesJSON, err := json.Marshal(map[string]int64{})
	if err != nil {
		return fmt.Errorf("failed to marshal languages: %w", err)
	}

	contributorsJSON, err := json.Marshal([]Contributor{})
	if err != nil {
		return fmt.Errorf("failed to marshal contributors: %w", err)
	}

	// Generate a new UUID for the repository
	repoID := uuid.New().String()

	// Convert topics to JSON for storage (DuckDB doesn't handle []string directly)
	topicsJSON, err := json.Marshal(repo.Repository.Topics)
	if err != nil {
		return fmt.Errorf("failed to marshal topics: %w", err)
	}

	// Build FTS text columns
	topicsText := strings.Join(repo.Repository.Topics, " ")

	// Insert repository with new schema
	insertRepoSQL := `
	INSERT INTO repositories (
		id, full_name, description, homepage, language, stargazers_count, forks_count, size_kb,
		created_at, updated_at, last_synced,
		open_issues_open, open_issues_total, open_prs_open, open_prs_total,
		commits_30d, commits_1y, commits_total,
		topics_array, languages, contributors,
		license_name, license_spdx_id,
		content_hash,
		topics_text, contributors_text
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var licenseName, licenseSPDXID string
	if repo.Repository.License != nil {
		licenseName = repo.Repository.License.Name
		licenseSPDXID = repo.Repository.License.SPDXID
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
		repo.ContentHash,
		topicsText,
		"", // contributors_text empty on initial store, populated by UpdateRepositoryMetrics
	)
	if err != nil {
		return fmt.Errorf("failed to insert repository: %w", err)
	}

	return tx.Commit()
}

// UpdateRepository updates an existing repository in the database.
//
// WORKAROUND FOR DUCKDB CONSTRAINT LIMITATION:
// DuckDB v1.8.x has a critical bug where UPDATE and DELETE+INSERT operations fail with
// "duplicate key" errors when executed within transactions on tables with PRIMARY KEY
// constraints. This affects JSON/LIST columns particularly.
//
// Root cause: DuckDB's over-eager constraint checking evaluates constraints before
// DELETE operations complete within the transaction scope.
//
// Solution: Perform DELETE+INSERT WITHOUT a transaction. While not atomic, the operations
// execute sequentially and fast enough that race conditions are minimal for single-row
// updates. This preserves metrics set by UpdateRepositoryMetrics.
//
// See: https://github.com/duckdb/duckdb/issues/11915 (UPDATE with LIST columns fails)
// See: https://github.com/duckdb/duckdb/issues/16520 (DELETE+INSERT in transaction fails)
// See: https://github.com/duckdb/duckdb/issues/8764 (UPDATE without changing PK fails)
// See: https://duckdb.org/docs/stable/sql/indexes#index-limitations
func (r *DuckDBRepository) UpdateRepository(
	ctx context.Context,
	repo processor.ProcessedRepo,
) error {
	// Step 1: Get existing repository data to preserve metrics
	var existingData struct {
		id              string
		openIssuesOpen  int
		openIssuesTotal int
		openPRsOpen     int
		openPRsTotal    int
		commits30d      int
		commits1y       int
		commitsTotal    int
		languages       interface{}
		contributors    interface{}
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT id,
			COALESCE(open_issues_open, 0),
			COALESCE(open_issues_total, 0),
			COALESCE(open_prs_open, 0),
			COALESCE(open_prs_total, 0),
			COALESCE(commits_30d, 0),
			COALESCE(commits_1y, 0),
			COALESCE(commits_total, 0),
			COALESCE(languages, '{}'),
			COALESCE(contributors, '[]')
		FROM repositories
		WHERE full_name = ?`,
		repo.Repository.FullName).
		Scan(&existingData.id,
			&existingData.openIssuesOpen,
			&existingData.openIssuesTotal,
			&existingData.openPRsOpen,
			&existingData.openPRsTotal,
			&existingData.commits30d,
			&existingData.commits1y,
			&existingData.commitsTotal,
			&existingData.languages,
			&existingData.contributors,
		)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Step 2: Delete existing repository
	_, err = r.db.ExecContext(ctx, "DELETE FROM repositories WHERE id = ?", existingData.id)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	// Step 3: Convert JSON data for reinsertion
	topicsJSON, err := json.Marshal(repo.Repository.Topics)
	if err != nil {
		return fmt.Errorf("failed to marshal topics: %w", err)
	}

	languagesJSON, err := json.Marshal(existingData.languages)
	if err != nil {
		return fmt.Errorf("failed to marshal languages: %w", err)
	}

	contributorsJSON, err := json.Marshal(existingData.contributors)
	if err != nil {
		return fmt.Errorf("failed to marshal contributors: %w", err)
	}

	// Step 4: INSERT repository with updated metadata but preserved metrics
	var licenseName, licenseSPDXID string
	if repo.Repository.License != nil {
		licenseName = repo.Repository.License.Name
		licenseSPDXID = repo.Repository.License.SPDXID
	}

	topicsText := strings.Join(repo.Repository.Topics, " ")

	insertSQL := `
		INSERT INTO repositories (
			id, full_name, description, homepage, language, stargazers_count, forks_count, size_kb,
			created_at, updated_at, last_synced,
			open_issues_open, open_issues_total, open_prs_open, open_prs_total,
			commits_30d, commits_1y, commits_total,
			topics_array, languages, contributors,
			license_name, license_spdx_id, content_hash,
			topics_text
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.db.ExecContext(ctx, insertSQL,
		existingData.id,
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
		existingData.openIssuesOpen,
		existingData.openIssuesTotal,
		existingData.openPRsOpen,
		existingData.openPRsTotal,
		existingData.commits30d,
		existingData.commits1y,
		existingData.commitsTotal,
		string(topicsJSON),
		string(languagesJSON),
		string(contributorsJSON),
		licenseName,
		licenseSPDXID,
		repo.ContentHash,
		topicsText,
	)
	if err != nil {
		return fmt.Errorf("failed to insert updated repository: %w", err)
	}

	return nil
}

// DeleteRepository removes a repository from the database
func (r *DuckDBRepository) DeleteRepository(ctx context.Context, fullName string) error {
	// Delete repository by full_name
	result, err := r.db.ExecContext(ctx, "DELETE FROM repositories WHERE full_name = ?", fullName)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil // Repository doesn't exist, nothing to delete
	}

	return nil
}

// GetRepository retrieves a specific repository by full name
func (r *DuckDBRepository) GetRepository(
	ctx context.Context,
	fullName string,
) (*StoredRepo, error) {
	query := `
		SELECT
		   id, full_name, description, homepage, language, stargazers_count, forks_count, size_kb,
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
		   content_hash,
		   purpose, summary_generated_at, COALESCE(summary_version, 0) as summary_version
	FROM repositories WHERE full_name = ?`

	row := r.db.QueryRowContext(ctx, query, fullName)

	var repo StoredRepo

	var languagesData, contributorsData interface{}

	var topicsData interface{}

	var purpose sql.NullString

	err := row.Scan(
		&repo.ID, &repo.FullName, &repo.Description, &repo.Homepage,
		&repo.Language, &repo.StargazersCount, &repo.ForksCount, &repo.SizeKB,
		&repo.CreatedAt, &repo.UpdatedAt, &repo.LastSynced,
		&repo.OpenIssuesOpen, &repo.OpenIssuesTotal, &repo.OpenPRsOpen, &repo.OpenPRsTotal,
		&repo.Commits30d, &repo.Commits1y, &repo.CommitsTotal,
		&topicsData, &languagesData, &contributorsData,
		&repo.LicenseName, &repo.LicenseSPDXID,
		&repo.ContentHash,
		&purpose, &repo.SummaryGeneratedAt, &repo.SummaryVersion,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("repository not found: %s", fullName)
		}

		return nil, fmt.Errorf("failed to scan repository: %w", err)
	}

	// Set purpose if it's valid
	if purpose.Valid {
		repo.Purpose = purpose.String
	}

	// Parse topics array
	if topicsData != nil {
		if topicsBytes, err := json.Marshal(topicsData); err == nil {
			_ = json.Unmarshal(topicsBytes, &repo.Topics)
		}
	}

	// Parse JSON fields
	if languagesData != nil {
		if languagesBytes, err := json.Marshal(languagesData); err == nil {
			_ = json.Unmarshal(languagesBytes, &repo.Languages)
		}
	}

	if contributorsData != nil {
		if contributorsBytes, err := json.Marshal(contributorsData); err == nil {
			_ = json.Unmarshal(contributorsBytes, &repo.Contributors)
		}
	}

	return &repo, nil
}

// ListRepositories retrieves a paginated list of repositories with a timeout
func (r *DuckDBRepository) ListRepositories(
	ctx context.Context,
	limit, offset int,
) ([]StoredRepo, error) {
	// Apply query timeout to prevent long-running queries
	queryCtx, cancel := r.withQueryTimeout(ctx)
	defer cancel()

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
		   content_hash,
		   purpose, summary_generated_at, COALESCE(summary_version, 0) as summary_version
	FROM repositories
	ORDER BY stargazers_count DESC, full_name
	LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(queryCtx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query repositories: %w", err)
	}
	defer rows.Close()

	var repos []StoredRepo

	for rows.Next() {
		var repo StoredRepo

		var languagesData, contributorsData interface{}

		var topicsData interface{}

		var purpose sql.NullString

		err := rows.Scan(
			&repo.ID, &repo.FullName, &repo.Description, &repo.Homepage,
			&repo.Language, &repo.StargazersCount, &repo.ForksCount, &repo.SizeKB,
			&repo.CreatedAt, &repo.UpdatedAt, &repo.LastSynced,
			&repo.OpenIssuesOpen, &repo.OpenIssuesTotal, &repo.OpenPRsOpen, &repo.OpenPRsTotal,
			&repo.Commits30d, &repo.Commits1y, &repo.CommitsTotal,
			&topicsData, &languagesData, &contributorsData,
			&repo.LicenseName, &repo.LicenseSPDXID,
			&repo.ContentHash,
			&purpose, &repo.SummaryGeneratedAt, &repo.SummaryVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}

		// Set purpose if it's valid
		if purpose.Valid {
			repo.Purpose = purpose.String
		}

		// Parse topics array
		if topicsData != nil {
			if topicsBytes, err := json.Marshal(topicsData); err == nil {
				_ = json.Unmarshal(topicsBytes, &repo.Topics)
			}
		}

		// Parse JSON fields

		if languagesData != nil {
			if languagesBytes, err := json.Marshal(languagesData); err == nil {
				_ = json.Unmarshal(languagesBytes, &repo.Languages)
			}
		}

		if contributorsData != nil {
			if contributorsBytes, err := json.Marshal(contributorsData); err == nil {
				_ = json.Unmarshal(contributorsBytes, &repo.Contributors)
			}
		}

		// Parse topics array
		if topicsData != nil {
			if topicsBytes, err := json.Marshal(topicsData); err == nil {
				_ = json.Unmarshal(topicsBytes, &repo.Topics)
			}
		}

		repos = append(repos, repo)
	}

	return repos, rows.Err()
}

// SearchRepositories performs text search across repositories.
// This method does NOT support SQL queries for security reasons.
// All searches are performed using parameterized queries to prevent SQL injection.
func (r *DuckDBRepository) SearchRepositories(
	ctx context.Context,
	query string,
) ([]SearchResult, error) {
	// Validate that the query is not a SQL statement
	// This is a defense-in-depth measure to prevent SQL injection
	trimmedQuery := strings.TrimSpace(strings.ToUpper(query))
	sqlKeywords := []string{
		"SELECT",
		"INSERT",
		"UPDATE",
		"DELETE",
		"DROP",
		"CREATE",
		"ALTER",
		"TRUNCATE",
	}

	for _, keyword := range sqlKeywords {
		if strings.HasPrefix(trimmedQuery, keyword) {
			return nil, fmt.Errorf(
				"SQL queries are not supported for security reasons. Please use simple search terms instead",
			)
		}
	}

	// Perform text search using parameterized queries
	return r.executeTextSearch(ctx, query)
}

// executeTextSearch performs FTS-based text search with BM25 scoring
func (r *DuckDBRepository) executeTextSearch(
	ctx context.Context,
	query string,
) ([]SearchResult, error) {
	queryCtx, cancel := r.withQueryTimeout(ctx)
	defer cancel()

	searchQuery := `
	SELECT r.id, r.full_name, r.description, r.language, r.stargazers_count, r.forks_count, r.size_kb,
		   r.created_at, r.updated_at, r.last_synced, r.topics_array, r.license_name, r.license_spdx_id,
		   r.content_hash, r.purpose,
		   fts_main_repositories.match_bm25(r.id, ?,
			   fields := 'full_name,description,purpose,topics_text,contributors_text') AS score
	FROM repositories r
	WHERE score IS NOT NULL
	ORDER BY score DESC
	LIMIT 50`

	rows, err := r.db.QueryContext(queryCtx, searchQuery, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search repositories: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var repo StoredRepo
		var score float64
		var topicsData interface{}
		var purpose sql.NullString

		err := rows.Scan(
			&repo.ID, &repo.FullName, &repo.Description, &repo.Language,
			&repo.StargazersCount, &repo.ForksCount, &repo.SizeKB,
			&repo.CreatedAt, &repo.UpdatedAt, &repo.LastSynced,
			&topicsData, &repo.LicenseName, &repo.LicenseSPDXID,
			&repo.ContentHash, &purpose,
			&score,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		if purpose.Valid {
			repo.Purpose = purpose.String
		}

		if topicsData != nil {
			if topicsBytes, err := json.Marshal(topicsData); err == nil {
				_ = json.Unmarshal(topicsBytes, &repo.Topics)
			}
		}

		matches := r.findMatches(repo, query)

		results = append(results, SearchResult{
			Repository: repo,
			Score:      score,
			Matches:    matches,
		})
	}

	return results, rows.Err()
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

	if strings.Contains(strings.ToLower(repo.Language), queryLower) {
		matches = append(matches, Match{
			Field:   "language",
			Content: repo.Language,
			Score:   0.7,
		})
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
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM repositories").
		Scan(&stats.TotalRepositories)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository count: %w", err)
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
	langRows, err := r.db.QueryContext(
		ctx,
		"SELECT language, COUNT(*) FROM repositories WHERE language IS NOT NULL GROUP BY language ORDER BY COUNT(*) DESC",
	)
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

	if err := langRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate language rows: %w", err)
	}

	return stats, nil
}

// Clear removes all data from the database
func (r *DuckDBRepository) Clear(ctx context.Context) error {
	// Delete all repositories
	_, err := r.db.ExecContext(ctx, "DELETE FROM repositories")
	if err != nil {
		return fmt.Errorf("failed to clear repositories: %w", err)
	}

	return nil
}

// UpdateRepositoryMetrics updates activity and metrics data for a repository.
//
// WORKAROUND: Like UpdateRepository, this avoids using UPDATE due to DuckDB's constraint
// checking issues with JSON columns. Instead, it uses DELETE+INSERT without a transaction.
// This is not atomic, but it's the only reliable way to update with DuckDB's current limitations.
//
// Note: Concurrent updates may conflict (optimistic concurrency - last write wins).
//
// See: https://github.com/duckdb/duckdb/issues/11915
// See: https://github.com/duckdb/duckdb/issues/8764
func (r *DuckDBRepository) UpdateRepositoryMetrics(
	ctx context.Context,
	fullName string,
	metrics RepositoryMetrics,
) error {
	// Step 1: Get existing repository data to preserve non-metrics fields
	var existingData struct {
		id                string
		fullName          string
		description       string
		homepage          string
		language          string
		stargazersCount   int
		forksCount        int
		sizeKB            int
		createdAt         time.Time
		updatedAt         time.Time
		lastSynced        time.Time
		topicsArray       interface{}
		licenseName       string
		licenseSPDXID     string
		contentHash       string
		purpose           sql.NullString
		summaryGeneratedAt *time.Time
		summaryVersion    int
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT id, full_name, description, COALESCE(homepage, ''), language,
			stargazers_count, forks_count, size_kb,
			created_at, updated_at, last_synced,
			COALESCE(topics_array, '[]'),
			COALESCE(license_name, ''), COALESCE(license_spdx_id, ''),
			COALESCE(content_hash, ''),
			purpose, summary_generated_at, COALESCE(summary_version, 0)
		FROM repositories
		WHERE full_name = ?`,
		fullName).
		Scan(&existingData.id, &existingData.fullName, &existingData.description,
			&existingData.homepage, &existingData.language,
			&existingData.stargazersCount, &existingData.forksCount, &existingData.sizeKB,
			&existingData.createdAt, &existingData.updatedAt, &existingData.lastSynced,
			&existingData.topicsArray,
			&existingData.licenseName, &existingData.licenseSPDXID,
			&existingData.contentHash,
			&existingData.purpose, &existingData.summaryGeneratedAt, &existingData.summaryVersion,
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no repository found with full_name: %s", fullName)
		}
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Step 2: Delete existing repository
	_, err = r.db.ExecContext(ctx, "DELETE FROM repositories WHERE id = ?", existingData.id)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	// Step 3: Convert JSON data
	languagesJSON, err := json.Marshal(metrics.Languages)
	if err != nil {
		return fmt.Errorf("failed to marshal languages: %w", err)
	}

	contributorsJSON, err := json.Marshal(metrics.Contributors)
	if err != nil {
		return fmt.Errorf("failed to marshal contributors: %w", err)
	}

	topicsJSON, err := json.Marshal(existingData.topicsArray)
	if err != nil {
		return fmt.Errorf("failed to marshal topics: %w", err)
	}

	// Build contributors_text from metrics
	var contributorLogins []string
	for _, c := range metrics.Contributors {
		contributorLogins = append(contributorLogins, c.Login)
	}
	contributorsText := strings.Join(contributorLogins, " ")

	// Step 4: INSERT with updated metrics but preserved other fields
	insertSQL := `
		INSERT INTO repositories (
			id, full_name, description, homepage, language, stargazers_count, forks_count, size_kb,
			created_at, updated_at, last_synced,
			open_issues_open, open_issues_total, open_prs_open, open_prs_total,
			commits_30d, commits_1y, commits_total,
			topics_array, languages, contributors,
			license_name, license_spdx_id, content_hash,
			purpose, summary_generated_at, summary_version,
			contributors_text
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var purposeVal interface{}
	if existingData.purpose.Valid {
		purposeVal = existingData.purpose.String
	}

	_, err = r.db.ExecContext(ctx, insertSQL,
		existingData.id, existingData.fullName, existingData.description,
		metrics.Homepage, existingData.language,
		existingData.stargazersCount, existingData.forksCount, existingData.sizeKB,
		existingData.createdAt, existingData.updatedAt,
		metrics.OpenIssuesOpen, metrics.OpenIssuesTotal,
		metrics.OpenPRsOpen, metrics.OpenPRsTotal,
		metrics.Commits30d, metrics.Commits1y, metrics.CommitsTotal,
		string(topicsJSON), string(languagesJSON), string(contributorsJSON),
		existingData.licenseName, existingData.licenseSPDXID, existingData.contentHash,
		purposeVal, existingData.summaryGeneratedAt, existingData.summaryVersion,
		contributorsText,
	)
	if err != nil {
		return fmt.Errorf("failed to insert updated repository: %w", err)
	}

	return nil
}

// UpdateRepositoryEmbedding updates the embedding for a repository
func (r *DuckDBRepository) UpdateRepositoryEmbedding(
	ctx context.Context,
	fullName string,
	embedding []float32,
) error {
	// Convert []float32 to the format DuckDB expects for FLOAT arrays
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	updateSQL := `UPDATE repositories SET repo_embedding = ? WHERE full_name = ?`

	_, err = r.db.ExecContext(ctx, updateSQL, string(embeddingJSON), fullName)
	if err != nil {
		return fmt.Errorf("failed to update repository embedding: %w", err)
	}

	return nil
}

// UpdateRepositorySummary updates the AI-generated summary for a repository
func (r *DuckDBRepository) UpdateRepositorySummary(
	ctx context.Context,
	fullName string,
	purpose string,
) error {
	updateSQL := `
	UPDATE repositories SET
		purpose = ?,
		summary_generated_at = CURRENT_TIMESTAMP,
		summary_version = 1
	WHERE full_name = ?`

	result, err := r.db.ExecContext(ctx, updateSQL, purpose, fullName)
	if err != nil {
		return fmt.Errorf("failed to update repository summary: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no repository found with full_name: %s", fullName)
	}

	return nil
}

// GetRepositoriesNeedingMetricsUpdate returns repositories that need metrics updates
func (r *DuckDBRepository) GetRepositoriesNeedingMetricsUpdate(
	ctx context.Context,
	staleDays int,
) ([]string, error) {
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
func (r *DuckDBRepository) GetRepositoriesNeedingSummaryUpdate(
	ctx context.Context,
	forceUpdate bool,
) ([]string, error) {
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

// RebuildFTSIndex installs the FTS extension and creates a full-text search index
func (r *DuckDBRepository) RebuildFTSIndex(ctx context.Context) error {
	statements := []string{
		"INSTALL fts",
		"LOAD fts",
		"PRAGMA create_fts_index('repositories', 'id', 'full_name', 'description', 'purpose', 'topics_text', 'contributors_text', stemmer = 'porter', stopwords = 'english', overwrite = 1)",
	}
	for _, stmt := range statements {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute FTS statement %q: %w", stmt, err)
		}
	}
	return nil
}

// SearchByEmbedding performs vector similarity search using pre-computed embeddings
func (r *DuckDBRepository) SearchByEmbedding(
	ctx context.Context,
	queryEmbedding []float32,
	limit int,
	minScore float64,
) ([]SearchResult, error) {
	queryCtx, cancel := r.withQueryTimeout(ctx)
	defer cancel()

	embeddingJSON, err := json.Marshal(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query embedding: %w", err)
	}

	searchQuery := `
	SELECT r.id, r.full_name, r.description, r.language, r.stargazers_count, r.forks_count, r.size_kb,
		   r.created_at, r.updated_at, r.last_synced, r.topics_array, r.license_name, r.license_spdx_id,
		   r.content_hash, r.purpose,
		   array_cosine_similarity(
			   CAST(repo_embedding AS FLOAT[384]),
			   ?::FLOAT[384]
		   ) AS score
	FROM repositories r
	WHERE repo_embedding IS NOT NULL
		AND array_cosine_similarity(
			CAST(repo_embedding AS FLOAT[384]),
			?::FLOAT[384]
		) >= ?
	ORDER BY score DESC
	LIMIT ?`

	rows, err := r.db.QueryContext(queryCtx, searchQuery,
		string(embeddingJSON), string(embeddingJSON), minScore, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search by embedding: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var repo StoredRepo
		var score float64
		var topicsData interface{}
		var purpose sql.NullString

		err := rows.Scan(
			&repo.ID, &repo.FullName, &repo.Description, &repo.Language,
			&repo.StargazersCount, &repo.ForksCount, &repo.SizeKB,
			&repo.CreatedAt, &repo.UpdatedAt, &repo.LastSynced,
			&topicsData, &repo.LicenseName, &repo.LicenseSPDXID,
			&repo.ContentHash, &purpose,
			&score,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan embedding search result: %w", err)
		}

		if purpose.Valid {
			repo.Purpose = purpose.String
		}

		if topicsData != nil {
			if topicsBytes, err := json.Marshal(topicsData); err == nil {
				_ = json.Unmarshal(topicsBytes, &repo.Topics)
			}
		}

		results = append(results, SearchResult{
			Repository: repo,
			Score:      score,
		})
	}

	return results, rows.Err()
}

// Close closes the database connection
func (r *DuckDBRepository) Close() error {
	if r.db != nil {
		return r.db.Close()
	}

	return nil
}
