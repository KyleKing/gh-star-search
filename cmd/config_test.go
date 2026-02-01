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
					Path:         "~/.local/share/gh-star-search/stars.duckdb",
					QueryTimeout: "30s",
				},
				Cache: config.CacheConfig{
					Directory:         "~/.cache/gh-star-search",
					MaxSizeMB:         500,
					TTLHours:          24,
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
					Path:         "/tmp/test.duckdb",
					QueryTimeout: "10s",
				},
				Cache: config.CacheConfig{
					Directory:         "/tmp/cache",
					MaxSizeMB:         100,
					TTLHours:          12,
					MetadataStaleDays: 3,
					StatsStaleDays:    1,
				},
				Logging: config.LoggingConfig{
					Level:     "debug",
					Format:    "json",
					Output:    "file",
					File:      "/tmp/test.log",
					AddSource: true,
				},
				Debug: config.DebugConfig{
					Enabled:  true,
					Verbose:  true,
					TraceAPI: true,
				},
			},
			wantErr: false,
			contains: []string{
				"Active Configuration:",
				"Path: /tmp/test.duckdb",
				"Level: debug",
				"Format: json",
				"Output: file",
				"File: /tmp/test.log",
				"Add Source: true",
				"Enabled: true",
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
