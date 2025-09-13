package config

import (
	"github.com/kyleking/gh-star-search/internal/llm"
)

// Config represents the application configuration
type Config struct {
	Database DatabaseConfig `json:"database"`
	LLM      LLMConfig      `json:"llm"`
	GitHub   GitHubConfig   `json:"github"`
	Cache    CacheConfig    `json:"cache"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Path           string `json:"path"`
	MaxConnections int    `json:"max_connections"`
	QueryTimeout   string `json:"query_timeout"`
}

// LLMConfig represents LLM service configuration
type LLMConfig struct {
	DefaultProvider string                `json:"default_provider"`
	Providers       map[string]llm.Config `json:"providers"`
	MaxTokens       int                   `json:"max_tokens"`
	Temperature     float64               `json:"temperature"`
}

// GitHubConfig represents GitHub API configuration
type GitHubConfig struct {
	RateLimit     int    `json:"rate_limit"`
	RetryAttempts int    `json:"retry_attempts"`
	Timeout       string `json:"timeout"`
}

// CacheConfig represents caching configuration
type CacheConfig struct {
	Directory   string `json:"directory"`
	MaxSizeMB   int    `json:"max_size_mb"`
	TTLHours    int    `json:"ttl_hours"`
	CleanupFreq string `json:"cleanup_frequency"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Path:           "~/.config/gh-star-search/database.db",
			MaxConnections: 10,
			QueryTimeout:   "30s",
		},
		LLM: LLMConfig{
			DefaultProvider: llm.ProviderOpenAI,
			Providers: map[string]llm.Config{
				llm.ProviderOpenAI: {
					Provider: llm.ProviderOpenAI,
					Model:    llm.ModelGPT35Turbo,
				},
			},
			MaxTokens:   2000,
			Temperature: 0.1,
		},
		GitHub: GitHubConfig{
			RateLimit:     5000,
			RetryAttempts: 3,
			Timeout:       "30s",
		},
		Cache: CacheConfig{
			Directory:   "~/.cache/gh-star-search",
			MaxSizeMB:   500,
			TTLHours:    24,
			CleanupFreq: "1h",
		},
	}
}
