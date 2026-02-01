package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/KyleKing/gh-star-search/internal/storage"
)

func TestRunStats(t *testing.T) {
	testStats := &storage.Stats{
		TotalRepositories:  150,
		DatabaseSizeMB:     25.5,
		LastSyncTime:       time.Date(2023, 6, 15, 14, 30, 0, 0, time.UTC),
		LanguageBreakdown: map[string]int{
			"Go":         50,
			"Python":     30,
			"JavaScript": 25,
			"TypeScript": 20,
			"Rust":       15,
			"Java":       10,
		},
		TopicBreakdown: map[string]int{
			"cli":       40,
			"web":       35,
			"api":       30,
			"tool":      25,
			"framework": 20,
			"library":   15,
		},
	}

	tests := []struct {
		name     string
		stats    *storage.Stats
		wantErr  bool
		contains []string
	}{
		{
			name:    "full stats",
			stats:   testStats,
			wantErr: false,
			contains: []string{
				"Database Statistics",
				"Total Repositories: 150",
				"Database Size: 25.50 MB",
				"Last Sync: 2023-06-15 14:30:00",
				"Language Breakdown:",
				"Go               50 repos (33.3%)",
				"Python           30 repos (20.0%)",
				"JavaScript       25 repos (16.7%)",
				"Top Topics:",
				"cli                   40 repos (26.7%)",
				"web                   35 repos (23.3%)",
			},
		},
		{
			name: "empty stats",
			stats: &storage.Stats{
				TotalRepositories:  0,
				DatabaseSizeMB:     0,
				LastSyncTime:       time.Time{},
				LanguageBreakdown:  make(map[string]int),
				TopicBreakdown:     make(map[string]int),
			},
			wantErr: false,
			contains: []string{
				"Total Repositories: 0",
				"Database Size: 0.00 MB",
				"Last Sync: Never",
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
			err := runStatsWithStorage(context.Background(), mockRepo)

			// Restore stdout and get output
			w.Close()

			os.Stdout = oldStdout

			var buf bytes.Buffer

			_, _ = buf.ReadFrom(r)
			output := buf.String()

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("runStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check output contains expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("runStats() output does not contain %q\nOutput: %s", expected, output)
				}
			}
		})
	}
}
