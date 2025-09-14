package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kyleking/gh-star-search/internal/cache"
	"github.com/kyleking/gh-star-search/internal/config"
)

// CachedClient wraps a GitHub client with caching capabilities
type CachedClient struct {
	client Client
	cache  cache.Cache
	config *config.Config
}

// NewCachedClient creates a new cached GitHub client
func NewCachedClient(client Client, cache cache.Cache, config *config.Config) *CachedClient {
	return &CachedClient{
		client: client,
		cache:  cache,
		config: config,
	}
}

// CacheEntry represents a cached API response with metadata
type CacheEntry struct {
	Data      interface{} `json:"data"`
	CachedAt  time.Time   `json:"cached_at"`
	ExpiresAt time.Time   `json:"expires_at"`
	Type      string      `json:"type"`
}

// GetStarredRepos fetches starred repositories with caching
func (c *CachedClient) GetStarredRepos(ctx context.Context, username string) ([]Repository, error) {
	cacheKey := "starred_repos:" + username
	ttl := time.Duration(c.config.Cache.MetadataStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		if repos, ok := cached.([]Repository); ok {
			return repos, nil
		}
	}

	// Fetch from API
	repos, err := c.client.GetStarredRepos(ctx, username)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.setCachedData(ctx, cacheKey, repos, ttl, "metadata")

	return repos, nil
}

// GetRepositoryContent fetches repository content with caching
func (c *CachedClient) GetRepositoryContent(ctx context.Context, repo Repository, paths []string) ([]Content, error) {
	cacheKey := fmt.Sprintf("content:%s:%v", repo.FullName, paths)
	ttl := time.Duration(c.config.Cache.MetadataStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		if content, ok := cached.([]Content); ok {
			return content, nil
		}
	}

	// Fetch from API
	content, err := c.client.GetRepositoryContent(ctx, repo, paths)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.setCachedData(ctx, cacheKey, content, ttl, "metadata")

	return content, nil
}

// GetRepositoryMetadata fetches repository metadata with caching
func (c *CachedClient) GetRepositoryMetadata(ctx context.Context, repo Repository) (*Metadata, error) {
	cacheKey := "metadata:" + repo.FullName
	ttl := time.Duration(c.config.Cache.MetadataStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		if metadata, ok := cached.(*Metadata); ok {
			return metadata, nil
		}
	}

	// Fetch from API
	metadata, err := c.client.GetRepositoryMetadata(ctx, repo)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.setCachedData(ctx, cacheKey, metadata, ttl, "metadata")

	return metadata, nil
}

// GetContributors fetches contributors with stats-level caching
func (c *CachedClient) GetContributors(ctx context.Context, fullName string, topN int) ([]Contributor, error) {
	cacheKey := fmt.Sprintf("contributors:%s:%d", fullName, topN)
	ttl := time.Duration(c.config.Cache.StatsStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		// Handle type conversion for JSON unmarshaling
		switch v := cached.(type) {
		case []Contributor:
			return v, nil
		case []interface{}:
			// Convert []interface{} to []Contributor
			contributors := make([]Contributor, len(v))

			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					contributor := Contributor{}
					if login, ok := itemMap["login"].(string); ok {
						contributor.Login = login
					}

					if contributions, ok := itemMap["contributions"].(float64); ok {
						contributor.Contributions = int(contributions)
					}

					if contributorType, ok := itemMap["type"].(string); ok {
						contributor.Type = contributorType
					}

					if avatarURL, ok := itemMap["avatar_url"].(string); ok {
						contributor.AvatarURL = avatarURL
					}

					contributors[i] = contributor
				}
			}

			return contributors, nil
		}
	}

	// Fetch from API
	contributors, err := c.client.GetContributors(ctx, fullName, topN)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.setCachedData(ctx, cacheKey, contributors, ttl, "stats")

	return contributors, nil
}

// GetTopics fetches topics with metadata-level caching
func (c *CachedClient) GetTopics(ctx context.Context, fullName string) ([]string, error) {
	cacheKey := "topics:" + fullName
	ttl := time.Duration(c.config.Cache.MetadataStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		// Handle type conversion for JSON unmarshaling
		switch v := cached.(type) {
		case []string:
			return v, nil
		case []interface{}:
			// Convert []interface{} to []string
			topics := make([]string, len(v))

			for i, item := range v {
				if str, ok := item.(string); ok {
					topics[i] = str
				}
			}

			return topics, nil
		}
	}

	// Fetch from API
	topics, err := c.client.GetTopics(ctx, fullName)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.setCachedData(ctx, cacheKey, topics, ttl, "metadata")

	return topics, nil
}

// GetLanguages fetches languages with metadata-level caching
func (c *CachedClient) GetLanguages(ctx context.Context, fullName string) (map[string]int64, error) {
	cacheKey := "languages:" + fullName
	ttl := time.Duration(c.config.Cache.MetadataStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		// Handle type conversion for JSON unmarshaling
		switch v := cached.(type) {
		case map[string]int64:
			return v, nil
		case map[string]interface{}:
			// Convert map[string]interface{} to map[string]int64
			languages := make(map[string]int64)

			for key, value := range v {
				if floatVal, ok := value.(float64); ok {
					languages[key] = int64(floatVal)
				}
			}

			return languages, nil
		}
	}

	// Fetch from API
	languages, err := c.client.GetLanguages(ctx, fullName)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.setCachedData(ctx, cacheKey, languages, ttl, "metadata")

	return languages, nil
}

