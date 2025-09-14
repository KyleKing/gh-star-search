package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/cli/go-gh/v2/pkg/api"
)

// Client defines the interface for GitHub API operations.
// It provides methods to fetch starred repositories, repository content,
// and additional metadata using the GitHub REST API.
type Client interface {
	// GetStarredRepos fetches all starred repositories for the authenticated user.
	// It handles pagination automatically and respects rate limits.
	// The username parameter is currently unused but reserved for future use.
	GetStarredRepos(ctx context.Context, username string) ([]Repository, error)

	// GetRepositoryContent fetches specific file contents from a repository.
	// It accepts a list of file paths and returns the content for files that exist.
	// Missing files are silently skipped rather than causing an error.
	GetRepositoryContent(ctx context.Context, repo Repository, paths []string) ([]Content, error)

	// GetRepositoryMetadata fetches additional metadata for a repository including
	// commit count, contributors, and release information.
	// Partial failures are handled gracefully - if some metadata cannot be fetched,
	// the available data is still returned.
	GetRepositoryMetadata(ctx context.Context, repo Repository) (*Metadata, error)

	// GetContributors fetches the top N contributors for a repository.
	// Returns contributor login names and their contribution counts.
	GetContributors(ctx context.Context, fullName string, topN int) ([]Contributor, error)

	// GetTopics fetches the topics/tags associated with a repository.
	GetTopics(ctx context.Context, fullName string) ([]string, error)

	// GetLanguages fetches the programming languages used in a repository.
	// Returns a map of language names to byte counts.
	GetLanguages(ctx context.Context, fullName string) (map[string]int64, error)

	// GetCommitActivity fetches commit activity statistics for a repository.
	// Returns weekly commit counts for the last 52 weeks.
	GetCommitActivity(ctx context.Context, fullName string) (*CommitActivity, error)

	// GetPullCounts fetches pull request counts (open and total) for a repository.
	GetPullCounts(ctx context.Context, fullName string) (open int, total int, err error)

	// GetIssueCounts fetches issue counts (open and total) for a repository.
	GetIssueCounts(ctx context.Context, fullName string) (open int, total int, err error)

	// GetHomepageText fetches text content from an external homepage URL.
	// This is optional and used for additional context extraction.
	GetHomepageText(ctx context.Context, url string) (string, error)
}

// RESTClientInterface defines the interface for REST API operations
type RESTClientInterface interface {
	Get(path string, resp interface{}) error
}

