package query

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KyleKing/gh-star-search/internal/processor"
	"github.com/KyleKing/gh-star-search/internal/storage"
)

// MockRepository for testing query engine
type mockQueryRepo struct {
	repos []storage.StoredRepo
}

func (m *mockQueryRepo) Initialize(_ context.Context) error {
	return nil
}

func (m *mockQueryRepo) StoreRepository(_ context.Context, _ processor.ProcessedRepo) error {
	return nil
}

func (m *mockQueryRepo) UpdateRepository(_ context.Context, _ processor.ProcessedRepo) error {
	return nil
}

func (m *mockQueryRepo) DeleteRepository(_ context.Context, _ string) error {
	return nil
}

func (m *mockQueryRepo) SearchRepositories(ctx context.Context, _ string) ([]storage.SearchResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	results := make([]storage.SearchResult, 0)
	for _, repo := range m.repos {
		results = append(results, storage.SearchResult{
			Repository: repo,
			Score:      0.5,
		})
	}
	return results, nil
}

func (m *mockQueryRepo) GetRepository(ctx context.Context, _ string) (*storage.StoredRepo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(m.repos) > 0 {
		return &m.repos[0], nil
	}
	return nil, errors.New("repository not found")
}

func (m *mockQueryRepo) ListRepositories(ctx context.Context, limit, offset int) ([]storage.StoredRepo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	start := offset
	if start >= len(m.repos) {
		return []storage.StoredRepo{}, nil
	}
	end := start + limit
	if end > len(m.repos) {
		end = len(m.repos)
	}
	return m.repos[start:end], nil
}

func (m *mockQueryRepo) GetStats(_ context.Context) (*storage.Stats, error) {
	return &storage.Stats{}, nil
}

func (m *mockQueryRepo) Clear(_ context.Context) error {
	m.repos = []storage.StoredRepo{}
	return nil
}

func (m *mockQueryRepo) Close() error {
	return nil
}

func (m *mockQueryRepo) GetRepositoriesNeedingMetricsUpdate(_ context.Context, _ int) ([]string, error) {
	return []string{}, nil
}

func (m *mockQueryRepo) GetRepositoriesNeedingSummaryUpdate(_ context.Context, _ bool) ([]string, error) {
	return []string{}, nil
}

func (m *mockQueryRepo) UpdateRepositoryEmbedding(_ context.Context, _ string, _ []float32) error {
	return nil
}

func (m *mockQueryRepo) UpdateRepositorySummary(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockQueryRepo) UpdateRepositoryMetrics(_ context.Context, _ string, _ storage.RepositoryMetrics) error {
	return nil
}

func (m *mockQueryRepo) RebuildFTSIndex(_ context.Context) error {
	return nil
}

func (m *mockQueryRepo) SearchByEmbedding(_ context.Context, _ []float32, _ int, _ float64) ([]storage.SearchResult, error) {
	return nil, nil
}

func TestSearchVector_ReturnsErrorWithoutEmbeddings(t *testing.T) {
	mockRepo := &mockQueryRepo{
		repos: []storage.StoredRepo{
			{
				FullName:    "test/repo1",
				Description: "A test repository for searching",
				Language:    "Go",
			},
		},
	}

	engine := NewSearchEngine(mockRepo, nil)
	ctx := context.Background()

	q := Query{
		Raw:  "test repository",
		Mode: ModeVector,
	}

	_, err := engine.Search(ctx, q, SearchOptions{Limit: 10})

	require.Error(t, err, "should return error when embeddings are unavailable")
	assert.Contains(t, err.Error(), "vector search requires embeddings")
}

func TestSearchEngine_ModeFuzzy(t *testing.T) {
	mockRepo := &mockQueryRepo{
		repos: []storage.StoredRepo{
			{
				FullName:    "user/golang-project",
				Description: "A Go programming language project",
				Language:    "Go",
			},
		},
	}

	engine := NewSearchEngine(mockRepo, nil)
	ctx := context.Background()

	q := Query{
		Raw:  "golang",
		Mode: ModeFuzzy,
	}

	results, err := engine.Search(ctx, q, SearchOptions{Limit: 10})

	require.NoError(t, err)
	assert.NotEmpty(t, results, "fuzzy search should return results")
}

