package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleking/gh-star-search/internal/config"
	"github.com/kyleking/gh-star-search/internal/github"
	"github.com/kyleking/gh-star-search/internal/processor"
	"github.com/kyleking/gh-star-search/internal/storage"
)

// createTestSyncService creates a sync service for testing
func createTestSyncService(githubClient github.Client, processor processor.Service, storage storage.Repository) *SyncService {
	return &SyncService{
		githubClient: githubClient,
		processor:    processor,
		storage:      storage,
		config:       config.DefaultConfig(),
		verbose:      true,
	}
}

// TestSyncIntegration tests the complete sync workflow end-to-end with enhanced change detection
func TestSyncIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database
	tempDir, err := os.MkdirTemp("", "sync_integration_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Initialize storage
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()
	if err := repo.Initialize(ctx); err != nil {
		t.Fatal(err)
	}

	// Create comprehensive mock data
	mockGitHub := &MockGitHubClient{
		starredRepos: []github.Repository{
			{
				FullName:        "user/active-repo",
				Description:     "An active repository",
				Language:        "Go",
				StargazersCount: 100,
				ForksCount:      10,
				Size:            2048,
				CreatedAt:       time.Now().Add(-365 * 24 * time.Hour),
				UpdatedAt:       time.Now().Add(-1 * time.Hour),
				Topics:          []string{"go", "cli", "tool"},
				License: &github.License{
					Key:    "mit",
					Name:   "MIT License",
					SPDXID: "MIT",
				},
			},
			{
				FullName:        "user/new-repo",
				Description:     "A newly starred repository",
				Language:        "Python",
				StargazersCount: 50,
				ForksCount:      5,
				Size:            1024,
				CreatedAt:       time.Now().Add(-30 * 24 * time.Hour),
				UpdatedAt:       time.Now().Add(-2 * time.Hour),
				Topics:          []string{"python", "web", "api"},
				License: &github.License{
					Key:    "apache-2.0",
					Name:   "Apache License 2.0",
					SPDXID: "Apache-2.0",
				},
			},
			{
				FullName:        "user/updated-repo",
				Description:     "A repository with updates",
				Language:        "JavaScript",
				StargazersCount: 200, // This will be different from existing
				ForksCount:      20,
				Size:            4096,
				CreatedAt:       time.Now().Add(-180 * 24 * time.Hour),
				UpdatedAt:       time.Now().Add(-30 * time.Minute),
				Topics:          []string{"javascript", "frontend", "react"},
			},
		},
		content: map[string][]github.Content{
			"user/active-repo": {
				{
					Path:     "README.md",
					Type:     "file",
					Content:  "IyBBY3RpdmUgUmVwb3NpdG9yeQoKVGhpcyBpcyBhbiBhY3RpdmUgR28gcmVwb3NpdG9yeSBmb3IgQ0xJIHRvb2xzLg==", // base64: "# Active Repository\n\nThis is an active Go repository for CLI tools."
					Size:     50,
					Encoding: "base64",
				},
				{
					Path:     "main.go",
					Type:     "file",
					Content:  "cGFja2FnZSBtYWluCgpmdW5jIG1haW4oKSB7CiAgICBwcmludGxuKCJIZWxsbyBXb3JsZCIpCn0=", // base64: "package main\n\nfunc main() {\n    println(\"Hello World\")\n}"
					Size:     45,
					Encoding: "base64",
				},
			},
			"user/new-repo": {
				{
					Path:     "README.md",
					Type:     "file",
					Content:  "IyBOZXcgUmVwb3NpdG9yeQoKQSBuZXcgUHl0aG9uIHdlYiBBUEkgcHJvamVjdC4=", // base64: "# New Repository\n\nA new Python web API project."
					Size:     35,
					Encoding: "base64",
				},
				{
					Path:     "requirements.txt",
					Type:     "file",
					Content:  "Zmxhc2s9PTIuMC4xCnJlcXVlc3RzPT0yLjI4LjE=", // base64: "flask==2.0.1\nrequests==2.28.1"
					Size:     25,
					Encoding: "base64",
				},
			},
			"user/updated-repo": {
				{
					Path:     "README.md",
					Type:     "file",
					Content:  "IyBVcGRhdGVkIFJlcG9zaXRvcnkKClJlY2VudGx5IHVwZGF0ZWQgSmF2YVNjcmlwdCBwcm9qZWN0Lg==", // base64: "# Updated Repository\n\nRecently updated JavaScript project."
					Size:     40,
					Encoding: "base64",
				},
			},
		},
	}

	processorService := processor.NewService(mockGitHub)
	syncService := createTestSyncService(mockGitHub, processorService, repo)

	// Step 1: Perform initial sync (all repositories should be new)
	t.Log("Step 1: Initial sync")

	err = syncService.performFullSync(ctx, 2, false)
	if err != nil {
		t.Fatalf("Initial sync failed: %v", err)
	}

	// Verify all repositories were stored
	stats, err := repo.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalRepositories != 3 {
		t.Errorf("Expected 3 repositories after initial sync, got %d", stats.TotalRepositories)
	}

	// Verify specific repositories
	activeRepo, err := repo.GetRepository(ctx, "user/active-repo")
	if err != nil {
		t.Fatalf("Failed to get active-repo: %v", err)
	}

	if activeRepo.StargazersCount != 100 {
		t.Errorf("Expected 100 stars for active-repo, got %d", activeRepo.StargazersCount)
	}

	if len(activeRepo.Chunks) == 0 {
		t.Error("Expected content chunks for active-repo")
	}

	// Step 2: Simulate repository changes and perform incremental sync
	t.Log("Step 2: Incremental sync with changes")

	// Update the mock data to simulate changes
	// Create updated version of updated-repo with new star count
	updatedRepo := github.Repository{
		FullName:        "user/updated-repo",
		Description:     "A repository with updates",
		Language:        "JavaScript",
		StargazersCount: 250, // Increased from 200
		ForksCount:      25,  // Increased from 20
		Size:            4096,
		CreatedAt:       time.Now().Add(-180 * 24 * time.Hour),
		UpdatedAt:       time.Now(), // Recently updated
		Topics:          []string{"javascript", "frontend", "react"},
	}

	// Create new repository
	brandNewRepo := github.Repository{
		FullName:        "user/brand-new-repo",
		Description:     "A brand new repository",
		Language:        "Rust",
		StargazersCount: 75,
		ForksCount:      8,
		Size:            1536,
		CreatedAt:       time.Now().Add(-7 * 24 * time.Hour),
		UpdatedAt:       time.Now().Add(-1 * time.Hour),
		Topics:          []string{"rust", "systems", "performance"},
	}

	// Update the starred repos list (remove new-repo, update updated-repo, add brand-new-repo)
	mockGitHub.starredRepos = []github.Repository{
		mockGitHub.starredRepos[0], // active-repo (unchanged)
		updatedRepo,                // updated-repo (with changes)
		brandNewRepo,               // brand-new-repo (new)
	}

	// Add content for the new repository
	mockGitHub.content["user/brand-new-repo"] = []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  "IyBCcmFuZCBOZXcgUmVwb3NpdG9yeQoKQSBmYXN0IFJ1c3QgYXBwbGljYXRpb24u", // base64: "# Brand New Repository\n\nA fast Rust application."
			Size:     30,
			Encoding: "base64",
		},
	}

	// Perform incremental sync
	err = syncService.performFullSync(ctx, 2, false)
	if err != nil {
		t.Fatalf("Incremental sync failed: %v", err)
	}

	// Debug: Check what the mock data looks like
	for _, repo := range mockGitHub.starredRepos {
		if repo.FullName == "user/updated-repo" {
			t.Logf("Mock updated-repo has %d stars", repo.StargazersCount)
		}
	}

	// Wait a moment to ensure any async operations complete
	time.Sleep(100 * time.Millisecond)

	// Verify the changes
	stats, err = repo.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats after incremental sync: %v", err)
	}

	if stats.TotalRepositories != 3 {
		t.Errorf("Expected 3 repositories after incremental sync, got %d", stats.TotalRepositories)
	}

	// Verify updated repository has new star count
	storedUpdatedRepo, err := repo.GetRepository(ctx, "user/updated-repo")
	if err != nil {
		t.Fatalf("Failed to get updated-repo: %v", err)
	}

	if storedUpdatedRepo.StargazersCount != 250 {
		t.Errorf("Expected 250 stars for updated-repo, got %d", storedUpdatedRepo.StargazersCount)
	}

	// Verify new repository was added
	newRepo, err := repo.GetRepository(ctx, "user/brand-new-repo")
	if err != nil {
		t.Fatalf("Failed to get brand-new-repo: %v", err)
	}

	if newRepo.Language != "Rust" {
		t.Errorf("Expected Rust language for brand-new-repo, got %s", newRepo.Language)
	}

	// Verify removed repository is gone
	_, err = repo.GetRepository(ctx, "user/new-repo")
	if err == nil {
		t.Error("Expected user/new-repo to be removed, but it still exists")
	}

	// Step 3: Test force sync (should reprocess all repositories)
	t.Log("Step 3: Force sync")

	err = syncService.performFullSync(ctx, 2, true)
	if err != nil {
		t.Fatalf("Force sync failed: %v", err)
	}

	// Verify repositories still exist after force sync
	stats, err = repo.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats after force sync: %v", err)
	}

	if stats.TotalRepositories != 3 {
		t.Errorf("Expected 3 repositories after force sync, got %d", stats.TotalRepositories)
	}

	// Step 4: Test content change detection with hash comparison
	t.Log("Step 4: Content change detection")

	// Update content for active-repo to test content change detection
	mockGitHub.content["user/active-repo"] = []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  "IyBBY3RpdmUgUmVwb3NpdG9yeSAoVXBkYXRlZCkKClRoaXMgaXMgYW4gdXBkYXRlZCBHbyByZXBvc2l0b3J5IGZvciBDTEkgdG9vbHMu", // base64: "# Active Repository (Updated)\n\nThis is an updated Go repository for CLI tools."
			Size:     60,
			Encoding: "base64",
		},
		{
			Path:     "main.go",
			Type:     "file",
			Content:  "cGFja2FnZSBtYWluCgpmdW5jIG1haW4oKSB7CiAgICBwcmludGxuKCJIZWxsbyBVcGRhdGVkIFdvcmxkIikKfQ==", // base64: "package main\n\nfunc main() {\n    println(\"Hello Updated World\")\n}"
			Size:     50,
			Encoding: "base64",
		},
	}

	// Update the repository's UpdatedAt timestamp to trigger processing
	for i, repo := range mockGitHub.starredRepos {
		if repo.FullName == "user/active-repo" {
			mockGitHub.starredRepos[i].UpdatedAt = time.Now()
			break
		}
	}

	// Get the current content hash
	oldActiveRepo, err := repo.GetRepository(ctx, "user/active-repo")
	if err != nil {
		t.Fatalf("Failed to get active-repo before content update: %v", err)
	}

	oldContentHash := oldActiveRepo.ContentHash

	// Perform sync to detect content changes
	err = syncService.performFullSync(ctx, 2, false)
	if err != nil {
		t.Fatalf("Content change sync failed: %v", err)
	}

	// Verify content hash changed
	updatedActiveRepo, err := repo.GetRepository(ctx, "user/active-repo")
	if err != nil {
		t.Fatalf("Failed to get active-repo after content update: %v", err)
	}

	if updatedActiveRepo.ContentHash == oldContentHash {
		t.Error("Expected content hash to change after content update")
	}

	t.Logf("Content hash changed: %s → %s", oldContentHash[:8], updatedActiveRepo.ContentHash[:8])

	t.Log("Integration test completed successfully")
}

