package processor

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kyleking/gh-star-search/internal/github"
)

// Service defines the interface for content processing operations
type Service interface {
	ProcessRepository(ctx context.Context, repo github.Repository, content []github.Content) (*ProcessedRepo, error)
	ExtractContent(ctx context.Context, repo github.Repository) ([]github.Content, error)
	GenerateSummary(ctx context.Context, chunks []ContentChunk) (*Summary, error)
}

// ContentChunk represents a processed piece of repository content
type ContentChunk struct {
	Source   string `json:"source"`   // file path or section
	Type     string `json:"type"`     // readme, code, docs, etc.
	Content  string `json:"content"`
	Tokens   int    `json:"tokens"`
	Priority int    `json:"priority"` // for size limit handling
}

// Summary represents the LLM-generated summary of repository content
type Summary struct {
	Purpose      string   `json:"purpose"`
	Technologies []string `json:"technologies"`
	UseCases     []string `json:"use_cases"`
	Features     []string `json:"features"`
	Installation string   `json:"installation"`
	Usage        string   `json:"usage"`
}

// ProcessedRepo represents a fully processed repository with summary and chunks
type ProcessedRepo struct {
	Repository  github.Repository `json:"repository"`
	Summary     Summary           `json:"summary"`
	Chunks      []ContentChunk    `json:"chunks"`
	ProcessedAt time.Time         `json:"processed_at"`
	ContentHash string            `json:"content_hash"` // For change detection
}

// ContentType constants for different types of repository content
const (
	ContentTypeReadme     = "readme"
	ContentTypeCode       = "code"
	ContentTypeDocs       = "docs"
	ContentTypeConfig     = "config"
	ContentTypeChangelog  = "changelog"
	ContentTypeLicense    = "license"
	ContentTypePackage    = "package"
)

// Priority constants for content processing
const (
	PriorityHigh   = 1
	PriorityMedium = 2
	PriorityLow    = 3
)

// Token limits for content processing
const (
	MaxTokensPerChunk = 2000
	MaxTotalTokens    = 50000 // Total tokens per repository
)

// GitHubClient interface for fetching repository content
type GitHubClient interface {
	GetRepositoryContent(ctx context.Context, repo github.Repository, paths []string) ([]github.Content, error)
}

// LLMService interface for content summarization
type LLMService interface {
	Summarize(ctx context.Context, prompt string, content string) (*SummaryResponse, error)
}

// SummaryResponse represents the response from LLM summarization
type SummaryResponse struct {
	Purpose      string   `json:"purpose"`
	Technologies []string `json:"technologies"`
	UseCases     []string `json:"use_cases"`
	Features     []string `json:"features"`
	Installation string   `json:"installation"`
	Usage        string   `json:"usage"`
	Confidence   float64  `json:"confidence"`
}

// serviceImpl implements the Service interface
type serviceImpl struct {
	githubClient GitHubClient
	llmService   LLMService
}

// NewService creates a new processor service
func NewService(githubClient GitHubClient, llmService LLMService) Service {
	return &serviceImpl{
		githubClient: githubClient,
		llmService:   llmService,
	}
}