func TestSearchEngine_DefaultModeIsFuzzy(t *testing.T) {
	mockRepo := &mockQueryRepo{
		repos: []storage.StoredRepo{
			{
				FullName:    "user/test-repo",
				Description: "Test repository",
			},
		},
	}

	engine := NewSearchEngine(mockRepo, nil)
	ctx := context.Background()

	q := Query{
		Raw:  "test",
		Mode: "",
	}

	results, err := engine.Search(ctx, q, SearchOptions{Limit: 10})

	require.NoError(t, err, "empty mode should default to fuzzy search")
	assert.NotEmpty(t, results)
}

func TestSearchEngine_EmptyQuery(t *testing.T) {
	mockRepo := &mockQueryRepo{
		repos: []storage.StoredRepo{
			{FullName: "user/repo1"},
			{FullName: "user/repo2"},
		},
	}

	engine := NewSearchEngine(mockRepo, nil)
	ctx := context.Background()

	q := Query{
		Raw:  "",
		Mode: ModeFuzzy,
	}

	results, err := engine.Search(ctx, q, SearchOptions{Limit: 10})

	require.NoError(t, err, "empty query should not error")
	assert.NotNil(t, results)
}

func TestSearchEngine_LimitResults(t *testing.T) {
	repos := make([]storage.StoredRepo, 20)
	for i := range 20 {
		repos[i] = storage.StoredRepo{
			FullName:    "user/repo" + string(rune(i)),
			Description: "Test repository",
		}
	}

	mockRepo := &mockQueryRepo{repos: repos}
	engine := NewSearchEngine(mockRepo, nil)
	ctx := context.Background()

	q := Query{
		Raw:  "test",
		Mode: ModeFuzzy,
	}

	results, err := engine.Search(ctx, q, SearchOptions{Limit: 5})

	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 5, "should respect limit option")
}

func TestSearchEngine_NoResults(t *testing.T) {
	mockRepo := &mockQueryRepo{
		repos: []storage.StoredRepo{
			{
				FullName:    "user/golang-project",
				Description: "Go project",
			},
		},
	}

	engine := NewSearchEngine(mockRepo, nil)
	ctx := context.Background()

	q := Query{
		Raw:  "python django flask",
		Mode: ModeFuzzy,
	}

	results, err := engine.Search(ctx, q, SearchOptions{Limit: 10})

	require.NoError(t, err, "no match should not error")
	assert.NotNil(t, results, "should return empty results slice, not nil")
}

func TestSearchEngine_ContextCancellation(t *testing.T) {
	mockRepo := &mockQueryRepo{
		repos: []storage.StoredRepo{{FullName: "user/repo"}},
	}

	engine := NewSearchEngine(mockRepo, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	q := Query{
		Raw:  "test",
		Mode: ModeFuzzy,
	}

	_, err := engine.Search(ctx, q, SearchOptions{Limit: 10})

	assert.Error(t, err, "should return error when context is canceled")
}

func TestSearchEngine_CaseInsensitiveSearch(t *testing.T) {
	mockRepo := &mockQueryRepo{
		repos: []storage.StoredRepo{
			{
				FullName:    "user/GoLang-Project",
				Description: "A GOLANG project",
				Language:    "Go",
			},
		},
	}

	engine := NewSearchEngine(mockRepo, nil)
	ctx := context.Background()

	tests := []struct {
		name  string
		query string
	}{
		{"lowercase", "golang"},
		{"uppercase", "GOLANG"},
		{"mixedcase", "GoLang"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Query{
				Raw:  tt.query,
				Mode: ModeFuzzy,
			}

			results, err := engine.Search(ctx, q, SearchOptions{Limit: 10})

			require.NoError(t, err)
			assert.NotEmpty(t, results, "case insensitive search should find results for %s", tt.query)
		})
	}
}
