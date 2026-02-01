package testutil

import (
	"context"
	"fmt"
	"sync"

	"github.com/kyleking/gh-star-search/internal/github"
)

// MockGitHubClient implements github.Client for testing with enhanced error injection
type MockGitHubClient struct {
	mu sync.RWMutex

	starredRepos []github.Repository
	content      map[string][]github.Content
	metadata     map[string]*github.Metadata
	errors       map[string]error
	callCounts   map[string]int
}

// MockOption is a functional option for configuring MockGitHubClient
type MockOption func(*MockGitHubClient)

// WithStarredRepos sets the starred repositories list
func WithStarredRepos(repos []github.Repository) MockOption {
	return func(m *MockGitHubClient) {
		m.starredRepos = repos
	}
}

// WithContent sets the content map for specific repositories
func WithContent(content map[string][]github.Content) MockOption {
	return func(m *MockGitHubClient) {
		m.content = content
	}
}

// WithMetadata sets the metadata map for specific repositories
func WithMetadata(metadata map[string]*github.Metadata) MockOption {
	return func(m *MockGitHubClient) {
		m.metadata = metadata
	}
}

// WithError sets an error for a specific operation or repository
func WithError(key string, err error) MockOption {
	return func(m *MockGitHubClient) {
		m.errors[key] = err
	}
}

// NewMockGitHubClient creates a new mock GitHub client with the given options
func NewMockGitHubClient(opts ...MockOption) *MockGitHubClient {
	mock := &MockGitHubClient{
		starredRepos: []github.Repository{},
		content:      make(map[string][]github.Content),
		metadata:     make(map[string]*github.Metadata),
		errors:       make(map[string]error),
		callCounts:   make(map[string]int),
	}

	for _, opt := range opts {
		opt(mock)
	}

	return mock
}

// GetStarredRepos returns the configured starred repositories
func (m *MockGitHubClient) GetStarredRepos(
	_ context.Context,
	_ string,
) ([]github.Repository, error) {
	m.mu.Lock()
	m.callCounts["GetStarredRepos"]++
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, exists := m.errors["starred"]; exists {
		return nil, err
	}

	return m.starredRepos, nil
}

// GetRepositoryContent returns the configured content for a repository
func (m *MockGitHubClient) GetRepositoryContent(
	_ context.Context,
	repo github.Repository,
	_ []string,
) ([]github.Content, error) {
	m.mu.Lock()
	m.callCounts["GetRepositoryContent"]++
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, exists := m.errors[repo.FullName]; exists {
		return nil, err
	}

	if content, exists := m.content[repo.FullName]; exists {
		return content, nil
	}

	return []github.Content{}, nil
}

// GetRepositoryMetadata returns the configured metadata for a repository
func (m *MockGitHubClient) GetRepositoryMetadata(
	_ context.Context,
	repo github.Repository,
) (*github.Metadata, error) {
	m.mu.Lock()
	m.callCounts["GetRepositoryMetadata"]++
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, exists := m.errors[repo.FullName+":metadata"]; exists {
		return nil, err
	}

	if metadata, exists := m.metadata[repo.FullName]; exists {
		return metadata, nil
	}

	return &github.Metadata{}, nil
}

// GetHomepageText returns empty text (not implemented in mock)
func (m *MockGitHubClient) GetHomepageText(_ context.Context, _ string) (string, error) {
	m.mu.Lock()
	m.callCounts["GetHomepageText"]++
	m.mu.Unlock()

	return "", nil
}

// GetContributors returns empty contributors list (not implemented in mock)
func (m *MockGitHubClient) GetContributors(_ context.Context, _ string, _ int) ([]github.Contributor, error) {
	m.mu.Lock()
	m.callCounts["GetContributors"]++
	m.mu.Unlock()

	return []github.Contributor{}, nil
}

// GetTopics returns empty topics list (not implemented in mock)
func (m *MockGitHubClient) GetTopics(_ context.Context, _ string) ([]string, error) {
	m.mu.Lock()
	m.callCounts["GetTopics"]++
	m.mu.Unlock()

	return []string{}, nil
}

// GetLanguages returns empty languages map (not implemented in mock)
func (m *MockGitHubClient) GetLanguages(_ context.Context, _ string) (map[string]int64, error) {
	m.mu.Lock()
	m.callCounts["GetLanguages"]++
	m.mu.Unlock()

	return make(map[string]int64), nil
}

// GetCommitActivity returns empty commit activity (not implemented in mock)
func (m *MockGitHubClient) GetCommitActivity(_ context.Context, _ string) (*github.CommitActivity, error) {
	m.mu.Lock()
	m.callCounts["GetCommitActivity"]++
	m.mu.Unlock()

	return &github.CommitActivity{
		Total: 0,
		Weeks: []github.WeeklyCommits{},
	}, nil
}

// GetPullCounts returns zero counts (not implemented in mock)
func (m *MockGitHubClient) GetPullCounts(_ context.Context, _ string) (int, int, error) {
	m.mu.Lock()
	m.callCounts["GetPullCounts"]++
	m.mu.Unlock()

	return 0, 0, nil
}

// GetIssueCounts returns zero counts (not implemented in mock)
func (m *MockGitHubClient) GetIssueCounts(_ context.Context, _ string) (int, int, error) {
	m.mu.Lock()
	m.callCounts["GetIssueCounts"]++
	m.mu.Unlock()

	return 0, 0, nil
}

// GetCallCount returns the number of times a method was called
func (m *MockGitHubClient) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCounts[method]
}

// ResetCallCounts resets all call counters
func (m *MockGitHubClient) ResetCallCounts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCounts = make(map[string]int)
}

// ErrorInjector provides systematic error injection for testing
type ErrorInjector struct {
	errors map[string]error
	counts map[string]int
	mu     sync.Mutex
}

// NewErrorInjector creates a new error injector
func NewErrorInjector() *ErrorInjector {
	return &ErrorInjector{
		errors: make(map[string]error),
		counts: make(map[string]int),
	}
}

// InjectError configures an error to be returned for a specific key
func (e *ErrorInjector) InjectError(key string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errors[key] = err
}

// InjectErrorAfterN configures an error to be returned after N successful calls
func (e *ErrorInjector) InjectErrorAfterN(key string, n int, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errors[fmt.Sprintf("%s:after:%d", key, n)] = err
}

// ShouldError checks if an error should be returned for the given key
func (e *ErrorInjector) ShouldError(key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.counts[key]++

	if err, exists := e.errors[key]; exists {
		return err
	}

	for k, err := range e.errors {
		if len(k) > len(key)+7 && k[:len(key)] == key && k[len(key):len(key)+7] == ":after:" {
			var n int
			if _, scanErr := fmt.Sscanf(k[len(key)+7:], "%d", &n); scanErr == nil {
				if e.counts[key] > n {
					return err
				}
			}
		}
	}

	return nil
}

// GetCount returns the number of times a key was checked
func (e *ErrorInjector) GetCount(key string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.counts[key]
}

// Reset clears all error configurations and counts
func (e *ErrorInjector) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errors = make(map[string]error)
	e.counts = make(map[string]int)
}
