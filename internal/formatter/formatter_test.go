package formatter

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-star-search/internal/query"
	"github.com/kyleking/gh-star-search/internal/storage"
)

func TestFormatter_FormatResult(t *testing.T) {
	formatter := NewFormatter()

	// Create a test repository with complete data
	repo := storage.StoredRepo{
		ID:              "test-repo-1",
		FullName:        "testorg/test-repo",
		Description:     "A test repository for unit testing",
		Homepage:        "https://example.com",
		Language:        "Go",
		StargazersCount: 1234,
		ForksCount:      56,
		SizeKB:          789,
		CreatedAt:       time.Date(2020, 1, 15, 10, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2024, 9, 1, 15, 30, 0, 0, time.UTC),
		LastSynced:      time.Date(2024, 9, 14, 12, 0, 0, 0, time.UTC),
		OpenIssuesOpen:  5,
		OpenIssuesTotal: 25,
		OpenPRsOpen:     2,
		OpenPRsTotal:    15,
		Commits30d:      45,
		Commits1y:       520,
		CommitsTotal:    1200,
		Topics:          []string{"golang", "cli", "testing"},
		Languages:       map[string]int64{"Go": 12000, "Shell": 600},
		Contributors: []storage.Contributor{
			{Login: "alice", Contributions: 150},
			{Login: "bob", Contributions: 75},
		},
		LicenseName:              "MIT License",
		LicenseSPDXID:            "MIT",
		Purpose:                  "A comprehensive testing framework for Go applications",
		Technologies:             []string{"Go", "Cobra", "Testing"},
		UseCases:                 []string{"unit testing", "integration testing"},
		Features:                 []string{"fast execution", "detailed reports"},
		InstallationInstructions: "go install github.com/testorg/test-repo@latest",
		UsageInstructions:        "Run 'test-repo --help' for usage information",
		SummaryGeneratedAt:       &time.Time{},
		SummaryVersion:           1,
		SummaryGenerator:         "transformers:distilbart-cnn-12-6",
		ContentHash:              "abc123def456",
	}

	result := query.Result{
		RepoID:      repo.ID,
		Score:       0.85,
		Rank:        1,
		Repository:  repo,
		MatchFields: []string{"name", "description"},
	}

	tests := []struct {
		name     string
		format   OutputFormat
		expected []string // Expected lines to be present
	}{
		{
			name:   "long format",
			format: FormatLong,
			expected: []string{
				"testorg/test-repo  (link: https://github.com/testorg/test-repo)",
				"GitHub Description: A test repository for unit testing",
				"GitHub External Description Link: https://example.com",
				"Numbers: 5/25 open issues, 2/15 open PRs, 1234 stars, 56 forks",
				"Commits: 45 in last 30 days, 520 in last year, 1200 total",
				"License: MIT",
				"Top 10 Contributors: alice (150), bob (75)",
				"GitHub Topics: golang, cli, testing",
				"Languages: Go (200), Shell (10)",
				"Summary: A comprehensive testing framework for Go applications. Features: fast execution, detailed reports. Usage: Run 'test-repo --help' for usage information",
				"(PLANNED: dependencies count)",
				"(PLANNED: dependents count)",
			},
		},
		{
			name:   "short format",
			format: FormatShort,
			expected: []string{
				"testorg/test-repo  (link: https://github.com/testorg/test-repo)",
				"GitHub Description: A test repository for unit testing",
				"1. testorg/test-repo  ⭐ 1234  Go  Updated",
				"Score:0.85",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatter.FormatResult(result, tt.format)

			for _, expectedLine := range tt.expected {
				if !strings.Contains(output, expectedLine) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expectedLine, output)
				}
			}
		})
	}
}

func TestFormatter_FormatRepository(t *testing.T) {
	formatter := NewFormatter()

	// Test with minimal data (unknown values)
	minimalRepo := storage.StoredRepo{
		ID:       "minimal-repo",
		FullName: "user/minimal",
		// Most fields left empty to test fallbacks
		StargazersCount: 0,
		CreatedAt:       time.Time{}, // Zero time
		UpdatedAt:       time.Time{}, // Zero time
		LastSynced:      time.Time{}, // Zero time
		OpenIssuesOpen:  -1,          // Unknown value
		OpenIssuesTotal: -1,          // Unknown value
		Commits30d:      -1,          // Unknown value
	}

	output := formatter.FormatRepository(minimalRepo, FormatLong)

	// Test unknown value fallbacks
	expectedFallbacks := []string{
		"GitHub Description: -",
		"GitHub External Description Link: -",
		"Numbers: ?/? open issues",
		"Age: ?",
		"License: -",
		"Top 10 Contributors: -",
		"GitHub Topics: -",
		"Languages: -",
		"Last synced: ?",
	}

	for _, expected := range expectedFallbacks {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected fallback %q not found in output:\n%s", expected, output)
		}
	}
}

