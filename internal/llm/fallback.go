package llm

import (
	"context"
	"regexp"
	"strings"

	"github.com/username/gh-star-search/internal/query"
)

// FallbackService provides basic functionality when LLM services are unavailable
type FallbackService struct{}

// NewFallbackService creates a new fallback service
func NewFallbackService() *FallbackService {
	return &FallbackService{}
}

// Configure is a no-op for the fallback service
func (f *FallbackService) Configure(config Config) error {
	return nil
}

// Summarize provides basic content analysis without LLM
func (f *FallbackService) Summarize(ctx context.Context, prompt string, content string) (*SummaryResponse, error) {
	summary := &SummaryResponse{
		Purpose:      f.extractPurpose(content),
		Technologies: f.extractTechnologies(content),
		UseCases:     f.extractUseCases(content),
		Features:     f.extractFeatures(content),
		Installation: f.extractInstallation(content),
		Usage:        f.extractUsage(content),
		Confidence:   0.3, // Low confidence for rule-based extraction
	}
	
	return summary, nil
}

// ParseQuery provides basic query parsing without LLM
func (f *FallbackService) ParseQuery(ctx context.Context, query string, schema query.Schema) (*QueryResponse, error) {
	sql, explanation := f.parseBasicQuery(query, schema)
	
	response := &QueryResponse{
		SQL:         sql,
		Parameters:  make(map[string]string),
		Explanation: explanation,
		Confidence:  0.4, // Low confidence for rule-based parsing
		Reasoning:   "Generated using rule-based fallback parser",
	}
	
	return response, nil
}

// extractPurpose attempts to extract the repository purpose from content
func (f *FallbackService) extractPurpose(content string) string {
	lines := strings.Split(content, "\n")
	
	// Look for common patterns in README files
	for i, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip headers and empty lines
		if strings.HasPrefix(line, "#") || len(line) < 20 {
			continue
		}
		
		// Look for descriptive sentences
		if len(line) > 20 && len(line) < 200 {
			// Check if it's likely a description
			if strings.Contains(strings.ToLower(line), "is a") ||
				strings.Contains(strings.ToLower(line), "provides") ||
				strings.Contains(strings.ToLower(line), "allows") ||
				strings.Contains(strings.ToLower(line), "helps") ||
				strings.Contains(strings.ToLower(line), "tool") ||
				strings.Contains(strings.ToLower(line), "library") ||
				strings.Contains(strings.ToLower(line), "framework") {
				return line
			}
		}
		
		// Take the first substantial paragraph if nothing else found
		if i < 10 && len(line) > 50 && len(line) < 300 {
			return line
		}
	}
	
	return ""
}

// extractTechnologies identifies technologies mentioned in the content
func (f *FallbackService) extractTechnologies(content string) []string {
	var technologies []string
	contentLower := strings.ToLower(content)
	
	// Common technology patterns
	techPatterns := map[string][]string{
		"JavaScript": {"javascript", "js", "node.js", "nodejs", "npm", "yarn"},
		"TypeScript": {"typescript", "ts"},
		"Python":     {"python", "py", "pip", "django", "flask", "fastapi"},
		"Go":         {"golang", " go ", "go.mod", "go.sum"},
		"Rust":       {"rust", "cargo.toml", "rustc"},
		"Java":       {"java", "maven", "gradle", "pom.xml"},
		"C++":        {"c++", "cpp", "cmake", "makefile"},
		"C":          {" c ", "gcc", "clang"},
		"C#":         {"c#", "csharp", ".net", "dotnet"},
		"Ruby":       {"ruby", "gem", "rails", "bundler"},
		"PHP":        {"php", "composer"},
		"Swift":      {"swift", "xcode"},
		"Kotlin":     {"kotlin", "android"},
		"Scala":      {"scala", "sbt"},
		"Clojure":    {"clojure", "leiningen"},
		"Haskell":    {"haskell", "cabal", "stack"},
		"R":          {" r ", "cran"},
		"MATLAB":     {"matlab", ".m"},
		"Shell":      {"bash", "zsh", "fish", "shell", ".sh"},
		"Docker":     {"docker", "dockerfile", "container"},
		"Kubernetes": {"kubernetes", "k8s", "kubectl"},
		"React":      {"react", "jsx", "reactjs"},
		"Vue":        {"vue", "vuejs"},
		"Angular":    {"angular", "angularjs"},
		"Django":     {"django"},
		"Flask":      {"flask"},
		"Express":    {"express", "expressjs"},
		"Spring":     {"spring", "springboot"},
		"Rails":      {"rails", "ruby on rails"},
	}
	
	for tech, patterns := range techPatterns {
		for _, pattern := range patterns {
			if strings.Contains(contentLower, pattern) {
				technologies = append(technologies, tech)
				break
			}
		}
	}
	
	return f.removeDuplicates(technologies)
}