// ProcessRepository processes a repository by extracting content and generating summaries
func (s *serviceImpl) ProcessRepository(ctx context.Context, repo github.Repository, content []github.Content) (*ProcessedRepo, error) {
	// Extract and chunk content
	chunks, err := s.extractAndChunkContent(ctx, repo, content)
	if err != nil {
		return nil, fmt.Errorf("failed to extract and chunk content: %w", err)
	}

	// Generate content hash for change detection
	contentHash := s.generateContentHash(chunks)

	// Generate summary from chunks
	summary, err := s.GenerateSummary(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// Create processed repository
	processed := &ProcessedRepo{
		Repository:  repo,
		Chunks:      chunks,
		ProcessedAt: time.Now(),
		ContentHash: contentHash,
		Summary:     *summary,
	}

	return processed, nil
}

// ExtractContent extracts relevant content from a repository
func (s *serviceImpl) ExtractContent(ctx context.Context, repo github.Repository) ([]github.Content, error) {
	// Define priority paths to extract
	priorityPaths := s.getPriorityPaths()

	// Fetch content from GitHub
	content, err := s.githubClient.GetRepositoryContent(ctx, repo, priorityPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository content: %w", err)
	}

	// Filter and validate content
	filteredContent := s.filterContent(content)

	return filteredContent, nil
}

// GenerateSummary generates a summary using the LLM service with fallback to basic analysis
func (s *serviceImpl) GenerateSummary(ctx context.Context, chunks []ContentChunk) (*Summary, error) {
	if s.llmService == nil {
		// Fallback to basic summary if no LLM service available
		return s.generateBasicSummary(chunks), nil
	}

	// Prepare content for LLM processing
	content := s.prepareContentForLLM(chunks)
	if content == "" {
		// If no content to process, return basic summary
		return s.generateBasicSummary(chunks), nil
	}

	// Use LLM service to generate summary
	llmResponse, err := s.llmService.Summarize(ctx, "", content)
	if err != nil {
		// Fallback to basic summary on LLM error
		fmt.Printf("LLM summarization failed, using fallback: %v\n", err)
		return s.generateBasicSummary(chunks), nil
	}

	// Convert LLM response to our Summary format
	summary := &Summary{
		Purpose:      llmResponse.Purpose,
		Technologies: llmResponse.Technologies,
		UseCases:     llmResponse.UseCases,
		Features:     llmResponse.Features,
		Installation: llmResponse.Installation,
		Usage:        llmResponse.Usage,
	}

	return summary, nil
}

// extractAndChunkContent processes repository content into chunks
func (s *serviceImpl) extractAndChunkContent(ctx context.Context, repo github.Repository, content []github.Content) ([]ContentChunk, error) {
	var allChunks []ContentChunk
	totalTokens := 0

	// Process each content file
	for _, file := range content {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Decode content if base64 encoded
		decodedContent, err := s.decodeContent(file)
		if err != nil {
			continue // Skip files we can't decode
		}

		// Determine content type and priority
		contentType := s.determineContentType(file.Path)
		priority := s.determinePriority(contentType, file.Path)

		// Create chunks from the content
		chunks := s.chunkContent(decodedContent, file.Path, contentType, priority)

		// Add chunks while respecting token limits
		for _, chunk := range chunks {
			if totalTokens+chunk.Tokens > MaxTotalTokens {
				break
			}
			allChunks = append(allChunks, chunk)
			totalTokens += chunk.Tokens
		}

		if totalTokens >= MaxTotalTokens {
			break
		}
	}

	// Sort chunks by priority (high priority first)
	sort.Slice(allChunks, func(i, j int) bool {
		return allChunks[i].Priority < allChunks[j].Priority
	})

	return allChunks, nil
}

// getPriorityPaths returns a list of file paths to prioritize for content extraction
func (s *serviceImpl) getPriorityPaths() []string {
	return []string{
		// README files (highest priority)
		"README.md", "README.rst", "README.txt", "README",
		"readme.md", "readme.rst", "readme.txt", "readme",

		// Package manifests
		"package.json", "Cargo.toml", "go.mod", "setup.py", "pom.xml",
		"composer.json", "Gemfile", "requirements.txt", "pyproject.toml",

		// Documentation
		"CHANGELOG.md", "CHANGELOG.rst", "CHANGELOG.txt", "CHANGELOG",
		"CHANGES.md", "CHANGES.rst", "CHANGES.txt", "CHANGES",
		"HISTORY.md", "HISTORY.rst", "HISTORY.txt", "HISTORY",

		// License files
		"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING",
		"license", "license.md", "license.txt", "copying",

		// Configuration files
		".github/README.md", "docs/README.md", "doc/README.md",
		"docs/index.md", "doc/index.md",

		// Main entry points (common patterns)
		"main.go", "main.py", "index.js", "index.ts", "app.js", "app.py",
		"src/main.go", "src/main.py", "src/index.js", "src/index.ts",
	}
}

// filterContent filters out unwanted content and validates files
func (s *serviceImpl) filterContent(content []github.Content) []github.Content {
	var filtered []github.Content

	for _, file := range content {
		// Skip if file is too large (> 1MB)
		if file.Size > 1024*1024 {
			continue
		}

		// Skip binary files
		if s.isBinaryFile(file.Path) {
			continue
		}

		// Skip if content type is file (not directory)
		if file.Type != "file" {
			continue
		}

		filtered = append(filtered, file)
	}

	return filtered
}

// decodeContent decodes base64 encoded content from GitHub API
func (s *serviceImpl) decodeContent(file github.Content) (string, error) {
	if file.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 content: %w", err)
		}

		// Validate UTF-8
		if !utf8.Valid(decoded) {
			return "", fmt.Errorf("content is not valid UTF-8")
		}

		return string(decoded), nil
	}

	return file.Content, nil
}

