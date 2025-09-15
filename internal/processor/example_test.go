package processor_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/kyleking/gh-star-search/internal/github"
	"github.com/kyleking/gh-star-search/internal/processor"
)

// mockClient implements the GitHubClient interface for examples
type mockClient struct{}

func (m *mockClient) GetRepositoryContent(
	_ context.Context,
	_ github.Repository,
	_ []string,
) ([]github.Content, error) {
	// Return sample content for demonstration
	readmeContent := `# Example Project

This is an example project demonstrating content extraction.

## Features
- Content extraction from repositories
- Intelligent chunking
- File type detection

## Installation
go get github.com/example/project`

	return []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  base64.StdEncoding.EncodeToString([]byte(readmeContent)),
			Encoding: "base64",
			Size:     len(readmeContent),
		},
	}, nil
}

func ExampleService_ProcessRepository() {
	// Create a GitHub repository
	repo := github.Repository{
		FullName:    "example/demo-project",
		Description: "A demonstration project",
		Language:    "Go",
	}

	// Create processor service with mock client
	client := &mockClient{}
	service := processor.NewService(client)

	// Extract content from repository
	ctx := context.Background()

	content, err := service.ExtractContent(ctx, repo)
	if err != nil {
		log.Fatal(err)
	}

	// Process the repository
	processed, err := service.ProcessRepository(ctx, repo, content)
	if err != nil {
		log.Fatal(err)
	}

	_, _ = fmt.Printf("Processed repository: %s\n", processed.Repository.FullName)
	_, _ = fmt.Printf("Number of chunks: %d\n", len(processed.Chunks))
	_, _ = fmt.Printf("Content hash length: %d\n", len(processed.ContentHash))

	// Display chunk information
	for i, chunk := range processed.Chunks {
		fmt.Printf("Chunk %d: %s (%s, priority %d)\n",
			i+1, chunk.Source, chunk.Type, chunk.Priority)
	}

	// Output:
	// Processed repository: example/demo-project
	// Number of chunks: 1
	// Content hash length: 64
	// Chunk 1: README.md (readme, priority 1)
}
