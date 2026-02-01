package embedding

import (
	"context"
	"errors"
	"fmt"
)

const (
	defaultEmbeddingDimensions = 384
	zeroDimensions             = 0
)

// Provider defines the interface for embedding providers
type Provider interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GetDimensions() int
	IsEnabled() bool
	GetName() string
}

// Config represents embedding provider configuration
type Config struct {
	Provider   string            `json:"provider"`
	Model      string            `json:"model"`
	Dimensions int               `json:"dimensions"`
	Enabled    bool              `json:"enabled"`
	Options    map[string]string `json:"options"`
}

// DefaultConfig returns default embedding configuration
func DefaultConfig() Config {
	return Config{
		Provider:   "local",
		Model:      "sentence-transformers/all-MiniLM-L6-v2",
		Dimensions: defaultEmbeddingDimensions,
		Enabled:    false,
		Options:    make(map[string]string),
	}
}

// NewProvider creates a Provider based on configuration.
// Returns a DisabledProvider if embeddings are not enabled.
// For local providers, uvPath and projectDir must be provided.
func NewProvider(config Config, uvPath, projectDir string) (Provider, error) {
	if !config.Enabled {
		return &DisabledProvider{}, nil
	}

	var provider Provider
	var err error

	switch config.Provider {
	case "local":
		provider, err = NewLocalProvider(config, uvPath, projectDir)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", config.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize embedding provider: %w", err)
	}

	if provider.GetDimensions() != config.Dimensions {
		return nil, fmt.Errorf("dimension mismatch: expected %d, got %d",
			config.Dimensions, provider.GetDimensions())
	}

	return provider, nil
}

// DisabledProvider is a no-op provider for when embeddings are disabled
type DisabledProvider struct{}

func (p *DisabledProvider) GenerateEmbedding(_ context.Context, _ string) ([]float32, error) {
	return nil, errors.New("embedding provider is disabled")
}

func (*DisabledProvider) GetDimensions() int { return zeroDimensions }
func (*DisabledProvider) IsEnabled() bool    { return false }
func (*DisabledProvider) GetName() string    { return "disabled" }
