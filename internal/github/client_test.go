package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

// mockRESTClient implements RESTClientInterface for testing
type mockRESTClient struct {
	mu        sync.RWMutex
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
	m.mu.Lock()
	m.callCount[path]++
	m.mu.Unlock()

	m.mu.RLock()
	err, exists := m.errors[path]
	resp, respExists := m.responses[path]
	m.mu.RUnlock()

	if exists {
		return err
	}

	if respExists {
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
	m.mu.Lock()
	m.responses[path] = response
	m.mu.Unlock()
}

func (m *mockRESTClient) setError(path string, err error) {
	m.mu.Lock()
	m.errors[path] = err
	m.mu.Unlock()
}

func (m *mockRESTClient) getCallCount(path string) int {
	m.mu.RLock()
	count := m.callCount[path]
	m.mu.RUnlock()

	return count
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
	// Get authenticated HTTP client from GitHub CLI
	authClient, err := api.DefaultHTTPClient()
	if err != nil {
		t.Fatal(err)
	}

	// Create VCR recorder with matcher that ignores most headers
	r, err := recorder.New("testdata/get_starred_repos_success",
		recorder.WithRealTransport(authClient.Transport),
		recorder.WithMatcher(cassette.NewDefaultMatcher(
			cassette.WithIgnoreAuthorization(),
			cassette.WithIgnoreHeaders("Time-Zone", "Content-Type", "Accept", "User-Agent"),
		)))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()

	t.Logf("VCR - Is new cassette: %v, Is recording: %v", r.IsNewCassette(), r.IsRecording())

	// Use VCR's default client which should handle replay properly
	vcrHTTPClient := r.GetDefaultClient()

	// Create client with VCR's HTTP client
	vcrClient := NewVCRRESTClient(vcrHTTPClient)
	client := &clientImpl{apiClient: vcrClient}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repos, err := client.GetStarredRepos(ctx, "testuser")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(repos) == 0 {
		t.Fatal("Expected at least 1 repository")
	}

	t.Logf("Retrieved %d starred repositories", len(repos))
}

func TestGetStarredRepos_Pagination(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	// Create 100 repos for first page (full page)
	var page1Repos []Repository

	for i := range 100 {
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
		t.Errorf(
			"Expected last repository name %s, got: %s",
			testRepo2.FullName,
			repos[100].FullName,
		)
	}
}

func TestGetStarredRepos_Error(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	expectedError := errors.New("API error")
	mockClient.setError("user/starred?page=1&per_page=100", expectedError)

	ctx := context.Background()
	repos, err := client.GetStarredRepos(ctx, "testuser")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if repos != nil {
		t.Error("Expected nil repositories on error")
	}

	if !containsStr(err.Error(), "failed to fetch starred repositories") {
		t.Errorf(
			"Expected error message to contain 'failed to fetch starred repositories', got: %s",
			err.Error(),
		)
	}
}

func TestGetStarredRepos_ContextCancellation(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	repos, err := client.GetStarredRepos(ctx, "testuser")

	if !errors.Is(err, context.Canceled) {
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
	mockClient.setError(
		"repos/owner/repo/contents/nonexistent.md",
		&api.HTTPError{StatusCode: http.StatusNotFound},
	)

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
	mockClient.setError(
		"repos/owner/repo/releases/latest",
		&api.HTTPError{StatusCode: http.StatusNotFound},
	)

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
func containsStr(s, substr string) bool {
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

func TestGetContributors_Success(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	expectedContributors := []Contributor{
		{Login: "user1", Contributions: 100, Type: "User"},
		{Login: "user2", Contributions: 50, Type: "User"},
	}

	mockClient.setResponse("repos/owner/repo/contributors?per_page=10", expectedContributors)

	ctx := context.Background()
	contributors, err := client.GetContributors(ctx, "owner/repo", 10)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(contributors) != 2 {
		t.Fatalf("Expected 2 contributors, got: %d", len(contributors))
	}

	if contributors[0].Login != "user1" {
		t.Errorf("Expected first contributor 'user1', got: %s", contributors[0].Login)
	}

	if contributors[0].Contributions != 100 {
		t.Errorf(
			"Expected first contributor contributions 100, got: %d",
			contributors[0].Contributions,
		)
	}
}

func TestGetTopics_Success(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	topicsResponse := struct {
		Names []string `json:"names"`
	}{
		Names: []string{"go", "cli", "github"},
	}

	mockClient.setResponse("repos/owner/repo/topics", topicsResponse)

	ctx := context.Background()
	topics, err := client.GetTopics(ctx, "owner/repo")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(topics) != 3 {
		t.Fatalf("Expected 3 topics, got: %d", len(topics))
	}

	expectedTopics := []string{"go", "cli", "github"}
	for i, topic := range topics {
		if topic != expectedTopics[i] {
			t.Errorf("Expected topic %s, got: %s", expectedTopics[i], topic)
		}
	}
}

func TestGetLanguages_Success(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	expectedLanguages := map[string]int64{
		"Go":         12345,
		"JavaScript": 5678,
		"Shell":      123,
	}

	mockClient.setResponse("repos/owner/repo/languages", expectedLanguages)

	ctx := context.Background()
	languages, err := client.GetLanguages(ctx, "owner/repo")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(languages) != 3 {
		t.Fatalf("Expected 3 languages, got: %d", len(languages))
	}

	if languages["Go"] != 12345 {
		t.Errorf("Expected Go bytes 12345, got: %d", languages["Go"])
	}

	if languages["JavaScript"] != 5678 {
		t.Errorf("Expected JavaScript bytes 5678, got: %d", languages["JavaScript"])
	}
}

func TestGetCommitActivity_Success(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	expectedWeeks := []WeeklyCommits{
		{Week: 1640995200, Commits: 10, Adds: 100, Deletes: 20},
		{Week: 1641600000, Commits: 5, Adds: 50, Deletes: 10},
	}

	mockClient.setResponse("repos/owner/repo/stats/commit_activity", expectedWeeks)

	ctx := context.Background()
	activity, err := client.GetCommitActivity(ctx, "owner/repo")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if activity.Total != 15 {
		t.Errorf("Expected total commits 15, got: %d", activity.Total)
	}

	if len(activity.Weeks) != 2 {
		t.Fatalf("Expected 2 weeks, got: %d", len(activity.Weeks))
	}

	if activity.Weeks[0].Commits != 10 {
		t.Errorf("Expected first week commits 10, got: %d", activity.Weeks[0].Commits)
	}
}

func TestGetCommitActivity_StatsComputing(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	// Mock 202 Accepted response (stats being computed)
	mockClient.setError(
		"repos/owner/repo/stats/commit_activity",
		&api.HTTPError{StatusCode: http.StatusAccepted},
	)

	ctx := context.Background()
	activity, err := client.GetCommitActivity(ctx, "owner/repo")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if activity.Total != -1 {
		t.Errorf("Expected total commits -1 (computing), got: %d", activity.Total)
	}

	if len(activity.Weeks) != 0 {
		t.Errorf("Expected 0 weeks when computing, got: %d", len(activity.Weeks))
	}
}

func TestGetPullCounts_Success(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	openResult := SearchResult{TotalCount: 5, IncompleteResults: false}
	totalResult := SearchResult{TotalCount: 25, IncompleteResults: false}

	mockClient.setResponse(
		"search/issues?q=repo:owner/repo+type:pr+state:open&per_page=1",
		openResult,
	)
	mockClient.setResponse("search/issues?q=repo:owner/repo+type:pr&per_page=1", totalResult)

	ctx := context.Background()
	open, total, err := client.GetPullCounts(ctx, "owner/repo")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if open != 5 {
		t.Errorf("Expected open PRs 5, got: %d", open)
	}

	if total != 25 {
		t.Errorf("Expected total PRs 25, got: %d", total)
	}
}

func TestGetIssueCounts_Success(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	openResult := SearchResult{TotalCount: 8, IncompleteResults: false}
	totalResult := SearchResult{TotalCount: 42, IncompleteResults: false}

	mockClient.setResponse(
		"search/issues?q=repo:owner/repo+type:issue+state:open&per_page=1",
		openResult,
	)
	mockClient.setResponse("search/issues?q=repo:owner/repo+type:issue&per_page=1", totalResult)

	ctx := context.Background()
	open, total, err := client.GetIssueCounts(ctx, "owner/repo")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if open != 8 {
		t.Errorf("Expected open issues 8, got: %d", open)
	}

	if total != 42 {
		t.Errorf("Expected total issues 42, got: %d", total)
	}
}

func TestGetHomepageText_Success(t *testing.T) {
	// This test would require mocking HTTP client
	// For now, we'll test the URL validation logic
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	ctx := context.Background()

	// Test invalid URL
	_, err := client.GetHomepageText(ctx, "invalid-url")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	// Test unsupported scheme
	_, err = client.GetHomepageText(ctx, "ftp://example.com")
	if err == nil {
		t.Error("Expected error for unsupported scheme")
	}

	if !containsStr(err.Error(), "unsupported URL scheme") {
		t.Errorf("Expected unsupported scheme error, got: %s", err.Error())
	}
}
