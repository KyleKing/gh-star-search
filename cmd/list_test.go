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

func TestRunList(t *testing.T) {
	tests := []struct {
		name     string
		repos    []storage.StoredRepo
		limit    int
		offset   int
		format   string
		wantErr  bool
		contains []string
	}{
		{
			name:     "empty database",
			repos:    []storage.StoredRepo{},
			limit:    50,
			offset:   0,
			format:   "table",
			wantErr:  false,
			contains: []string{"No repositories found"},
		},
		{
			name: "table format",
			repos: []storage.StoredRepo{
				{
					FullName:        "user/repo1",
					Language:        "Go",
					StargazersCount: 100,
					ForksCount:      10,
					UpdatedAt:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					Description:     "Test repository 1",
				},
				{
					FullName:        "user/repo2",
					Language:        "Python",
					StargazersCount: 50,
					ForksCount:      5,
					UpdatedAt:       time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
					Description:     "Test repository 2",
				},
			},
			limit:    50,
			offset:   0,
			format:   "table",
			wantErr:  false,
			contains: []string{"user/repo1", "user/repo2", "Go", "Python", "100", "50"},
		},
		{
			name: "json format",
			repos: []storage.StoredRepo{
				{
					FullName:        "user/repo1",
					Language:        "Go",
					StargazersCount: 100,
					ForksCount:      10,
					UpdatedAt:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					Description:     "Test repository 1",
				},
			},
			limit:    50,
			offset:   0,
			format:   "json",
			wantErr:  false,
			contains: []string{`"full_name": "user/repo1"`, `"language": "Go"`},
		},
		{
			name: "csv format",
			repos: []storage.StoredRepo{
				{
					FullName:        "user/repo1",
					Language:        "Go",
					StargazersCount: 100,
					ForksCount:      10,
					UpdatedAt:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					Description:     "Test repository 1",
				},
			},
			limit:    50,
			offset:   0,
			format:   "csv",
			wantErr:  false,
			contains: []string{"Name,Language,Stars", "user/repo1,Go,100"},
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
				repos: tt.repos,
			}

			// Run the command with mock storage
			err := RunListWithStorage(
				context.Background(),
				tt.limit,
				tt.offset,
				tt.format,
				mockRepo,
			)

			// Restore stdout and get output
			w.Close()

			os.Stdout = oldStdout

			var buf bytes.Buffer

			_, _ = buf.ReadFrom(r)
			output := buf.String()

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("runList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check output contains expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("runList() output does not contain %q\nOutput: %s", expected, output)
				}
			}
		})
	}
}

func TestOutputTable(t *testing.T) {
	repos := []storage.StoredRepo{
		{
			FullName:        "user/test-repo",
			Language:        "Go",
			StargazersCount: 123,
			ForksCount:      45,
			UpdatedAt:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Description:     "A test repository for unit testing",
		},
		{
			FullName:        "user/another-repo",
			Language:        "",
			StargazersCount: 67,
			ForksCount:      8,
			UpdatedAt:       time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			Description:     "Another test repository with a very long description that should be truncated when displayed in the table format",
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTable(repos)

	// Restore stdout and get output
	w.Close()

	os.Stdout = oldStdout

	var buf bytes.Buffer

	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("outputTable() error = %v", err)
		return
	}

	// Check that output contains expected elements
	expectedStrings := []string{
		"NAME", "LANGUAGE", "STARS", "FORKS", "UPDATED", "DESCRIPTION",
		"user/test-repo", "Go", "123", "45", "2023-01-01",
		"user/another-repo", "N/A", "67", "8", "2023-02-01",
		"Another test repository with a very long description that...", // Should be truncated
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("outputTable() output does not contain %q\nOutput: %s", expected, output)
		}
	}
}
