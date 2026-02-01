package embedding

import (
	"context"
	"fmt"
)

// Manager wraps an embedding Provider for use by the query engine
type Manager struct {
	provider Provider
}

// NewManager creates a Manager from the given config.
// Returns nil if embeddings are not enabled.
func NewManager(config Config, uvPath, projectDir string) (*Manager, error) {
	if !config.Enabled {
		return nil, nil
	}

	provider, err := NewProvider(config, uvPath, projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding provider: %w", err)
	}

	return &Manager{provider: provider}, nil
}

// GenerateEmbedding generates an embedding vector for the given text
func (m *Manager) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return m.provider.GenerateEmbedding(ctx, text)
}

// IsEnabled returns whether the manager's provider is enabled
func (m *Manager) IsEnabled() bool {
	return m.provider != nil && m.provider.IsEnabled()
}
