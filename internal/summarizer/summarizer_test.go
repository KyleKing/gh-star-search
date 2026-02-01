package summarizer

import (
	"context"
	"testing"
)

func TestSummarizer_Heuristic(t *testing.T) {
	// Create summarizer
	s, err := New()
	if err != nil {
		t.Skipf("Failed to create summarizer (Python may not be available): %v", err)
	}

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
			wantErr: false, // Should return empty summary
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

			// Check result
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

func TestSummarizer_Auto(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Skipf("Failed to create summarizer (Python may not be available): %v", err)
	}

	text := "This is a comprehensive library that provides multiple utilities for developers. " +
		"It includes features for data processing, workflow management, and API integration. " +
		"The library is designed to be flexible and extensible."

	summary, err := s.SummarizeSimple(context.Background(), text)
	if err != nil {
		t.Fatalf("SummarizeSimple() error = %v", err)
	}

	if summary == "" {
		t.Error("SummarizeSimple() returned empty summary")
	}

	t.Logf("Summary: %s", summary)
}