// Repository represents a GitHub repository with essential metadata
type Repository struct {
	FullName        string    `json:"full_name"`
	Description     string    `json:"description"`
	Homepage        string    `json:"homepage"`
	Language        string    `json:"language"`
	StargazersCount int       `json:"stargazers_count"`
	ForksCount      int       `json:"forks_count"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedAt       time.Time `json:"created_at"`
	Topics          []string  `json:"topics"`
	License         *License  `json:"license"`
	Size            int       `json:"size"`
	DefaultBranch   string    `json:"default_branch"`
	OpenIssuesCount int       `json:"open_issues_count"`
	HasWiki         bool      `json:"has_wiki"`
	HasPages        bool      `json:"has_pages"`
	Archived        bool      `json:"archived"`
	Disabled        bool      `json:"disabled"`
	Private         bool      `json:"private"`
	Fork            bool      `json:"fork"`
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
	Type     string `json:"type"`
	Content  string `json:"content"`
	Size     int    `json:"size"`
	Encoding string `json:"encoding"`
	SHA      string `json:"sha"`
}

// Metadata represents additional repository metadata
type Metadata struct {
	CommitCount    int       `json:"commit_count"`
	Contributors   []string  `json:"contributors"`
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

// Contributor represents a repository contributor
type Contributor struct {
	Login         string `json:"login"`
	Contributions int    `json:"contributions"`
	Type          string `json:"type"`
	AvatarURL     string `json:"avatar_url"`
}

// CommitActivity represents weekly commit activity statistics
type CommitActivity struct {
	Weeks []WeeklyCommits `json:"weeks"`
	Total int             `json:"total"`
}

// WeeklyCommits represents commit counts for a specific week
type WeeklyCommits struct {
	Week    int64 `json:"w"` // Unix timestamp for the start of the week
	Commits int   `json:"c"` // Number of commits
	Adds    int   `json:"a"` // Lines added
	Deletes int   `json:"d"` // Lines deleted
}

// SearchResult represents a search result from GitHub API
type SearchResult struct {
	TotalCount        int                      `json:"total_count"`
	IncompleteResults bool                     `json:"incomplete_results"`
	Items             []map[string]interface{} `json:"items"`
}

// clientImpl implements the Client interface using go-gh
type clientImpl struct {
	apiClient RESTClientInterface
}

// NewClient creates a new GitHub client using existing GitHub CLI authentication
func NewClient() (Client, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub API client: %w", err)
	}

	return &clientImpl{
		apiClient: client,
	}, nil
}

// GetStarredRepos fetches all starred repositories for the authenticated user
func (c *clientImpl) GetStarredRepos(ctx context.Context, _ string) ([]Repository, error) {
	var allRepos []Repository

	page := 1
	perPage := 100

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var repos []Repository

		err := c.apiClient.Get(fmt.Sprintf("user/starred?page=%d&per_page=%d", page, perPage), &repos)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch starred repositories (page %d): %w", page, err)
		}

		if len(repos) == 0 {
			break
		}

		allRepos = append(allRepos, repos...)

		// If we got fewer repos than requested, we've reached the end
		if len(repos) < perPage {
			break
		}

		page++

		// Rate limiting: GitHub allows 5000 requests per hour for authenticated users
		// Add a small delay between requests to be respectful
		time.Sleep(100 * time.Millisecond)
	}

	return allRepos, nil
}

// GetRepositoryContent fetches specific file contents from a repository
func (c *clientImpl) GetRepositoryContent(ctx context.Context, repo Repository, paths []string) ([]Content, error) {
	contents := make([]Content, 0, len(paths))

	for _, path := range paths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var content Content

		err := c.apiClient.Get(fmt.Sprintf("repos/%s/contents/%s", repo.FullName, path), &content)
		if err != nil {
			// If file doesn't exist, skip it rather than failing
			var httpErr *api.HTTPError
			if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
				continue
			}

			return nil, fmt.Errorf("failed to fetch content for %s in %s: %w", path, repo.FullName, err)
		}

		contents = append(contents, content)

		// Small delay between content requests
		time.Sleep(50 * time.Millisecond)
	}

	return contents, nil
}

// GetRepositoryMetadata fetches additional metadata for a repository
func (c *clientImpl) GetRepositoryMetadata(ctx context.Context, repo Repository) (*Metadata, error) {
	metadata := &Metadata{}

	// Fetch commit count from the default branch
	if err := c.fetchCommitCount(ctx, repo, metadata); err != nil {
		// Don't fail completely if commit count fails
		metadata.CommitCount = 0
	}

	// Fetch contributors
	if err := c.fetchContributors(ctx, repo, metadata); err != nil {
		// Don't fail completely if contributors fails
		metadata.Contributors = []string{}
	}

	// Fetch latest release
	if err := c.fetchLatestRelease(ctx, repo, metadata); err != nil {
		// Don't fail completely if release info fails
		metadata.LatestRelease = nil
		metadata.ReleaseCount = 0
	}

	return metadata, nil
}

// fetchCommitCount gets the total number of commits in the default branch
func (c *clientImpl) fetchCommitCount(ctx context.Context, repo Repository, metadata *Metadata) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Get commits from the default branch with per_page=1 to get total count from headers
	var commits []map[string]interface{}

	err := c.apiClient.Get(fmt.Sprintf("repos/%s/commits?sha=%s&per_page=1", repo.FullName, repo.DefaultBranch), &commits)
	if err != nil {
		return fmt.Errorf("failed to fetch commit count: %w", err)
	}

	// For a more accurate count, we'd need to parse Link headers, but for now use a simple approach
	// This is a limitation of the REST API - GraphQL would be better for this
	if len(commits) > 0 {
		metadata.CommitCount = 1 // At least one commit exists

		if lastCommit, ok := commits[0]["commit"].(map[string]interface{}); ok {
			if committer, ok := lastCommit["committer"].(map[string]interface{}); ok {
				if dateStr, ok := committer["date"].(string); ok {
					if date, err := time.Parse(time.RFC3339, dateStr); err == nil {
						metadata.LastCommitDate = date
					}
				}
			}
		}
	}

	return nil
}

// fetchContributors gets the list of contributors for the repository
func (c *clientImpl) fetchContributors(ctx context.Context, repo Repository, metadata *Metadata) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var contributors []map[string]interface{}

	err := c.apiClient.Get(fmt.Sprintf("repos/%s/contributors?per_page=10", repo.FullName), &contributors)
	if err != nil {
		return fmt.Errorf("failed to fetch contributors: %w", err)
	}

	var contributorNames []string

	for _, contributor := range contributors {
		if login, ok := contributor["login"].(string); ok {
			contributorNames = append(contributorNames, login)
		}
	}

	metadata.Contributors = contributorNames

	return nil
}

// fetchLatestRelease gets the latest release information
func (c *clientImpl) fetchLatestRelease(ctx context.Context, repo Repository, metadata *Metadata) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// First get the latest release
	var release Release

	err := c.apiClient.Get(fmt.Sprintf("repos/%s/releases/latest", repo.FullName), &release)
	if err != nil {
		// If no releases exist, that's okay
		var httpErr *api.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil
		}

		return fmt.Errorf("failed to fetch latest release: %w", err)
	}

	metadata.LatestRelease = &release

	// Get total release count
	var releases []map[string]interface{}

	err = c.apiClient.Get(fmt.Sprintf("repos/%s/releases?per_page=1", repo.FullName), &releases)
	if err != nil {
		return fmt.Errorf("failed to fetch release count: %w", err)
	}

	// This is a simplified count - in reality we'd need to paginate through all releases
	metadata.ReleaseCount = len(releases)

	return nil
}

// GetContributors fetches the top N contributors for a repository
func (c *clientImpl) GetContributors(ctx context.Context, fullName string, topN int) ([]Contributor, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var contributors []Contributor

	err := c.apiClient.Get(fmt.Sprintf("repos/%s/contributors?per_page=%d", fullName, topN), &contributors)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contributors for %s: %w", fullName, err)
	}

	return contributors, nil
}

// GetTopics fetches the topics associated with a repository
func (c *clientImpl) GetTopics(ctx context.Context, fullName string) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var response struct {
		Names []string `json:"names"`
	}

	err := c.apiClient.Get(fmt.Sprintf("repos/%s/topics", fullName), &response)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch topics for %s: %w", fullName, err)
	}

	return response.Names, nil
}

// GetLanguages fetches the programming languages used in a repository
func (c *clientImpl) GetLanguages(ctx context.Context, fullName string) (map[string]int64, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var languages map[string]int64

	err := c.apiClient.Get(fmt.Sprintf("repos/%s/languages", fullName), &languages)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch languages for %s: %w", fullName, err)
	}

	return languages, nil
}

// GetCommitActivity fetches commit activity statistics for a repository
func (c *clientImpl) GetCommitActivity(ctx context.Context, fullName string) (*CommitActivity, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var weeks []WeeklyCommits

	err := c.apiClient.Get(fmt.Sprintf("repos/%s/stats/commit_activity", fullName), &weeks)
	if err != nil {
		// Handle 202 Accepted response (stats being computed)
		var httpErr *api.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusAccepted {
			return &CommitActivity{
				Weeks: []WeeklyCommits{},
				Total: -1, // Indicates stats are being computed
			}, nil
		}

		return nil, fmt.Errorf("failed to fetch commit activity for %s: %w", fullName, err)
	}

	// Calculate total commits
	total := 0
	for _, week := range weeks {
		total += week.Commits
	}

	return &CommitActivity{
		Weeks: weeks,
		Total: total,
	}, nil
}

// GetPullCounts fetches pull request counts for a repository
func (c *clientImpl) GetPullCounts(ctx context.Context, fullName string) (open int, total int, err error) {
	select {
	case <-ctx.Done():
		return 0, 0, ctx.Err()
	default:
	}

	// Get open PRs
	var openResult SearchResult

	err = c.apiClient.Get(fmt.Sprintf("search/issues?q=repo:%s+type:pr+state:open&per_page=1", fullName), &openResult)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch open PR count for %s: %w", fullName, err)
	}

	open = openResult.TotalCount

	// Get total PRs (open + closed)
	var totalResult SearchResult

	err = c.apiClient.Get(fmt.Sprintf("search/issues?q=repo:%s+type:pr&per_page=1", fullName), &totalResult)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch total PR count for %s: %w", fullName, err)
	}

	total = totalResult.TotalCount

	return open, total, nil
}

// GetIssueCounts fetches issue counts for a repository
func (c *clientImpl) GetIssueCounts(ctx context.Context, fullName string) (open int, total int, err error) {
	select {
	case <-ctx.Done():
		return 0, 0, ctx.Err()
	default:
	}

	// Get open issues (excluding PRs)
	var openResult SearchResult

	err = c.apiClient.Get(fmt.Sprintf("search/issues?q=repo:%s+type:issue+state:open&per_page=1", fullName), &openResult)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch open issue count for %s: %w", fullName, err)
	}

	open = openResult.TotalCount

	// Get total issues (open + closed, excluding PRs)
	var totalResult SearchResult

	err = c.apiClient.Get(fmt.Sprintf("search/issues?q=repo:%s+type:issue&per_page=1", fullName), &totalResult)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch total issue count for %s: %w", fullName, err)
	}

	total = totalResult.TotalCount

	return open, total, nil
}

// stripHTMLTags removes HTML tags from text and cleans whitespace
func stripHTMLTags(html string) string {
	// Use html-to-markdown library with only base plugin for plain text output
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
		),
	)

	// Convert HTML to plain text (no markdown formatting)
	text, err := conv.ConvertString(html)
	if err != nil {
		return html
	}
	return text
}

// GetHomepageText fetches text content from an external homepage URL
func (c *clientImpl) GetHomepageText(ctx context.Context, urlStr string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTP/HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "gh-star-search/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Limit response size to prevent abuse
	const maxSize = 1024 * 1024 // 1MB
	limitedReader := io.LimitReader(resp.Body, maxSize)

	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Extract text from HTML
	text := stripHTMLTags(string(body))

	return text, nil
}
