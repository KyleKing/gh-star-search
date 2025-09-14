package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	Database   DatabaseConfig  `json:"database"`
	GitHub     GitHubConfig    `json:"github"`
	Cache      CacheConfig     `json:"cache"`
	Logging    LoggingConfig   `json:"logging"`
	Debug      DebugConfig     `json:"debug"`
	Search     SearchConfig    `json:"search"`
	Embeddings EmbeddingConfig `json:"embeddings"`
	Summary    SummaryConfig   `json:"summary"`
	Refresh    RefreshConfig   `json:"refresh"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Path           string `json:"path"`
	MaxConnections int    `json:"max_connections"`
	QueryTimeout   string `json:"query_timeout"`
}

// GitHubConfig represents GitHub API configuration
type GitHubConfig struct {
	RateLimit     int    `json:"rate_limit"`
	RetryAttempts int    `json:"retry_attempts"`
	Timeout       string `json:"timeout"`
}

// CacheConfig represents caching configuration
type CacheConfig struct {
	Directory         string `json:"directory"`
	MaxSizeMB         int    `json:"max_size_mb"`
	TTLHours          int    `json:"ttl_hours"`
	CleanupFreq       string `json:"cleanup_frequency"`
	MetadataStaleDays int    `json:"metadata_stale_days"`
	StatsStaleDays    int    `json:"stats_stale_days"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string `json:"level"`        // debug, info, warn, error
	Format     string `json:"format"`       // text, json
	Output     string `json:"output"`       // stdout, stderr, file
	File       string `json:"file"`         // log file path when output is file
	MaxSizeMB  int    `json:"max_size_mb"`  // max log file size
	MaxBackups int    `json:"max_backups"`  // max number of backup files
	MaxAgeDays int    `json:"max_age_days"` // max age of log files
}

// DebugConfig represents debug configuration
type DebugConfig struct {
	Enabled     bool `json:"enabled"`
	ProfilePort int  `json:"profile_port"`
	MetricsPort int  `json:"metrics_port"`
	Verbose     bool `json:"verbose"`
	TraceAPI    bool `json:"trace_api"`
}

// SearchConfig represents search configuration
type SearchConfig struct {
	DefaultMode string  `json:"default_mode"` // "fuzzy" or "vector"
	MinScore    float64 `json:"min_score"`    // Minimum score threshold
}

// EmbeddingConfig represents embedding configuration
type EmbeddingConfig struct {
	Provider   string            `json:"provider"`   // "local" or "remote"
	Model      string            `json:"model"`      // Model name/path
	Dimensions int               `json:"dimensions"` // Expected embedding dimensions
	Enabled    bool              `json:"enabled"`    // Whether embeddings are enabled
	Options    map[string]string `json:"options"`    // Provider-specific options
}

// SummaryConfig represents summarization configuration
type SummaryConfig struct {
	Version           int    `json:"version"`            // Summary format version
	TransformersModel string `json:"transformers_model"` // Python transformers model
	Enabled           bool   `json:"enabled"`            // Whether summarization is enabled
}

// RefreshConfig represents refresh and caching configuration
type RefreshConfig struct {
	MetadataStaleDays int  `json:"metadata_stale_days"` // Days before metadata refresh
	StatsStaleDays    int  `json:"stats_stale_days"`    // Days before stats refresh
	ForceSummary      bool `json:"force_summary"`       // Force summary regeneration
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Path:           "~/.config/gh-star-search/database.db",
			MaxConnections: 10,
			QueryTimeout:   "30s",
		},
		GitHub: GitHubConfig{
			RateLimit:     5000,
			RetryAttempts: 3,
			Timeout:       "30s",
		},
		Cache: CacheConfig{
			Directory:         "~/.cache/gh-star-search",
			MaxSizeMB:         500,
			TTLHours:          24,
			CleanupFreq:       "1h",
			MetadataStaleDays: 14,
			StatsStaleDays:    7,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "text",
			Output:     "stdout",
			File:       "~/.config/gh-star-search/logs/app.log",
			MaxSizeMB:  10,
			MaxBackups: 5,
			MaxAgeDays: 30,
		},
		Debug: DebugConfig{
			Enabled:     false,
			ProfilePort: 6060,
			MetricsPort: 8080,
			Verbose:     false,
			TraceAPI:    false,
		},
		Search: SearchConfig{
			DefaultMode: "fuzzy",
			MinScore:    0.0,
		},
		Embeddings: EmbeddingConfig{
			Provider:   "local",
			Model:      "sentence-transformers/all-MiniLM-L6-v2",
			Dimensions: 384,
			Enabled:    false,
			Options:    make(map[string]string),
		},
		Summary: SummaryConfig{
			Version:           1,
			TransformersModel: "distilbart-cnn-12-6",
			Enabled:           true,
		},
		Refresh: RefreshConfig{
			MetadataStaleDays: 14,
			StatsStaleDays:    7,
			ForceSummary:      false,
		},
	}
}

