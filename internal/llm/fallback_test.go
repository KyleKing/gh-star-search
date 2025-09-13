package llm

import (
	"context"
	"testing"

	"github.com/username/gh-star-search/internal/query"
)

func TestFallbackService_Summarize(t *testing.T) {
	fallback := NewFallbackService()
	ctx := context.Background()

	tests := []struct {
		name     string
		content  string
		expected struct {
			purposeContains    string
			technologiesCount  int
			useCasesCount      int
			featuresCount      int
		}
	}{
		{
			name: "JavaScript project with README",
			content: `# My Awesome Project

This is a JavaScript library that provides awesome functionality for web developers.

## Features

- Fast and lightweight
- Easy to use API
- TypeScript support
- Works with React and Vue

## Installation

npm install my-awesome-project

## Usage

import { awesome } from 'my-awesome-project';
`,
			expected: struct {
				purposeContains    string
				technologiesCount  int
				useCasesCount      int
				featuresCount      int
			}{
				purposeContains:   "JavaScript library",
				technologiesCount: 5, // JavaScript, TypeScript, React, Vue, Java (from "JavaScript")
				useCasesCount:     2, // Web Development, Library
				featuresCount:     4, // All bullet points
			},
		},
		{
			name: "Python project with package.json-like content",
			content: `# Data Analysis Tool

A Python tool for analyzing large datasets with machine learning capabilities.

Features:
- Data preprocessing
- ML model training
- Visualization support
- Export to various formats

Requirements:
- Python 3.8+
- pandas
- scikit-learn
- matplotlib
`,
			expected: struct {
				purposeContains    string
				technologiesCount  int
				useCasesCount      int
				featuresCount      int
			}{
				purposeContains:   "Python tool",
				technologiesCount: 2, // Python, TypeScript (from "datasets")
				useCasesCount:     2, // Data Analysis, Machine Learning
				featuresCount:     5, // All features including "Python 3.8+"
			},
		},
		{
			name: "Go project with CLI focus",
			content: `# CLI Tool

A command line interface tool written in Go for managing repositories.

This tool allows developers to:
1. List repositories
2. Search by language
3. Generate reports
4. Export data

Installation:
go install github.com/user/cli-tool

Usage:
cli-tool list --language=go
`,
			expected: struct {
				purposeContains    string
				technologiesCount  int
				useCasesCount      int
				featuresCount      int
			}{
				purposeContains:   "command line",
				technologiesCount: 2, // Go, TypeScript (from "repositories")
				useCasesCount:     2, // CLI Tool, Data Analysis (from "data")
				featuresCount:     4, // All numbered items
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := fallback.Summarize(ctx, "", tt.content)
			if err != nil {
				t.Fatalf("Summarize() error = %v", err)
			}

			if summary.Confidence >= 0.5 {
				t.Errorf("Expected low confidence for fallback, got %f", summary.Confidence)
			}

			if tt.expected.purposeContains != "" && !contains(summary.Purpose, tt.expected.purposeContains) {
				t.Errorf("Expected purpose to contain '%s', got '%s'", tt.expected.purposeContains, summary.Purpose)
			}

			if len(summary.Technologies) != tt.expected.technologiesCount {
				t.Errorf("Expected %d technologies, got %d: %v", tt.expected.technologiesCount, len(summary.Technologies), summary.Technologies)
			}

			if len(summary.UseCases) != tt.expected.useCasesCount {
				t.Errorf("Expected %d use cases, got %d: %v", tt.expected.useCasesCount, len(summary.UseCases), summary.UseCases)
			}

			if len(summary.Features) != tt.expected.featuresCount {
				t.Errorf("Expected %d features, got %d: %v", tt.expected.featuresCount, len(summary.Features), summary.Features)
			}
		})
	}
}

