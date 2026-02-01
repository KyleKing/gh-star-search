package github

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStarredRepos_EmptyResponse(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	emptyRepos := []Repository{}
	mockClient.setResponse("user/starred?page=1&per_page=50", emptyRepos)

	ctx := context.Background()
	repos, err := client.GetStarredRepos(ctx, "testuser")

	require.NoError(t, err)
	assert.Empty(t, repos, "should return empty slice for no starred repositories")
	assert.Equal(t, 1, mockClient.getCallCount("user/starred?page=1&per_page=50"), "should only call page 1")
}

func TestGetStarredRepos_SinglePage(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	repos := []Repository{
		createTestRepository(),
		{
			FullName:        "owner/repo2",
			Description:     "Second test repo",
			StargazersCount: 50,
		},
	}
	mockClient.setResponse("user/starred?page=1&per_page=50", repos)
	mockClient.setResponse("user/starred?page=2&per_page=50", []Repository{})

	ctx := context.Background()
	result, err := client.GetStarredRepos(ctx, "testuser")

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "owner/repo", result[0].FullName)
	assert.Equal(t, "owner/repo2", result[1].FullName)
}

func TestGetStarredRepos_ExactPageSize(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	page1Repos := make([]Repository, 50)
	for i := range 50 {
		page1Repos[i] = Repository{
			FullName:        "owner/repo" + string(rune(i)),
			StargazersCount: i + 1,
		}
	}

	mockClient.setResponse("user/starred?page=1&per_page=50", page1Repos)
	mockClient.setResponse("user/starred?page=2&per_page=50", []Repository{})

	ctx := context.Background()
	result, err := client.GetStarredRepos(ctx, "testuser")

	require.NoError(t, err)
	assert.Len(t, result, 50, "should handle exactly one full page")
	assert.GreaterOrEqual(t, mockClient.getCallCount("user/starred?page=1&per_page=50"), 1, "should call page 1")
}

func TestGetStarredRepos_RateLimit429(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	mockClient.setError("user/starred?page=1&per_page=50",
		&api.HTTPError{StatusCode: http.StatusTooManyRequests})

	ctx := context.Background()
	_, err := client.GetStarredRepos(ctx, "testuser")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "429", "should return rate limit error")
}

func TestGetStarredRepos_NetworkTimeout(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond)

	_, err := client.GetStarredRepos(ctx, "testuser")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGetStarredRepos_MultiplePages(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	page1 := make([]Repository, 50)
	for i := range 50 {
		page1[i] = Repository{FullName: "owner/repo" + string(rune(i))}
	}

	page2 := make([]Repository, 30)
	for i := range 30 {
		page2[i] = Repository{FullName: "owner/page2-" + string(rune(i))}
	}

	mockClient.setResponse("user/starred?page=1&per_page=50", page1)
	mockClient.setResponse("user/starred?page=2&per_page=50", page2)
	mockClient.setResponse("user/starred?page=3&per_page=50", []Repository{})

	ctx := context.Background()
	result, err := client.GetStarredRepos(ctx, "testuser")

	require.NoError(t, err)
	assert.Len(t, result, 80, "should aggregate repos from multiple pages")
}

func TestGetRepositoryContent_NotFound404(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	repo := createTestRepository()
	mockClient.setError("repos/owner/repo/contents/README.md", &api.HTTPError{StatusCode: http.StatusNotFound})

	ctx := context.Background()
	content, err := client.GetRepositoryContent(ctx, repo, []string{"README.md"})

	require.NoError(t, err, "404 errors should be gracefully skipped")
	assert.Empty(t, content, "should return empty content when file not found")
}

func TestGetRepositoryContent_EmptyRepository(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	repo := createTestRepository()
	mockClient.setResponse("repos/owner/repo/contents/", []Content{})

	ctx := context.Background()
	content, err := client.GetRepositoryContent(ctx, repo, []string{"README.md"})

	require.NoError(t, err)
	assert.Empty(t, content, "should handle empty repository gracefully")
}

func TestGetRepositoryContent_LargeFile(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	repo := createTestRepository()
	largeContent := Content{
		Path:     "large-file.bin",
		Type:     "file",
		Size:     10 * 1024 * 1024,
		Encoding: "base64",
		Content:  "",
	}

	mockClient.setResponse("repos/owner/repo/contents/large-file.bin", largeContent)

	ctx := context.Background()
	content, err := client.GetRepositoryContent(ctx, repo, []string{"large-file.bin"})

	require.NoError(t, err)
	assert.Len(t, content, 1)
	assert.Equal(t, 10*1024*1024, content[0].Size)
}

func TestGetRepositoryContent_ContextCancellation(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	repo := createTestRepository()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetRepositoryContent(ctx, repo, []string{"README.md"})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestGetRepositoryMetadata_EmptyMetadata(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	repo := createTestRepository()

	ctx := context.Background()
	result, err := client.GetRepositoryMetadata(ctx, repo)

	require.NoError(t, err, "should not error on missing metadata")
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.CommitCount, "should have zero commit count when API fails")
}

func TestGetCommitActivity_EmptyRepository(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	mockClient.setResponse("repos/owner/repo/stats/commit_activity", []WeeklyCommits{})

	ctx := context.Background()
	activity, err := client.GetCommitActivity(ctx, "owner/repo")

	require.NoError(t, err)
	assert.NotNil(t, activity)
	assert.Equal(t, 0, activity.Total)
	assert.Empty(t, activity.Weeks)
}

func TestGetPullCounts_RepositoryWithNoPRs(t *testing.T) {
	t.Skip("Complex API mocking required - covered by integration tests")
}

func TestGetIssueCounts_RepositoryWithNoIssues(t *testing.T) {
	t.Skip("Complex API mocking required - covered by integration tests")
}

func TestGetLanguages_EmptyLanguages(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	mockClient.setResponse("repos/owner/repo/languages", map[string]int64{})

	ctx := context.Background()
	languages, err := client.GetLanguages(ctx, "owner/repo")

	require.NoError(t, err)
	assert.Empty(t, languages, "should handle repository with no detected languages")
}

func TestGetContributors_EmptyContributors(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	mockClient.setResponse("repos/owner/repo/contributors?per_page=10", []Contributor{})

	ctx := context.Background()
	contributors, err := client.GetContributors(ctx, "owner/repo", 10)

	require.NoError(t, err)
	assert.Empty(t, contributors, "should handle repository with no contributors")
}

func TestGetTopics_EmptyTopics(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	mockClient.setResponse("repos/owner/repo/topics", map[string]interface{}{
		"names": []string{},
	})

	ctx := context.Background()
	topics, err := client.GetTopics(ctx, "owner/repo")

	require.NoError(t, err)
	assert.Empty(t, topics, "should handle repository with no topics")
}

func TestGetStarredRepos_ServerError500(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	mockClient.setError("user/starred?page=1&per_page=50",
		&api.HTTPError{StatusCode: http.StatusInternalServerError})

	ctx := context.Background()
	_, err := client.GetStarredRepos(ctx, "testuser")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500", "should handle internal server error")
}

func TestGetRepositoryContent_Unauthorized401(t *testing.T) {
	mockClient := newMockRESTClient()
	client := &clientImpl{apiClient: mockClient}

	repo := createTestRepository()
	mockClient.setError("repos/owner/repo/contents/private-file.txt",
		&api.HTTPError{StatusCode: http.StatusUnauthorized})

	ctx := context.Background()
	_, err := client.GetRepositoryContent(ctx, repo, []string{"private-file.txt"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "401", "should handle unauthorized error")
}
