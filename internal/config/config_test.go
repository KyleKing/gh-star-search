package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFromFile(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	testConfig := map[string]interface{}{
		"database": map[string]interface{}{
			"path":            "/custom/path/db.db",
			"max_connections": 20,
			"query_timeout":   "60s",
		},
		"logging": map[string]interface{}{
			"level":  "debug",
			"format": "json",
			"output": "file",
			"file":   "/custom/log/path.log",
		},
		"debug": map[string]interface{}{
			"enabled": true,
			"verbose": true,
		},
	}

	data, err := json.MarshalIndent(testConfig, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(configPath, data, 0600)
	require.NoError(t, err)

	// Test loading
	config, err := LoadConfigWithOverrides(nil)
	require.NoError(t, err)
	err = loadConfigFromFile(config, configPath)
	require.NoError(t, err)

	assert.Equal(t, "/custom/path/db.db", config.Database.Path)
	assert.Equal(t, 20, config.Database.MaxConnections)
	assert.Equal(t, "60s", config.Database.QueryTimeout)
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "file", config.Logging.Output)
	assert.Equal(t, "/custom/log/path.log", config.Logging.File)
	assert.True(t, config.Debug.Enabled)
	assert.True(t, config.Debug.Verbose)
}

func TestLoadConfigFromFileInvalidJSON(t *testing.T) {
	// Create temporary config file with invalid JSON
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	err := os.WriteFile(configPath, []byte("invalid json"), 0600)
	require.NoError(t, err)

	config, err := LoadConfigWithOverrides(nil)
	require.NoError(t, err)
	err = loadConfigFromFile(config, configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestApplyFlagOverrides(t *testing.T) {
	config, err := LoadConfigWithOverrides(nil)
	require.NoError(t, err)

	overrides := map[string]interface{}{
		"db-path":   "/flag/db/path.db",
		"log-level": "error",
		"verbose":   true,
		"debug":     true,
		"cache-dir": "/flag/cache",
	}

	err = applyFlagOverrides(config, overrides)
	require.NoError(t, err)

	assert.Equal(t, "/flag/db/path.db", config.Database.Path)
	assert.Equal(t, "error", config.Logging.Level)
	assert.True(t, config.Debug.Verbose)
	assert.True(t, config.Debug.Enabled)
	assert.Equal(t, "/flag/cache", config.Cache.Directory)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name          string
		modifyConfig  func(*Config)
		expectError   bool
		errorContains string
	}{
		{
			name: "valid config",
			modifyConfig: func(_ *Config) {
				// No modifications - should be valid
			},
			expectError: false,
		},
		{
			name: "invalid log level",
			modifyConfig: func(c *Config) {
				c.Logging.Level = "invalid"
			},
			expectError:   true,
			errorContains: "invalid log level",
		},
		{
			name: "invalid log format",
			modifyConfig: func(c *Config) {
				c.Logging.Format = "invalid"
			},
			expectError:   true,
			errorContains: "invalid log format",
		},
		{
			name: "invalid log output",
			modifyConfig: func(c *Config) {
				c.Logging.Output = "invalid"
			},
			expectError:   true,
			errorContains: "invalid log output",
		},
		{
			name: "invalid database timeout",
			modifyConfig: func(c *Config) {
				c.Database.QueryTimeout = "invalid"
			},
			expectError:   true,
			errorContains: "invalid database query timeout",
		},

		{
			name: "invalid cache cleanup frequency",
			modifyConfig: func(c *Config) {
				c.Cache.CleanupFreq = "invalid"
			},
			expectError:   true,
			errorContains: "invalid cache cleanup frequency",
		},
		{
			name: "invalid max connections",
			modifyConfig: func(c *Config) {
				c.Database.MaxConnections = -1
			},
			expectError:   true,
			errorContains: "database max connections must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadConfigWithOverrides(nil)
			require.NoError(t, err)
			tt.modifyConfig(config)

			err = validateConfig(config)
			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absolute path",
			input:    "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "relative path",
			input:    "relative/path",
			expected: "relative/path",
		},
		{
			name:     "home directory only",
			input:    "~",
			expected: os.Getenv("HOME"),
		},
		{
			name:     "home directory with path",
			input:    "~/config/file.json",
			expected: filepath.Join(os.Getenv("HOME"), "config/file.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)

			if tt.expected == os.Getenv("HOME") && tt.expected == "" {
				// Skip test if HOME is not set
				t.Skip("HOME environment variable not set")
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigExpandAllPaths(t *testing.T) {
	config := &Config{
		Database: DatabaseConfig{
			Path: "~/db/test.db",
		},
		Cache: CacheConfig{
			Directory: "~/cache",
		},
		Logging: LoggingConfig{
			File: "~/logs/app.log",
		},
	}

	config.ExpandAllPaths()

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME environment variable not set")
	}

	assert.Equal(t, filepath.Join(homeDir, "db/test.db"), config.Database.Path)
	assert.Equal(t, filepath.Join(homeDir, "cache"), config.Cache.Directory)
	assert.Equal(t, filepath.Join(homeDir, "logs/app.log"), config.Logging.File)
}

func TestSaveConfig(t *testing.T) {
	// Use a temporary config path to avoid interference with other tests
	tempConfigPath := filepath.Join(t.TempDir(), "test_config.json")
	t.Setenv("GH_STAR_SEARCH_CONFIG", tempConfigPath)

	config, err := LoadConfigWithOverrides(nil)
	require.NoError(t, err)

	config.Database.Path = "/custom/path"
	config.Logging.Level = "debug"

	err = SaveConfig(config)
	require.NoError(t, err)

	// Verify file was created and contains expected data
	data, err := os.ReadFile(tempConfigPath)
	require.NoError(t, err)

	var loadedConfig Config
	err = json.Unmarshal(data, &loadedConfig)
	require.NoError(t, err)

	assert.Equal(t, config.Database.Path, loadedConfig.Database.Path)
	assert.Equal(t, config.Logging.Level, loadedConfig.Logging.Level)
}

func TestLoadConfigWithOverrides(t *testing.T) {
	// Set a temporary config path to avoid interference with other tests
	originalConfigPath := os.Getenv("GH_STAR_SEARCH_CONFIG")
	tempConfigPath := filepath.Join(t.TempDir(), "test_config.json")
	t.Setenv("GH_STAR_SEARCH_CONFIG", tempConfigPath)

	// Restore original config path after test
	defer func() {
		if originalConfigPath != "" {
			os.Setenv("GH_STAR_SEARCH_CONFIG", originalConfigPath)
		} else {
			os.Unsetenv("GH_STAR_SEARCH_CONFIG")
		}
	}()

	// Test with no config file and no overrides
	config, err := LoadConfigWithOverrides(nil)
	require.NoError(t, err)

	// Should return default config
	defaultConfig, err := LoadConfigWithOverrides(nil)
	require.NoError(t, err)
	assert.Equal(t, defaultConfig.Database.Path, config.Database.Path)
	assert.Equal(t, defaultConfig.Logging.Level, config.Logging.Level)
}

func TestMergeConfigs(t *testing.T) {
	target, err := LoadConfigWithOverrides(nil)
	require.NoError(t, err)

	source := &Config{
		Database: DatabaseConfig{
			Path:           "/new/path",
			MaxConnections: 25,
		},
		Logging: LoggingConfig{
			Level: "debug",
		},
	}

	mergeConfigs(target, source)

	assert.Equal(t, "/new/path", target.Database.Path)
	assert.Equal(t, 25, target.Database.MaxConnections)
	assert.Equal(t, "debug", target.Logging.Level)
	// Other values should remain from target
	assert.Equal(t, "30s", target.Database.QueryTimeout)
	assert.Equal(t, "text", target.Logging.Format)
}
