package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config represents the application configuration
type Config struct {
	Database   DatabaseConfig  `json:"database" envPrefix:"GH_STAR_SEARCH_"`
	GitHub     GitHubConfig    `json:"github" envPrefix:"GH_STAR_SEARCH_"`
	Cache      CacheConfig     `json:"cache" envPrefix:"GH_STAR_SEARCH_"`
	Logging    LoggingConfig   `json:"logging" envPrefix:"GH_STAR_SEARCH_"`
	Debug      DebugConfig     `json:"debug" envPrefix:"GH_STAR_SEARCH_"`
	Search     SearchConfig    `json:"search" envPrefix:"GH_STAR_SEARCH_"`
	Embeddings EmbeddingConfig `json:"embeddings" envPrefix:"GH_STAR_SEARCH_"`
	Summary    SummaryConfig   `json:"summary" envPrefix:"GH_STAR_SEARCH_"`
	Refresh    RefreshConfig   `json:"refresh" envPrefix:"GH_STAR_SEARCH_"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Path           string `json:"path" env:"DB_PATH" envDefault:"~/.config/gh-star-search/database.db"`
	MaxConnections int    `json:"max_connections" env:"DB_MAX_CONNECTIONS" envDefault:"10"`
	QueryTimeout   string `json:"query_timeout" env:"DB_QUERY_TIMEOUT" envDefault:"30s"`
}

// GitHubConfig represents GitHub API configuration
type GitHubConfig struct {
	RateLimit     int    `json:"rate_limit" env:"GITHUB_RATE_LIMIT" envDefault:"5000"`
	RetryAttempts int    `json:"retry_attempts" env:"GITHUB_RETRY_ATTEMPTS" envDefault:"3"`
	Timeout       string `json:"timeout" env:"GITHUB_TIMEOUT" envDefault:"30s"`
}

// CacheConfig represents caching configuration
type CacheConfig struct {
	Directory         string `json:"directory" env:"CACHE_DIR" envDefault:"~/.cache/gh-star-search"`
	MaxSizeMB         int    `json:"max_size_mb" env:"CACHE_MAX_SIZE_MB" envDefault:"500"`
	TTLHours          int    `json:"ttl_hours" env:"CACHE_TTL_HOURS" envDefault:"24"`
	CleanupFreq       string `json:"cleanup_frequency" env:"CACHE_CLEANUP_FREQ" envDefault:"1h"`
	MetadataStaleDays int    `json:"metadata_stale_days" env:"CACHE_METADATA_STALE_DAYS" envDefault:"14"`
	StatsStaleDays    int    `json:"stats_stale_days" env:"CACHE_STATS_STALE_DAYS" envDefault:"7"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string `json:"level" env:"LOG_LEVEL" envDefault:"info"`                                // debug, info, warn, error
	Format     string `json:"format" env:"LOG_FORMAT" envDefault:"text"`                              // text, json
	Output     string `json:"output" env:"LOG_OUTPUT" envDefault:"stdout"`                            // stdout, stderr, file
	File       string `json:"file" env:"LOG_FILE" envDefault:"~/.config/gh-star-search/logs/app.log"` // log file path when output is file
	MaxSizeMB  int    `json:"max_size_mb" env:"LOG_MAX_SIZE_MB" envDefault:"10"`                      // max log file size
	MaxBackups int    `json:"max_backups" env:"LOG_MAX_BACKUPS" envDefault:"5"`                       // max number of backup files
	MaxAgeDays int    `json:"max_age_days" env:"LOG_MAX_AGE_DAYS" envDefault:"30"`                    // max age of log files
	AddSource  bool   `json:"add_source" env:"LOG_ADD_SOURCE" envDefault:"false"`                     // add source file and line info to logs
}

// DebugConfig represents debug configuration
type DebugConfig struct {
	Enabled     bool `json:"enabled" env:"DEBUG" envDefault:"false"`
	ProfilePort int  `json:"profile_port" env:"DEBUG_PROFILE_PORT" envDefault:"6060"`
	MetricsPort int  `json:"metrics_port" env:"DEBUG_METRICS_PORT" envDefault:"8080"`
	Verbose     bool `json:"verbose" env:"VERBOSE" envDefault:"false"`
	TraceAPI    bool `json:"trace_api" env:"DEBUG_TRACE_API" envDefault:"false"`
}

// SearchConfig represents search configuration
type SearchConfig struct {
	DefaultMode string  `json:"default_mode" env:"SEARCH_DEFAULT_MODE" envDefault:"fuzzy"` // "fuzzy" or "vector"
	MinScore    float64 `json:"min_score" env:"SEARCH_MIN_SCORE" envDefault:"0.0"`         // Minimum score threshold
}

// EmbeddingConfig represents embedding configuration
type EmbeddingConfig struct {
	Provider   string            `json:"provider" env:"EMBEDDINGS_PROVIDER" envDefault:"local"`                            // "local" or "remote"
	Model      string            `json:"model" env:"EMBEDDINGS_MODEL" envDefault:"sentence-transformers/all-MiniLM-L6-v2"` // Model name/path
	Dimensions int               `json:"dimensions" env:"EMBEDDINGS_DIMENSIONS" envDefault:"384"`                          // Expected embedding dimensions
	Enabled    bool              `json:"enabled" env:"EMBEDDINGS_ENABLED" envDefault:"false"`                              // Whether embeddings are enabled
	Options    map[string]string `json:"options"`                                                                          // Provider-specific options
}

// SummaryConfig represents summarization configuration
type SummaryConfig struct {
	Version           int    `json:"version" env:"SUMMARY_VERSION" envDefault:"1"`                                         // Summary format version
	TransformersModel string `json:"transformers_model" env:"SUMMARY_TRANSFORMERS_MODEL" envDefault:"distilbart-cnn-12-6"` // Python transformers model
	Enabled           bool   `json:"enabled" env:"SUMMARY_ENABLED" envDefault:"true"`                                      // Whether summarization is enabled
}

// RefreshConfig represents refresh and caching configuration
type RefreshConfig struct {
	MetadataStaleDays int  `json:"metadata_stale_days" env:"REFRESH_METADATA_STALE_DAYS" envDefault:"14"` // Days before metadata refresh
	StatsStaleDays    int  `json:"stats_stale_days" env:"REFRESH_STATS_STALE_DAYS" envDefault:"7"`        // Days before stats refresh
	ForceSummary      bool `json:"force_summary" env:"REFRESH_FORCE_SUMMARY" envDefault:"false"`          // Force summary regeneration
}

// LoadConfig loads configuration from file, environment variables, and command-line flags
func LoadConfig() (*Config, error) {
	return LoadConfigWithOverrides(nil)
}

// LoadConfigWithOverrides loads configuration with optional command-line flag overrides
func LoadConfigWithOverrides(flagOverrides map[string]interface{}) (*Config, error) {
	// Start with empty configuration (defaults will be set by env.Parse)
	config := &Config{}

	// Load from config file if it exists
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		if err := loadConfigFromFile(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Apply environment variable overrides using env library (also sets defaults)
	if err := env.ParseWithOptions(config, env.Options{
		Prefix: "GH_STAR_SEARCH_",
	}); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Initialize maps that env cannot handle
	if config.Embeddings.Options == nil {
		config.Embeddings.Options = make(map[string]string)
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

// parseInt parses string to int
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)

	return result, err
}

// parseBool parses string to bool
func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", s)
	}
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
