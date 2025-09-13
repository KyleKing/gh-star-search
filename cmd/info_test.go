package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-star-search/internal/processor"
	"github.com/kyleking/gh-star-search/internal/storage"
)

func TestRunInfo(t *testing.T) {
	testRepo := storage.StoredRepo{
		ID:                       "test-id",
		FullName:                 "user/test-repo",
		Description:              "A test repository",
		Language:                 "Go",
		StargazersCount:          123,
		ForksCount:               45,
		SizeKB:                   1024,
		CreatedAt:                time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:                time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
		LastSynced:               time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
		Topics:                   []string{"cli", "golang", "tool"},
		LicenseName:              "MIT License",
		LicenseSPDXID:            "MIT",
		Purpose:                  "This is a test repository for demonstration purposes",
		Technologies:             []string{"Go", "CLI", "Testing"},
		UseCases:                 []string{"Testing", "Development", "Learning"},
		Features:                 []string{"Fast", "Reliable", "Easy to use"},
		InstallationInstructions: "go install github.com/user/test-repo@latest",
		UsageInstructions:        "Run test-repo --help for usage information",
		ContentHash:              "abc123",
		Chunks: []processor.ContentChunk{
			{Source: "README.md", Type: "readme", Content: "# Test Repo", Tokens: 10, Priority: 1},
			{Source: "main.go", Type: "code", Content: "package main", Tokens: 5, Priority: 2},
		},
	}

	tests := []struct {
		name     string
		repoName string
		repo     *storage.StoredRepo
		wantErr  bool
		contains []string
	}{
		{
			name:     "existing repository",
			repoName: "user/test-repo",
			repo:     &testRepo,
			wantErr:  false,
			contains: []string{
				"Repository: user/test-repo",
				"Description: A test repository",
				"Language: Go",
				"Stars: 123",
				"Forks: 45",
				"Size: 1024 KB",
				"License: MIT License (MIT)",
				"Topics: cli, golang, tool",
				"Technologies: Go, CLI, Testing",
				"Purpose:",
				"This is a test repository for demonstration purposes",
				"Use Cases:",
				"• Testing",
				"• Development",
				"• Learning",
				"Features:",
				"• Fast",
				"• Reliable",
				"• Easy to use",
				"Installation:",
				"go install github.com/user/test-repo@latest",
				"Usage:",
				"Run test-repo --help for usage information",
				"Content Chunks: 2",
				"readme: 1 chunks",
				"code: 1 chunks",
			},
		},
		{
			name:     "repository not found",
			repoName: "user/nonexistent",
			repo:     nil,
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

			// Create mock storage
			mockRepo := &MockRepository{}
			if tt.repo != nil {
				mockRepo.repos = []storage.StoredRepo{*tt.repo}
			}

			// Run the command with mock storage
			err := runInfoWithStorage(context.Background(), tt.repoName, mockRepo)

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("runInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check output contains expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("runInfo() output does not contain %q\nOutput: %s", expected, output)
				}
			}
		})
	}
}

func TestGetStringOrNA(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "N/A"},
		{"Go", "Go"},
		{"Python", "Python"},
		{" ", " "},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input_%s", tt.input), func(t *testing.T) {
			result := getStringOrNA(tt.input)
			if result != tt.expected {
				t.Errorf("getStringOrNA(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
