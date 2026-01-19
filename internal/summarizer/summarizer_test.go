package summarizer

import (
	"context"
	"testing"
	"time"
)

func TestSummarizer_Heuristic(t *testing.T) {
	// Create summarizer
	s, err := New(Config{
		Timeout: 10 * time.Second,
	})
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
			wantErr: false, // Should return a result with success=false
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
				if result.Success {
					t.Error("Expected success=false for empty text")
				}
				return
			}

			if !result.Success {
				t.Errorf("Summarize() failed: %s", result.Error)
				return
			}

			if result.Summary == "" {
				t.Error("Summarize() returned empty summary for non-empty text")
			}

			if result.Method != "heuristic" {
				t.Errorf("Expected method=heuristic, got %s", result.Method)
			}

			t.Logf("Input length: %d, Output length: %d", result.InputLength, result.OutputLength)
			t.Logf("Summary: %s", result.Summary)
		})
	}
}

func TestSummarizer_WithFallback(t *testing.T) {
	s, err := New(Config{
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create summarizer (Python may not be available): %v", err)
	}

	text := "This is a comprehensive library that provides multiple utilities for developers. " +
		"It includes features for data processing, workflow management, and API integration. " +
		"The library is designed to be flexible and extensible."

	result, err := s.SummarizeWithFallback(context.Background(), text)
	if err != nil {
		t.Fatalf("SummarizeWithFallback() error = %v", err)
	}

	if !result.Success {
		t.Errorf("SummarizeWithFallback() failed: %s", result.Error)
	}

	if result.Summary == "" {
		t.Error("SummarizeWithFallback() returned empty summary")
	}

	t.Logf("Method used: %s", result.Method)
	t.Logf("Summary: %s", result.Summary)
}

func TestFindPython(t *testing.T) {
	pythonPath, err := findPython()
	if err != nil {
		t.Skipf("Python not found in PATH: %v", err)
	}

	if pythonPath == "" {
		t.Error("findPython() returned empty path")
	}

	t.Logf("Found Python at: %s", pythonPath)
}
