package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/kyleking/gh-star-search/internal/processor"
	"github.com/kyleking/gh-star-search/internal/storage"
)

// MockRepository implements storage.Repository for testing
type MockRepository struct {
	repos  []storage.StoredRepo
	stats  *storage.Stats
	closed bool
}

func (m *MockRepository) Initialize(ctx context.Context) error {
	return nil
}

func (m *MockRepository) StoreRepository(ctx context.Context, repo processor.ProcessedRepo) error {
	return nil
}

func (m *MockRepository) UpdateRepository(ctx context.Context, repo processor.ProcessedRepo) error {
	return nil
}

func (m *MockRepository) DeleteRepository(ctx context.Context, fullName string) error {
	return nil
}

func (m *MockRepository) SearchRepositories(ctx context.Context, query string) ([]storage.SearchResult, error) {
	return nil, nil
}

func (m *MockRepository) GetRepository(ctx context.Context, fullName string) (*storage.StoredRepo, error) {
	for _, repo := range m.repos {
		if repo.FullName == fullName {
			return &repo, nil
		}
	}
	return nil, fmt.Errorf("repository not found: %s", fullName)
}

func (m *MockRepository) ListRepositories(ctx context.Context, limit, offset int) ([]storage.StoredRepo, error) {
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

func (m *MockRepository) GetStats(ctx context.Context) (*storage.Stats, error) {
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

func (m *MockRepository) Clear(ctx context.Context) error {
	m.repos = []storage.StoredRepo{}
	return nil
}

func (m *MockRepository) Close() error {
	m.closed = true
	return nil
}

func (m *MockRepository) GetRepositoriesNeedingMetricsUpdate(ctx context.Context, staleDays int) ([]string, error) {
	return []string{}, nil
}

func (m *MockRepository) GetRepositoriesNeedingSummaryUpdate(ctx context.Context, forceUpdate bool) ([]string, error) {
	return []string{}, nil
}

func (m *MockRepository) InitializeWithPrompt(ctx context.Context, autoApprove bool) error {
	return nil
}

func (m *MockRepository) UpdateRepositoryEmbedding(ctx context.Context, fullName string, embedding []float32) error {
	return nil
}

func (m *MockRepository) UpdateRepositoryMetrics(ctx context.Context, fullName string, metrics storage.RepositoryMetrics) error {
	return nil
}
