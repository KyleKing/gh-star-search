package cmd

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KyleKing/gh-star-search/internal/config"
	"github.com/KyleKing/gh-star-search/internal/github"
	"github.com/KyleKing/gh-star-search/internal/processor"
	"github.com/KyleKing/gh-star-search/internal/storage"
	testutil "github.com/KyleKing/gh-star-search/internal/testutil"
)

func TestProcessBatch_PartialFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	repo, cleanup := storage.NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	repos := []github.Repository{
		testutil.NewTestRepository(
			testutil.WithFullName("user/good-repo-1"),
			testutil.WithStars(100),
		),
		testutil.NewTestRepository(
			testutil.WithFullName("user/error-repo"),
			testutil.WithStars(200),
		),
		testutil.NewTestRepository(
			testutil.WithFullName("user/good-repo-2"),
			testutil.WithStars(300),
		),
	}

	mockGitHub := testutil.NewMockGitHubClient(
		testutil.WithStarredRepos(repos),
		testutil.WithContent(map[string][]github.Content{
			"user/good-repo-1": {testutil.NewTestContent("README.md", "Good repo 1")},
			"user/good-repo-2": {testutil.NewTestContent("README.md", "Good repo 2")},
		}),
		testutil.WithError("user/error-repo", errors.New("simulated API error")),
	)

	cfg, _ := config.LoadConfig()
	processorService := processor.NewService(mockGitHub)
	syncService := &SyncService{
		githubClient: mockGitHub,
		processor:    processorService,
		storage:      repo,
		config:       cfg,
		verbose:      false,
	}

	stats := &SyncStats{}
	progress := NewProgressTracker(len(repos), "Testing")
	isNewRepo := map[string]bool{
		"user/good-repo-1": true,
		"user/error-repo":  true,
		"user/good-repo-2": true,
	}

	err := syncService.processBatch(ctx, repos, stats, progress, isNewRepo, false)
	require.NoError(t, err, "processBatch should handle partial failures gracefully")

	assert.Positive(t, stats.ProcessedRepos, "should process some repositories successfully")
	assert.Positive(t, stats.ErrorRepos, "should record errors")

	stored, err := repo.GetRepository(ctx, "user/good-repo-1")
	require.NoError(t, err, "good-repo-1 should be stored")
	assert.Equal(t, 100, stored.StargazersCount)

	_, err = repo.GetRepository(ctx, "user/error-repo")
	assert.Error(t, err, "error-repo should not be stored")
}

func TestProcessBatch_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	repo, cleanup := storage.NewTestDB(t)
	defer cleanup()

	repos := make([]github.Repository, 20)
	for i := range repos {
		repos[i] = testutil.NewTestRepository(
			testutil.WithFullName(fmt.Sprintf("user/repo-%d", i)),
		)
	}

	mockGitHub := testutil.NewMockGitHubClient(
		testutil.WithStarredRepos(repos),
	)

	cfg, _ := config.LoadConfig()
	processorService := processor.NewService(mockGitHub)
	syncService := &SyncService{
		githubClient: mockGitHub,
		processor:    processorService,
		storage:      repo,
		config:       cfg,
		verbose:      false,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stats := &SyncStats{}
	progress := NewProgressTracker(len(repos), "Testing")
	isNewRepo := make(map[string]bool)
	for _, r := range repos {
		isNewRepo[r.FullName] = true
	}

	err := syncService.processBatch(ctx, repos, stats, progress, isNewRepo, false)
	require.Error(t, err, "should return error when context is canceled")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestProgressTracker_ConcurrentUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	const numUpdates = 100
	tracker := NewProgressTracker(numUpdates, "Testing")

	testutil.RunConcurrent(t, numUpdates, func(i int) {
		tracker.Update(fmt.Sprintf("repo-%d", i))
	})

	tracker.mu.Lock()
	processed := tracker.processed
	tracker.mu.Unlock()

	assert.Equal(t, numUpdates, processed, "all updates should be counted")
}

func TestSyncStats_ConcurrentIncrements(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	stats := &SyncStats{}
	const numIncrements = 100

	testutil.RunConcurrent(t, numIncrements, func(_ int) {
		stats.SafeIncrement("new")
		stats.SafeIncrement("processed")
	})

	stats.mu.Lock()
	newRepos := stats.NewRepos
	processed := stats.ProcessedRepos
	stats.mu.Unlock()

	assert.Equal(t, numIncrements, newRepos, "all new repo increments should be counted")
	assert.Equal(t, numIncrements, processed, "all processed increments should be counted")
}

func TestSyncStats_RaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	testutil.AssertNoRaces(t, func() {
		stats := &SyncStats{}
		stats.SafeIncrement("new")
		stats.SafeIncrement("updated")
		stats.SafeIncrement("removed")
		stats.SafeIncrement("skipped")
		stats.SafeIncrement("error")
		stats.SafeIncrement("processed")
		stats.SafeIncrement("content_changes")
		stats.SafeIncrement("metadata_changes")

		stats.mu.Lock()
		_ = stats.NewRepos
		_ = stats.UpdatedRepos
		stats.mu.Unlock()
	}, 50)
}

