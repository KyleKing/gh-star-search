package processor

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/gh-star-search/internal/github"
)

func TestIntegrationContentExtraction(t *testing.T) {
	// Read test fixtures
	readmeContent, err := os.ReadFile(filepath.Join("testdata", "sample_readme.md"))
	if err != nil {
		t.Fatalf("Failed to read README fixture: %v", err)
	}

	codeContent, err := os.ReadFile(filepath.Join("testdata", "sample_code.go"))
	if err != nil {
		t.Fatalf("Failed to read code fixture: %v", err)
	}

	packageContent, err := os.ReadFile(filepath.Join("testdata", "sample_package.json"))
	if err != nil {
		t.Fatalf("Failed to read package fixture: %v", err)
	}

	// Create repository with realistic content
	repo := github.Repository{
		FullName:        "example/sample-project",
		Description:     "A sample project for testing",
		Language:        "Go",
		StargazersCount: 42,
		ForksCount:      7,
		DefaultBranch:   "main",
	}

	// Create content with base64 encoding (as GitHub API returns)
	content := []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  base64.StdEncoding.EncodeToString(readmeContent),
			Encoding: "base64",
			Size:     len(readmeContent),
			SHA:      "abc123",
		},
		{
			Path:     "main.go",
			Type:     "file",
			Content:  base64.StdEncoding.EncodeToString(codeContent),
			Encoding: "base64",
			Size:     len(codeContent),
			SHA:      "def456",
		},
		{
			Path:     "package.json",
			Type:     "file",
			Content:  base64.StdEncoding.EncodeToString(packageContent),
			Encoding: "base64",
			Size:     len(packageContent),
			SHA:      "ghi789",
		},
		// Add some files that should be filtered out
		{
			Path:     "image.png",
			Type:     "file",
			Content:  "binary-content",
			Encoding: "base64",
			Size:     1000,
			SHA:      "binary123",
		},
		{
			Path:     "large-file.txt",
			Type:     "file",
			Content:  base64.StdEncoding.EncodeToString([]byte("content")),
			Encoding: "base64",
			Size:     2 * 1024 * 1024, // 2MB - should be filtered
			SHA:      "large123",
		},
	}

	// Create service
	client := &mockGitHubClient{content: content}
	llmService := &mockLLMService{}
	service := NewService(client, llmService)

	// Test full processing pipeline
	ctx := context.Background()
	processed, err := service.ProcessRepository(ctx, repo, content)

	if err != nil {
		t.Fatalf("ProcessRepository failed: %v", err)
	}

	// Verify processed repository
	if processed.Repository.FullName != repo.FullName {
		t.Errorf("Repository name mismatch: got %s, want %s", processed.Repository.FullName, repo.FullName)
	}

	if len(processed.Chunks) == 0 {
		t.Fatal("No chunks were created")
	}

	// Verify chunk types and priorities
	chunkTypes := make(map[string]int)
	priorities := make(map[int]int)

	for _, chunk := range processed.Chunks {
		chunkTypes[chunk.Type]++
		priorities[chunk.Priority]++

		// Verify chunk has required fields
		if chunk.Source == "" {
			t.Error("Chunk missing source")
		}

		if chunk.Content == "" {
			t.Error("Chunk missing content")
		}

		if chunk.Tokens <= 0 {
			t.Error("Chunk has invalid token count")
		}

		if chunk.Tokens > MaxTokensPerChunk {
			t.Errorf("Chunk exceeds max tokens: %d > %d", chunk.Tokens, MaxTokensPerChunk)
		}
	}

	// Should have README, code, and package chunks
	if chunkTypes[ContentTypeReadme] == 0 {
		t.Error("No README chunks found")
	}

	if chunkTypes[ContentTypeCode] == 0 {
		t.Error("No code chunks found")
	}

	if chunkTypes[ContentTypePackage] == 0 {
		t.Error("No package chunks found")
	}

	// Should have high priority chunks (README, package)
	if priorities[PriorityHigh] == 0 {
		t.Error("No high priority chunks found")
	}

	// Verify content hash is generated
	if processed.ContentHash == "" {
		t.Error("Content hash not generated")
	}

	// Verify processed time is set
	if processed.ProcessedAt.IsZero() {
		t.Error("Processed time not set")
	}

	t.Logf("Successfully processed repository with %d chunks", len(processed.Chunks))
	t.Logf("Chunk types: %+v", chunkTypes)
	t.Logf("Priority distribution: %+v", priorities)
}

