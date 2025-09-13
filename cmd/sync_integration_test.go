package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/username/gh-star-search/internal/config"
	"github.com/username/gh-star-search/internal/github"
	"github.com/username/gh-star-search/internal/processor"
	"github.com/username/gh-star-search/internal/storage"
)

// TestSyncIntegration tests the complete sync workflow end-to-end
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

	mockLLM := &MockLLMService{
		responses: map[string]*processor.SummaryResponse{
			"default": {
				Purpose:      "A comprehensive test repository",
				Technologies: []string{"Go", "CLI", "Testing"},
				UseCases:     []string{"Command line tools", "Testing utilities"},
				Features:     []string{"Cross-platform", "Easy to use", "Well documented"},
				Installation: "go install github.com/user/repo",
				Usage:        "repo [command] [flags]",
				Confidence:   0.9,
			},
		},
	}

	processorService := processor.NewService(mockGitHub, mockLLM)

	syncService := &SyncService{
		githubClient: mockGitHub,
		processor:    processorService,
		storage:      repo,
		config:       config.DefaultConfig(),
		verbose:      true,
	}

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

	mockLLM := &MockLLMService{
		responses: map[string]*processor.SummaryResponse{
			"default": {
				Purpose:      "A repository for testing specific sync functionality",
				Technologies: []string{"Go", "Testing"},
				UseCases:     []string{"Unit testing", "Specific sync testing"},
				Features:     []string{"Focused testing", "Isolated functionality"},
				Installation: "go get github.com/user/specific-repo",
				Usage:        "specific-repo test",
				Confidence:   0.95,
			},
		},
	}

	processorService := processor.NewService(mockGitHub, mockLLM)

	syncService := &SyncService{
		githubClient: mockGitHub,
		processor:    processorService,
		storage:      repo,
		config:       config.DefaultConfig(),
		verbose:      true,
	}

	// Test syncing specific repository
	err = syncService.syncSpecificRepository(ctx, "user/specific-repo", false)
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
	err = syncService.syncSpecificRepository(ctx, "user/non-existent", false)
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
			"user/error-repo": fmt.Errorf("simulated API error"),
		},
	}

	mockLLM := &MockLLMService{}
	processorService := processor.NewService(mockGitHub, mockLLM)

	syncService := &SyncService{
		githubClient: mockGitHub,
		processor:    processorService,
		storage:      repo,
		config:       config.DefaultConfig(),
		verbose:      true,
	}

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