// determineContentType determines the type of content based on file path
func (s *serviceImpl) determineContentType(path string) string {
	lowerPath := strings.ToLower(path)
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	// README files
	if strings.HasPrefix(base, "readme") {
		return ContentTypeReadme
	}

	// Documentation files
	if strings.Contains(lowerPath, "doc") || strings.Contains(lowerPath, "wiki") {
		return ContentTypeDocs
	}

	// Changelog files
	if strings.Contains(base, "changelog") || strings.Contains(base, "changes") || strings.Contains(base, "history") {
		return ContentTypeChangelog
	}

	// License files
	if strings.Contains(base, "license") || strings.Contains(base, "copying") {
		return ContentTypeLicense
	}

	// Package manifests
	packageFiles := []string{"package.json", "cargo.toml", "go.mod", "setup.py", "pom.xml", "composer.json", "gemfile", "requirements.txt", "pyproject.toml"}
	for _, pkg := range packageFiles {
		if base == pkg {
			return ContentTypePackage
		}
	}

	// Configuration files
	configExts := []string{".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".config"}
	for _, configExt := range configExts {
		if ext == configExt {
			return ContentTypeConfig
		}
	}

	// Code files
	codeExts := []string{".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".hpp", ".rs", ".rb", ".php", ".cs", ".swift", ".kt", ".scala", ".clj", ".hs", ".ml", ".r", ".m", ".sh", ".bash", ".zsh", ".fish"}
	for _, codeExt := range codeExts {
		if ext == codeExt {
			return ContentTypeCode
		}
	}

	return ContentTypeDocs // Default to docs
}

// determinePriority determines processing priority based on content type and path
func (s *serviceImpl) determinePriority(contentType, path string) int {
	switch contentType {
	case ContentTypeReadme:
		return PriorityHigh
	case ContentTypePackage, ContentTypeChangelog:
		return PriorityHigh
	case ContentTypeDocs:
		// Main documentation gets high priority
		if strings.Contains(strings.ToLower(path), "index") || strings.Contains(strings.ToLower(path), "getting") {
			return PriorityHigh
		}
		return PriorityMedium
	case ContentTypeCode:
		// Main entry points get medium priority
		base := strings.ToLower(filepath.Base(path))
		if strings.Contains(base, "main") || strings.Contains(base, "index") || strings.Contains(base, "app") {
			return PriorityMedium
		}
		return PriorityLow
	case ContentTypeConfig:
		return PriorityMedium
	case ContentTypeLicense:
		return PriorityLow
	default:
		return PriorityLow
	}
}

// chunkContent splits content into manageable chunks
func (s *serviceImpl) chunkContent(content, source, contentType string, priority int) []ContentChunk {
	var chunks []ContentChunk

	// Estimate tokens (rough approximation: 1 token ≈ 4 characters)
	estimateTokens := func(text string) int {
		return len(text) / 4
	}

	// If content is small enough, return as single chunk
	if estimateTokens(content) <= MaxTokensPerChunk {
		return []ContentChunk{{
			Source:   source,
			Type:     contentType,
			Content:  content,
			Tokens:   estimateTokens(content),
			Priority: priority,
		}}
	}

	// Split content based on type
	var sections []string

	switch contentType {
	case ContentTypeReadme, ContentTypeDocs, ContentTypeChangelog:
		sections = s.splitMarkdownContent(content)
	case ContentTypeCode:
		sections = s.splitCodeContent(content)
	default:
		sections = s.splitGenericContent(content)
	}

	// Create chunks from sections
	for i, section := range sections {
		if strings.TrimSpace(section) == "" {
			continue
		}

		tokens := estimateTokens(section)
		if tokens > MaxTokensPerChunk {
			// Further split large sections
			subSections := s.splitLargeSection(section, MaxTokensPerChunk)
			for j, subSection := range subSections {
				chunks = append(chunks, ContentChunk{
					Source:   fmt.Sprintf("%s#%d.%d", source, i+1, j+1),
					Type:     contentType,
					Content:  subSection,
					Tokens:   estimateTokens(subSection),
					Priority: priority,
				})
			}
		} else {
			chunks = append(chunks, ContentChunk{
				Source:   fmt.Sprintf("%s#%d", source, i+1),
				Type:     contentType,
				Content:  section,
				Tokens:   tokens,
				Priority: priority,
			})
		}
	}

	return chunks
}

// splitMarkdownContent splits markdown content by headers and sections
func (s *serviceImpl) splitMarkdownContent(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var currentSection strings.Builder

	headerRegex := regexp.MustCompile(`^#{1,6}\s+`)

	for _, line := range lines {
		if headerRegex.MatchString(line) && currentSection.Len() > 0 {
			// Start new section
			sections = append(sections, currentSection.String())
			currentSection.Reset()
		}
		currentSection.WriteString(line + "\n")
	}

	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}

	return sections
}

// splitCodeContent splits code content by functions, classes, or logical blocks
func (s *serviceImpl) splitCodeContent(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var currentSection strings.Builder

	// Simple heuristic: split on function/class definitions
	functionRegex := regexp.MustCompile(`^(func|function|def|class|interface|type|struct|impl|fn)\s+`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if functionRegex.MatchString(trimmed) && currentSection.Len() > 0 {
			// Start new section
			sections = append(sections, currentSection.String())
			currentSection.Reset()
		}
		currentSection.WriteString(line + "\n")
	}

	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}

	// If no functions found, split by paragraphs
	if len(sections) <= 1 {
		return s.splitGenericContent(content)
	}

	return sections
}