func TestIntegrationContentExtractionWithLargeContent(t *testing.T) {
	// Create a large README that should be split into multiple chunks
	largeReadme := "# Large Project\n\n"
	for i := range 100 {
		largeReadme += "## Section " + string(rune('A'+i%26)) + "\n\n"
		largeReadme += "This is a detailed section with lots of content. " +
			"It contains multiple paragraphs and detailed explanations. " +
			"The content is designed to test the chunking algorithm's ability " +
			"to split large documents into manageable pieces while preserving " +
			"logical boundaries and maintaining readability.\n\n"
		largeReadme += "### Subsection\n\n"
		largeReadme += "More detailed content with examples and code snippets. " +
			"This subsection provides additional context and information " +
			"that helps users understand the concepts being discussed.\n\n"
	}

	repo := github.Repository{
		FullName: "example/large-project",
		Language: "Markdown",
	}

	content := []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  base64.StdEncoding.EncodeToString([]byte(largeReadme)),
			Encoding: "base64",
			Size:     len(largeReadme),
		},
	}

	client := &mockGitHubClient{content: content}
	llmService := &mockLLMService{}
	service := NewService(client, llmService)

	ctx := context.Background()
	processed, err := service.ProcessRepository(ctx, repo, content)

	if err != nil {
		t.Fatalf("ProcessRepository failed: %v", err)
	}

	// Should create multiple chunks for large content
	if len(processed.Chunks) <= 1 {
		t.Errorf("Large content should be split into multiple chunks, got %d", len(processed.Chunks))
	}

	// Verify total tokens don't exceed limit
	totalTokens := 0
	for _, chunk := range processed.Chunks {
		totalTokens += chunk.Tokens
	}

	if totalTokens > MaxTotalTokens {
		t.Errorf("Total tokens exceed limit: %d > %d", totalTokens, MaxTotalTokens)
	}

	// Verify each chunk respects token limits
	for i, chunk := range processed.Chunks {
		if chunk.Tokens > MaxTokensPerChunk {
			t.Errorf("Chunk %d exceeds max tokens: %d > %d", i, chunk.Tokens, MaxTokensPerChunk)
		}
	}

	t.Logf("Large content split into %d chunks with %d total tokens", len(processed.Chunks), totalTokens)
}

func TestIntegrationContentExtractionErrorHandling(t *testing.T) {
	repo := github.Repository{
		FullName: "example/error-test",
	}

	// Test with invalid base64 content
	invalidContent := []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  "invalid-base64-content!!!",
			Encoding: "base64",
			Size:     100,
		},
	}

	client := &mockGitHubClient{content: invalidContent}
	llmService := &mockLLMService{}
	service := NewService(client, llmService)

	ctx := context.Background()
	processed, err := service.ProcessRepository(ctx, repo, invalidContent)

	// Should not fail completely, but should handle errors gracefully
	if err != nil {
		t.Fatalf("ProcessRepository should handle decode errors gracefully: %v", err)
	}

	// Should still return a processed repository, even with no valid chunks
	if processed == nil {
		t.Fatal("ProcessRepository should return a result even with invalid content")
	}

	// May have no chunks due to decode errors, which is acceptable
	t.Logf("Processed repository with invalid content: %d chunks", len(processed.Chunks))
}

func TestIntegrationBasicSummaryGeneration(t *testing.T) {
	// Test the basic summary generation with realistic content
	readmeContent := `# Web Scraper Tool

A powerful web scraping tool built with Python and BeautifulSoup.

## Features
- Multi-threaded scraping
- Rate limiting
- Data export to CSV/JSON
- Proxy support

## Installation
pip install web-scraper-tool

## Usage
python -m scraper --url https://example.com`

	packageContent := `{
  "name": "web-scraper-tool",
  "description": "A powerful web scraping tool",
  "main": "scraper.py",
  "dependencies": {
    "beautifulsoup4": "^4.11.0",
    "requests": "^2.28.0"
  }
}`

	chunks := []ContentChunk{
		{
			Source:   "README.md",
			Type:     ContentTypeReadme,
			Content:  readmeContent,
			Tokens:   100,
			Priority: PriorityHigh,
		},
		{
			Source:   "package.json",
			Type:     ContentTypePackage,
			Content:  packageContent,
			Tokens:   50,
			Priority: PriorityHigh,
		},
	}

	service := &serviceImpl{}
	summary := service.generateBasicSummary(chunks)

	if summary == nil {
		t.Fatal("generateBasicSummary returned nil")
	}

	// Should extract purpose from README
	if summary.Purpose == "" {
		t.Error("Summary should extract purpose from README content")
	}

	// Should detect technologies
	if len(summary.Technologies) == 0 {
		t.Error("Summary should detect technologies from content")
	}

	t.Logf("Generated summary: Purpose=%q, Technologies=%v", summary.Purpose, summary.Technologies)
}