// TestSyncSpecificRepository tests syncing a single repository
func TestSyncSpecificRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database
	tempDir, err := os.MkdirTemp("", "sync_specific_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Initialize storage
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()
	if err := repo.Initialize(ctx); err != nil {
		t.Fatal(err)
	}

	// Create mock data for specific repository
	mockGitHub := &MockGitHubClient{
		starredRepos: []github.Repository{
			{
				FullName:        "user/specific-repo",
				Description:     "A specific repository for testing",
				Language:        "Go",
				StargazersCount: 42,
				ForksCount:      7,
				Size:            1024,
				CreatedAt:       time.Now().Add(-60 * 24 * time.Hour),
				UpdatedAt:       time.Now().Add(-2 * time.Hour),
				Topics:          []string{"go", "testing", "specific"},
			},
		},
		content: map[string][]github.Content{
			"user/specific-repo": {
				{
					Path:     "README.md",
					Type:     "file",
					Content:  "IyBTcGVjaWZpYyBSZXBvc2l0b3J5CgpUaGlzIGlzIGEgc3BlY2lmaWMgcmVwb3NpdG9yeSBmb3IgdGVzdGluZy4=", // base64: "# Specific Repository\n\nThis is a specific repository for testing."
					Size:     45,
					Encoding: "base64",
				},
			},
		},
	}

	processorService := processor.NewService(mockGitHub)
	syncService := createTestSyncService(mockGitHub, processorService, repo)

	// Test syncing specific repository
	err = syncService.syncSpecificRepository(ctx, "user/specific-repo")
	if err != nil {
		t.Fatalf("Failed to sync specific repository: %v", err)
	}

	// Verify repository was stored
	stored, err := repo.GetRepository(ctx, "user/specific-repo")
	if err != nil {
		t.Fatalf("Failed to retrieve specific repository: %v", err)
	}

	if stored.FullName != "user/specific-repo" {
		t.Errorf("Expected FullName 'user/specific-repo', got '%s'", stored.FullName)
	}

	if stored.StargazersCount != 42 {
		t.Errorf("Expected StargazersCount 42, got %d", stored.StargazersCount)
	}

	if len(stored.Chunks) == 0 {
		t.Error("Expected content chunks to be stored")
	}

	// Test syncing non-existent repository
	err = syncService.syncSpecificRepository(ctx, "user/non-existent")
	if err == nil {
		t.Error("Expected error when syncing non-existent repository")
	}
}

