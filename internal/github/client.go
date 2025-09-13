package github

import (
	"context"
	"time"
)

// Client defines the interface for GitHub API operations
type Client interface {
	GetStarredRepos(ctx context.Context, username string) ([]Repository, error)
	GetRepositoryContent(ctx context.Context, repo Repository, paths []string) ([]Content, error)
	GetRepositoryMetadata(ctx context.Context, repo Repository) (*Metadata, error)
}

// Repository represents a GitHub repository with metadata
type Repository struct {
	FullName        string    `json:"full_name"`
	Description     string    `json:"description"`
	Language        string    `json:"language"`
	StargazersCount int       `json:"stargazers_count"`
	ForksCount      int       `json:"forks_count"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedAt       time.Time `json:"created_at"`
	Topics          []string  `json:"topics"`
	License         *License  `json:"license"`
	Size            int       `json:"size"`
	DefaultBranch   string    `json:"default_branch"`
	OpenIssues      int       `json:"open_issues_count"`
	HasWiki         bool      `json:"has_wiki"`
	HasPages        bool      `json:"has_pages"`
	Archived        bool      `json:"archived"`
	Disabled        bool      `json:"disabled"`
}

// License represents repository license information
type License struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	SPDXID string `json:"spdx_id"`
	URL    string `json:"url"`
}

// Content represents file content from a repository
type Content struct {
	Path     string `json:"path"`
	Type     string `json:"type"` // file, dir, symlink, submodule
	Content  string `json:"content"`
	Size     int    `json:"size"`
	Encoding string `json:"encoding"` // base64, utf-8
	SHA      string `json:"sha"`
}

// Metadata represents additional repository metadata
type Metadata struct {
	CommitCount    int       `json:"commit_count"`
	Contributors   int       `json:"contributors"`
	LastCommitDate time.Time `json:"last_commit_date"`
	ReleaseCount   int       `json:"release_count"`
	LatestRelease  *Release  `json:"latest_release"`
}

// Release represents a GitHub release
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
}