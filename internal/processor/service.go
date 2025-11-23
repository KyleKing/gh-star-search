package processor

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	ProcessRepository(
		ctx context.Context,
		repo github.Repository,
		content []github.Content,
	) (*ProcessedRepo, error)
	ExtractContent(ctx context.Context, repo github.Repository) ([]github.Content, error)
}

// ContentChunk represents a processed piece of repository content
type ContentChunk struct {
	Source   string `json:"source"` // file path or section
	Type     string `json:"type"`   // readme, code, docs, etc.
	Content  string `json:"content"`
	Tokens   int    `json:"tokens"`
	Priority int    `json:"priority"` // for size limit handling
}

// ProcessedRepo represents a fully processed repository with chunks
type ProcessedRepo struct {
	Repository  github.Repository `json:"repository"`
	Chunks      []ContentChunk    `json:"chunks"`
	ProcessedAt time.Time         `json:"processed_at"`
	ContentHash string            `json:"content_hash"` // For change detection
}

// ContentType constants for different types of repository content
const (
	ContentTypeReadme    = "readme"
	ContentTypeCode      = "code"
	ContentTypeDocs      = "docs"
	ContentTypeConfig    = "config"
	ContentTypeChangelog = "changelog"
	ContentTypeLicense   = "license"
	ContentTypePackage   = "package"
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
	GetRepositoryContent(
		ctx context.Context,
		repo github.Repository,
		paths []string,
	) ([]github.Content, error)
}

// serviceImpl implements the Service interface
type serviceImpl struct {
	githubClient GitHubClient
	cache        ContentCache
}

// ContentCache interface for caching repository content
type ContentCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
}

// NewService creates a new processor service
func NewService(githubClient GitHubClient) Service {
	return &serviceImpl{
		githubClient: githubClient,
	}
}

// NewServiceWithCache creates a new processor service with caching
func NewServiceWithCache(githubClient GitHubClient, cache ContentCache) Service {
	return &serviceImpl{
		githubClient: githubClient,
		cache:        cache,
	}
}

// ProcessRepository processes a repository by extracting content and generating summaries
func (s *serviceImpl) ProcessRepository(
	ctx context.Context,
	repo github.Repository,
	content []github.Content,
) (*ProcessedRepo, error) {
	// Extract and chunk content
	chunks, err := s.extractAndChunkContent(ctx, repo, content)
	if err != nil {
		return nil, fmt.Errorf("failed to extract and chunk content: %w", err)
	}

	// Generate content hash for change detection
	contentHash := s.generateContentHash(chunks)

	// Create processed repository
	processed := &ProcessedRepo{
		Repository:  repo,
		Chunks:      chunks,
		ProcessedAt: time.Now(),
		ContentHash: contentHash,
	}

	return processed, nil
}

// ExtractContent extracts relevant content from a repository with caching
func (s *serviceImpl) ExtractContent(
	ctx context.Context,
	repo github.Repository,
) ([]github.Content, error) {
	// Try to get content from cache first
	if s.cache != nil {
		cacheKey := fmt.Sprintf("content:%s:%s", repo.FullName, repo.UpdatedAt.Format(time.RFC3339))

		if cachedData, err := s.cache.Get(ctx, cacheKey); err == nil {
			var content []github.Content
			if err := json.Unmarshal(cachedData, &content); err == nil {
				return content, nil
			}
		}
	}

	// Define priority paths to extract
	priorityPaths := s.getPriorityPaths()

	// Fetch content from GitHub
	content, err := s.githubClient.GetRepositoryContent(ctx, repo, priorityPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository content: %w", err)
	}

	// Filter and validate content
	filteredContent := s.filterContent(content)

	// Cache the result if cache is available
	if s.cache != nil {
		cacheKey := fmt.Sprintf("content:%s:%s", repo.FullName, repo.UpdatedAt.Format(time.RFC3339))
		if cachedData, err := json.Marshal(filteredContent); err == nil {
			// Cache for 24 hours
			_ = s.cache.Set(ctx, cacheKey, cachedData, 24*time.Hour)
		}
	}

	return filteredContent, nil
}