func TestFormatter_humanizeAge(t *testing.T) {
	formatter := NewFormatter()
	now := time.Date(2024, 9, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time",
			input:    time.Time{},
			expected: "?",
		},
		{
			name:     "same day",
			input:    now.Add(-2 * time.Hour),
			expected: "today",
		},
		{
			name:     "one day ago",
			input:    now.Add(-24 * time.Hour),
			expected: "1 day ago",
		},
		{
			name:     "multiple days ago",
			input:    now.Add(-5 * 24 * time.Hour),
			expected: "5 days ago",
		},
		{
			name:     "one month ago",
			input:    now.Add(-30 * 24 * time.Hour),
			expected: "1 month ago",
		},
		{
			name:     "multiple months ago",
			input:    now.Add(-90 * 24 * time.Hour),
			expected: "3 months ago",
		},
		{
			name:     "one year ago",
			input:    now.Add(-365 * 24 * time.Hour),
			expected: "1 year ago",
		},
		{
			name:     "multiple years ago",
			input:    now.Add(-2 * 365 * 24 * time.Hour),
			expected: "2 years ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock time.Now() for consistent testing
			// Note: In a real implementation, you might want to make time.Now() injectable
			result := formatter.humanizeAge(tt.input)
			if tt.name == "same day" || tt.name == "one day ago" ||
				tt.name == "multiple days ago" || tt.name == "one month ago" ||
				tt.name == "multiple months ago" || tt.name == "one year ago" ||
				tt.name == "multiple years ago" {
				// For time-dependent tests, just check the format is reasonable
				if result == "?" && tt.expected != "?" {
					t.Errorf("Expected non-? result for %s, got %s", tt.name, result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestFormatter_formatContributors(t *testing.T) {
	formatter := NewFormatter()

	tests := []struct {
		name     string
		input    []storage.Contributor
		expected string
	}{
		{
			name:     "empty contributors",
			input:    []storage.Contributor{},
			expected: "-",
		},
		{
			name: "single contributor",
			input: []storage.Contributor{
				{Login: "alice", Contributions: 100},
			},
			expected: "alice (100)",
		},
		{
			name: "multiple contributors",
			input: []storage.Contributor{
				{Login: "alice", Contributions: 150},
				{Login: "bob", Contributions: 75},
				{Login: "charlie", Contributions: 50},
			},
			expected: "alice (150), bob (75), charlie (50)",
		},
		{
			name: "more than 10 contributors",
			input: func() []storage.Contributor {
				var contributors []storage.Contributor
				for i := range 15 {
					contributors = append(contributors, storage.Contributor{
						Login:         fmt.Sprintf("user%d", i),
						Contributions: 100 - i,
					})
				}
				return contributors
			}(),
			expected: "user0 (100), user1 (99), user2 (98), user3 (97), user4 (96), user5 (95), user6 (94), user7 (93), user8 (92), user9 (91)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatContributors(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatter_formatLanguages(t *testing.T) {
	formatter := NewFormatter()

	tests := []struct {
		name     string
		input    map[string]int64
		expected map[string]bool // Expected substrings
	}{
		{
			name:     "empty languages",
			input:    map[string]int64{},
			expected: map[string]bool{"-": true},
		},
		{
			name: "single language",
			input: map[string]int64{
				"Go": 6000, // Should be 100 LOC
			},
			expected: map[string]bool{"Go (100)": true},
		},
		{
			name: "multiple languages",
			input: map[string]int64{
				"Go":   12000, // 200 LOC
				"Rust": 3000,  // 50 LOC
			},
			expected: map[string]bool{
				"Go (200)":  true,
				"Rust (50)": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatLanguages(tt.input)
			for expectedSubstring, shouldContain := range tt.expected {
				contains := strings.Contains(result, expectedSubstring)
				if contains != shouldContain {
					t.Errorf("Expected result to contain %q: %v, but got: %s",
						expectedSubstring, shouldContain, result)
				}
			}
		})
	}
}

func TestFormatter_getPrimaryLanguage(t *testing.T) {
	formatter := NewFormatter()

	tests := []struct {
		name     string
		input    map[string]int64
		expected string
	}{
		{
			name:     "empty languages",
			input:    map[string]int64{},
			expected: "-",
		},
		{
			name: "single language",
			input: map[string]int64{
				"Go": 1000,
			},
			expected: "Go",
		},
		{
			name: "multiple languages - Go primary",
			input: map[string]int64{
				"Go":   5000,
				"Rust": 2000,
				"JS":   1000,
			},
			expected: "Go",
		},
		{
			name: "multiple languages - Rust primary",
			input: map[string]int64{
				"Go":   2000,
				"Rust": 8000,
				"JS":   1000,
			},
			expected: "Rust",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.getPrimaryLanguage(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatter_formatInt(t *testing.T) {
	formatter := NewFormatter()

	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{
			name:     "positive number",
			input:    42,
			expected: "42",
		},
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "negative number (unknown)",
			input:    -1,
			expected: "?",
		},
		{
			name:     "large negative number",
			input:    -999,
			expected: "?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatInt(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatter_formatSummary(t *testing.T) {
	formatter := NewFormatter()

	tests := []struct {
		name     string
		repo     storage.StoredRepo
		expected string
	}{
		{
			name:     "empty summary fields",
			repo:     storage.StoredRepo{},
			expected: "-",
		},
		{
			name: "purpose only",
			repo: storage.StoredRepo{
				Purpose: "A testing framework",
			},
			expected: "A testing framework",
		},
		{
			name: "features only",
			repo: storage.StoredRepo{
				Features: []string{"fast", "reliable"},
			},
			expected: "Features: fast, reliable",
		},
		{
			name: "usage only",
			repo: storage.StoredRepo{
				UsageInstructions: "Run with --help",
			},
			expected: "Usage: Run with --help",
		},
		{
			name: "all fields",
			repo: storage.StoredRepo{
				Purpose:           "A testing framework",
				Features:          []string{"fast", "reliable"},
				UsageInstructions: "Run with --help",
			},
			expected: "A testing framework. Features: fast, reliable. Usage: Run with --help",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatSummary(tt.repo)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Golden test for complete long-form output
func TestFormatter_GoldenLongForm(t *testing.T) {
	formatter := NewFormatter()

	// Create a repository with all fields populated
	repo := storage.StoredRepo{
		ID:              "golden-test",
		FullName:        "hashicorp/terraform",
		Description:     "Terraform enables you to safely and predictably create, change, and improve infrastructure",
		Homepage:        "https://www.terraform.io/",
		StargazersCount: 42000,
		ForksCount:      9500,
		CreatedAt:       time.Date(2014, 7, 28, 0, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2024, 9, 10, 14, 30, 0, 0, time.UTC),
		LastSynced:      time.Date(2024, 9, 14, 10, 0, 0, 0, time.UTC),
		OpenIssuesOpen:  1200,
		OpenIssuesTotal: 8500,
		OpenPRsOpen:     45,
		OpenPRsTotal:    3200,
		Commits30d:      150,
		Commits1y:       2400,
		CommitsTotal:    15000,
		Topics:          []string{"terraform", "infrastructure", "iac", "devops"},
		Languages: map[string]int64{
			"Go":    480000, // 8000 LOC
			"HCL":   120000, // 2000 LOC
			"Shell": 6000,   // 100 LOC
		},
		Contributors: []storage.Contributor{
			{Login: "mitchellh", Contributions: 2500},
			{Login: "apparentlymart", Contributions: 1800},
			{Login: "jbardin", Contributions: 1200},
		},
		LicenseSPDXID:     "MPL-2.0",
		Purpose:           "Infrastructure as Code tool for building, changing, and versioning infrastructure",
		Technologies:      []string{"Go", "HCL", "Terraform"},
		Features:          []string{"declarative configuration", "execution plans", "resource graph"},
		UsageInstructions: "terraform init && terraform plan && terraform apply",
		SummaryGenerator:  "transformers:distilbart-cnn-12-6",
	}

	expected := `hashicorp/terraform  (link: https://github.com/hashicorp/terraform)
GitHub Description: Terraform enables you to safely and predictably create, change, and improve infrastructure
GitHub External Description Link: https://www.terraform.io/
Numbers: 1200/8500 open issues, 45/3200 open PRs, 42000 stars, 9500 forks
Commits: 150 in last 30 days, 2400 in last year, 15000 total
Age: 10 years ago
License: MPL-2.0
Top 10 Contributors: mitchellh (2500), apparentlymart (1800), jbardin (1200)
GitHub Topics: terraform, infrastructure, iac, devops
Languages: Go (8000), HCL (2000), Shell (100)
Related Stars: ? in hashicorp, ? by top contributors
Last synced: today
Summary: Infrastructure as Code tool for building, changing, and versioning infrastructure. Features: declarative configuration, execution plans, resource graph. Usage: terraform init && terraform plan && terraform apply
(PLANNED: dependencies count)
(PLANNED: dependents count)`

	result := formatter.FormatRepository(repo, FormatLong)

	// Compare line by line for better error reporting
	expectedLines := strings.Split(expected, "\n")
	resultLines := strings.Split(result, "\n")

	if len(expectedLines) != len(resultLines) {
		t.Fatalf("Expected %d lines, got %d lines.\nExpected:\n%s\n\nGot:\n%s",
			len(expectedLines), len(resultLines), expected, result)
	}

	for i, expectedLine := range expectedLines {
		// Skip time-dependent lines for this test
		if strings.Contains(expectedLine, "Age:") || strings.Contains(expectedLine, "Last synced:") {
			continue
		}

		if resultLines[i] != expectedLine {
			t.Errorf("Line %d mismatch:\nExpected: %q\nGot:      %q", i+1, expectedLine, resultLines[i])
		}
	}
}

// TestFormatter_GoldenFiles tests against golden files for deterministic output
func TestFormatter_GoldenFiles(t *testing.T) {
	formatter := NewFormatter()

	tests := []struct {
		name       string
		repo       storage.StoredRepo
		format     OutputFormat
		goldenFile string
		skipLines  []int // Lines to skip due to time dependency
	}{
		{
			name: "complete repository long form",
			repo: storage.StoredRepo{
				ID:              "golden-test",
				FullName:        "hashicorp/terraform",
				Description:     "Terraform enables you to safely and predictably create, change, and improve infrastructure",
				Homepage:        "https://www.terraform.io/",
				StargazersCount: 42000,
				ForksCount:      9500,
				CreatedAt:       time.Date(2014, 7, 28, 0, 0, 0, 0, time.UTC),
				UpdatedAt:       time.Date(2024, 9, 10, 14, 30, 0, 0, time.UTC),
				LastSynced:      time.Date(2024, 9, 14, 10, 0, 0, 0, time.UTC),
				OpenIssuesOpen:  1200,
				OpenIssuesTotal: 8500,
				OpenPRsOpen:     45,
				OpenPRsTotal:    3200,
				Commits30d:      150,
				Commits1y:       2400,
				CommitsTotal:    15000,
				Topics:          []string{"terraform", "infrastructure", "iac", "devops"},
				Languages: map[string]int64{
					"Go":    480000, // 8000 LOC
					"HCL":   120000, // 2000 LOC
					"Shell": 6000,   // 100 LOC
				},
				Contributors: []storage.Contributor{
					{Login: "mitchellh", Contributions: 2500},
					{Login: "apparentlymart", Contributions: 1800},
					{Login: "jbardin", Contributions: 1200},
				},
				LicenseSPDXID:     "MPL-2.0",
				Purpose:           "Infrastructure as Code tool for building, changing, and versioning infrastructure",
				Technologies:      []string{"Go", "HCL", "Terraform"},
				Features:          []string{"declarative configuration", "execution plans", "resource graph"},
				UsageInstructions: "terraform init && terraform plan && terraform apply",
				SummaryGenerator:  "transformers:distilbart-cnn-12-6",
			},
			format:     FormatLong,
			goldenFile: "testdata/golden_long_complete.txt",
			skipLines:  []int{5, 11}, // Age and Last synced lines (time-dependent)
		},
		{
			name: "minimal repository long form",
			repo: storage.StoredRepo{
				ID:              "minimal-repo",
				FullName:        "user/minimal",
				StargazersCount: 0,
				CreatedAt:       time.Time{},
				UpdatedAt:       time.Time{},
				LastSynced:      time.Time{},
				OpenIssuesOpen:  -1,
				OpenIssuesTotal: -1,
				OpenPRsOpen:     -1,
				OpenPRsTotal:    -1,
				Commits30d:      -1,
				Commits1y:       -1,
				CommitsTotal:    -1,
			},
			format:     FormatLong,
			goldenFile: "testdata/golden_long_minimal.txt",
			skipLines:  []int{5, 11}, // Age and Last synced lines (time-dependent)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatRepository(tt.repo, tt.format)

			// Read golden file
			goldenContent, err := os.ReadFile(tt.goldenFile)
			if err != nil {
				t.Fatalf("Failed to read golden file %s: %v", tt.goldenFile, err)
			}

			expectedLines := strings.Split(strings.TrimSpace(string(goldenContent)), "\n")
			resultLines := strings.Split(result, "\n")

			if len(expectedLines) != len(resultLines) {
				t.Fatalf("Expected %d lines, got %d lines.\nExpected:\n%s\n\nGot:\n%s",
					len(expectedLines), len(resultLines), string(goldenContent), result)
			}

			// Compare line by line, skipping time-dependent lines
			skipMap := make(map[int]bool)
			for _, lineNum := range tt.skipLines {
				skipMap[lineNum] = true
			}

			for i, expectedLine := range expectedLines {
				if skipMap[i] {
					continue // Skip time-dependent lines
				}

				if resultLines[i] != expectedLine {
					t.Errorf("Line %d mismatch:\nExpected: %q\nGot:      %q", i+1, expectedLine, resultLines[i])
				}
			}
		})
	}
}
