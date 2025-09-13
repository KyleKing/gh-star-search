package storage

import (
	"context"
	"time"

	"github.com/kyleking/gh-star-search/internal/processor"
)

// Repository defines the interface for database operations
type Repository interface {
	Initialize(ctx context.Context) error
	StoreRepository(ctx context.Context, repo processor.ProcessedRepo) error
	UpdateRepository(ctx context.Context, repo processor.ProcessedRepo) error
	DeleteRepository(ctx context.Context, fullName string) error
	SearchRepositories(ctx context.Context, query string) ([]SearchResult, error)
	GetRepository(ctx context.Context, fullName string) (*StoredRepo, error)
	ListRepositories(ctx context.Context, limit, offset int) ([]StoredRepo, error)
	GetStats(ctx context.Context) (*Stats, error)
	Clear(ctx context.Context) error
	Close() error
}

// StoredRepo represents a repository as stored in the database
type StoredRepo struct {
	ID                     string                `json:"id"`
	FullName               string                `json:"full_name"`
	Description            string                `json:"description"`
	Language               string                `json:"language"`
	StargazersCount        int                   `json:"stargazers_count"`
	ForksCount             int                   `json:"forks_count"`
	SizeKB                 int                   `json:"size_kb"`
	CreatedAt              time.Time             `json:"created_at"`
	UpdatedAt              time.Time             `json:"updated_at"`
	LastSynced             time.Time             `json:"last_synced"`
	Topics                 []string              `json:"topics"`
	LicenseName            string                `json:"license_name"`
	LicenseSPDXID          string                `json:"license_spdx_id"`
	Purpose                string                `json:"purpose"`
	Technologies           []string              `json:"technologies"`
	UseCases               []string              `json:"use_cases"`
	Features               []string              `json:"features"`
	InstallationInstructions string              `json:"installation_instructions"`
	UsageInstructions      string                `json:"usage_instructions"`
	ContentHash            string                `json:"content_hash"`
	Chunks                 []processor.ContentChunk `json:"chunks,omitempty"`
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
	TotalRepositories int       `json:"total_repositories"`
	LastSyncTime      time.Time `json:"last_sync_time"`
	DatabaseSizeMB    float64   `json:"database_size_mb"`
	TotalContentChunks int      `json:"total_content_chunks"`
	LanguageBreakdown map[string]int `json:"language_breakdown"`
	TopicBreakdown    map[string]int `json:"topic_breakdown"`
}