// splitGenericContent splits content by paragraphs or line breaks
func (s *serviceImpl) splitGenericContent(content string) []string {
	// Split by double newlines (paragraphs)
	sections := strings.Split(content, "\n\n")

	// If sections are too few, split by single newlines
	if len(sections) <= 2 {
		lines := strings.Split(content, "\n")
		var sections []string
		var currentSection strings.Builder
		linesPerSection := 50 // Arbitrary limit

		for i, line := range lines {
			currentSection.WriteString(line + "\n")
			if (i+1)%linesPerSection == 0 {
				sections = append(sections, currentSection.String())
				currentSection.Reset()
			}
		}

		if currentSection.Len() > 0 {
			sections = append(sections, currentSection.String())
		}

		return sections
	}

	return sections
}

// splitLargeSection splits a section that's too large into smaller pieces
func (s *serviceImpl) splitLargeSection(content string, maxTokens int) []string {
	var sections []string
	lines := strings.Split(content, "\n")
	var currentSection strings.Builder

	estimateTokens := func(text string) int {
		return len(text) / 4
	}

	for _, line := range lines {
		testContent := currentSection.String() + line + "\n"
		if estimateTokens(testContent) > maxTokens && currentSection.Len() > 0 {
			sections = append(sections, currentSection.String())
			currentSection.Reset()
		}
		currentSection.WriteString(line + "\n")
	}

	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}

	return sections
}

