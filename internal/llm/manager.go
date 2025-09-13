package llm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/kyleking/gh-star-search/internal/types"
)

// Manager handles multiple LLM providers with fallback strategies
type Manager struct {
	providers map[string]Service
	fallback  Service
	config    ManagerConfig
}

// ManagerConfig configures the LLM manager behavior
type ManagerConfig struct {
	DefaultProvider   string        `json:"default_provider"`
	FallbackProviders []string      `json:"fallback_providers"`
	RetryAttempts     int           `json:"retry_attempts"`
	RetryDelay        time.Duration `json:"retry_delay"`
	Timeout           time.Duration `json:"timeout"`
	EnableFallback    bool          `json:"enable_fallback"`
}

// NewManager creates a new LLM manager with the given configuration
func NewManager(config ManagerConfig) *Manager {
	return &Manager{
		providers: make(map[string]Service),
		fallback:  NewFallbackService(),
		config:    config,
	}
}

// NewManagerFromConfig creates a new LLM manager from LLMConfig
func NewManagerFromConfig(llmConfig interface{}) (Service, error) {
	// For now, return a simple fallback service since we don't have actual LLM providers implemented
	// In a real implementation, this would initialize the appropriate providers based on the config
	return NewFallbackService(), nil
}

// RegisterProvider registers a new LLM provider
func (m *Manager) RegisterProvider(name string, service Service) error {
	if name == "" {
		return errors.New("provider name cannot be empty")
	}

	if service == nil {
		return errors.New("service cannot be nil")
	}

	m.providers[name] = service

	return nil
}

// Configure configures a specific provider
func (m *Manager) Configure(config Config) error {
	provider, exists := m.providers[config.Provider]
	if !exists {
		return fmt.Errorf("provider %s not registered", config.Provider)
	}

	return provider.Configure(config)
}

// Summarize generates a summary using the configured providers with fallback
func (m *Manager) Summarize(ctx context.Context, prompt string, content string) (*SummaryResponse, error) {
	// Create context with timeout
	if m.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.config.Timeout)
		defer cancel()
	}

	// Try default provider first
	if m.config.DefaultProvider != "" {
		if provider, exists := m.providers[m.config.DefaultProvider]; exists {
			response, err := m.tryProviderSummarize(ctx, provider, prompt, content)
			if err == nil {
				return response, nil
			}

			log.Printf("Default provider %s failed: %v", m.config.DefaultProvider, err)
		}
	}

	// Try fallback providers
	for _, providerName := range m.config.FallbackProviders {
		if provider, exists := m.providers[providerName]; exists {
			response, err := m.tryProviderSummarize(ctx, provider, prompt, content)
			if err == nil {
				return response, nil
			}

			log.Printf("Fallback provider %s failed: %v", providerName, err)
		}
	}

	// Use rule-based fallback if enabled
	if m.config.EnableFallback {
		log.Println("Using rule-based fallback for summarization")
		return m.fallback.Summarize(ctx, prompt, content)
	}

	return nil, errors.New("all LLM providers failed and fallback is disabled")
}

// ParseQuery converts natural language to SQL using the configured providers with fallback
func (m *Manager) ParseQuery(ctx context.Context, query string, schema types.Schema) (*QueryResponse, error) {
	// Create context with timeout
	if m.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.config.Timeout)
		defer cancel()
	}

	// Try default provider first
	if m.config.DefaultProvider != "" {
		if provider, exists := m.providers[m.config.DefaultProvider]; exists {
			response, err := m.tryProviderParseQuery(ctx, provider, query, schema)
			if err == nil {
				return response, nil
			}

			log.Printf("Default provider %s failed: %v", m.config.DefaultProvider, err)
		}
	}

	// Try fallback providers
	for _, providerName := range m.config.FallbackProviders {
		if provider, exists := m.providers[providerName]; exists {
			response, err := m.tryProviderParseQuery(ctx, provider, query, schema)
			if err == nil {
				return response, nil
			}

			log.Printf("Fallback provider %s failed: %v", providerName, err)
		}
	}

	// Use rule-based fallback if enabled
	if m.config.EnableFallback {
		log.Println("Using rule-based fallback for query parsing")
		return m.fallback.ParseQuery(ctx, query, schema)
	}

	return nil, errors.New("all LLM providers failed and fallback is disabled")
}

// tryProviderSummarize attempts to use a provider for summarization with retries
func (m *Manager) tryProviderSummarize(ctx context.Context, provider Service, prompt string, content string) (*SummaryResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= m.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(m.config.RetryDelay):
			}
		}

		response, err := provider.Summarize(ctx, prompt, content)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			break
		}
	}

	return nil, fmt.Errorf("provider failed after %d attempts: %w", m.config.RetryAttempts+1, lastErr)
}

// tryProviderParseQuery attempts to use a provider for query parsing with retries
func (m *Manager) tryProviderParseQuery(ctx context.Context, provider Service, query string, schema types.Schema) (*QueryResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= m.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(m.config.RetryDelay):
			}
		}

		response, err := provider.ParseQuery(ctx, query, schema)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			break
		}
	}

	return nil, fmt.Errorf("provider failed after %d attempts: %w", m.config.RetryAttempts+1, lastErr)
}

// GetAvailableProviders returns a list of registered provider names
func (m *Manager) GetAvailableProviders() []string {
	var providers []string
	for name := range m.providers {
		providers = append(providers, name)
	}

	return providers
}

// IsProviderRegistered checks if a provider is registered
func (m *Manager) IsProviderRegistered(name string) bool {
	_, exists := m.providers[name]
	return exists
}

// DefaultManagerConfig returns a sensible default configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		DefaultProvider:   ProviderOpenAI,
		FallbackProviders: []string{ProviderAnthropic, ProviderLocal},
		RetryAttempts:     2,
		RetryDelay:        time.Second * 2,
		Timeout:           time.Minute * 2,
		EnableFallback:    true,
	}
}