// TestSyncErrorHandling tests error handling during sync operations
func TestSyncErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database
	tempDir, err := os.MkdirTemp("", "sync_error_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Initialize storage
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()
	if err := repo.Initialize(ctx); err != nil {
		t.Fatal(err)
	}

	// Create mock with errors
	mockGitHub := &MockGitHubClient{
		starredRepos: []github.Repository{
			{
				FullName:        "user/good-repo",
				Description:     "A working repository",
				Language:        "Go",
				StargazersCount: 10,
				UpdatedAt:       time.Now(),
			},
			{
				FullName:        "user/error-repo",
				Description:     "A repository that will cause errors",
				Language:        "Go",
				StargazersCount: 20,
				UpdatedAt:       time.Now(),
			},
		},
		content: map[string][]github.Content{
			"user/good-repo": {
				{
					Path:     "README.md",
					Type:     "file",
					Content:  "R29vZCByZXBvc2l0b3J5", // base64: "Good repository"
					Size:     15,
					Encoding: "base64",
				},
			},
		},
		errors: map[string]error{
			"user/error-repo": errors.New("simulated API error"),
		},
	}

	processorService := processor.NewService(mockGitHub)
	syncService := createTestSyncService(mockGitHub, processorService, repo)

	// Perform sync with errors
	err = syncService.performFullSync(ctx, 2, false)
	// Should not fail completely due to one repository error
	if err != nil {
		t.Fatalf("Sync should handle individual repository errors gracefully: %v", err)
	}

	// Verify good repository was processed
	stored, err := repo.GetRepository(ctx, "user/good-repo")
	if err != nil {
		t.Fatalf("Good repository should have been processed: %v", err)
	}

	if stored.FullName != "user/good-repo" {
		t.Errorf("Expected good-repo to be stored")
	}

	// Verify error repository was not processed
	_, err = repo.GetRepository(ctx, "user/error-repo")
	if err == nil {
		t.Error("Error repository should not have been stored")
	}
}

