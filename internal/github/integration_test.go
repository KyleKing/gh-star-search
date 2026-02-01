package github

import (
	"context"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/kyleking/gh-star-search/internal/cache"
	"github.com/kyleking/gh-star-search/internal/config"
)

func TestCachedClientIntegration(t *testing.T) {
	// Create a mock client
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	// Create a file cache for testing
	cacheDir := t.TempDir()

	fileCache, err := cache.NewFileCache(cacheDir, 10, 1*time.Hour, 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	defer fileCache.Close()

	// Create config
	cfg, _ := config.LoadConfig()
	cfg.Cache.MetadataStaleDays = 365 // Very long TTL for testing
	cfg.Cache.StatsStaleDays = 365    // Very long TTL for testing

	// Create cached client
	cachedClient := NewCachedClient(client, fileCache, cfg)

	// Set up mock responses
	expectedTopics := struct {
		Names []string `json:"names"`
	}{
		Names: []string{"go", "cli"},
	}
	mockClient.setResponse("repos/owner/repo/topics", expectedTopics)

	ctx := context.Background()

	// Test GetTopics (metadata-level caching)
	topics1, err := cachedClient.GetTopics(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(topics1) != 2 {
		t.Fatalf("Expected 2 topics, got: %d", len(topics1))
	}

	// Call again - should use cache
	topics2, err := cachedClient.GetTopics(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Expected no error on cached call, got: %v", err)
	}

	if len(topics2) != 2 {
		t.Fatalf("Expected 2 topics from cache, got: %d", len(topics2))
	}

	// Verify only one API call was made (second was cached)
	callCount := mockClient.getCallCount("repos/owner/repo/topics")
	if callCount != 1 {
		t.Logf("Debug: API call count: %d", callCount)
		// Check cache stats
		stats, _ := cachedClient.GetCacheStats(ctx)
		t.Logf("Debug: Cache stats: %+v", stats)
		t.Errorf("Expected 1 API call, got: %d", callCount)
	}

	if topics1[0] != "go" || topics1[1] != "cli" {
		t.Errorf("Expected topics [go, cli], got: %v", topics1)
	}
}

func TestWorkerPoolIntegration(t *testing.T) {
	// Create a mock client
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	// Set up mock responses for multiple repositories
	repos := []Repository{
		{FullName: "owner/repo1"},
		{FullName: "owner/repo2"},
	}

	// Mock responses for repo1
	mockClient.setResponse("repos/owner/repo1/contributors?per_page=10", []Contributor{
		{Login: "user1", Contributions: 100},
	})
	mockClient.setResponse("repos/owner/repo1/topics", struct {
		Names []string `json:"names"`
	}{Names: []string{"go"}})
	mockClient.setResponse("repos/owner/repo1/languages", map[string]int64{"Go": 12345})

	// Mock responses for repo2
	mockClient.setResponse("repos/owner/repo2/contributors?per_page=10", []Contributor{
		{Login: "user2", Contributions: 200},
	})
	mockClient.setResponse("repos/owner/repo2/topics", struct {
		Names []string `json:"names"`
	}{Names: []string{"rust"}})
	mockClient.setResponse("repos/owner/repo2/languages", map[string]int64{"Rust": 54321})

	// Mock commit activity (with 202 response for repo1, success for repo2)
	mockClient.setError("repos/owner/repo1/stats/commit_activity", &api.HTTPError{StatusCode: 202})
	mockClient.setResponse("repos/owner/repo2/stats/commit_activity", []WeeklyCommits{
		{Week: 1640995200, Commits: 5},
	})

	// Mock PR and issue counts
	mockClient.setResponse(
		"search/issues?q=repo:owner/repo1+type:pr+state:open&per_page=1",
		SearchResult{TotalCount: 2},
	)
	mockClient.setResponse(
		"search/issues?q=repo:owner/repo1+type:pr&per_page=1",
		SearchResult{TotalCount: 10},
	)
	mockClient.setResponse(
		"search/issues?q=repo:owner/repo1+type:issue+state:open&per_page=1",
		SearchResult{TotalCount: 3},
	)
	mockClient.setResponse(
		"search/issues?q=repo:owner/repo1+type:issue&per_page=1",
		SearchResult{TotalCount: 15},
	)

	mockClient.setResponse(
		"search/issues?q=repo:owner/repo2+type:pr+state:open&per_page=1",
		SearchResult{TotalCount: 1},
	)
	mockClient.setResponse(
		"search/issues?q=repo:owner/repo2+type:pr&per_page=1",
		SearchResult{TotalCount: 5},
	)
	mockClient.setResponse(
		"search/issues?q=repo:owner/repo2+type:issue+state:open&per_page=1",
		SearchResult{TotalCount: 0},
	)
	mockClient.setResponse(
		"search/issues?q=repo:owner/repo2+type:issue&per_page=1",
		SearchResult{TotalCount: 2},
	)

	// Create batch executor
	batchExecutor := NewBatchExecutor(client, 2, 10)

	ctx := context.Background()

	// Fetch metrics for all repositories
	metrics := batchExecutor.FetchRepositoryMetrics(ctx, repos)

	if len(metrics) != 2 {
		t.Fatalf("Expected metrics for 2 repositories, got: %d", len(metrics))
	}

	// Verify repo1 metrics
	repo1Metrics := metrics["owner/repo1"]
	if repo1Metrics == nil {
		t.Fatal("Expected metrics for owner/repo1")
	}

	if len(repo1Metrics.Contributors) != 1 {
		t.Errorf("Expected 1 contributor for repo1, got: %d", len(repo1Metrics.Contributors))
	}

	if len(repo1Metrics.Topics) != 1 || repo1Metrics.Topics[0] != "go" {
		t.Errorf("Expected topics [go] for repo1, got: %v", repo1Metrics.Topics)
	}

	if repo1Metrics.Languages["Go"] != 12345 {
		t.Errorf("Expected Go bytes 12345 for repo1, got: %d", repo1Metrics.Languages["Go"])
	}

	// Commit activity should indicate stats being computed (Total = -1)
	if repo1Metrics.CommitActivity == nil || repo1Metrics.CommitActivity.Total != -1 {
		t.Errorf(
			"Expected commit activity total -1 (computing) for repo1, got: %v",
			repo1Metrics.CommitActivity,
		)
	}

	if repo1Metrics.OpenPRs != 2 || repo1Metrics.TotalPRs != 10 {
		t.Errorf(
			"Expected PRs 2/10 for repo1, got: %d/%d",
			repo1Metrics.OpenPRs,
			repo1Metrics.TotalPRs,
		)
	}

	if repo1Metrics.OpenIssues != 3 || repo1Metrics.TotalIssues != 15 {
		t.Errorf(
			"Expected issues 3/15 for repo1, got: %d/%d",
			repo1Metrics.OpenIssues,
			repo1Metrics.TotalIssues,
		)
	}

	// Verify repo2 metrics
	repo2Metrics := metrics["owner/repo2"]
	if repo2Metrics == nil {
		t.Fatal("Expected metrics for owner/repo2")
	}

	if len(repo2Metrics.Contributors) != 1 {
		t.Errorf("Expected 1 contributor for repo2, got: %d", len(repo2Metrics.Contributors))
	}

	if repo2Metrics.Contributors[0].Login != "user2" {
		t.Errorf(
			"Expected contributor user2 for repo2, got: %s",
			repo2Metrics.Contributors[0].Login,
		)
	}

	if repo2Metrics.CommitActivity == nil || repo2Metrics.CommitActivity.Total != 5 {
		t.Errorf("Expected commit activity total 5 for repo2, got: %v", repo2Metrics.CommitActivity)
	}
}

func TestCacheInvalidation(t *testing.T) {
	// Create a mock client
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	// Create a file cache for testing
	cacheDir := t.TempDir()

	fileCache, err := cache.NewFileCache(cacheDir, 10, 1*time.Hour, 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	defer fileCache.Close()

	// Create config
	cfg, _ := config.LoadConfig()

	// Create cached client
	cachedClient := NewCachedClient(client, fileCache, cfg)

	// Set up mock response
	expectedTopics := struct {
		Names []string `json:"names"`
	}{
		Names: []string{"go", "cli"},
	}
	mockClient.setResponse("repos/owner/repo/topics", expectedTopics)

	ctx := context.Background()

	// Cache some data
	topics, err := cachedClient.GetTopics(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(topics) != 2 {
		t.Fatalf("Expected 2 topics, got: %d", len(topics))
	}

	// Invalidate cache
	err = cachedClient.InvalidateCache(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Expected no error invalidating cache, got: %v", err)
	}

	// Update mock response
	newTopics := struct {
		Names []string `json:"names"`
	}{
		Names: []string{"rust", "web"},
	}
	mockClient.setResponse("repos/owner/repo/topics", newTopics)

	// Fetch again - should get new data since cache was invalidated
	topics2, err := cachedClient.GetTopics(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Expected no error after invalidation, got: %v", err)
	}

	if len(topics2) != 2 {
		t.Fatalf("Expected 2 topics after invalidation, got: %d", len(topics2))
	}

	if topics2[0] != "rust" || topics2[1] != "web" {
		t.Errorf("Expected topics [rust, web] after invalidation, got: %v", topics2)
	}

	// Verify API was called twice (once before invalidation, once after)
	if mockClient.getCallCount("repos/owner/repo/topics") != 2 {
		t.Errorf(
			"Expected 2 API calls, got: %d",
			mockClient.getCallCount("repos/owner/repo/topics"),
		)
	}
}