// LoadConfig loads configuration from file, environment variables, and command-line flags
func LoadConfig() (*Config, error) {
	return LoadConfigWithOverrides(nil)
}

// LoadConfigWithOverrides loads configuration with optional command-line flag overrides
func LoadConfigWithOverrides(flagOverrides map[string]interface{}) (*Config, error) {
	// Start with default configuration
	config := DefaultConfig()

	// Load from config file if it exists
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		if err := loadConfigFromFile(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Apply environment variable overrides
	if err := applyEnvironmentOverrides(config); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Apply command-line flag overrides
	if flagOverrides != nil {
		if err := applyFlagOverrides(config, flagOverrides); err != nil {
			return nil, fmt.Errorf("failed to apply flag overrides: %w", err)
		}
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadConfigFromFile loads configuration from a JSON file
func loadConfigFromFile(config *Config, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON into a temporary struct to merge with defaults
	var fileConfig Config
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge file config with defaults
	mergeConfigs(config, &fileConfig)

	return nil
}

// applyEnvironmentOverrides applies environment variable overrides to configuration
func applyEnvironmentOverrides(config *Config) error {
	// Database configuration
	if val := os.Getenv("GH_STAR_SEARCH_DB_PATH"); val != "" {
		config.Database.Path = val
	}

	if val := os.Getenv("GH_STAR_SEARCH_DB_MAX_CONNECTIONS"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.Database.MaxConnections = intVal
		}
	}

	if val := os.Getenv("GH_STAR_SEARCH_DB_QUERY_TIMEOUT"); val != "" {
		config.Database.QueryTimeout = val
	}

	// GitHub configuration
	if val := os.Getenv("GH_STAR_SEARCH_GITHUB_RATE_LIMIT"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.GitHub.RateLimit = intVal
		}
	}

	if val := os.Getenv("GH_STAR_SEARCH_GITHUB_RETRY_ATTEMPTS"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.GitHub.RetryAttempts = intVal
		}
	}

	if val := os.Getenv("GH_STAR_SEARCH_GITHUB_TIMEOUT"); val != "" {
		config.GitHub.Timeout = val
	}

	// Cache configuration
	if val := os.Getenv("GH_STAR_SEARCH_CACHE_DIR"); val != "" {
		config.Cache.Directory = val
	}

	if val := os.Getenv("GH_STAR_SEARCH_CACHE_MAX_SIZE_MB"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.Cache.MaxSizeMB = intVal
		}
	}

	if val := os.Getenv("GH_STAR_SEARCH_CACHE_TTL_HOURS"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.Cache.TTLHours = intVal
		}
	}

	if val := os.Getenv("GH_STAR_SEARCH_CACHE_METADATA_STALE_DAYS"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.Cache.MetadataStaleDays = intVal
		}
	}

	if val := os.Getenv("GH_STAR_SEARCH_CACHE_STATS_STALE_DAYS"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.Cache.StatsStaleDays = intVal
		}
	}

	// Logging configuration
	if val := os.Getenv("GH_STAR_SEARCH_LOG_LEVEL"); val != "" {
		config.Logging.Level = val
	}

	if val := os.Getenv("GH_STAR_SEARCH_LOG_FORMAT"); val != "" {
		config.Logging.Format = val
	}

	if val := os.Getenv("GH_STAR_SEARCH_LOG_OUTPUT"); val != "" {
		config.Logging.Output = val
	}

	if val := os.Getenv("GH_STAR_SEARCH_LOG_FILE"); val != "" {
		config.Logging.File = val
	}

	// Debug configuration
	if val := os.Getenv("GH_STAR_SEARCH_DEBUG"); val != "" {
		config.Debug.Enabled = strings.ToLower(val) == "true"
	}

	if val := os.Getenv("GH_STAR_SEARCH_VERBOSE"); val != "" {
		config.Debug.Verbose = strings.ToLower(val) == "true"
	}

	return nil
}

