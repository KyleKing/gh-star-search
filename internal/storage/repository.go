package storage

import (
	"context"
	"time"

	"github.com/kyleking/gh-star-search/internal/processor"
)

// Repository defines the interface for database operations
type Repository interface {
	Initialize(ctx context.Context) error
	InitializeWithPrompt(ctx context.Context, autoConfirm bool) error
	StoreRepository(ctx context.Context, repo processor.ProcessedRepo) error
	UpdateRepository(ctx context.Context, repo processor.ProcessedRepo) error
	DeleteRepository(ctx context.Context, fullName string) error
	SearchRepositories(ctx context.Context, query string) ([]SearchResult, error)
	GetRepository(ctx context.Context, fullName string) (*StoredRepo, error)
	ListRepositories(ctx context.Context, limit, offset int) ([]StoredRepo, error)
	GetStats(ctx context.Context) (*Stats, error)
	Clear(ctx context.Context) error
	Close() error
	
	// New methods for enhanced functionality
	UpdateRepositoryMetrics(ctx context.Context, fullName string, metrics RepositoryMetrics) error
	UpdateRepositoryEmbedding(ctx context.Context, fullName string, embedding []float32) error
	GetRepositoriesNeedingMetricsUpdate(ctx context.Context, staleDays int) ([]string, error)
	GetRepositoriesNeedingSummaryUpdate(ctx context.Context, forceUpdate bool) ([]string, error)
}

// StoredRepo represents a repository as stored in the database
type StoredRepo struct {
	ID                       string                   `json:"id"`
	FullName                 string                   `json:"full_name"`
	Description              string                   `json:"description"`
	Homepage                 string                   `json:"homepage"`
	Language                 string                   `json:"language"`
	StargazersCount          int                      `json:"stargazers_count"`
	ForksCount               int                      `json:"forks_count"`
	SizeKB                   int                      `json:"size_kb"`
	CreatedAt                time.Time                `json:"created_at"`
	UpdatedAt                time.Time                `json:"updated_at"`
	LastSynced               time.Time                `json:"last_synced"`
	
	// Activity & Metrics
	OpenIssuesOpen           int                      `json:"open_issues_open"`
	OpenIssuesTotal          int                      `json:"open_issues_total"`
	OpenPRsOpen              int                      `json:"open_prs_open"`
	OpenPRsTotal             int                      `json:"open_prs_total"`
	Commits30d               int                      `json:"commits_30d"`
	Commits1y                int                      `json:"commits_1y"`
	CommitsTotal             int                      `json:"commits_total"`
	
	// Metadata arrays and objects
	Topics                   []string                 `json:"topics"`
	Languages                map[string]int64         `json:"languages"`
	Contributors             []Contributor            `json:"contributors"`
	
	// License
	LicenseName              string                   `json:"license_name"`
	LicenseSPDXID            string                   `json:"license_spdx_id"`
	
	// Summary fields
	Purpose                  string                   `json:"purpose"`
	Technologies             []string                 `json:"technologies"`
	UseCases                 []string                 `json:"use_cases"`
	Features                 []string                 `json:"features"`
	InstallationInstructions string                   `json:"installation_instructions"`
	UsageInstructions        string                   `json:"usage_instructions"`
	SummaryGeneratedAt       *time.Time               `json:"summary_generated_at,omitempty"`
	SummaryVersion           int                      `json:"summary_version"`
	SummaryGenerator         string                   `json:"summary_generator"`
	
	// Embedding
	RepoEmbedding            []float32                `json:"repo_embedding,omitempty"`
	
	// Content tracking
	ContentHash              string                   `json:"content_hash"`
	
	// Legacy chunks (deprecated, will be removed)
	Chunks                   []processor.ContentChunk `json:"chunks,omitempty"`
}

// Contributor represents a repository contributor
type Contributor struct {
	Login         string `json:"login"`
	Contributions int    `json:"contributions"`
}

// RepositoryMetrics represents activity and metrics data for a repository
type RepositoryMetrics struct {
	OpenIssuesOpen   int                      `json:"open_issues_open"`
	OpenIssuesTotal  int                      `json:"open_issues_total"`
	OpenPRsOpen      int                      `json:"open_prs_open"`
	OpenPRsTotal     int                      `json:"open_prs_total"`
	Commits30d       int                      `json:"commits_30d"`
	Commits1y        int                      `json:"commits_1y"`
	CommitsTotal     int                      `json:"commits_total"`
	Languages        map[string]int64         `json:"languages"`
	Contributors     []Contributor            `json:"contributors"`
	Homepage         string                   `json:"homepage"`
}

// SearchResult represents a search result with relevance scoring
type SearchResult struct {
	Repository StoredRepo `json:"repository"`
	Score      float64    `json:"score"`
	Matches    []Match    `json:"matches"`
}

// Match represents a specific field match in search results
type Match struct {
	Field   string  `json:"field"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// Stats represents database statistics
type Stats struct {
	TotalRepositories  int            `json:"total_repositories"`
	LastSyncTime       time.Time      `json:"last_sync_time"`
	DatabaseSizeMB     float64        `json:"database_size_mb"`
	TotalContentChunks int            `json:"total_content_chunks"`
	LanguageBreakdown  map[string]int `json:"language_breakdown"`
	TopicBreakdown     map[string]int `json:"topic_breakdown"`
}
