package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-star-search/internal/storage"
)

func TestRunClear(t *testing.T) {
	testStats := &storage.Stats{
		TotalRepositories:  10,
		TotalContentChunks: 50,
		DatabaseSizeMB:     5.5,
		LastSyncTime:       time.Now(),
		LanguageBreakdown:  map[string]int{"Go": 5, "Python": 5},
		TopicBreakdown:     map[string]int{"cli": 10},
	}

	tests := []struct {
		name     string
		stats    *storage.Stats
		force    bool
		wantErr  bool
		contains []string
	}{
		{
			name:    "force clear with data",
			stats:   testStats,
			force:   true,
			wantErr: false,
			contains: []string{
				"This will delete:",
				"• 10 repositories",
				"• 50 content chunks",
				"• 5.50 MB of data",
				"Database cleared successfully.",
			},
		},
		{
			name: "empty database",
			stats: &storage.Stats{
				TotalRepositories:  0,
				TotalContentChunks: 0,
				DatabaseSizeMB:     0,
				LastSyncTime:       time.Time{},
				LanguageBreakdown:  make(map[string]int),
				TopicBreakdown:     make(map[string]int),
			},
			force:   false,
			wantErr: false,
			contains: []string{
				"Database is already empty.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create mock storage
			mockRepo := &MockRepository{
				stats: tt.stats,
			}

			// Run the command with mock storage
			err := runClearWithStorage(context.Background(), tt.force, mockRepo)

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("runClear() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check output contains expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("runClear() output does not contain %q\nOutput: %s", expected, output)
				}
			}
		})
	}
}
