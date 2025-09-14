package processor

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/kyleking/gh-star-search/internal/github"
)

func BenchmarkProcessRepository(b *testing.B) {
	// Create a realistic repository with multiple files
	repo := github.Repository{
		FullName:    "benchmark/test-repo",
		Description: "A benchmark test repository",
		Language:    "Go",
	}

	// Create sample content
	readmeContent := strings.Repeat("# Section\nThis is content for benchmarking.\n\n", 100)
	codeContent := strings.Repeat("func TestFunction() {\n\t// Test code\n}\n\n", 50)
	packageContent := `{"name": "test", "dependencies": {"lodash": "^4.0.0"}}`

	content := []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  base64.StdEncoding.EncodeToString([]byte(readmeContent)),
			Encoding: "base64",
			Size:     len(readmeContent),
		},
		{
			Path:     "main.go",
			Type:     "file",
			Content:  base64.StdEncoding.EncodeToString([]byte(codeContent)),
			Encoding: "base64",
			Size:     len(codeContent),
		},
		{
			Path:     "package.json",
			Type:     "file",
			Content:  base64.StdEncoding.EncodeToString([]byte(packageContent)),
			Encoding: "base64",
			Size:     len(packageContent),
		},
	}

	client := &mockGitHubClient{content: content}
	service := NewService(client)
	ctx := context.Background()

	b.ResetTimer()

	for range b.N {
		_, err := service.ProcessRepository(ctx, repo, content)
		if err != nil {
			b.Fatalf("ProcessRepository failed: %v", err)
		}
	}
}

func BenchmarkChunkContent(b *testing.B) {
	service := &serviceImpl{}

	// Create large content for chunking
	content := strings.Repeat("This is a line of content that will be chunked for performance testing.\n", 1000)

	b.ResetTimer()

	for range b.N {
		chunks := service.chunkContent(content, "test.md", ContentTypeReadme, PriorityHigh)
		if len(chunks) == 0 {
			b.Fatal("No chunks created")
		}
	}
}

func BenchmarkDetermineContentType(b *testing.B) {
	service := &serviceImpl{}

	paths := []string{
		"README.md",
		"main.go",
		"package.json",
		"docs/index.md",
		"src/utils.js",
		"config.yaml",
		"LICENSE",
		"CHANGELOG.md",
	}

	b.ResetTimer()

	for range b.N {
		for _, path := range paths {
			service.determineContentType(path)
		}
	}
}

func BenchmarkFilterContent(b *testing.B) {
	service := &serviceImpl{}

	// Create content with mix of valid and invalid files
	content := []github.Content{
		{Path: "README.md", Type: "file", Size: 1000},
		{Path: "main.go", Type: "file", Size: 2000},
		{Path: "image.png", Type: "file", Size: 500000},
		{Path: "large.txt", Type: "file", Size: 2 * 1024 * 1024},
		{Path: "directory", Type: "dir", Size: 0},
		{Path: "config.json", Type: "file", Size: 300},
		{Path: "binary.exe", Type: "file", Size: 1000000},
		{Path: "docs.md", Type: "file", Size: 1500},
	}

	b.ResetTimer()

	for range b.N {
		filtered := service.filterContent(content)
		if len(filtered) == 0 {
			b.Fatal("No content filtered")
		}
	}
}

func BenchmarkGenerateContentHash(b *testing.B) {
	service := &serviceImpl{}

	// Create chunks for hashing
	chunks := make([]ContentChunk, 100)
	for i := range chunks {
		chunks[i] = ContentChunk{
			Source:  "file" + string(rune('A'+i%26)) + ".md",
			Content: strings.Repeat("Content for chunk ", i+1),
			Type:    ContentTypeReadme,
		}
	}

	b.ResetTimer()

	for range b.N {
		hash := service.generateContentHash(chunks)
		if hash == "" {
			b.Fatal("Empty hash generated")
		}
	}
}

func BenchmarkSplitMarkdownContent(b *testing.B) {
	service := &serviceImpl{}

	// Create markdown content with multiple sections
	content := ""
	for i := range 50 {
		content += "# Section " + string(rune('A'+i%26)) + "\n\n"
		content += strings.Repeat("This is paragraph content with details and explanations.\n\n", 10)
		content += "## Subsection\n\n"
		content += strings.Repeat("More detailed content in the subsection.\n\n", 5)
	}

	b.ResetTimer()

	for range b.N {
		sections := service.splitMarkdownContent(content)
		if len(sections) == 0 {
			b.Fatal("No sections created")
		}
	}
}

func BenchmarkDecodeContent(b *testing.B) {
	service := &serviceImpl{}

	// Create base64 encoded content
	originalText := strings.Repeat("This is test content for decoding benchmarks.\n", 100)
	encoded := base64.StdEncoding.EncodeToString([]byte(originalText))

	file := github.Content{
		Content:  encoded,
		Encoding: "base64",
	}

	b.ResetTimer()

	for range b.N {
		decoded, err := service.decodeContent(file)
		if err != nil {
			b.Fatalf("decodeContent failed: %v", err)
		}

		if decoded != originalText {
			b.Fatal("Decoded content doesn't match original")
		}
	}
}
