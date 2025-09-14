package query

import (
	"testing"

	"github.com/kyleking/gh-star-search/internal/storage"
)

func TestSearchEngine_CalculateFuzzyScore(t *testing.T) {
	engine := &SearchEngine{}

	// Test repository
	repo := storage.StoredRepo{
		FullName:     "facebook/react",
		Description:  "A declarative, efficient, and flexible JavaScript library for building user interfaces",
		Purpose:      "JavaScript library for building user interfaces with component-based architecture",
		Technologies: []string{"JavaScript", "JSX", "Virtual DOM"},
		Features:     []string{"component-based", "virtual DOM", "declarative"},
		Topics:       []string{"javascript", "react", "frontend", "ui"},
	}

	tests := []struct {
		name        string
		queryTerms  []string
		expectScore bool // Whether we expect a score > 0
	}{
		{
			name:        "exact name match",
			queryTerms:  []string{"react"},
			expectScore: true,
		},
		{
			name:        "description match",
			queryTerms:  []string{"javascript", "library"},
			expectScore: true,
		},
		{
			name:        "technology match",
			queryTerms:  []string{"jsx"},
			expectScore: true,
		},
		{
			name:        "no match",
			queryTerms:  []string{"python", "django"},
			expectScore: false,
		},
		{
			name:        "partial match",
			queryTerms:  []string{"javascript", "python"},
			expectScore: true, // Should match javascript
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := engine.calculateFuzzyScore(repo, tt.queryTerms)
			
			if tt.expectScore && score <= 0 {
				t.Errorf("Expected score > 0, got %f", score)
			}
			
			if !tt.expectScore && score > 0 {
				t.Errorf("Expected score = 0, got %f", score)
			}
		})
	}
}

func TestSearchEngine_ApplyRankingBoosts(t *testing.T) {
	engine := &SearchEngine{}

	tests := []struct {
		name      string
		repo      storage.StoredRepo
		baseScore float64
		expectMin float64 // Minimum expected score after boost
	}{
		{
			name: "high stars boost",
			repo: storage.StoredRepo{
				StargazersCount: 10000,
			},
			baseScore: 0.5,
			expectMin: 0.5, // Should get some boost
		},
		{
			name: "low stars minimal boost",
			repo: storage.StoredRepo{
				StargazersCount: 10,
			},
			baseScore: 0.5,
			expectMin: 0.5, // Should get minimal boost
		},
		{
			name: "zero score no boost",
			repo: storage.StoredRepo{
				StargazersCount: 10000,
			},
			baseScore: 0.0,
			expectMin: 0.0, // Zero score should remain zero
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := engine.applyRankingBoosts(tt.repo, tt.baseScore)
			
			if score < tt.expectMin {
				t.Errorf("Expected score >= %f, got %f", tt.expectMin, score)
			}
			
			// Score should never exceed 1.0 after clamping
			if score > 1.0 {
				t.Errorf("Score should be clamped to 1.0, got %f", score)
			}
		})
	}
}

func TestSearchEngine_IdentifyMatchedFields(t *testing.T) {
	engine := &SearchEngine{}

	repo := storage.StoredRepo{
		FullName:     "facebook/react",
		Description:  "A JavaScript library",
		Purpose:      "Building user interfaces",
		Technologies: []string{"JavaScript", "JSX"},
		Features:     []string{"component-based"},
		Topics:       []string{"javascript", "frontend"},
	}

	tests := []struct {
		name           string
		queryTerms     []string
		expectedFields []string
	}{
		{
			name:           "name match",
			queryTerms:     []string{"react"},
			expectedFields: []string{"name"},
		},
		{
			name:           "description match",
			queryTerms:     []string{"javascript"},
			expectedFields: []string{"description", "technologies", "topics"},
		},
		{
			name:           "multiple matches",
			queryTerms:     []string{"javascript", "component"},
			expectedFields: []string{"description", "technologies", "features", "topics"},
		},
		{
			name:           "no matches",
			queryTerms:     []string{"python"},
			expectedFields: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := engine.identifyMatchedFields(repo, tt.queryTerms)
			
			// Check that all expected fields are present
			fieldMap := make(map[string]bool)
			for _, field := range fields {
				fieldMap[field] = true
			}
			
			for _, expected := range tt.expectedFields {
				if !fieldMap[expected] {
					t.Errorf("Expected field %s not found in results: %v", expected, fields)
				}
			}
		})
	}
}

func TestTokenizeQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "simple query",
			query:    "react javascript",
			expected: []string{"react", "javascript"},
		},
		{
			name:     "query with extra spaces",
			query:    "  react   javascript  ",
			expected: []string{"react", "javascript"},
		},
		{
			name:     "single term",
			query:    "react",
			expected: []string{"react"},
		},
		{
			name:     "empty query",
			query:    "",
			expected: []string{},
		},
		{
			name:     "quoted terms",
			query:    "web framework",
			expected: []string{"web", "framework"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizeQuery(tt.query)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d terms, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			
			for i, term := range result {
				if term != tt.expected[i] {
					t.Errorf("Expected term %s at position %d, got %s", tt.expected[i], i, term)
				}
			}
		})
	}
}

func TestSortAndRankResults(t *testing.T) {
	results := []Result{
		{
			Score: 0.5,
			Repository: storage.StoredRepo{
				FullName:        "repo1",
				StargazersCount: 100,
			},
		},
		{
			Score: 0.8,
			Repository: storage.StoredRepo{
				FullName:        "repo2",
				StargazersCount: 50,
			},
		},
		{
			Score: 0.5,
			Repository: storage.StoredRepo{
				FullName:        "repo3",
				StargazersCount: 200,
			},
		},
	}

	sorted := sortAndRankResults(results)

	// Should be sorted by score desc, then by stars desc
	expectedOrder := []string{"repo2", "repo3", "repo1"}
	
	for i, result := range sorted {
		if result.Repository.FullName != expectedOrder[i] {
			t.Errorf("Expected %s at position %d, got %s", expectedOrder[i], i, result.Repository.FullName)
		}
		
		if result.Rank != i+1 {
			t.Errorf("Expected rank %d, got %d", i+1, result.Rank)
		}
	}
}