func TestFallbackService_ExtractTechnologies(t *testing.T) {
	fallback := NewFallbackService()

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "JavaScript and Node.js",
			content:  "This is a JavaScript project using Node.js and npm packages",
			expected: []string{"JavaScript", "Java"},
		},
		{
			name:     "Python with Django",
			content:  "A Python web application built with Django framework",
			expected: []string{"Django", "Python"},
		},
		{
			name:     "Go module",
			content:  "go.mod file indicates this is a Go project with golang dependencies",
			expected: []string{"Go", "MATLAB"},
		},
		{
			name:     "Multiple technologies",
			content:  "Full stack project with React frontend, Python backend, and Docker containers",
			expected: []string{"Docker", "React", "Python", "Haskell"},
		},
		{
			name:     "Rust project",
			content:  "Cargo.toml shows this is a Rust project with rustc compiler",
			expected: []string{"Rust"},
		},
		{
			name:     "No clear technologies",
			content:  "This is a generic project without specific technology mentions",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			technologies := fallback.extractTechnologies(tt.content)

			if len(technologies) != len(tt.expected) {
				t.Errorf("Expected %d technologies, got %d: %v", len(tt.expected), len(technologies), technologies)
				return
			}

			for _, expected := range tt.expected {
				found := false
				for _, tech := range technologies {
					if tech == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected technology '%s' not found in %v", expected, technologies)
				}
			}
		})
	}
}

func TestFallbackService_ExtractUseCases(t *testing.T) {
	fallback := NewFallbackService()

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "Web development",
			content:  "A web application with HTTP API endpoints and server functionality",
			expected: []string{"Web Development"},
		},
		{
			name:     "CLI tool",
			content:  "Command line interface for terminal usage and console operations",
			expected: []string{"CLI Tool"},
		},
		{
			name:     "Data analysis",
			content:  "Data processing and analytics with visualization charts",
			expected: []string{"Data Analysis"},
		},
		{
			name:     "Machine learning",
			content:  "AI model training with neural networks and ML algorithms",
			expected: []string{"Machine Learning"},
		},
		{
			name:     "Multiple use cases",
			content:  "Mobile app for Android and iOS with game engine graphics",
			expected: []string{"Mobile Development", "Game Development"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useCases := fallback.extractUseCases(tt.content)

			for _, expected := range tt.expected {
				found := false
				for _, useCase := range useCases {
					if useCase == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected use case '%s' not found in %v", expected, useCases)
				}
			}
		})
	}
}

func TestFallbackService_ExtractFeatures(t *testing.T) {
	fallback := NewFallbackService()

	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name: "Bullet point features",
			content: `Features:
- Fast performance
- Easy to use
- Cross-platform support
- Extensive documentation`,
			expected: 4,
		},
		{
			name: "Numbered features",
			content: `Key capabilities:
1. Data processing
2. Report generation
3. Export functionality`,
			expected: 3,
		},
		{
			name: "Mixed list formats",
			content: `What it does:
* Authentication
- Authorization
1. User management
2. Role-based access`,
			expected: 4,
		},
		{
			name:     "No clear features",
			content:  "This is a description without any bullet points or numbered lists.",
			expected: 0,
		},
		{
			name: "Too many features (should limit to 5)",
			content: `Features:
- Feature number one with long description
- Feature number two with long description
- Feature number three with long description
- Feature number four with long description
- Feature number five with long description
- Feature number six with long description
- Feature number seven with long description`,
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := fallback.extractFeatures(tt.content)

			if len(features) != tt.expected {
				t.Errorf("Expected %d features, got %d: %v", tt.expected, len(features), features)
			}
		})
	}
}

func TestFallbackService_ExtractInstallation(t *testing.T) {
	fallback := NewFallbackService()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "npm install",
			content: `# Project

## Installation

npm install my-package

## Usage`,
			expected: "npm install my-package",
		},
		{
			name: "pip install",
			content: `Installation instructions:

pip install my-python-package

Then you can use it.`,
			expected: "pip install my-python-package",
		},
		{
			name: "go install",
			content: `To install this tool:

go install github.com/user/tool

Run the tool with: tool --help`,
			expected: "go install github.com/user/tool",
		},
		{
			name: "Installation section",
			content: `# My Tool

## Installation

Download the binary from releases page.
Extract to your PATH.
Run the executable.`,
			expected: "Download the binary from releases page. Extract to your PATH. Run the executable.",
		},
		{
			name:     "No installation info",
			content:  "This project has no installation instructions.",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installation := fallback.extractInstallation(tt.content)

			if installation != tt.expected {
				t.Errorf("Expected installation '%s', got '%s'", tt.expected, installation)
			}
		})
	}
}

