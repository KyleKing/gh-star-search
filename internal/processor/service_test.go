package processor

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/kyleking/gh-star-search/internal/github"
)

// mockGitHubClient implements GitHubClient for testing
type mockGitHubClient struct {
	content []github.Content
	err     error
}

func (m *mockGitHubClient) GetRepositoryContent(_ context.Context, _ github.Repository, _ []string) ([]github.Content, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.content, nil
}

func TestNewService(t *testing.T) {
	client := &mockGitHubClient{}
	service := NewService(client)

	if service == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestDetermineContentType(t *testing.T) {
	service := &serviceImpl{}

	tests := []struct {
		path     string
		expected string
	}{
		{"README.md", ContentTypeReadme},
		{"readme.txt", ContentTypeReadme},
		{"package.json", ContentTypePackage},
		{"go.mod", ContentTypePackage},
		{"CHANGELOG.md", ContentTypeChangelog},
		{"LICENSE", ContentTypeLicense},
		{"docs/index.md", ContentTypeDocs},
		{"main.go", ContentTypeCode},
		{"app.py", ContentTypeCode},
		{"config.yaml", ContentTypeConfig},
		{"unknown.txt", ContentTypeDocs},
	}

	for _, test := range tests {
		result := service.determineContentType(test.path)
		if result != test.expected {
			t.Errorf("determineContentType(%q) = %q, want %q", test.path, result, test.expected)
		}
	}
}

func TestDeterminePriority(t *testing.T) {
	service := &serviceImpl{}

	tests := []struct {
		contentType string
		path        string
		expected    int
	}{
		{ContentTypeReadme, "README.md", PriorityHigh},
		{ContentTypePackage, "package.json", PriorityHigh},
		{ContentTypeDocs, "docs/index.md", PriorityHigh},
		{ContentTypeDocs, "docs/guide.md", PriorityMedium},
		{ContentTypeCode, "main.go", PriorityMedium},
		{ContentTypeCode, "utils.go", PriorityLow},
		{ContentTypeConfig, "config.yaml", PriorityMedium},
		{ContentTypeLicense, "LICENSE", PriorityLow},
	}

	for _, test := range tests {
		result := service.determinePriority(test.contentType, test.path)
		if result != test.expected {
			t.Errorf("determinePriority(%q, %q) = %d, want %d", test.contentType, test.path, result, test.expected)
		}
	}
}

func TestIsBinaryFile(t *testing.T) {
	service := &serviceImpl{}

	tests := []struct {
		path     string
		expected bool
	}{
		{"README.md", false},
		{"main.go", false},
		{"image.png", true},
		{"binary.exe", true},
		{"font.woff", true},
		{"archive.zip", true},
		{"document.pdf", true},
		{"script.sh", false},
		{"data.json", false},
	}

	for _, test := range tests {
		result := service.isBinaryFile(test.path)
		if result != test.expected {
			t.Errorf("isBinaryFile(%q) = %v, want %v", test.path, result, test.expected)
		}
	}
}

func TestDecodeContent(t *testing.T) {
	service := &serviceImpl{}

	// Test base64 encoded content
	originalText := "Hello, World!"
	encoded := base64.StdEncoding.EncodeToString([]byte(originalText))

	file := github.Content{
		Content:  encoded,
		Encoding: "base64",
	}

	decoded, err := service.decodeContent(file)
	if err != nil {
		t.Fatalf("decodeContent failed: %v", err)
	}

	if decoded != originalText {
		t.Errorf("decodeContent() = %q, want %q", decoded, originalText)
	}

	// Test plain text content
	plainFile := github.Content{
		Content:  originalText,
		Encoding: "",
	}

	decoded, err = service.decodeContent(plainFile)
	if err != nil {
		t.Fatalf("decodeContent failed for plain text: %v", err)
	}

	if decoded != originalText {
		t.Errorf("decodeContent() for plain text = %q, want %q", decoded, originalText)
	}
}

func TestFilterContent(t *testing.T) {
	service := &serviceImpl{}

	content := []github.Content{
		{Path: "README.md", Type: "file", Size: 1000},
		{Path: "image.png", Type: "file", Size: 500},
		{Path: "large.txt", Type: "file", Size: 2 * 1024 * 1024}, // 2MB
		{Path: "directory", Type: "dir", Size: 0},
		{Path: "main.go", Type: "file", Size: 2000},
	}

	filtered := service.filterContent(content)

	// Should keep README.md and main.go, filter out image.png (binary), large.txt (too big), directory (not file)
	expected := 2
	if len(filtered) != expected {
		t.Errorf("filterContent() returned %d files, want %d", len(filtered), expected)
	}

	// Check that the right files are kept
	paths := make(map[string]bool)
	for _, file := range filtered {
		paths[file.Path] = true
	}

	if !paths["README.md"] {
		t.Error("filterContent() should keep README.md")
	}

	if !paths["main.go"] {
		t.Error("filterContent() should keep main.go")
	}
}

func TestSplitMarkdownContent(t *testing.T) {
	service := &serviceImpl{}

	content := `# Title
This is the introduction.

## Section 1
Content of section 1.

### Subsection 1.1
Content of subsection 1.1.

## Section 2
Content of section 2.`

	sections := service.splitMarkdownContent(content)

	if len(sections) != 4 {
		t.Errorf("splitMarkdownContent() returned %d sections, want 4", len(sections))
	}

	// First section should contain title and introduction
	if !contains(sections[0], "# Title") || !contains(sections[0], "introduction") {
		t.Error("First section should contain title and introduction")
	}
}

func TestSplitCodeContent(t *testing.T) {
	service := &serviceImpl{}

	content := `package main

import "fmt"

func main() {
    fmt.Println("Hello")
}

func helper() {
    // Helper function
}

type MyStruct struct {
    Field string
}`

	sections := service.splitCodeContent(content)

	// Should split on function and type definitions
	if len(sections) < 3 {
		t.Errorf("splitCodeContent() returned %d sections, want at least 3", len(sections))
	}
}

func TestChunkContent(t *testing.T) {
	service := &serviceImpl{}

	// Test small content (should return single chunk)
	smallContent := "This is a small piece of content."
	chunks := service.chunkContent(smallContent, "test.md", ContentTypeReadme, PriorityHigh)

	if len(chunks) != 1 {
		t.Errorf("chunkContent() for small content returned %d chunks, want 1", len(chunks))
	}

	if chunks[0].Content != smallContent {
		t.Error("Chunk content doesn't match original")
	}

	if chunks[0].Type != ContentTypeReadme {
		t.Error("Chunk type doesn't match expected")
	}

	if chunks[0].Priority != PriorityHigh {
		t.Error("Chunk priority doesn't match expected")
	}

	// Test large content (should be split into multiple chunks)
	largeContent := strings.Repeat("This is a line of content that will be repeated many times.\n", 200)
	chunks = service.chunkContent(largeContent, "large.md", ContentTypeReadme, PriorityHigh)

	if len(chunks) <= 1 {
		t.Errorf("chunkContent() for large content returned %d chunks, want > 1", len(chunks))
	}

	// Verify all chunks have reasonable token counts
	for i, chunk := range chunks {
		if chunk.Tokens <= 0 {
			t.Errorf("Chunk %d has invalid token count: %d", i, chunk.Tokens)
		}

		if chunk.Tokens > MaxTokensPerChunk {
			t.Errorf("Chunk %d exceeds max tokens: %d > %d", i, chunk.Tokens, MaxTokensPerChunk)
		}
	}
}

func TestProcessRepository(t *testing.T) {
	// Create sample repository
	repo := github.Repository{
		FullName:    "test/repo",
		Description: "A test repository",
		Language:    "Go",
	}

	// Create sample content
	readmeContent := base64.StdEncoding.EncodeToString([]byte(`# Test Repository
This is a test repository for unit testing.

## Features
- Feature 1
- Feature 2

## Installation
Run go install.`))

	content := []github.Content{
		{
			Path:     "README.md",
			Type:     "file",
			Content:  readmeContent,
			Encoding: "base64",
			Size:     100,
		},
	}

	// Create service with mock client
	client := &mockGitHubClient{content: content}
	service := NewService(client)

	// Process repository
	ctx := context.Background()
	processed, err := service.ProcessRepository(ctx, repo, content)

	if err != nil {
		t.Fatalf("ProcessRepository failed: %v", err)
	}

	if processed == nil {
		t.Fatal("ProcessRepository returned nil")
	}

	if processed.Repository.FullName != repo.FullName {
		t.Error("Processed repository doesn't match original")
	}

	if len(processed.Chunks) == 0 {
		t.Error("ProcessRepository should create chunks")
	}

	if processed.ContentHash == "" {
		t.Error("ProcessRepository should generate content hash")
	}

	if processed.ProcessedAt.IsZero() {
		t.Error("ProcessRepository should set processed time")
	}
}

func TestExtractContent(t *testing.T) {
	repo := github.Repository{
		FullName: "test/repo",
	}

	// Create sample content including binary and text files
	content := []github.Content{
		{Path: "README.md", Type: "file", Size: 100},
		{Path: "image.png", Type: "file", Size: 500},
		{Path: "main.go", Type: "file", Size: 200},
	}

	client := &mockGitHubClient{content: content}
	service := NewService(client)

	ctx := context.Background()
	extracted, err := service.ExtractContent(ctx, repo)

	if err != nil {
		t.Fatalf("ExtractContent failed: %v", err)
	}

	// Should filter out binary files
	if len(extracted) != 2 {
		t.Errorf("ExtractContent returned %d files, want 2", len(extracted))
	}
}

func TestGenerateContentHash(t *testing.T) {
	service := &serviceImpl{}

	chunks1 := []ContentChunk{
		{Source: "README.md", Content: "Hello"},
		{Source: "main.go", Content: "package main"},
	}

	chunks2 := []ContentChunk{
		{Source: "main.go", Content: "package main"},
		{Source: "README.md", Content: "Hello"},
	}

	chunks3 := []ContentChunk{
		{Source: "README.md", Content: "Hello World"},
		{Source: "main.go", Content: "package main"},
	}

	hash1 := service.generateContentHash(chunks1)
	hash2 := service.generateContentHash(chunks2)
	hash3 := service.generateContentHash(chunks3)

	// Same content in different order should produce same hash
	if hash1 != hash2 {
		t.Error("Content hash should be consistent regardless of chunk order")
	}

	// Different content should produce different hash
	if hash1 == hash3 {
		t.Error("Different content should produce different hashes")
	}

	if hash1 == "" {
		t.Error("Content hash should not be empty")
	}
}

func TestGenerateBasicSummary(t *testing.T) {
	service := &serviceImpl{}

	chunks := []ContentChunk{
		{
			Source:  "README.md",
			Type:    ContentTypeReadme,
			Content: "# Test Project\nThis is a test project for demonstration purposes.\n\n## Features\n- Feature 1\n- Feature 2",
		},
		{
			Source:  "go.mod",
			Type:    ContentTypePackage,
			Content: "module test\n\ngo 1.21",
		},
	}

	summary := service.generateBasicSummary(chunks)

	if summary == nil {
		t.Fatal("generateBasicSummary returned nil")
	}

	if summary.Purpose == "" {
		t.Error("Summary should extract purpose from README")
	}

	if len(summary.Technologies) == 0 {
		t.Error("Summary should extract technologies from package files")
	}

	// Should detect Go from go.mod
	found := false

	for _, tech := range summary.Technologies {
		if tech == "Go" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Summary should detect Go technology from go.mod")
	}
}

func TestRemoveDuplicates(t *testing.T) {
	input := []string{"Go", "JavaScript", "Go", "Python", "JavaScript"}
	result := removeDuplicates(input)

	expected := 3 // Go, JavaScript, Python
	if len(result) != expected {
		t.Errorf("removeDuplicates returned %d items, want %d", len(result), expected)
	}

	// Check that all unique items are present
	items := make(map[string]bool)
	for _, item := range result {
		items[item] = true
	}

	if !items["Go"] || !items["JavaScript"] || !items["Python"] {
		t.Error("removeDuplicates should preserve all unique items")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