// extractUseCases identifies potential use cases from content
func (f *FallbackService) extractUseCases(content string) []string {
	var useCases []string
	contentLower := strings.ToLower(content)
	
	// Common use case patterns
	useCasePatterns := map[string][]string{
		"Web Development":     {"web", "website", "webapp", "http", "server", "api"},
		"CLI Tool":           {"command line", "cli", "terminal", "console"},
		"Library":            {"library", "package", "module", "framework"},
		"Data Analysis":      {"data", "analysis", "analytics", "visualization", "chart"},
		"Machine Learning":   {"machine learning", "ml", "ai", "neural", "model"},
		"Database":           {"database", "db", "sql", "nosql", "storage"},
		"Testing":            {"test", "testing", "unit test", "integration"},
		"DevOps":             {"devops", "deployment", "ci/cd", "automation"},
		"Mobile Development": {"mobile", "android", "ios", "app"},
		"Game Development":   {"game", "gaming", "engine", "graphics"},
		"Security":           {"security", "encryption", "auth", "authentication"},
		"Monitoring":         {"monitoring", "logging", "metrics", "observability"},
	}
	
	for useCase, patterns := range useCasePatterns {
		for _, pattern := range patterns {
			if strings.Contains(contentLower, pattern) {
				useCases = append(useCases, useCase)
				break
			}
		}
	}
	
	return f.removeDuplicates(useCases)
}

// extractFeatures identifies key features from content
func (f *FallbackService) extractFeatures(content string) []string {
	var features []string
	
	// Look for bullet points and feature lists
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Check for bullet points or numbered lists
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") ||
			regexp.MustCompile(`^\d+\.\s`).MatchString(line) {
			
			// Clean up the feature text
			feature := strings.TrimPrefix(line, "- ")
			feature = strings.TrimPrefix(feature, "* ")
			feature = regexp.MustCompile(`^\d+\.\s`).ReplaceAllString(feature, "")
			feature = strings.TrimSpace(feature)
			
			if len(feature) > 10 && len(feature) < 100 {
				features = append(features, feature)
			}
		}
	}
	
	// Limit to most relevant features
	if len(features) > 5 {
		features = features[:5]
	}
	
	return features
}

// extractInstallation looks for installation instructions
func (f *FallbackService) extractInstallation(content string) string {
	contentLower := strings.ToLower(content)
	lines := strings.Split(content, "\n")
	
	// Look for installation sections
	inInstallSection := false
	var installLines []string
	
	for _, line := range lines {
		lineLower := strings.ToLower(strings.TrimSpace(line))
		
		// Check for installation headers
		if strings.Contains(lineLower, "install") && (strings.HasPrefix(lineLower, "#") || strings.HasPrefix(lineLower, "##")) {
			inInstallSection = true
			continue
		}
		
		// Check for end of section
		if inInstallSection && strings.HasPrefix(lineLower, "#") && !strings.Contains(lineLower, "install") {
			break
		}
		
		// Collect installation lines
		if inInstallSection && strings.TrimSpace(line) != "" {
			installLines = append(installLines, strings.TrimSpace(line))
			if len(installLines) >= 3 { // Limit to first few lines
				break
			}
		}
	}
	
	if len(installLines) > 0 {
		return strings.Join(installLines, " ")
	}
	
	// Look for common installation commands
	installPatterns := []string{
		"npm install", "pip install", "go install", "cargo install",
		"brew install", "apt install", "yum install",
	}
	
	for _, pattern := range installPatterns {
		if strings.Contains(contentLower, pattern) {
			// Find the line with the install command
			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), pattern) {
					return strings.TrimSpace(line)
				}
			}
		}
	}
	
	return ""
}

