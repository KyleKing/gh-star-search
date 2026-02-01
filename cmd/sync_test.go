package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KyleKing/gh-star-search/internal/config"
	"github.com/KyleKing/gh-star-search/internal/github"
	"github.com/KyleKing/gh-star-search/internal/processor"
	"github.com/KyleKing/gh-star-search/internal/storage"
)

// MockGitHubClient implements github.Client for testing
type MockGitHubClient struct {
	starredRepos []github.Repository
	content      map[string][]github.Content
	metadata     map[string]*github.Metadata
	errors       map[string]error
}

func (m *MockGitHubClient) GetStarredRepos(
	_ context.Context,
	_ string,
) ([]github.Repository, error) {
	if err, exists := m.errors["starred"]; exists {
		return nil, err
	}

	return m.starredRepos, nil
}

func (m *MockGitHubClient) GetRepositoryContent(
	_ context.Context,
	repo github.Repository,
	_ []string,
) ([]github.Content, error) {
	if err, exists := m.errors[repo.FullName]; exists {
		return nil, err
	}

	if content, exists := m.content[repo.FullName]; exists {
		return content, nil
	}

	return []github.Content{}, nil
}

func (m *MockGitHubClient) GetRepositoryMetadata(
	_ context.Context,
	repo github.Repository,
) (*github.Metadata, error) {
	if err, exists := m.errors[repo.FullName+"_metadata"]; exists {
		return nil, err
	}

	if metadata, exists := m.metadata[repo.FullName]; exists {
		return metadata, nil
	}

	return &github.Metadata{}, nil
}

func (m *MockGitHubClient) GetContributors(
	_ context.Context,
	fullName string,
	_ int,
) ([]github.Contributor, error) {
	if err, exists := m.errors[fullName+"_contributors"]; exists {
		return nil, err
	}

	return []github.Contributor{{Login: "testuser", Contributions: 10}}, nil
}

func (m *MockGitHubClient) GetTopics(_ context.Context, fullName string) ([]string, error) {
	if err, exists := m.errors[fullName+"_topics"]; exists {
		return nil, err
	}

	return []string{"test", "mock"}, nil
}

func (m *MockGitHubClient) GetLanguages(
	_ context.Context,
	fullName string,
) (map[string]int64, error) {
	if err, exists := m.errors[fullName+"_languages"]; exists {
		return nil, err
	}

	return map[string]int64{"Go": 12345}, nil
}

func (m *MockGitHubClient) GetCommitActivity(
	_ context.Context,
	fullName string,
) (*github.CommitActivity, error) {
	if err, exists := m.errors[fullName+"_commits"]; exists {
		return nil, err
	}

	return &github.CommitActivity{
		Weeks: []github.WeeklyCommits{{Week: 1640995200, Commits: 5}},
		Total: 5,
	}, nil
}

func (m *MockGitHubClient) GetPullCounts(
	_ context.Context,
	fullName string,
) (int, int, error) {
	if err, exists := m.errors[fullName+"_prs"]; exists {
		return 0, 0, err
	}

	return 2, 10, nil
}

func (m *MockGitHubClient) GetIssueCounts(
	_ context.Context,
	fullName string,
) (int, int, error) {
	if err, exists := m.errors[fullName+"_issues"]; exists {
		return 0, 0, err
	}

	return 3, 15, nil
}

func (m *MockGitHubClient) GetHomepageText(_ context.Context, url string) (string, error) {
	if err, exists := m.errors["homepage_"+url]; exists {
		return "", err
	}

	return "Mock homepage text", nil
}

func TestSyncService_DetermineSyncOperations(t *testing.T) {
	baseTime := time.Now().Add(-2 * time.Hour)

	// Create test repositories
	starredRepos := []github.Repository{
		{
			FullName:        "user/repo1",
			UpdatedAt:       baseTime.Add(-30 * time.Minute), // Older than LastSynced
			StargazersCount: 100,
			ForksCount:      10,
			Size:            1000,
		},
		{
			FullName:        "user/repo2",
			UpdatedAt:       baseTime.Add(-30 * time.Minute), // Older than LastSynced
			StargazersCount: 200,                             // Different star count - needs update
			ForksCount:      20,
			Size:            2000,
		},
		{
			FullName:        "user/repo3",
			UpdatedAt:       time.Now(),
			StargazersCount: 300,
			ForksCount:      30,
			Size:            3000,
		},
	}

	existingRepos := map[string]*storage.StoredRepo{
		"user/repo1": {
			FullName:        "user/repo1",
			StargazersCount: 100,
			ForksCount:      10,
			SizeKB:          1000,
			LastSynced:      baseTime,
		},
		"user/repo2": {
			FullName:        "user/repo2",
			StargazersCount: 150, // Different star count - needs update
			ForksCount:      20,
			SizeKB:          2000,
			LastSynced:      baseTime,
		},
		"user/old-repo": {
			FullName:   "user/old-repo",
			LastSynced: baseTime,
		},
	}

	syncService := &SyncService{verbose: false}
	operations := syncService.determineSyncOperations(starredRepos, existingRepos, false)

	// Verify operations
	if len(operations.toAdd) != 1 {
		t.Errorf("Expected 1 repository to add, got %d", len(operations.toAdd))
	}

	if operations.toAdd[0].FullName != "user/repo3" {
		t.Errorf("Expected to add user/repo3, got %s", operations.toAdd[0].FullName)
	}

	if len(operations.toUpdate) != 1 {
		t.Errorf("Expected 1 repository to update, got %d", len(operations.toUpdate))
	}

	if operations.toUpdate[0].FullName != "user/repo2" {
		t.Errorf("Expected to update user/repo2, got %s", operations.toUpdate[0].FullName)
	}

	if len(operations.toRemove) != 1 {
		t.Errorf("Expected 1 repository to remove, got %d", len(operations.toRemove))
	}

	if operations.toRemove[0] != "user/old-repo" {
		t.Errorf("Expected to remove user/old-repo, got %s", operations.toRemove[0])
	}
}