// applyFlagOverrides applies command-line flag overrides to configuration
func applyFlagOverrides(config *Config, overrides map[string]interface{}) error {
	for key, value := range overrides {
		switch key {
		case "db-path":
			if str, ok := value.(string); ok && str != "" {
				config.Database.Path = str
			}
		case "log-level":
			if str, ok := value.(string); ok && str != "" {
				config.Logging.Level = str
			}
		case "verbose":
			if b, ok := value.(bool); ok {
				config.Debug.Verbose = b
			}
		case "debug":
			if b, ok := value.(bool); ok {
				config.Debug.Enabled = b
			}
		case "cache-dir":
			if str, ok := value.(string); ok && str != "" {
				config.Cache.Directory = str
			}
		}
	}

	return nil
}

// mergeConfigs merges source configuration into target configuration
func mergeConfigs(target, source *Config) {
	// Database
	if source.Database.Path != "" {
		target.Database.Path = source.Database.Path
	}

	if source.Database.MaxConnections > 0 {
		target.Database.MaxConnections = source.Database.MaxConnections
	}

	if source.Database.QueryTimeout != "" {
		target.Database.QueryTimeout = source.Database.QueryTimeout
	}

	// GitHub
	if source.GitHub.RateLimit > 0 {
		target.GitHub.RateLimit = source.GitHub.RateLimit
	}

	if source.GitHub.RetryAttempts > 0 {
		target.GitHub.RetryAttempts = source.GitHub.RetryAttempts
	}

	if source.GitHub.Timeout != "" {
		target.GitHub.Timeout = source.GitHub.Timeout
	}

	// Cache
	if source.Cache.Directory != "" {
		target.Cache.Directory = source.Cache.Directory
	}

	if source.Cache.MaxSizeMB > 0 {
		target.Cache.MaxSizeMB = source.Cache.MaxSizeMB
	}

	if source.Cache.TTLHours > 0 {
		target.Cache.TTLHours = source.Cache.TTLHours
	}

	if source.Cache.CleanupFreq != "" {
		target.Cache.CleanupFreq = source.Cache.CleanupFreq
	}

	if source.Cache.MetadataStaleDays > 0 {
		target.Cache.MetadataStaleDays = source.Cache.MetadataStaleDays
	}

	if source.Cache.StatsStaleDays > 0 {
		target.Cache.StatsStaleDays = source.Cache.StatsStaleDays
	}

	// Logging
	if source.Logging.Level != "" {
		target.Logging.Level = source.Logging.Level
	}

	if source.Logging.Format != "" {
		target.Logging.Format = source.Logging.Format
	}

	if source.Logging.Output != "" {
		target.Logging.Output = source.Logging.Output
	}

	if source.Logging.File != "" {
		target.Logging.File = source.Logging.File
	}

	if source.Logging.MaxSizeMB > 0 {
		target.Logging.MaxSizeMB = source.Logging.MaxSizeMB
	}

	if source.Logging.MaxBackups > 0 {
		target.Logging.MaxBackups = source.Logging.MaxBackups
	}

	if source.Logging.MaxAgeDays > 0 {
		target.Logging.MaxAgeDays = source.Logging.MaxAgeDays
	}

	// Debug
	target.Debug.Enabled = source.Debug.Enabled
	target.Debug.Verbose = source.Debug.Verbose
	target.Debug.TraceAPI = source.Debug.TraceAPI

	if source.Debug.ProfilePort > 0 {
		target.Debug.ProfilePort = source.Debug.ProfilePort
	}

	if source.Debug.MetricsPort > 0 {
		target.Debug.MetricsPort = source.Debug.MetricsPort
	}

	// Search
	if source.Search.DefaultMode != "" {
		target.Search.DefaultMode = source.Search.DefaultMode
	}

	if source.Search.MinScore > 0 {
		target.Search.MinScore = source.Search.MinScore
	}

	// Embeddings
	if source.Embeddings.Provider != "" {
		target.Embeddings.Provider = source.Embeddings.Provider
	}

	if source.Embeddings.Model != "" {
		target.Embeddings.Model = source.Embeddings.Model
	}

	if source.Embeddings.Dimensions > 0 {
		target.Embeddings.Dimensions = source.Embeddings.Dimensions
	}

	target.Embeddings.Enabled = source.Embeddings.Enabled
	if source.Embeddings.Options != nil {
		target.Embeddings.Options = source.Embeddings.Options
	}

	// Summary
	if source.Summary.Version > 0 {
		target.Summary.Version = source.Summary.Version
	}

	if source.Summary.TransformersModel != "" {
		target.Summary.TransformersModel = source.Summary.TransformersModel
	}

	target.Summary.Enabled = source.Summary.Enabled

	// Refresh
	if source.Refresh.MetadataStaleDays > 0 {
		target.Refresh.MetadataStaleDays = source.Refresh.MetadataStaleDays
	}

	if source.Refresh.StatsStaleDays > 0 {
		target.Refresh.StatsStaleDays = source.Refresh.StatsStaleDays
	}

	target.Refresh.ForceSummary = source.Refresh.ForceSummary
}