// TestSyncIncrementalUpdates tests incremental sync with various types of changes
func TestSyncIncrementalUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database
	tempDir, err := os.MkdirTemp("", "sync_incremental_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Initialize storage
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()
	if err := repo.Initialize(ctx); err != nil {
		t.Fatal(err)
	}

	// Create initial mock data
	mockGitHub := &MockGitHubClient{
		starredRepos: []github.Repository{
			{
				FullName:        "user/test-repo",
				Description:     "Original description",
				Language:        "Go",
				StargazersCount: 100,
				ForksCount:      10,
				Size:            1024,
				CreatedAt:       time.Now().Add(-365 * 24 * time.Hour),
				UpdatedAt:       time.Now().Add(-2 * time.Hour),
				Topics:          []string{"go", "test"},
				License: &github.License{
					Key:    "mit",
					Name:   "MIT License",
					SPDXID: "MIT",
				},
			},
		},
		content: map[string][]github.Content{
			"user/test-repo": {
				{
					Path:     "README.md",
					Type:     "file",
					Content:  "IyBUZXN0IFJlcG9zaXRvcnkKCk9yaWdpbmFsIGNvbnRlbnQ=", // base64: "# Test Repository\n\nOriginal content"
					Size:     30,
					Encoding: "base64",
				},
			},
		},
	}

	processorService := processor.NewService(mockGitHub)
	syncService := createTestSyncService(mockGitHub, processorService, repo)

	// Step 1: Initial sync
	t.Log("Step 1: Initial sync")

	err = syncService.performFullSync(ctx, 1, false)
	if err != nil {
		t.Fatalf("Initial sync failed: %v", err)
	}

	// Get initial state
	initialRepo, err := repo.GetRepository(ctx, "user/test-repo")
	if err != nil {
		t.Fatalf("Failed to get initial repository: %v", err)
	}

	// Step 2: Test metadata-only changes
	t.Log("Step 2: Metadata-only changes")

	mockGitHub.starredRepos[0].StargazersCount = 150
	mockGitHub.starredRepos[0].ForksCount = 15
	mockGitHub.starredRepos[0].Description = "Updated description"
	mockGitHub.starredRepos[0].Topics = []string{"go", "test", "updated"}

	err = syncService.performFullSync(ctx, 1, false)
	if err != nil {
		t.Fatalf("Metadata update sync failed: %v", err)
	}

	// Verify metadata changes
	metadataUpdatedRepo, err := repo.GetRepository(ctx, "user/test-repo")
	if err != nil {
		t.Fatalf("Failed to get metadata updated repository: %v", err)
	}

	if metadataUpdatedRepo.StargazersCount != 150 {
		t.Errorf("Expected 150 stars, got %d", metadataUpdatedRepo.StargazersCount)
	}

	if metadataUpdatedRepo.Description != "Updated description" {
		t.Errorf("Expected updated description, got %s", metadataUpdatedRepo.Description)
	}

	if metadataUpdatedRepo.ContentHash != initialRepo.ContentHash {
		t.Error("Content hash should not change for metadata-only updates")
	}

	// Step 3: Test content-only changes (simulate GitHub repository update)
	t.Log("Step 3: Content-only changes")

	// Update both content AND UpdatedAt to simulate a real GitHub repository update
	mockGitHub.content["user/test-repo"] = []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  "IyBUZXN0IFJlcG9zaXRvcnkKClVwZGF0ZWQgY29udGVudA==", // base64: "# Test Repository\n\nUpdated content"
			Size:     32,
			Encoding: "base64",
		},
	}

	// Update the repository's UpdatedAt timestamp to trigger processing
	mockGitHub.starredRepos[0].UpdatedAt = time.Now()

	err = syncService.performFullSync(ctx, 1, false)
	if err != nil {
		t.Fatalf("Content update sync failed: %v", err)
	}

	// Verify content changes
	contentUpdatedRepo, err := repo.GetRepository(ctx, "user/test-repo")
	if err != nil {
		t.Fatalf("Failed to get content updated repository: %v", err)
	}

	if contentUpdatedRepo.ContentHash == metadataUpdatedRepo.ContentHash {
		t.Error("Content hash should change for content updates")
	}

	// Step 4: Test no changes (should skip)
	t.Log("Step 4: No changes (should skip)")

	err = syncService.performFullSync(ctx, 1, false)
	if err != nil {
		t.Fatalf("No-change sync failed: %v", err)
	}

	// Verify no changes
	noChangeRepo, err := repo.GetRepository(ctx, "user/test-repo")
	if err != nil {
		t.Fatalf("Failed to get no-change repository: %v", err)
	}

	if noChangeRepo.ContentHash != contentUpdatedRepo.ContentHash {
		t.Error("Content hash should not change when no updates are made")
	}
	// Note: LastSynced is only updated when repositories are actually processed
	// Since no changes were detected, the repository wasn't processed, so LastSynced remains the same
	// This is correct behavior - we only update LastSynced when we actually sync the repository

	// Step 5: Test force sync
	t.Log("Step 5: Force sync")

	// Add a small delay to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	err = syncService.performFullSync(ctx, 1, true)
	if err != nil {
		t.Fatalf("Force sync failed: %v", err)
	}

	// Verify force sync updated LastSynced
	forceSyncRepo, err := repo.GetRepository(ctx, "user/test-repo")
	if err != nil {
		t.Fatalf("Failed to get force sync repository: %v", err)
	}

	t.Logf("Content updated LastSynced: %v", contentUpdatedRepo.LastSynced)
	t.Logf("Force sync LastSynced: %v", forceSyncRepo.LastSynced)

	if !forceSyncRepo.LastSynced.After(contentUpdatedRepo.LastSynced) {
		t.Error("Force sync should update LastSynced timestamp")
	}

	t.Log("Incremental updates test completed successfully")
}

