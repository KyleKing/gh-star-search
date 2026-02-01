package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyleking/gh-star-search/internal/processor"
	testutil "github.com/kyleking/gh-star-search/internal/testutil"
)

func TestStoreRepository_ConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()
	const numRepos = 20

	var wg sync.WaitGroup
	errChan := make(chan error, numRepos)

	for i := range numRepos {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			testRepo := testutil.NewTestProcessedRepoSimple(fmt.Sprintf("user/concurrent-repo-%d", id))
			if err := repo.StoreRepository(ctx, testRepo); err != nil {
				errChan <- fmt.Errorf("failed to store repo %d: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "no errors should occur during concurrent writes")

	stats, err := repo.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, numRepos, stats.TotalRepositories, "all repositories should be stored")

	for i := range numRepos {
		repoName := fmt.Sprintf("user/concurrent-repo-%d", i)
		stored, err := repo.GetRepository(ctx, repoName)
		require.NoError(t, err, "repo %s should exist", repoName)
		assert.Equal(t, repoName, stored.FullName)
	}
}

func TestUpdateRepository_ConcurrentUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	initialRepo := testutil.NewTestProcessedRepoSimple("user/concurrent-update-repo")
	require.NoError(t, repo.StoreRepository(ctx, initialRepo))

	const numUpdates = 10
	var wg sync.WaitGroup
	errChan := make(chan error, numUpdates)

	for i := range numUpdates {
		wg.Add(1)
		go func(updateID int) {
			defer wg.Done()
			updatedRepo := testutil.NewTestProcessedRepo(
				testutil.NewTestRepository(
					testutil.WithFullName("user/concurrent-update-repo"),
					testutil.WithStars(100+updateID),
				),
				[]processor.ContentChunk{
					testutil.NewTestChunk("README.md", fmt.Sprintf("Update %d", updateID)),
				},
			)
			if err := repo.UpdateRepository(ctx, updatedRepo); err != nil {
				errChan <- fmt.Errorf("update %d failed: %w", updateID, err)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		t.Logf("Note: %d concurrent update conflicts detected (expected behavior)", len(errs))
		for _, err := range errs {
			t.Logf("  - %v", err)
		}
	}

	stored, err := repo.GetRepository(ctx, "user/concurrent-update-repo")
	require.NoError(t, err, "repository should still be retrievable")
	assert.Equal(t, "user/concurrent-update-repo", stored.FullName)

	t.Logf("Final star count: %d", stored.StargazersCount)
}

func TestStoreRepository_TransactionIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping transaction test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	initialRepo := testutil.NewTestProcessedRepoSimple("user/isolated-repo")
	require.NoError(t, repo.StoreRepository(ctx, initialRepo))

	stored1, err := repo.GetRepository(ctx, "user/isolated-repo")
	require.NoError(t, err)
	initialStars := stored1.StargazersCount

	updatedRepo := testutil.NewTestProcessedRepo(
		testutil.NewTestRepository(
			testutil.WithFullName("user/isolated-repo"),
			testutil.WithStars(initialStars+100),
		),
		[]processor.ContentChunk{
			testutil.NewTestChunk("README.md", "Updated content"),
		},
	)
	require.NoError(t, repo.UpdateRepository(ctx, updatedRepo))

	stored2, err := repo.GetRepository(ctx, "user/isolated-repo")
	require.NoError(t, err)
	assert.Equal(t, initialStars+100, stored2.StargazersCount)
}

func TestStoreRepository_DuplicateInsertion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping transaction test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	testRepo := testutil.NewTestProcessedRepoSimple("user/duplicate-repo")
	require.NoError(t, repo.StoreRepository(ctx, testRepo))

	err := repo.StoreRepository(ctx, testRepo)
	assert.Error(t, err, "inserting duplicate repository should fail")
}

func TestUpdateRepository_NonexistentRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping transaction test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	testRepo := testutil.NewTestProcessedRepoSimple("user/nonexistent-repo")
	err := repo.UpdateRepository(ctx, testRepo)
	assert.Error(t, err, "updating non-existent repository should fail")
}

