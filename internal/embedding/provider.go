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
	// GenerateEmbedding generates an embedding for the given text
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

	// GetDimensions returns the dimensionality of embeddings produced by this provider
	GetDimensions() int

	// IsEnabled returns whether the provider is enabled and ready to use
	IsEnabled() bool

	// GetName returns the provider name for identification
	GetName() string
}

// Config represents embedding provider configuration
type Config struct {
	Provider   string            `json:"provider"`   // "local" or "remote"
	Model      string            `json:"model"`      // Model name/path
	Dimensions int               `json:"dimensions"` // Expected embedding dimensions
	Enabled    bool              `json:"enabled"`    // Whether embeddings are enabled
	Options    map[string]string `json:"options"`
	// Provider-specific options
}

// DefaultConfig returns default embedding configuration
func DefaultConfig() Config {
	return Config{
		Provider:   "local",
		Model:      "sentence-transformers/all-MiniLM-L6-v2",
		Dimensions: defaultEmbeddingDimensions,
		Enabled:    false, // Disabled by default
		Options:    make(map[string]string),
	}
}

// Manager manages embedding providers
type Manager struct {
	config   Config
	provider Provider
}

// NewManager creates a new embedding manager
func NewManager(config Config) (*Manager, error) {
	manager := &Manager{
		config: config,
	}

	if !config.Enabled {
		manager.provider = &DisabledProvider{}
		return manager, nil
	}

	// Initialize provider based on configuration
	var provider Provider

	var err error

	switch config.Provider {
	case "local":
		provider, err = NewLocalProviderImpl(config)
	case "remote":
		provider, err = NewRemoteProvider(config)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s",
			config.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize embedding provider: %w",
			err)
	}

	// Validate dimensions
	if provider.GetDimensions() != config.Dimensions {
		return nil, fmt.Errorf("dimension mismatch: expected %d, got %d",
			config.Dimensions, provider.GetDimensions())
	}

	manager.provider = provider

	return manager, nil
}

// GenerateEmbedding generates an embedding using the configured provider
func (m *Manager) GenerateEmbedding(ctx context.Context,
	text string) ([]float32, error) {
	if !m.provider.IsEnabled() {
		return nil, errors.New("embedding provider is disabled")
	}

	return m.provider.GenerateEmbedding(ctx, text)
}

// IsEnabled returns whether embeddings are enabled
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled && m.provider.IsEnabled()
}

// GetDimensions returns the embedding dimensions
func (m *Manager) GetDimensions() int {
	return m.config.Dimensions
}

// DisabledProvider is a no-op provider for when embeddings are disabled
type DisabledProvider struct{}

func (p *DisabledProvider) GenerateEmbedding(_ context.Context,
	_ string) ([]float32, error) {
	return nil, errors.New("embedding provider is disabled")
}

func (*DisabledProvider) GetDimensions() int {
	return zeroDimensions
}

func (*DisabledProvider) IsEnabled() bool {
	return false
}

func (*DisabledProvider) GetName() string {
	return "disabled"
}

// RemoteProvider implements remote API embedding generation (placeholder)
type RemoteProvider struct {
	config Config
}

func NewRemoteProvider(config Config) (*RemoteProvider, error) {
	// TODO: Implement remote embedding provider
	// This would typically use OpenAI, Cohere, or similar APIs
	return &RemoteProvider{config: config}, nil
}

func (p *RemoteProvider) GenerateEmbedding(_ context.Context,
	_ string) ([]float32, error) {
	// TODO: Implement remote embedding generation
	// For now, return a placeholder embedding
	return make([]float32, p.config.Dimensions), nil
}

func (p *RemoteProvider) GetDimensions() int {
	return p.config.Dimensions
}

func (p *RemoteProvider) IsEnabled() bool {
	// TODO: Check if API key is configured and valid
	return false // Disabled for now
}

func (p *RemoteProvider) GetName() string {
	return "remote:" + p.config.Model
}