// TestSyncProgressTracking tests progress indicators and batch processing
func TestSyncProgressTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database
	tempDir, err := os.MkdirTemp("", "sync_progress_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Initialize storage
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()
	if err := repo.Initialize(ctx); err != nil {
		t.Fatal(err)
	}

	// Create mock data with multiple repositories for batch testing
	repos := make([]github.Repository, 7) // Test with 7 repos to test batching
	content := make(map[string][]github.Content)

	for i := range 7 {
		repoName := fmt.Sprintf("user/repo%d", i+1)
		repos[i] = github.Repository{
			FullName:        repoName,
			Description:     fmt.Sprintf("Test repository %d", i+1),
			Language:        "Go",
			StargazersCount: (i + 1) * 10,
			ForksCount:      i + 1,
			Size:            (i + 1) * 100,
			CreatedAt:       time.Now().Add(-time.Duration(i+1) * 24 * time.Hour),
			UpdatedAt:       time.Now().Add(-time.Duration(i+1) * time.Hour),
			Topics:          []string{"go", fmt.Sprintf("test%d", i+1)},
		}

		// Create base64 encoded content for each repo
		readmeContent := fmt.Sprintf("# Repository %d\n\nTest content for repo %d", i+1, i+1)
		encodedContent := base64.StdEncoding.EncodeToString([]byte(readmeContent))

		content[repoName] = []github.Content{
			{
				Path:     "README.md",
				Type:     "file",
				Content:  encodedContent,
				Size:     20 + i,
				Encoding: "base64",
			},
		}
	}

	mockGitHub := &MockGitHubClient{
		starredRepos: repos,
		content:      content,
	}

	processorService := processor.NewService(mockGitHub)
	syncService := createTestSyncService(mockGitHub, processorService, repo)
	syncService.verbose = false // Disable verbose to test progress indicators

	// Test batch processing with batch size of 3
	t.Log("Testing batch processing with 7 repositories (batch size: 3)")

	err = syncService.performFullSync(ctx, 3, false)
	if err != nil {
		t.Fatalf("Batch sync failed: %v", err)
	}

	// Verify all repositories were processed
	stats, err := repo.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalRepositories != 7 {
		t.Errorf("Expected 7 repositories, got %d", stats.TotalRepositories)
	}

	// Verify each repository was stored correctly
	for i := range 7 {
		repoName := fmt.Sprintf("user/repo%d", i+1)
		stored, err := repo.GetRepository(ctx, repoName)

		if err != nil {
			t.Errorf("Repository %s was not stored: %v", repoName, err)
		} else if stored.StargazersCount != (i+1)*10 {
			t.Errorf("Repository %s has wrong star count: expected %d, got %d", repoName, (i+1)*10, stored.StargazersCount)
		}
	}

	t.Log("Progress tracking test completed successfully")
}