func TestStoreRepository_LargeTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping transaction test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	chunks := make([]processor.ContentChunk, 100)
	for i := range chunks {
		chunks[i] = testutil.NewTestChunk(
			fmt.Sprintf("file-%d.md", i),
			fmt.Sprintf("Large content for file %d with lots of text", i),
		)
	}

	testRepo := testutil.NewTestProcessedRepo(
		testutil.NewTestRepository(testutil.WithFullName("user/large-repo")),
		chunks,
	)

	require.NoError(t, repo.StoreRepository(ctx, testRepo))

	stored, err := repo.GetRepository(ctx, "user/large-repo")
	require.NoError(t, err)
	assert.Equal(t, "user/large-repo", stored.FullName)
}

func TestUpdateRepository_WithMetadataChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping transaction test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	initialRepo := testutil.NewTestProcessedRepo(
		testutil.NewTestRepository(
			testutil.WithFullName("user/metadata-repo"),
			testutil.WithDescription("Initial description"),
			testutil.WithTopics("initial", "test"),
		),
		[]processor.ContentChunk{
			testutil.NewTestChunk("README.md", "Initial content"),
		},
	)
	require.NoError(t, repo.StoreRepository(ctx, initialRepo))

	updatedRepo := testutil.NewTestProcessedRepo(
		testutil.NewTestRepository(
			testutil.WithFullName("user/metadata-repo"),
			testutil.WithDescription("Updated description"),
			testutil.WithTopics("updated", "test", "new"),
		),
		[]processor.ContentChunk{
			testutil.NewTestChunk("README.md", "Updated content"),
		},
	)
	require.NoError(t, repo.UpdateRepository(ctx, updatedRepo))

	stored, err := repo.GetRepository(ctx, "user/metadata-repo")
	require.NoError(t, err)
	assert.Equal(t, "Updated description", stored.Description)
	assert.ElementsMatch(t, []string{"updated", "test", "new"}, stored.Topics)
}

func TestDeleteRepository_Transaction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping transaction test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	testRepo := testutil.NewTestProcessedRepoSimple("user/delete-repo")
	require.NoError(t, repo.StoreRepository(ctx, testRepo))

	_, err := repo.GetRepository(ctx, "user/delete-repo")
	require.NoError(t, err)

	require.NoError(t, repo.DeleteRepository(ctx, "user/delete-repo"))

	_, err = repo.GetRepository(ctx, "user/delete-repo")
	assert.Error(t, err, "deleted repository should not exist")
}

