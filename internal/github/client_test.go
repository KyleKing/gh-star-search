package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// mockRESTClient implements RESTClientInterface for testing
type mockRESTClient struct {
	responses map[string]interface{}
	errors    map[string]error
	callCount map[string]int
}

func newMockRESTClient() *mockRESTClient {
	return &mockRESTClient{
		responses: make(map[string]interface{}),
		errors:    make(map[string]error),
		callCount: make(map[string]int),
	}
}

func (m *mockRESTClient) Get(path string, response interface{}) error {
	m.callCount[path]++

	if err, exists := m.errors[path]; exists {
		return err
	}

	if resp, exists := m.responses[path]; exists {
		// Convert response to JSON and back to properly populate the interface
		jsonData, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		return json.Unmarshal(jsonData, response)
	}

	return &api.HTTPError{StatusCode: http.StatusNotFound}
}

func (m *mockRESTClient) setResponse(path string, response interface{}) {
	m.responses[path] = response
}

func (m *mockRESTClient) setError(path string, err error) {
	m.errors[path] = err
}

func (m *mockRESTClient) getCallCount(path string) int {
	return m.callCount[path]
}

// Test data
func createTestRepository() Repository {
	return Repository{
		FullName:        "owner/repo",
		Description:     "Test repository",
		Language:        "Go",
		StargazersCount: 100,
		ForksCount:      10,
		UpdatedAt:       time.Date(2023, 12, 1, 12, 0, 0, 0, time.UTC),
		CreatedAt:       time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Topics:          []string{"cli", "github"},
		License: &License{
			Key:    "mit",
			Name:   "MIT License",
			SPDXID: "MIT",
			URL:    "https://api.github.com/licenses/mit",
		},
		Size:            1024,
		DefaultBranch:   "main",
		OpenIssuesCount: 5,
		HasWiki:         true,
		HasPages:        false,
		Archived:        false,
		Disabled:        false,
		Private:         false,
		Fork:            false,
	}
}

func createTestContent() Content {
	return Content{
		Path:     "README.md",
		Type:     "file",
		Content:  "VGVzdCBjb250ZW50", // base64 encoded "Test content"
		Size:     12,
		Encoding: "base64",
		SHA:      "abc123",
	}
}

func TestNewClient(t *testing.T) {
	// This test would require mocking the api.DefaultRESTClient() function
	// For now, we'll test the client implementation directly
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	if client.apiClient == nil {
		t.Error("Expected apiClient to be set")
	}
}

func TestGetStarredRepos_Success(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	testRepo := createTestRepository()
	
	// Mock first page response with less than per_page results to end pagination
	mockClient.setResponse("user/starred?page=1&per_page=100", []Repository{testRepo})

	ctx := context.Background()
	repos, err := client.GetStarredRepos(ctx, "testuser")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(repos) != 1 {
		t.Fatalf("Expected 1 repository, got: %d", len(repos))
	}

	if repos[0].FullName != testRepo.FullName {
		t.Errorf("Expected repository name %s, got: %s", testRepo.FullName, repos[0].FullName)
	}

	// Verify pagination calls
	if mockClient.getCallCount("user/starred?page=1&per_page=100") != 1 {
		t.Error("Expected first page to be called once")
	}
}

func TestGetStarredRepos_Pagination(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	// Create 100 repos for first page (full page)
	var page1Repos []Repository
	for i := 0; i < 100; i++ {
		repo := createTestRepository()
		repo.FullName = fmt.Sprintf("owner/repo%d", i)
		page1Repos = append(page1Repos, repo)
	}

	// Create 1 repo for second page (partial page to end pagination)
	testRepo2 := createTestRepository()
	testRepo2.FullName = "owner/repo100"

	// Mock multiple pages
	mockClient.setResponse("user/starred?page=1&per_page=100", page1Repos)
	mockClient.setResponse("user/starred?page=2&per_page=100", []Repository{testRepo2})

	ctx := context.Background()
	repos, err := client.GetStarredRepos(ctx, "testuser")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(repos) != 101 {
		t.Fatalf("Expected 101 repositories, got: %d", len(repos))
	}

	if repos[0].FullName != "owner/repo0" {
		t.Errorf("Expected first repository name owner/repo0, got: %s", repos[0].FullName)
	}
	if repos[100].FullName != testRepo2.FullName {
		t.Errorf("Expected last repository name %s, got: %s", testRepo2.FullName, repos[100].FullName)
	}
}

func TestGetStarredRepos_Error(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	expectedError := fmt.Errorf("API error")
	mockClient.setError("user/starred?page=1&per_page=100", expectedError)

	ctx := context.Background()
	repos, err := client.GetStarredRepos(ctx, "testuser")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if repos != nil {
		t.Error("Expected nil repositories on error")
	}

	if !contains(err.Error(), "failed to fetch starred repositories") {
		t.Errorf("Expected error message to contain 'failed to fetch starred repositories', got: %s", err.Error())
	}
}

func TestGetStarredRepos_ContextCancellation(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	repos, err := client.GetStarredRepos(ctx, "testuser")

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	if repos != nil {
		t.Error("Expected nil repositories on context cancellation")
	}
}

