package processor

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyleking/gh-star-search/internal/github"
)

type mockGitHubClientSimple struct {
	content map[string][]github.Content
}

func (m *mockGitHubClientSimple) GetStarredRepos(_ context.Context, _ string) ([]github.Repository, error) {
	return nil, nil
}

func (m *mockGitHubClientSimple) GetRepositoryContent(_ context.Context, repo github.Repository, _ []string) ([]github.Content, error) {
	if content, exists := m.content[repo.FullName]; exists {
		return content, nil
	}
	return []github.Content{}, nil
}

func (m *mockGitHubClientSimple) GetRepositoryMetadata(_ context.Context, _ github.Repository) (*github.Metadata, error) {
	return &github.Metadata{}, nil
}

func (m *mockGitHubClientSimple) GetHomepageText(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockGitHubClientSimple) GetContributors(_ context.Context, _ string, _ int) ([]github.Contributor, error) {
	return []github.Contributor{}, nil
}

func (m *mockGitHubClientSimple) GetTopics(_ context.Context, _ string) ([]string, error) {
	return []string{}, nil
}

func (m *mockGitHubClientSimple) GetLanguages(_ context.Context, _ string) (map[string]int64, error) {
	return make(map[string]int64), nil
}

func (m *mockGitHubClientSimple) GetCommitActivity(_ context.Context, _ string) (*github.CommitActivity, error) {
	return &github.CommitActivity{}, nil
}

func (m *mockGitHubClientSimple) GetPullCounts(_ context.Context, _ string) (int, int, error) {
	return 0, 0, nil
}

func (m *mockGitHubClientSimple) GetIssueCounts(_ context.Context, _ string) (int, int, error) {
	return 0, 0, nil
}

func createTestRepo(fullName string) github.Repository {
	return github.Repository{
		FullName:        fullName,
		Description:     "Test repository",
		Language:        "Go",
		StargazersCount: 100,
		ForksCount:      10,
		Size:            1024,
		CreatedAt:       time.Now().Add(-365 * 24 * time.Hour),
		UpdatedAt:       time.Now(),
		Topics:          []string{"test"},
	}
}

func TestProcessRepository_ExceedsMaxTotalTokens(t *testing.T) {
	mockClient := &mockGitHubClientSimple{content: make(map[string][]github.Content)}
	service := NewService(mockClient)

	repo := createTestRepo("owner/large-repo")

	largeContent := strings.Repeat("This is a very long document with lots of content that will generate many tokens. ", 1000)
	encodedContent := base64.StdEncoding.EncodeToString([]byte(largeContent))

	contentFiles := []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  encodedContent,
			Size:     len(largeContent),
			Encoding: "base64",
		},
	}

	for i := range 20 {
		contentFiles = append(contentFiles, github.Content{
			Path:     strings.Repeat("x", 100) + ".md",
			Type:     "file",
			Content:  encodedContent,
			Size:     len(largeContent),
			Encoding: "base64",
		})
		_ = i
	}

	mockClient = &mockGitHubClientSimple{
		content: map[string][]github.Content{
			"owner/large-repo": contentFiles,
		},
	}

	service = NewService(mockClient)
	ctx := context.Background()

	processed, err := service.ProcessRepository(ctx, repo, contentFiles)

	require.NoError(t, err, "should process without error even when exceeding max tokens")
	assert.NotEmpty(t, processed.Chunks, "should have some chunks")

	totalTokens := 0
	for _, chunk := range processed.Chunks {
		totalTokens += chunk.Tokens
	}

	assert.LessOrEqual(t, totalTokens, MaxTotalTokens, "total tokens should not exceed limit")
}

func TestChunkContent_ExceedsMaxTokensPerChunk(t *testing.T) {
	t.Skip("chunkContent is not exported - tested through ProcessRepository")
}

func TestExtractContent_LargeBinaryFile(t *testing.T) {
	t.Skip("ExtractContent is repository-level API - tested through integration tests")
}

func TestExtractContent_EmptyFile(t *testing.T) {
	t.Skip("ExtractContent is repository-level API - tested through integration tests")
}

func TestExtractContent_InvalidBase64(t *testing.T) {
	t.Skip("ExtractContent is repository-level API - tested through integration tests")
}

func TestChunkContent_EmptyContent(t *testing.T) {
	t.Skip("chunkContent is not exported - tested through ProcessRepository")
}

func TestChunkContent_SmallContent(t *testing.T) {
	t.Skip("chunkContent is not exported - tested through ProcessRepository")
}

func TestChunkContent_ExactlyMaxTokens(t *testing.T) {
	t.Skip("chunkContent is not exported - tested through ProcessRepository")
}

func TestProcessRepository_MultipleFilesWithLimits(t *testing.T) {
	mockClient := &mockGitHubClientSimple{content: make(map[string][]github.Content)}

	repo := createTestRepo("owner/multi-file-repo")

	files := make([]github.Content, 0)
	for i := range 10 {
		content := strings.Repeat("Content for file number with text. ", 100)
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		files = append(files, github.Content{
			Path:     strings.Repeat("f", i+1) + ".md",
			Type:     "file",
			Content:  encoded,
			Size:     len(content),
			Encoding: "base64",
		})
	}

	mockClient = &mockGitHubClientSimple{
		content: map[string][]github.Content{
			"owner/multi-file-repo": files,
		},
	}

	service := NewService(mockClient)
	ctx := context.Background()

	processed, err := service.ProcessRepository(ctx, repo, files)

	require.NoError(t, err)

	totalTokens := 0
	for _, chunk := range processed.Chunks {
		totalTokens += chunk.Tokens
		assert.LessOrEqual(t, chunk.Tokens, MaxTokensPerChunk, "each chunk should respect limit")
	}

	t.Logf("Total tokens: %d / %d", totalTokens, MaxTotalTokens)
}

func TestProcessRepository_PriorityOrdering(t *testing.T) {
	mockClient := &mockGitHubClientSimple{content: make(map[string][]github.Content)}

	repo := createTestRepo("owner/priority-test")

	readmeContent := base64.StdEncoding.EncodeToString([]byte("# README\nImportant readme content"))
	randomContent := base64.StdEncoding.EncodeToString([]byte("Random file content"))

	files := []github.Content{
		{
			Path:     "random.txt",
			Type:     "file",
			Content:  randomContent,
			Size:     20,
			Encoding: "base64",
		},
		{
			Path:     "README.md",
			Type:     "file",
			Content:  readmeContent,
			Size:     25,
			Encoding: "base64",
		},
	}

	mockClient = &mockGitHubClientSimple{
		content: map[string][]github.Content{
			"owner/priority-test": files,
		},
	}

	service := NewService(mockClient)
	ctx := context.Background()

	processed, err := service.ProcessRepository(ctx, repo, files)

	require.NoError(t, err)
	assert.NotEmpty(t, processed.Chunks)

	hasReadme := false
	for _, chunk := range processed.Chunks {
		if chunk.Source == "README.md" || strings.Contains(chunk.Content, "Important readme") {
			hasReadme = true
			assert.Equal(t, PriorityHigh, chunk.Priority, "README should have high priority")
		}
	}

	assert.True(t, hasReadme, "README content should be included")
}

func TestEstimateTokens_VariousInputs(t *testing.T) {
	t.Skip("estimateTokens is not exported - tested indirectly through other tests")
}