func TestProgressTracker_RaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	testutil.AssertNoRaces(t, func() {
		tracker := NewProgressTracker(100, "Testing")
		tracker.Update("test-repo")

		tracker.mu.Lock()
		_ = tracker.processed
		tracker.mu.Unlock()
	}, 50)
}

func TestProcessBatch_ConcurrentRepoModification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	repo, cleanup := storage.NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	const numRepos = 10
	repos := make([]github.Repository, numRepos)
	contentMap := make(map[string][]github.Content)

	for i := range numRepos {
		repoName := fmt.Sprintf("user/repo-%d", i)
		repos[i] = testutil.NewTestRepository(
			testutil.WithFullName(repoName),
			testutil.WithStars((i+1)*10),
		)
		contentMap[repoName] = []github.Content{
			testutil.NewTestContent("README.md", fmt.Sprintf("Repo %d", i)),
		}
	}

	mockGitHub := testutil.NewMockGitHubClient(
		testutil.WithStarredRepos(repos),
		testutil.WithContent(contentMap),
	)

	cfg, _ := config.LoadConfig()
	processorService := processor.NewService(mockGitHub)
	syncService := &SyncService{
		githubClient: mockGitHub,
		processor:    processorService,
		storage:      repo,
		config:       cfg,
		verbose:      false,
	}

	stats := &SyncStats{}
	progress := NewProgressTracker(len(repos), "Testing")
	isNewRepo := make(map[string]bool)
	for _, r := range repos {
		isNewRepo[r.FullName] = true
	}

	err := syncService.processBatch(ctx, repos, stats, progress, isNewRepo, false)
	require.NoError(t, err)

	for i := range numRepos {
		repoName := fmt.Sprintf("user/repo-%d", i)
		stored, err := repo.GetRepository(ctx, repoName)
		require.NoError(t, err, "repo %s should be stored", repoName)
		assert.Equal(t, (i+1)*10, stored.StargazersCount)
	}
}

func TestProcessBatch_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	repo, cleanup := storage.NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	const numRepos = 50
	repos := make([]github.Repository, numRepos)
	contentMap := make(map[string][]github.Content)

	for i := range numRepos {
		repoName := fmt.Sprintf("user/stress-repo-%d", i)
		repos[i] = testutil.NewTestRepository(
			testutil.WithFullName(repoName),
			testutil.WithStars((i+1)*100),
		)
		contentMap[repoName] = []github.Content{
			testutil.NewTestContent("README.md", fmt.Sprintf("Stress test repo %d\n\nLonger content here...", i)),
		}
	}

	mockGitHub := testutil.NewMockGitHubClient(
		testutil.WithStarredRepos(repos),
		testutil.WithContent(contentMap),
	)

	cfg, _ := config.LoadConfig()
	processorService := processor.NewService(mockGitHub)
	syncService := &SyncService{
		githubClient: mockGitHub,
		processor:    processorService,
		storage:      repo,
		config:       cfg,
		verbose:      false,
	}

	stats := &SyncStats{}
	progress := NewProgressTracker(len(repos), "Stress Testing")
	isNewRepo := make(map[string]bool)
	for _, r := range repos {
		isNewRepo[r.FullName] = true
	}

	startTime := time.Now()
	err := syncService.processBatch(ctx, repos, stats, progress, isNewRepo, false)
	duration := time.Since(startTime)

	require.NoError(t, err)
	assert.LessOrEqual(t, duration, 30*time.Second, "stress test should complete in reasonable time")
	assert.Equal(t, numRepos, stats.ProcessedRepos+stats.ErrorRepos, "all repos should be accounted for")

	t.Logf("Processed %d repositories in %v", numRepos, duration)
	t.Logf("Stats: Processed=%d, New=%d, Errors=%d", stats.ProcessedRepos, stats.NewRepos, stats.ErrorRepos)
}

func TestSyncStats_AllFieldsConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	stats := &SyncStats{}
	const iterations = 20

	var wg sync.WaitGroup
	fields := []string{"new", "updated", "removed", "skipped", "error", "processed", "content_changes", "metadata_changes"}

	for _, field := range fields {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			for range iterations {
				stats.SafeIncrement(f)
			}
		}(field)
	}

	wg.Wait()

	stats.mu.Lock()
	defer stats.mu.Unlock()

	assert.Equal(t, iterations, stats.NewRepos)
	assert.Equal(t, iterations, stats.UpdatedRepos)
	assert.Equal(t, iterations, stats.RemovedRepos)
	assert.Equal(t, iterations, stats.SkippedRepos)
	assert.Equal(t, iterations, stats.ErrorRepos)
	assert.Equal(t, iterations, stats.ProcessedRepos)
	assert.Equal(t, iterations, stats.ContentChanges)
	assert.Equal(t, iterations, stats.MetadataChanges)
}