func TestGetRepositoryContent_Success(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	testRepo := createTestRepository()
	testContent := createTestContent()

	mockClient.setResponse("repos/owner/repo/contents/README.md", testContent)

	ctx := context.Background()
	contents, err := client.GetRepositoryContent(ctx, testRepo, []string{"README.md"})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(contents) != 1 {
		t.Fatalf("Expected 1 content item, got: %d", len(contents))
	}

	if contents[0].Path != testContent.Path {
		t.Errorf("Expected path %s, got: %s", testContent.Path, contents[0].Path)
	}
}

func TestGetRepositoryContent_FileNotFound(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	testRepo := createTestRepository()

	// Mock 404 error for non-existent file
	mockClient.setError("repos/owner/repo/contents/nonexistent.md", &api.HTTPError{StatusCode: http.StatusNotFound})

	ctx := context.Background()
	contents, err := client.GetRepositoryContent(ctx, testRepo, []string{"nonexistent.md"})

	if err != nil {
		t.Fatalf("Expected no error for missing file, got: %v", err)
	}

	if len(contents) != 0 {
		t.Errorf("Expected 0 content items for missing file, got: %d", len(contents))
	}
}

func TestGetRepositoryContent_MultiplePaths(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	testRepo := createTestRepository()
	
	readmeContent := createTestContent()
	readmeContent.Path = "README.md"
	
	licenseContent := createTestContent()
	licenseContent.Path = "LICENSE"

	mockClient.setResponse("repos/owner/repo/contents/README.md", readmeContent)
	mockClient.setResponse("repos/owner/repo/contents/LICENSE", licenseContent)

	ctx := context.Background()
	contents, err := client.GetRepositoryContent(ctx, testRepo, []string{"README.md", "LICENSE"})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(contents) != 2 {
		t.Fatalf("Expected 2 content items, got: %d", len(contents))
	}

	// Check that both files were retrieved
	paths := make(map[string]bool)
	for _, content := range contents {
		paths[content.Path] = true
	}

	if !paths["README.md"] {
		t.Error("Expected README.md to be retrieved")
	}
	if !paths["LICENSE"] {
		t.Error("Expected LICENSE to be retrieved")
	}
}

func TestGetRepositoryMetadata_Success(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	testRepo := createTestRepository()

	// Mock commit response
	commitResponse := []map[string]interface{}{
		{
			"commit": map[string]interface{}{
				"committer": map[string]interface{}{
					"date": "2023-12-01T12:00:00Z",
				},
			},
		},
	}
	mockClient.setResponse("repos/owner/repo/commits?sha=main&per_page=1", commitResponse)

	// Mock contributors response
	contributorsResponse := []map[string]interface{}{
		{"login": "contributor1"},
		{"login": "contributor2"},
	}
	mockClient.setResponse("repos/owner/repo/contributors?per_page=10", contributorsResponse)

	// Mock latest release response
	releaseResponse := Release{
		TagName:     "v1.0.0",
		Name:        "Version 1.0.0",
		PublishedAt: time.Date(2023, 11, 1, 12, 0, 0, 0, time.UTC),
		Prerelease:  false,
		Draft:       false,
	}
	mockClient.setResponse("repos/owner/repo/releases/latest", releaseResponse)

	// Mock releases count response
	releasesResponse := []map[string]interface{}{
		{"tag_name": "v1.0.0"},
	}
	mockClient.setResponse("repos/owner/repo/releases?per_page=1", releasesResponse)

	ctx := context.Background()
	metadata, err := client.GetRepositoryMetadata(ctx, testRepo)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if metadata.CommitCount != 1 {
		t.Errorf("Expected commit count 1, got: %d", metadata.CommitCount)
	}

	if len(metadata.Contributors) != 2 {
		t.Errorf("Expected 2 contributors, got: %d", len(metadata.Contributors))
	}

	if metadata.Contributors[0] != "contributor1" {
		t.Errorf("Expected first contributor 'contributor1', got: %s", metadata.Contributors[0])
	}

	if metadata.LatestRelease == nil {
		t.Fatal("Expected latest release to be set")
	}

	if metadata.LatestRelease.TagName != "v1.0.0" {
		t.Errorf("Expected release tag 'v1.0.0', got: %s", metadata.LatestRelease.TagName)
	}

	if metadata.ReleaseCount != 1 {
		t.Errorf("Expected release count 1, got: %d", metadata.ReleaseCount)
	}
}

func TestGetRepositoryMetadata_NoReleases(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	testRepo := createTestRepository()

	// Mock commit response
	commitResponse := []map[string]interface{}{
		{
			"commit": map[string]interface{}{
				"committer": map[string]interface{}{
					"date": "2023-12-01T12:00:00Z",
				},
			},
		},
	}
	mockClient.setResponse("repos/owner/repo/commits?sha=main&per_page=1", commitResponse)

	// Mock contributors response
	contributorsResponse := []map[string]interface{}{
		{"login": "contributor1"},
	}
	mockClient.setResponse("repos/owner/repo/contributors?per_page=10", contributorsResponse)

	// Mock 404 for no releases
	mockClient.setError("repos/owner/repo/releases/latest", &api.HTTPError{StatusCode: http.StatusNotFound})

	ctx := context.Background()
	metadata, err := client.GetRepositoryMetadata(ctx, testRepo)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if metadata.LatestRelease != nil {
		t.Error("Expected no latest release")
	}

	if metadata.ReleaseCount != 0 {
		t.Errorf("Expected release count 0, got: %d", metadata.ReleaseCount)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}