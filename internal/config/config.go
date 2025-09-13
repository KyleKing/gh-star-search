package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

// LoadConfig loads configuration from file or returns default config
func LoadConfig() (*Config, error) {
	configPath := getConfigPath()

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Fill in missing values with defaults
	defaultConfig := DefaultConfig()
	if config.LLM.DefaultProvider == "" {
		config.LLM = defaultConfig.LLM
	}

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config) error {
	configPath := getConfigPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfigPath returns the path to the configuration file
func getConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./config.json"
	}
	return filepath.Join(homeDir, ".config", "gh-star-search", "config.json")
}