func TestSyncService_NeedsUpdate(t *testing.T) {
	syncService := &SyncService{}

	baseTime := time.Now().Add(-2 * time.Hour)

	tests := []struct {
		name     string
		repo     github.Repository
		existing *storage.StoredRepo
		expected bool
	}{
		{
			name: "updated repository",
			repo: github.Repository{
				FullName:        "user/repo",
				UpdatedAt:       time.Now(),
				StargazersCount: 100,
				ForksCount:      10,
				Size:            1000,
			},
			existing: &storage.StoredRepo{
				FullName:        "user/repo",
				StargazersCount: 100,
				ForksCount:      10,
				SizeKB:          1000,
				LastSynced:      baseTime,
			},
			expected: true, // UpdatedAt is after LastSynced
		},
		{
			name: "different star count",
			repo: github.Repository{
				FullName:        "user/repo",
				UpdatedAt:       baseTime.Add(-1 * time.Hour),
				StargazersCount: 150,
				ForksCount:      10,
				Size:            1000,
			},
			existing: &storage.StoredRepo{
				FullName:        "user/repo",
				StargazersCount: 100,
				ForksCount:      10,
				SizeKB:          1000,
				LastSynced:      baseTime,
			},
			expected: true,
		},
		{
			name: "no changes",
			repo: github.Repository{
				FullName:        "user/repo",
				UpdatedAt:       baseTime.Add(-1 * time.Hour),
				StargazersCount: 100,
				ForksCount:      10,
				Size:            1000,
			},
			existing: &storage.StoredRepo{
				FullName:        "user/repo",
				StargazersCount: 100,
				ForksCount:      10,
				SizeKB:          1000,
				LastSynced:      baseTime,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncService.needsUpdate(tt.repo, tt.existing)
			if result != tt.expected {
				t.Errorf("needsUpdate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestSyncService_ProcessRepository(t *testing.T) {
	// Create temporary database
	tempDir, err := os.MkdirTemp("", "sync_test")
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

	// Create mock services
	mockGitHub := &MockGitHubClient{
		content: map[string][]github.Content{
			"user/test-repo": {
				{
					Path:     "README.md",
					Type:     "file",
					Content:  "VGhpcyBpcyBhIHRlc3QgcmVwb3NpdG9yeSBmb3IgdW5pdCB0ZXN0aW5nIHB1cnBvc2VzLiBJdCBjb250YWlucyB2YXJpb3VzIHRlc3QgZmlsZXMgYW5kIGV4YW1wbGVzIHRvIHZhbGlkYXRlIHRoZSBmdW5jdGlvbmFsaXR5IG9mIHRoZSBzeXN0ZW0u", // base64 encoded longer description
					Size:     150,
					Encoding: "base64",
				},
				{
					Path:     "go.mod",
					Type:     "file",
					Content:  "bW9kdWxlIGdpdGh1Yi5jb20va3lsZWtpbmcvZ2gtc3Rhci1zZWFyY2gKCm9vIDEuMjE=", // base64 encoded go.mod content
					Size:     60,
					Encoding: "base64",
				},
				{
					Path:     "package.json",
					Type:     "file",
					Content:  "ewogICJuYW1lIjogInRlc3QtcmVwbyIsCiAgInZlcnNpb24iOiAiMS4wLjAiLAogICJkZXNjcmlwdGlvbiI6ICJUZXN0IHJlcG8gd2l0aCBKYXZhU2NyaXB0IiwKICAibWFpbiI6ICJpbmRleC5qcyIKfQ==", // base64 encoded package.json content
					Size:     120,
					Encoding: "base64",
				},
			},
		},
	}

	processorService := processor.NewService(mockGitHub)

	syncService := &SyncService{
		githubClient: mockGitHub,
		processor:    processorService,
		storage:      repo,
		config:       func() *config.Config { cfg, _ := config.LoadConfig(); return cfg }(),
		verbose:      true,
	}

	// Test repository
	testRepo := github.Repository{
		FullName:        "user/test-repo",
		Description:     "A test repository",
		Language:        "Go",
		StargazersCount: 42,
		ForksCount:      5,
		Size:            1024,
		CreatedAt:       time.Now().Add(-30 * 24 * time.Hour),
		UpdatedAt:       time.Now().Add(-1 * time.Hour),
		Topics:          []string{"testing", "go"},
		License: &github.License{
			Key:    "mit",
			Name:   "MIT License",
			SPDXID: "MIT",
		},
	}

	// Process the repository
	err = syncService.processRepository(ctx, testRepo, true)
	if err != nil {
		t.Fatalf("Failed to process repository: %v", err)
	}

	// Verify repository was stored
	stored, err := repo.GetRepository(ctx, "user/test-repo")
	if err != nil {
		t.Fatalf("Failed to retrieve stored repository: %v", err)
	}

	// Verify stored data
	if stored.FullName != "user/test-repo" {
		t.Errorf("Expected FullName 'user/test-repo', got '%s'", stored.FullName)
	}

	if stored.StargazersCount != 42 {
		t.Errorf("Expected StargazersCount 42, got %d", stored.StargazersCount)
	}}

func TestSyncService_ProcessRepositoriesInBatches(t *testing.T) {
	// Create temporary database
	tempDir, err := os.MkdirTemp("", "batch_test")
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

	// Create mock services
	mockGitHub := &MockGitHubClient{
		content: map[string][]github.Content{
			"user/repo1": {{Path: "README.md", Type: "file", Content: "Repo 1", Size: 6}},
			"user/repo2": {{Path: "README.md", Type: "file", Content: "Repo 2", Size: 6}},
			"user/repo3": {{Path: "README.md", Type: "file", Content: "Repo 3", Size: 6}},
		},
	}

	processorService := processor.NewService(mockGitHub)

	syncService := &SyncService{
		githubClient: mockGitHub,
		processor:    processorService,
		storage:      repo,
		config:       func() *config.Config { cfg, _ := config.LoadConfig(); return cfg }(),
		verbose:      false,
	}

	// Test repositories
	testRepos := []github.Repository{
		{FullName: "user/repo1", StargazersCount: 10, UpdatedAt: time.Now()},
		{FullName: "user/repo2", StargazersCount: 20, UpdatedAt: time.Now()},
		{FullName: "user/repo3", StargazersCount: 30, UpdatedAt: time.Now()},
	}

	stats := &SyncStats{}

	// Create operations for the new batch processing signature
	operations := &syncOperations{
		toAdd:    testRepos, // All repos are new
		toUpdate: []github.Repository{},
		toRemove: []string{},
	}

	// Process in batches of 2
	err = syncService.processRepositoriesInBatches(ctx, testRepos, 2, stats, operations)
	if err != nil {
		t.Fatalf("Failed to process repositories in batches: %v", err)
	}

	// Verify all repositories were processed
	for _, testRepo := range testRepos {
		stored, err := repo.GetRepository(ctx, testRepo.FullName)
		if err != nil {
			t.Errorf("Repository %s was not stored: %v", testRepo.FullName, err)
		} else if stored.FullName != testRepo.FullName {
			t.Errorf("Expected %s, got %s", testRepo.FullName, stored.FullName)
		}
	}

	// Verify stats were updated correctly
	if stats.NewRepos != 3 {
		t.Errorf("Expected 3 new repos, got %d", stats.NewRepos)
	}

	if stats.ProcessedRepos != 3 {
		t.Errorf("Expected 3 processed repos, got %d", stats.ProcessedRepos)
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "home directory expansion",
			input:    "~/config/database.db",
			expected: filepath.Join(os.Getenv("HOME"), "config/database.db"),
		},
		{
			name:     "absolute path unchanged",
			input:    "/tmp/database.db",
			expected: "/tmp/database.db",
		},
		{
			name:     "relative path unchanged",
			input:    "database.db",
			expected: "database.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if tt.name == "home directory expansion" {
				// For home directory test, just check it doesn't start with ~
				if strings.HasPrefix(result, "~") {
					t.Errorf("expandPath() failed to expand home directory: %s", result)
				}
			} else if result != tt.expected {
				t.Errorf("expandPath() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestSyncService_GetUpdateReason(t *testing.T) {
	syncService := &SyncService{}

	baseTime := time.Now().Add(-2 * time.Hour)

	tests := []struct {
		name     string
		repo     github.Repository
		existing *storage.StoredRepo
		expected string
	}{
		{
			name: "star count change",
			repo: github.Repository{
				FullName:        "user/repo",
				UpdatedAt:       baseTime.Add(-1 * time.Hour),
				StargazersCount: 150,
				ForksCount:      10,
				Size:            1000,
				Description:     "Test repo",
			},
			existing: &storage.StoredRepo{
				FullName:        "user/repo",
				StargazersCount: 100,
				ForksCount:      10,
				SizeKB:          1000,
				Description:     "Test repo",
				LastSynced:      baseTime,
			},
			expected: "stars: 100 → 150",
		},
		{
			name: "multiple changes",
			repo: github.Repository{
				FullName:        "user/repo",
				UpdatedAt:       time.Now(),
				StargazersCount: 150,
				ForksCount:      15,
				Size:            2000,
				Description:     "Updated test repo",
			},
			existing: &storage.StoredRepo{
				FullName:        "user/repo",
				StargazersCount: 100,
				ForksCount:      10,
				SizeKB:          1000,
				Description:     "Test repo",
				LastSynced:      baseTime,
			},
			expected: "repository updated, stars: 100 → 150, forks: 10 → 15, size changed, description changed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncService.getUpdateReason(tt.repo, tt.existing)
			if result != tt.expected {
				t.Errorf("getUpdateReason() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestSyncService_TopicsEqual(t *testing.T) {
	syncService := &SyncService{}

	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "identical topics",
			a:        []string{"go", "cli", "tool"},
			b:        []string{"go", "cli", "tool"},
			expected: true,
		},
		{
			name:     "different order same topics",
			a:        []string{"go", "cli", "tool"},
			b:        []string{"tool", "go", "cli"},
			expected: true,
		},
		{
			name:     "different topics",
			a:        []string{"go", "cli"},
			b:        []string{"python", "web"},
			expected: false,
		},
		{
			name:     "different lengths",
			a:        []string{"go", "cli"},
			b:        []string{"go", "cli", "tool"},
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncService.topicsEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("topicsEqual() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestSyncService_LicenseChanged(t *testing.T) {
	syncService := &SyncService{}

	tests := []struct {
		name         string
		newLicense   *github.License
		existingName string
		existingSPDX string
		expected     bool
	}{
		{
			name: "no change",
			newLicense: &github.License{
				Name:   "MIT License",
				SPDXID: "MIT",
			},
			existingName: "MIT License",
			existingSPDX: "MIT",
			expected:     false,
		},
		{
			name: "name changed",
			newLicense: &github.License{
				Name:   "Apache License 2.0",
				SPDXID: "Apache-2.0",
			},
			existingName: "MIT License",
			existingSPDX: "Apache-2.0",
			expected:     true,
		},
		{
			name:         "license removed",
			newLicense:   nil,
			existingName: "MIT License",
			existingSPDX: "MIT",
			expected:     true,
		},
		{
			name: "license added",
			newLicense: &github.License{
				Name:   "MIT License",
				SPDXID: "MIT",
			},
			existingName: "",
			existingSPDX: "",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncService.licenseChanged(tt.newLicense, tt.existingName, tt.existingSPDX)
			if result != tt.expected {
				t.Errorf("licenseChanged() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestProgressTracker(t *testing.T) {
	// Test progress tracker functionality
	tracker := NewProgressTracker(5, "Testing progress")

	if tracker.total != 5 {
		t.Errorf("Expected total 5, got %d", tracker.total)
	}

	if tracker.processed != 0 {
		t.Errorf("Expected processed 0, got %d", tracker.processed)
	}

	// Test update
	tracker.Update("test-repo")

	if tracker.processed != 1 {
		t.Errorf("Expected processed 1 after update, got %d", tracker.processed)
	}

	// Test multiple updates
	tracker.Update("test-repo-2")
	tracker.Update("test-repo-3")

	if tracker.processed != 3 {
		t.Errorf("Expected processed 3 after multiple updates, got %d", tracker.processed)
	}
}

func TestSyncStats_SafeIncrement(t *testing.T) {
	stats := &SyncStats{}

	// Test concurrent increments
	var wg sync.WaitGroup

	// Start multiple goroutines to test thread safety
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()
			stats.SafeIncrement("new")
			stats.SafeIncrement("updated")
			stats.SafeIncrement("processed")
		}()
	}

	wg.Wait()

	// Each field should be incremented 10 times
	if stats.NewRepos != 10 {
		t.Errorf("Expected NewRepos 10, got %d", stats.NewRepos)
	}

	if stats.UpdatedRepos != 10 {
		t.Errorf("Expected UpdatedRepos 10, got %d", stats.UpdatedRepos)
	}

	if stats.ProcessedRepos != 10 {
		t.Errorf("Expected ProcessedRepos 10, got %d", stats.ProcessedRepos)
	}
}