// extractUsage looks for usage instructions or examples
func (f *FallbackService) extractUsage(content string) string {
	lines := strings.Split(content, "\n")
	
	// Look for usage sections
	inUsageSection := false
	var usageLines []string
	
	for _, line := range lines {
		lineLower := strings.ToLower(strings.TrimSpace(line))
		
		// Check for usage headers
		if (strings.Contains(lineLower, "usage") || strings.Contains(lineLower, "example")) &&
			(strings.HasPrefix(lineLower, "#") || strings.HasPrefix(lineLower, "##")) {
			inUsageSection = true
			continue
		}
		
		// Check for end of section
		if inUsageSection && strings.HasPrefix(lineLower, "#") &&
			!strings.Contains(lineLower, "usage") && !strings.Contains(lineLower, "example") {
			break
		}
		
		// Collect usage lines
		if inUsageSection && strings.TrimSpace(line) != "" {
			usageLines = append(usageLines, strings.TrimSpace(line))
			if len(usageLines) >= 3 { // Limit to first few lines
				break
			}
		}
	}
	
	if len(usageLines) > 0 {
		return strings.Join(usageLines, " ")
	}
	
	return ""
}

// parseBasicQuery provides simple query parsing without LLM
func (f *FallbackService) parseBasicQuery(query string, schema query.Schema) (string, string) {
	queryLower := strings.ToLower(query)
	
	// Check for specific technologies first (more specific than generic list/show)
	if strings.Contains(queryLower, "javascript") || strings.Contains(queryLower, "js") {
		return "SELECT full_name, description, stargazers_count FROM repositories WHERE language = 'JavaScript' OR technologies LIKE '%JavaScript%' ORDER BY stargazers_count DESC LIMIT 20;",
			"Finds JavaScript repositories"
	}
	
	if strings.Contains(queryLower, "python") {
		return "SELECT full_name, description, stargazers_count FROM repositories WHERE language = 'Python' OR technologies LIKE '%Python%' ORDER BY stargazers_count DESC LIMIT 20;",
			"Finds Python repositories"
	}
	
	if strings.Contains(queryLower, "go") || strings.Contains(queryLower, "golang") {
		return "SELECT full_name, description, stargazers_count FROM repositories WHERE language = 'Go' OR technologies LIKE '%Go%' ORDER BY stargazers_count DESC LIMIT 20;",
			"Finds Go repositories"
	}
	
	if strings.Contains(queryLower, "recent") || strings.Contains(queryLower, "updated") {
		return "SELECT full_name, description, updated_at, stargazers_count FROM repositories ORDER BY updated_at DESC LIMIT 20;",
			"Shows recently updated repositories"
	}
	
	if strings.Contains(queryLower, "popular") || strings.Contains(queryLower, "stars") {
		return "SELECT full_name, description, stargazers_count FROM repositories ORDER BY stargazers_count DESC LIMIT 20;",
			"Shows most popular repositories by stars"
	}
	
	// Basic patterns for common queries (less specific, so check last)
	if strings.Contains(queryLower, "list") || strings.Contains(queryLower, "show") {
		return "SELECT full_name, description, language, stargazers_count FROM repositories ORDER BY stargazers_count DESC LIMIT 20;",
			"Lists repositories ordered by star count"
	}
	
	// Default query
	return "SELECT full_name, description, language, stargazers_count FROM repositories ORDER BY stargazers_count DESC LIMIT 20;",
		"Default query showing all repositories"
}

// removeDuplicates removes duplicate strings from a slice
func (f *FallbackService) removeDuplicates(slice []string) []string {
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