// GetCommitActivity fetches commit activity with stats-level caching
func (c *CachedClient) GetCommitActivity(ctx context.Context, fullName string) (*CommitActivity, error) {
	cacheKey := "commits:" + fullName
	ttl := time.Duration(c.config.Cache.StatsStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		if activity, ok := cached.(*CommitActivity); ok {
			return activity, nil
		}
	}

	// Fetch from API
	activity, err := c.client.GetCommitActivity(ctx, fullName)
	if err != nil {
		return nil, err
	}

	// Cache the result (even if stats are being computed)
	c.setCachedData(ctx, cacheKey, activity, ttl, "stats")

	return activity, nil
}

// GetPullCounts fetches PR counts with stats-level caching
func (c *CachedClient) GetPullCounts(ctx context.Context, fullName string) (open int, total int, err error) {
	cacheKey := "prs:" + fullName
	ttl := time.Duration(c.config.Cache.StatsStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		// Handle type conversion for JSON unmarshaling
		switch v := cached.(type) {
		case map[string]int:
			return v["open"], v["total"], nil
		case map[string]interface{}:
			// Convert map[string]interface{} to map[string]int
			var openVal, totalVal int
			if openFloat, ok := v["open"].(float64); ok {
				openVal = int(openFloat)
			}

			if totalFloat, ok := v["total"].(float64); ok {
				totalVal = int(totalFloat)
			}

			return openVal, totalVal, nil
		}
	}

	// Fetch from API
	openPRs, totalPRs, err := c.client.GetPullCounts(ctx, fullName)
	if err != nil {
		return 0, 0, err
	}

	// Cache the result
	counts := map[string]int{"open": openPRs, "total": totalPRs}
	c.setCachedData(ctx, cacheKey, counts, ttl, "stats")

	return openPRs, totalPRs, nil
}

// GetIssueCounts fetches issue counts with stats-level caching
func (c *CachedClient) GetIssueCounts(ctx context.Context, fullName string) (open int, total int, err error) {
	cacheKey := "issues:" + fullName
	ttl := time.Duration(c.config.Cache.StatsStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		// Handle type conversion for JSON unmarshaling
		switch v := cached.(type) {
		case map[string]int:
			return v["open"], v["total"], nil
		case map[string]interface{}:
			// Convert map[string]interface{} to map[string]int
			var openVal, totalVal int
			if openFloat, ok := v["open"].(float64); ok {
				openVal = int(openFloat)
			}

			if totalFloat, ok := v["total"].(float64); ok {
				totalVal = int(totalFloat)
			}

			return openVal, totalVal, nil
		}
	}

	// Fetch from API
	openIssues, totalIssues, err := c.client.GetIssueCounts(ctx, fullName)
	if err != nil {
		return 0, 0, err
	}

	// Cache the result
	counts := map[string]int{"open": openIssues, "total": totalIssues}
	c.setCachedData(ctx, cacheKey, counts, ttl, "stats")

	return openIssues, totalIssues, nil
}

// GetHomepageText fetches homepage text with metadata-level caching
func (c *CachedClient) GetHomepageText(ctx context.Context, url string) (string, error) {
	cacheKey := "homepage:" + url
	ttl := time.Duration(c.config.Cache.MetadataStaleDays) * 24 * time.Hour

	// Try to get from cache first
	if cached, err := c.getCachedData(ctx, cacheKey, ttl); err == nil {
		if text, ok := cached.(string); ok {
			return text, nil
		}
	}

	// Fetch from API
	text, err := c.client.GetHomepageText(ctx, url)
	if err != nil {
		return "", err
	}

	// Cache the result
	c.setCachedData(ctx, cacheKey, text, ttl, "metadata")

	return text, nil
}

// getCachedData retrieves and validates cached data
func (c *CachedClient) getCachedData(ctx context.Context, key string, maxAge time.Duration) (interface{}, error) {
	data, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache entry: %w", err)
	}

	// Check if entry has expired based on maxAge
	if time.Since(entry.CachedAt) > maxAge {
		return nil, errors.New("cache entry expired")
	}

	return entry.Data, nil
}

// setCachedData stores data in cache with metadata
func (c *CachedClient) setCachedData(ctx context.Context, key string, data interface{}, ttl time.Duration, entryType string) {
	entry := CacheEntry{
		Data:      data,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Type:      entryType,
	}

	entryData, err := json.Marshal(entry)
	if err != nil {
		// Log error but don't fail the operation
		return
	}

	// Store in cache (ignore errors for now)
	c.cache.Set(ctx, key, entryData, ttl)
}

// InvalidateCache removes cached data for a specific repository
func (c *CachedClient) InvalidateCache(ctx context.Context, fullName string) error {
	// List of cache key patterns to invalidate
	patterns := []string{
		fmt.Sprintf("contributors:%s:", fullName),
		"topics:" + fullName,
		"languages:" + fullName,
		"commits:" + fullName,
		"prs:" + fullName,
		"issues:" + fullName,
		"metadata:" + fullName,
		fmt.Sprintf("content:%s:", fullName),
	}

	for _, pattern := range patterns {
		// For simplicity, we'll delete exact matches
		// In a more sophisticated implementation, we'd support pattern matching
		c.cache.Delete(ctx, pattern)
	}

	return nil
}

// GetCacheStats returns cache statistics
func (c *CachedClient) GetCacheStats(ctx context.Context) (*cache.Stats, error) {
	return c.cache.GetStats(ctx)
}
