package processor

import (
	"context"
	"time"

	"github.com/username/gh-star-search/internal/github"
)

// Service defines the interface for content processing operations
type Service interface {
	ProcessRepository(ctx context.Context, repo github.Repository, content []github.Content) (*ProcessedRepo, error)
	ExtractContent(ctx context.Context, repo github.Repository) ([]github.Content, error)
	GenerateSummary(ctx context.Context, chunks []ContentChunk) (*Summary, error)
}

// ContentChunk represents a processed piece of repository content
type ContentChunk struct {
	Source   string `json:"source"`   // file path or section
	Type     string `json:"type"`     // readme, code, docs, etc.
	Content  string `json:"content"`
	Tokens   int    `json:"tokens"`
	Priority int    `json:"priority"` // for size limit handling
}

// Summary represents the LLM-generated summary of repository content
type Summary struct {
	Purpose      string   `json:"purpose"`
	Technologies []string `json:"technologies"`
	UseCases     []string `json:"use_cases"`
	Features     []string `json:"features"`
	Installation string   `json:"installation"`
	Usage        string   `json:"usage"`
}

// ProcessedRepo represents a fully processed repository with summary and chunks
type ProcessedRepo struct {
	Repository  github.Repository `json:"repository"`
	Summary     Summary           `json:"summary"`
	Chunks      []ContentChunk    `json:"chunks"`
	ProcessedAt time.Time         `json:"processed_at"`
	ContentHash string            `json:"content_hash"` // For change detection
}

// ContentType constants for different types of repository content
const (
	ContentTypeReadme     = "readme"
	ContentTypeCode       = "code"
	ContentTypeDocs       = "docs"
	ContentTypeConfig     = "config"
	ContentTypeChangelog  = "changelog"
	ContentTypeLicense    = "license"
	ContentTypePackage    = "package"
)

// Priority constants for content processing
const (
	PriorityHigh   = 1
	PriorityMedium = 2
	PriorityLow    = 3
)