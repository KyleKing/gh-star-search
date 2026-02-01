package testutil

import (
	"encoding/base64"
	"time"

	"github.com/KyleKing/gh-star-search/internal/github"
	"github.com/KyleKing/gh-star-search/internal/processor"
)

// RepositoryOption is a functional option for configuring test repositories
type RepositoryOption func(*github.Repository)

// WithFullName sets the repository full name
func WithFullName(name string) RepositoryOption {
	return func(r *github.Repository) {
		r.FullName = name
	}
}

// WithStars sets the stargazers count
func WithStars(count int) RepositoryOption {
	return func(r *github.Repository) {
		r.StargazersCount = count
	}
}

// WithLanguage sets the primary language
func WithLanguage(lang string) RepositoryOption {
	return func(r *github.Repository) {
		r.Language = lang
	}
}

// WithDescription sets the repository description
func WithDescription(desc string) RepositoryOption {
	return func(r *github.Repository) {
		r.Description = desc
	}
}

// WithTopics sets the repository topics
func WithTopics(topics ...string) RepositoryOption {
	return func(r *github.Repository) {
		r.Topics = topics
	}
}

// WithForks sets the forks count
func WithForks(count int) RepositoryOption {
	return func(r *github.Repository) {
		r.ForksCount = count
	}
}

// WithSize sets the repository size in KB
func WithSize(sizeKB int) RepositoryOption {
	return func(r *github.Repository) {
		r.Size = sizeKB
	}
}

// WithLicense sets the repository license
func WithLicense(key, name, spdxID string) RepositoryOption {
	return func(r *github.Repository) {
		r.License = &github.License{
			Key:    key,
			Name:   name,
			SPDXID: spdxID,
		}
	}
}

// WithHomepage sets the repository homepage
func WithHomepage(url string) RepositoryOption {
	return func(r *github.Repository) {
		r.Homepage = url
	}
}

// WithCreatedAt sets the creation timestamp
func WithCreatedAt(t time.Time) RepositoryOption {
	return func(r *github.Repository) {
		r.CreatedAt = t
	}
}

// WithUpdatedAt sets the update timestamp
func WithUpdatedAt(t time.Time) RepositoryOption {
	return func(r *github.Repository) {
		r.UpdatedAt = t
	}
}

// NewTestRepository creates a test repository with sensible defaults
// and applies any provided options.
func NewTestRepository(opts ...RepositoryOption) github.Repository {
	now := time.Now()
	repo := github.Repository{
		FullName:        TestFullName,
		Description:     TestDescription,
		Language:        TestLanguage,
		StargazersCount: TestStarCount,
		ForksCount:      10,
		Size:            1024,
		CreatedAt:       now.Add(-365 * 24 * time.Hour),
		UpdatedAt:       now.Add(-1 * time.Hour),
		Topics:          []string{"test", "example"},
		License: &github.License{
			Key:    "mit",
			Name:   "MIT License",
			SPDXID: "MIT",
		},
	}

	for _, opt := range opts {
		opt(&repo)
	}

	return repo
}

// NewTestContent creates a test GitHub content object with the given path and content.
// The content is automatically base64 encoded.
func NewTestContent(path, content string) github.Content {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	return github.Content{
		Path:     path,
		Type:     "file",
		Content:  encoded,
		Size:     len(content),
		Encoding: "base64",
		SHA:      "abc123", // Placeholder SHA
	}
}

// ContentChunkOption is a functional option for configuring content chunks
type ContentChunkOption func(*processor.ContentChunk)

// WithChunkType sets the chunk type (readme, code, etc.)
func WithChunkType(chunkType string) ContentChunkOption {
	return func(c *processor.ContentChunk) {
		c.Type = chunkType
	}
}

// WithTokens sets the token count
func WithTokens(tokens int) ContentChunkOption {
	return func(c *processor.ContentChunk) {
		c.Tokens = tokens
	}
}

// WithPriority sets the chunk priority
func WithPriority(priority int) ContentChunkOption {
	return func(c *processor.ContentChunk) {
		c.Priority = priority
	}
}

// NewTestChunk creates a test content chunk
func NewTestChunk(source, content string, opts ...ContentChunkOption) processor.ContentChunk {
	chunk := processor.ContentChunk{
		Source:   source,
		Type:     "readme",
		Content:  content,
		Tokens:   len(content) / 4, // Rough estimate
		Priority: 1,
	}

	for _, opt := range opts {
		opt(&chunk)
	}

	return chunk
}

// NewTestProcessedRepo creates a test ProcessedRepo with the given repository and chunks
func NewTestProcessedRepo(repo github.Repository, chunks []processor.ContentChunk) processor.ProcessedRepo {
	return processor.ProcessedRepo{
		Repository:  repo,
		Chunks:      chunks,
		ProcessedAt: time.Now(),
		ContentHash: "test-hash-" + repo.FullName,
	}
}

// NewTestProcessedRepoSimple creates a simple ProcessedRepo with minimal setup
func NewTestProcessedRepoSimple(fullName string) processor.ProcessedRepo {
	repo := NewTestRepository(WithFullName(fullName))
	chunk := NewTestChunk("README.md", "# "+fullName+"\n\nTest repository")
	return NewTestProcessedRepo(repo, []processor.ContentChunk{chunk})
}