// isBinaryFile checks if a file is likely binary based on extension
func (s *serviceImpl) isBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := []string{
		".exe", ".bin", ".dll", ".so", ".dylib", ".a", ".lib",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".ico", ".svg",
		".mp3", ".mp4", ".avi", ".mov", ".wav", ".flac",
		".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".woff", ".woff2", ".ttf", ".otf", ".eot",
	}

	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return true
		}
	}

	return false
}

// generateContentHash creates a hash of the processed content for change detection
func (s *serviceImpl) generateContentHash(chunks []ContentChunk) string {
	hasher := sha256.New()

	// Sort chunks by source for consistent hashing
	sortedChunks := make([]ContentChunk, len(chunks))
	copy(sortedChunks, chunks)
	sort.Slice(sortedChunks, func(i, j int) bool {
		return sortedChunks[i].Source < sortedChunks[j].Source
	})

	for _, chunk := range sortedChunks {
		hasher.Write([]byte(chunk.Source + chunk.Content))
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// prepareContentForLLM combines and formats content chunks for LLM processing
func (s *serviceImpl) prepareContentForLLM(chunks []ContentChunk) string {
	var contentBuilder strings.Builder
	totalTokens := 0
	maxTokens := 8000 // Leave room for prompt and response

	// Sort chunks by priority (high priority first)
	sortedChunks := make([]ContentChunk, len(chunks))
	copy(sortedChunks, chunks)
	sort.Slice(sortedChunks, func(i, j int) bool {
		return sortedChunks[i].Priority < sortedChunks[j].Priority
	})

	// Add chunks while respecting token limits
	for _, chunk := range sortedChunks {
		if totalTokens+chunk.Tokens > maxTokens {
			break
		}

		// Add section header
		contentBuilder.WriteString(fmt.Sprintf("\n=== %s (%s) ===\n", chunk.Source, chunk.Type))
		contentBuilder.WriteString(chunk.Content)
		contentBuilder.WriteString("\n")

		totalTokens += chunk.Tokens
	}

	return contentBuilder.String()
}

// generateBasicSummary creates a basic summary without LLM processing
func (s *serviceImpl) generateBasicSummary(chunks []ContentChunk) *Summary {
	summary := &Summary{
		Technologies: []string{},
		UseCases:     []string{},
		Features:     []string{},
	}

	// Extract basic information from chunks
	for _, chunk := range chunks {
		if chunk.Type == ContentTypeReadme {
			// Try to extract purpose from README
			lines := strings.Split(chunk.Content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if len(line) > 20 && len(line) < 200 && !strings.HasPrefix(line, "#") {
					if summary.Purpose == "" {
						summary.Purpose = line
						break
					}
				}
			}
		}

		if chunk.Type == ContentTypePackage {
			// Extract technologies from package files
			content := strings.ToLower(chunk.Content)
			if strings.Contains(content, "javascript") || strings.Contains(content, "node") || strings.Contains(chunk.Source, "package.json") {
				summary.Technologies = append(summary.Technologies, "JavaScript")
			}
			if strings.Contains(content, "python") || strings.Contains(content, "beautifulsoup") || strings.Contains(content, "django") || strings.Contains(content, "flask") {
				summary.Technologies = append(summary.Technologies, "Python")
			}
			if strings.Contains(content, "go") && strings.Contains(chunk.Source, "go.mod") {
				summary.Technologies = append(summary.Technologies, "Go")
			}
			if strings.Contains(chunk.Source, "cargo.toml") || strings.Contains(content, "rust") {
				summary.Technologies = append(summary.Technologies, "Rust")
			}
			if strings.Contains(chunk.Source, "pom.xml") || strings.Contains(content, "java") {
				summary.Technologies = append(summary.Technologies, "Java")
			}
		}
	}

	// Remove duplicates from technologies
	summary.Technologies = removeDuplicates(summary.Technologies)

	return summary
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}