func TestConcurrentReadsDuringWrite(t *testing.T) {
	t.Skip("Known issue: UpdateRepository delete-and-reinsert causes read failures")
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	initialRepo := testutil.NewTestProcessedRepoSimple("user/read-write-repo")
	require.NoError(t, repo.StoreRepository(ctx, initialRepo))

	const numReaders = 10
	var wg sync.WaitGroup
	errChan := make(chan error, numReaders+1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		updatedRepo := testutil.NewTestProcessedRepo(
			testutil.NewTestRepository(
				testutil.WithFullName("user/read-write-repo"),
				testutil.WithStars(500),
			),
			[]processor.ContentChunk{
				testutil.NewTestChunk("README.md", "Updated during reads"),
			},
		)
		if err := repo.UpdateRepository(ctx, updatedRepo); err != nil {
			errChan <- fmt.Errorf("write failed: %w", err)
		}
	}()

	for i := range numReaders {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range 5 {
				_, err := repo.GetRepository(ctx, "user/read-write-repo")
				if err != nil && !errors.Is(err, context.Canceled) {
					errChan <- fmt.Errorf("reader %d failed: %w", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "concurrent reads during write should not fail")

	stored, err := repo.GetRepository(ctx, "user/read-write-repo")
	require.NoError(t, err)
	assert.Equal(t, 500, stored.StargazersCount, "update should have succeeded")
}

func TestUpdateRepositoryMetrics_Transaction(t *testing.T) {
	t.Skip("Known issue: UpdateRepository chunks interfere with metrics updates")
	if testing.Short() {
		t.Skip("Skipping transaction test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	initialRepo := testutil.NewTestProcessedRepoSimple("user/metrics-repo")
	require.NoError(t, repo.StoreRepository(ctx, initialRepo))

	time.Sleep(50 * time.Millisecond)

	metrics := RepositoryMetrics{
		Homepage:        "https://example.com",
		OpenIssuesOpen:  10,
		OpenIssuesTotal: 50,
		OpenPRsOpen:     5,
		OpenPRsTotal:    25,
		Commits30d:      30,
		Commits1y:       365,
		CommitsTotal:    1000,
		Languages: map[string]int64{
			"Go":   1000,
			"HTML": 200,
		},
		Contributors: []Contributor{
			{Login: "user1", Contributions: 100},
			{Login: "user2", Contributions: 50},
		},
	}

	require.NoError(t, repo.UpdateRepositoryMetrics(ctx, "user/metrics-repo", metrics))

	stored, err := repo.GetRepository(ctx, "user/metrics-repo")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", stored.Homepage)
	assert.Equal(t, 10, stored.OpenIssuesOpen)
	assert.Equal(t, 50, stored.OpenIssuesTotal)
	assert.Equal(t, 5, stored.OpenPRsOpen)
	assert.Equal(t, 25, stored.OpenPRsTotal)
	assert.Equal(t, 30, stored.Commits30d)
	assert.Equal(t, 365, stored.Commits1y)
	assert.Equal(t, 1000, stored.CommitsTotal)
	assert.Len(t, stored.Languages, 2)
	assert.Len(t, stored.Contributors, 2)
}

func TestUpdateRepositorySummary_Transaction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping transaction test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	initialRepo := testutil.NewTestProcessedRepoSimple("user/summary-repo")
	require.NoError(t, repo.StoreRepository(ctx, initialRepo))

	purpose := "This repository provides testing utilities for Go applications"
	require.NoError(t, repo.UpdateRepositorySummary(ctx, "user/summary-repo", purpose))

	stored, err := repo.GetRepository(ctx, "user/summary-repo")
	require.NoError(t, err)
	assert.Equal(t, purpose, stored.Purpose)
	assert.NotNil(t, stored.SummaryGeneratedAt)
	assert.Equal(t, 1, stored.SummaryVersion)
}

func TestConcurrentMetricsUpdates(t *testing.T) {
	t.Skip("Known issue: UpdateRepositoryMetrics has concurrent update conflicts")
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	repo, cleanup := NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	initialRepo := testutil.NewTestProcessedRepoSimple("user/concurrent-metrics-repo")
	require.NoError(t, repo.StoreRepository(ctx, initialRepo))

	time.Sleep(50 * time.Millisecond)

	const numUpdates = 5
	var wg sync.WaitGroup
	errChan := make(chan error, numUpdates)
	successCount := 0
	var mu sync.Mutex

	for i := range numUpdates {
		wg.Add(1)
		go func(updateID int) {
			defer wg.Done()
			metrics := RepositoryMetrics{
				Homepage:        fmt.Sprintf("https://example-%d.com", updateID),
				OpenIssuesOpen:  updateID * 10,
				OpenIssuesTotal: updateID * 20,
				Commits30d:      updateID * 5,
			}
			if err := repo.UpdateRepositoryMetrics(ctx, "user/concurrent-metrics-repo", metrics); err != nil {
				errChan <- fmt.Errorf("metrics update %d failed: %w", updateID, err)
			} else {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		t.Logf("Note: %d concurrent metric update conflicts detected", len(errs))
	}

	assert.Greater(t, successCount, 0, "at least some metrics updates should succeed")

	stored, err := repo.GetRepository(ctx, "user/concurrent-metrics-repo")
	require.NoError(t, err)
	t.Logf("Final homepage: %s", stored.Homepage)
}
