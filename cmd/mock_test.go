package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/KyleKing/gh-star-search/internal/processor"
	"github.com/KyleKing/gh-star-search/internal/storage"
)

// MockRepository implements storage.Repository for testing
type MockRepository struct {
	repos  []storage.StoredRepo
	stats  *storage.Stats
	closed bool
}

func (m *MockRepository) Initialize(_ context.Context) error {
	return nil
}

func (m *MockRepository) StoreRepository(_ context.Context, _ processor.ProcessedRepo) error {
	return nil
}

func (m *MockRepository) UpdateRepository(_ context.Context, _ processor.ProcessedRepo) error {
	return nil
}

func (m *MockRepository) DeleteRepository(_ context.Context, _ string) error {
	return nil
}

func (m *MockRepository) SearchRepositories(
	_ context.Context,
	_ string,
) ([]storage.SearchResult, error) {
	return nil, nil
}

func (m *MockRepository) GetRepository(
	_ context.Context,
	fullName string,
) (*storage.StoredRepo, error) {
	for _, repo := range m.repos {
		if repo.FullName == fullName {
			return &repo, nil
		}
	}

	return nil, fmt.Errorf("repository not found: %s", fullName)
}

func (m *MockRepository) ListRepositories(
	_ context.Context,
	limit, offset int,
) ([]storage.StoredRepo, error) {
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

func (m *MockRepository) GetStats(_ context.Context) (*storage.Stats, error) {
	if m.stats != nil {
		return m.stats, nil
	}

	return &storage.Stats{
		TotalRepositories:  len(m.repos),
		TotalContentChunks: 0,
		DatabaseSizeMB:     1.5,
		LastSyncTime:       time.Now(),
		LanguageBreakdown:  make(map[string]int),
		TopicBreakdown:     make(map[string]int),
	}, nil
}

func (m *MockRepository) Clear(_ context.Context) error {
	m.repos = []storage.StoredRepo{}
	return nil
}

func (m *MockRepository) Close() error {
	m.closed = true
	return nil
}

func (m *MockRepository) GetRepositoriesNeedingMetricsUpdate(
	_ context.Context,
	_ int,
) ([]string, error) {
	return []string{}, nil
}

func (m *MockRepository) GetRepositoriesNeedingSummaryUpdate(
	_ context.Context,
	_ bool,
) ([]string, error) {
	return []string{}, nil
}

func (m *MockRepository) InitializeWithPrompt(_ context.Context, _ bool) error {
	return nil
}

func (m *MockRepository) UpdateRepositoryEmbedding(_ context.Context, _ string, _ []float32) error {
	return nil
}

func (m *MockRepository) UpdateRepositorySummary(_ context.Context, _, _ string) error {
	return nil
}

func (m *MockRepository) UpdateRepositoryMetrics(
	_ context.Context,
	_ string,
	_ storage.RepositoryMetrics,
) error {
	return nil
}