// extractAndChunkContent processes repository content into chunks
func (s *serviceImpl) extractAndChunkContent(
	ctx context.Context,
	_ github.Repository,
	content []github.Content,
) ([]ContentChunk, error) {
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
// Focuses on top-level documentation and key source files, avoiding tests and large assets
func (s *serviceImpl) getPriorityPaths() []string {
	return []string{
		// README files (highest priority - top level only)
		"README.md", "README.rst", "README.txt", "README",
		"readme.md", "readme.rst", "readme.txt", "readme",

		// Package manifests (top level)
		"package.json", "Cargo.toml", "go.mod", "setup.py", "pom.xml",
		"composer.json", "Gemfile", "requirements.txt", "pyproject.toml",
		"CMakeLists.txt", "Makefile", "build.gradle", "yarn.lock",

		// Documentation files (top level)
		"CHANGELOG.md", "CHANGELOG.rst", "CHANGELOG.txt", "CHANGELOG",
		"CHANGES.md", "CONTRIBUTING.md", "AUTHORS.md", "CONTRIBUTORS.md",

		// License files (top level)
		"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING",
		"license", "license.md", "license.txt", "copying",

		// Documentation directories - fetch index/main files only
		"docs/README.md", "docs/index.md", "docs/getting-started.md",
		"doc/README.md", "doc/index.md",
		".github/README.md",

		// Source directories - main entry points only (limit to avoid tests)
		"src/main.go", "src/main.py", "src/index.js", "src/index.ts",
		"src/app.js", "src/app.py", "src/lib.rs", "src/main.rs",
		"main.go", "main.py", "index.js", "index.ts", "app.js", "app.py",
		"lib.rs", "main.rs",
	}
}

// filterContent filters out unwanted content and validates files
// Excludes tests, images, videos, and other non-essential files
func (s *serviceImpl) filterContent(content []github.Content) []github.Content {
	var filtered []github.Content

	for _, file := range content {
		// Skip if file is too large (> 500KB for more selective downloading)
		if file.Size > 512*1024 {
			continue
		}

		// Skip if not a file
		if file.Type != "file" {
			continue
		}

		// Skip test files
		if s.isTestFile(file.Path) {
			continue
		}

		// Skip image and media files
		if s.isMediaFile(file.Path) {
			continue
		}

		// Skip binary files
		if s.isBinaryFile(file.Path) {
			continue
		}

		// Skip common build/vendor directories
		if s.isExcludedPath(file.Path) {
			continue
		}

		filtered = append(filtered, file)
	}

	return filtered
}

// isTestFile checks if a file is a test file
func (s *serviceImpl) isTestFile(path string) bool {
	lowerPath := strings.ToLower(path)
	lowerBase := strings.ToLower(filepath.Base(path))

	// Common test patterns
	testPatterns := []string{
		"test", "tests", "_test", "spec", "specs",
		"__tests__", "test_", ".test.", ".spec.",
	}

	for _, pattern := range testPatterns {
		if strings.Contains(lowerPath, pattern) || strings.Contains(lowerBase, pattern) {
			return true
		}
	}

	return false
}

// isMediaFile checks if a file is an image, video, or other media
func (s *serviceImpl) isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	mediaExts := []string{
		// Images
		".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg", ".ico",
		".webp", ".tiff", ".tif",
		// Videos
		".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm",
		// Audio
		".mp3", ".wav", ".ogg", ".flac", ".m4a",
		// Fonts
		".ttf", ".otf", ".woff", ".woff2", ".eot",
		// Archives
		".zip", ".tar", ".gz", ".rar", ".7z", ".bz2",
	}

	for _, mediaExt := range mediaExts {
		if ext == mediaExt {
			return true
		}
	}

	return false
}

// isExcludedPath checks if a path should be excluded
func (s *serviceImpl) isExcludedPath(path string) bool {
	lowerPath := strings.ToLower(path)

	// Exclude common directories we don't want to process
	excludedDirs := []string{
		"node_modules/", "vendor/", "build/", "dist/",
		"target/", "bin/", "obj/", ".git/",
		"__pycache__/", ".venv/", "venv/",
		"coverage/", ".next/", ".nuxt/",
		"examples/", "example/", "demo/", "demos/",
	}

	for _, dir := range excludedDirs {
		if strings.Contains(lowerPath, dir) {
			return true
		}
	}

	return false
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
			return "", errors.New("content is not valid UTF-8")
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
	if strings.Contains(base, "changelog") || strings.Contains(base, "changes") ||
		strings.Contains(base, "history") {
		return ContentTypeChangelog
	}

	// License files
	if strings.Contains(base, "license") || strings.Contains(base, "copying") {
		return ContentTypeLicense
	}

	// Package manifests
	packageFiles := []string{
		"package.json",
		"cargo.toml",
		"go.mod",
		"setup.py",
		"pom.xml",
		"composer.json",
		"gemfile",
		"requirements.txt",
		"pyproject.toml",
	}
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
	codeExts := []string{
		".go",
		".py",
		".js",
		".ts",
		".java",
		".c",
		".cpp",
		".h",
		".hpp",
		".rs",
		".rb",
		".php",
		".cs",
		".swift",
		".kt",
		".scala",
		".clj",
		".hs",
		".ml",
		".r",
		".m",
		".sh",
		".bash",
		".zsh",
		".fish",
	}
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
		if strings.Contains(strings.ToLower(path), "index") ||
			strings.Contains(strings.ToLower(path), "getting") {
			return PriorityHigh
		}

		return PriorityMedium
	case ContentTypeCode:
		// Main entry points get medium priority
		base := strings.ToLower(filepath.Base(path))
		if strings.Contains(base, "main") || strings.Contains(base, "index") ||
			strings.Contains(base, "app") {
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
func (s *serviceImpl) chunkContent(
	content, source, contentType string,
	priority int,
) []ContentChunk {
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
	functionRegex := regexp.MustCompile(
		`^(func|function|def|class|interface|type|struct|impl|fn)\s+`,
	)

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

	return hex.EncodeToString(hasher.Sum(nil))
}