// validateConfig validates the configuration for common errors
func validateConfig(config *Config) error {
	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[strings.ToLower(config.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", config.Logging.Level)
	}

	// Validate log format
	validLogFormats := map[string]bool{
		"text": true, "json": true,
	}
	if !validLogFormats[strings.ToLower(config.Logging.Format)] {
		return fmt.Errorf("invalid log format: %s (must be text or json)", config.Logging.Format)
	}

	// Validate log output
	validLogOutputs := map[string]bool{
		"stdout": true, "stderr": true, "file": true,
	}
	if !validLogOutputs[strings.ToLower(config.Logging.Output)] {
		return fmt.Errorf("invalid log output: %s (must be stdout, stderr, or file)", config.Logging.Output)
	}

	// Validate timeout durations
	if _, err := time.ParseDuration(config.Database.QueryTimeout); err != nil {
		return fmt.Errorf("invalid database query timeout: %s", config.Database.QueryTimeout)
	}

	if _, err := time.ParseDuration(config.GitHub.Timeout); err != nil {
		return fmt.Errorf("invalid GitHub timeout: %s", config.GitHub.Timeout)
	}

	if _, err := time.ParseDuration(config.Cache.CleanupFreq); err != nil {
		return fmt.Errorf("invalid cache cleanup frequency: %s", config.Cache.CleanupFreq)
	}

	// Validate numeric values
	if config.Database.MaxConnections <= 0 {
		return fmt.Errorf("database max connections must be positive: %d", config.Database.MaxConnections)
	}

	if config.GitHub.RateLimit <= 0 {
		return fmt.Errorf("GitHub rate limit must be positive: %d", config.GitHub.RateLimit)
	}

	if config.GitHub.RetryAttempts < 0 {
		return fmt.Errorf("GitHub retry attempts must be non-negative: %d", config.GitHub.RetryAttempts)
	}

	return nil
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
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfigPath returns the path to the configuration file
func getConfigPath() string {
	// Check for custom config path from environment
	if configPath := os.Getenv("GH_STAR_SEARCH_CONFIG"); configPath != "" {
		return expandPath(configPath)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./config.json"
	}

	return filepath.Join(homeDir, ".config", "gh-star-search", "config.json")
}

// expandPath expands ~ to home directory in file paths
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == "~" {
		return homeDir
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}

	return path
}

// ExpandAllPaths expands all paths in the configuration
func (c *Config) ExpandAllPaths() {
	c.Database.Path = expandPath(c.Database.Path)
	c.Cache.Directory = expandPath(c.Cache.Directory)
	c.Logging.File = expandPath(c.Logging.File)
}

// GetConfigDir returns the configuration directory
func GetConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".config/gh-star-search"
	}

	return filepath.Join(homeDir, ".config", "gh-star-search")
}

// GetCacheDir returns the cache directory
func GetCacheDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".cache/gh-star-search"
	}

	return filepath.Join(homeDir, ".cache", "gh-star-search")
}

// GetLogDir returns the log directory
func GetLogDir() string {
	return filepath.Join(GetConfigDir(), "logs")
}

// EnsureDirectories creates necessary directories for the configuration
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		filepath.Dir(c.Database.Path),
		c.Cache.Directory,
		filepath.Dir(c.Logging.File),
	}

	for _, dir := range dirs {
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}

	return nil
}
