package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/KyleKing/gh-star-search/internal/config"
)

func TestRunConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantErr  bool
		contains []string
	}{
		{
			name: "basic configuration display",
			cfg: &config.Config{
				Database: config.DatabaseConfig{
					Path:           "~/.local/share/gh-star-search/stars.duckdb",
					MaxConnections: 10,
					QueryTimeout:   "30s",
				},
				Cache: config.CacheConfig{
					Directory:         "~/.cache/gh-star-search",
					MaxSizeMB:         500,
					TTLHours:          24,
					CleanupFreq:       "1h",
					MetadataStaleDays: 7,
					StatsStaleDays:    1,
				},
				Logging: config.LoggingConfig{
					Level:  "info",
					Format: "text",
					Output: "stdout",
				},
				Debug: config.DebugConfig{
					Enabled:  false,
					Verbose:  false,
					TraceAPI: false,
				},
			},
			wantErr: false,
			contains: []string{
				"Active Configuration:",
				"Database:",
				"Path: ~/.local/share/gh-star-search/stars.duckdb",
				"Max Connections: 10",
				"Query Timeout: 30s",
				"Cache:",
				"Directory: ~/.cache/gh-star-search",
				"Max Size: 500 MB",
				"TTL: 24 hours",
				"Logging:",
				"Level: info",
				"Format: text",
				"Output: stdout",
				"Debug:",
				"Enabled: false",
				"Verbose: false",
				"Trace API: false",
			},
		},
		{
			name: "configuration with debug enabled",
			cfg: &config.Config{
				Database: config.DatabaseConfig{
					Path:           "/tmp/test.duckdb",
					MaxConnections: 5,
					QueryTimeout:   "10s",
				},
				Cache: config.CacheConfig{
					Directory:         "/tmp/cache",
					MaxSizeMB:         100,
					TTLHours:          12,
					CleanupFreq:       "30m",
					MetadataStaleDays: 3,
					StatsStaleDays:    1,
				},
				Logging: config.LoggingConfig{
					Level:      "debug",
					Format:     "json",
					Output:     "file",
					File:       "/tmp/test.log",
					MaxSizeMB:  10,
					MaxBackups: 3,
					MaxAgeDays: 7,
					AddSource:  true,
				},
				Debug: config.DebugConfig{
					Enabled:     true,
					ProfilePort: 6060,
					MetricsPort: 9090,
					Verbose:     true,
					TraceAPI:    true,
				},
			},
			wantErr: false,
			contains: []string{
				"Active Configuration:",
				"Path: /tmp/test.duckdb",
				"Max Connections: 5",
				"Level: debug",
				"Format: json",
				"Output: file",
				"File: /tmp/test.log",
				"Max Size: 10 MB",
				"Max Backups: 3",
				"Max Age: 7 days",
				"Add Source: true",
				"Enabled: true",
				"Profile Port: 6060",
				"Metrics Port: 9090",
				"Verbose: true",
				"Trace API: true",
				"Raw Configuration (JSON):",
			},
		},
		{
			name:     "nil configuration error",
			cfg:      nil,
			wantErr:  true,
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run the command with config
			err := RunConfigWithConfig(tt.cfg)

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("runConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check output contains expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf(
						"RunConfigWithConfig() output does not contain %q\nOutput: %s",
						expected,
						output,
					)
				}
			}
		})
	}
}