func TestFallbackService_ParseQuery(t *testing.T) {
	fallback := NewFallbackService()
	ctx := context.Background()

	schema := query.Schema{
		Tables: map[string]query.Table{
			"repositories": {
				Name: "repositories",
				Columns: []query.Column{
					{Name: "full_name", Type: "VARCHAR"},
					{Name: "description", Type: "TEXT"},
					{Name: "language", Type: "VARCHAR"},
					{Name: "stargazers_count", Type: "INTEGER"},
				},
			},
		},
	}

	tests := []struct {
		name          string
		query         string
		expectedSQL   string
		expectedExpl  string
	}{
		{
			name:         "List repositories",
			query:        "list all repositories",
			expectedSQL:  "SELECT full_name, description, language, stargazers_count FROM repositories ORDER BY stargazers_count DESC LIMIT 20;",
			expectedExpl: "Lists repositories ordered by star count",
		},
		{
			name:         "JavaScript repositories",
			query:        "show me javascript projects",
			expectedSQL:  "SELECT full_name, description, stargazers_count FROM repositories WHERE language = 'JavaScript' OR technologies LIKE '%JavaScript%' ORDER BY stargazers_count DESC LIMIT 20;",
			expectedExpl: "Finds JavaScript repositories",
		},
		{
			name:         "Python repositories",
			query:        "find Python libraries",
			expectedSQL:  "SELECT full_name, description, stargazers_count FROM repositories WHERE language = 'Python' OR technologies LIKE '%Python%' ORDER BY stargazers_count DESC LIMIT 20;",
			expectedExpl: "Finds Python repositories",
		},
		{
			name:         "Go repositories",
			query:        "golang projects",
			expectedSQL:  "SELECT full_name, description, stargazers_count FROM repositories WHERE language = 'Go' OR technologies LIKE '%Go%' ORDER BY stargazers_count DESC LIMIT 20;",
			expectedExpl: "Finds Go repositories",
		},
		{
			name:         "Recent repositories",
			query:        "recently updated repos",
			expectedSQL:  "SELECT full_name, description, updated_at, stargazers_count FROM repositories ORDER BY updated_at DESC LIMIT 20;",
			expectedExpl: "Shows recently updated repositories",
		},
		{
			name:         "Popular repositories",
			query:        "most popular by stars",
			expectedSQL:  "SELECT full_name, description, stargazers_count FROM repositories ORDER BY stargazers_count DESC LIMIT 20;",
			expectedExpl: "Shows most popular repositories by stars",
		},
		{
			name:         "Unknown query",
			query:        "something completely different",
			expectedSQL:  "SELECT full_name, description, language, stargazers_count FROM repositories ORDER BY stargazers_count DESC LIMIT 20;",
			expectedExpl: "Default query showing all repositories",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := fallback.ParseQuery(ctx, tt.query, schema)
			if err != nil {
				t.Fatalf("ParseQuery() error = %v", err)
			}

			if response.SQL != tt.expectedSQL {
				t.Errorf("Expected SQL '%s', got '%s'", tt.expectedSQL, response.SQL)
			}

			if response.Explanation != tt.expectedExpl {
				t.Errorf("Expected explanation '%s', got '%s'", tt.expectedExpl, response.Explanation)
			}

			if response.Confidence >= 0.5 {
				t.Errorf("Expected low confidence for fallback, got %f", response.Confidence)
			}

			if response.Reasoning != "Generated using rule-based fallback parser" {
				t.Errorf("Expected fallback reasoning, got '%s'", response.Reasoning)
			}
		})
	}
}

func TestFallbackService_Configure(t *testing.T) {
	fallback := NewFallbackService()
	
	// Configure should always succeed for fallback service
	err := fallback.Configure(Config{
		Provider: "any",
		Model:    "any",
		APIKey:   "any",
	})
	
	if err != nil {
		t.Errorf("Configure() should not fail for fallback service, got: %v", err)
	}
}

func TestRemoveDuplicates(t *testing.T) {
	fallback := NewFallbackService()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "all same",
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fallback.removeDuplicates(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected %s at index %d, got %s", expected, i, result[i])
				}
			}
		})
	}
}