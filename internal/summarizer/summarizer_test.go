package summarizer

import (
	"context"
	"testing"

	"github.com/KyleKing/gh-star-search/internal/python"
)

func setupSummarizer(t *testing.T) *Summarizer {
	t.Helper()

	uvPath, err := python.FindUV()
	if err != nil {
		t.Skipf("uv not installed: %v", err)
	}

	cacheDir := t.TempDir()
	projectDir, err := python.EnsureEnvironment(context.Background(), uvPath, cacheDir)
	if err != nil {
		t.Skipf("Failed to prepare Python environment: %v", err)
	}

	return New(uvPath, projectDir)
}

func TestSummarizer_Heuristic(t *testing.T) {
	s := setupSummarizer(t)

	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{
			name: "simple text",
			text: "This is a library that provides utilities for developers. " +
				"It offers tools for data processing and helps manage complex workflows.",
			wantErr: false,
		},
		{
			name: "repository description",
			text: "gh-star-search is a GitHub CLI extension for searching starred repositories. " +
				"It provides fuzzy search and vector similarity search capabilities. " +
				"The tool uses DuckDB for local storage and caching.",
			wantErr: false,
		},
		{
			name:    "empty text",
			text:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.Summarize(context.Background(), tt.text, MethodHeuristic)

			if (err != nil) != tt.wantErr {
				t.Errorf("Summarize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if tt.text == "" {
				if result.Summary != "" {
					t.Error("Expected empty summary for empty text")
				}
				return
			}

			if result.Error != "" {
				t.Errorf("Summarize() failed: %s", result.Error)
				return
			}

			if result.Summary == "" {
				t.Error("Summarize() returned empty summary for non-empty text")
			}

			if result.Method != "heuristic" {
				t.Errorf("Expected method=heuristic, got %s", result.Method)
			}

			t.Logf("Summary: %s", result.Summary)
		})
	}
}
