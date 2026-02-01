package query

import (
	"math"
	"testing"
	"time"

	"github.com/kyleking/gh-star-search/internal/storage"
)

func TestSearchEngine_CalculateFuzzyScore(t *testing.T) {
	engine := &SearchEngine{}

	// Test repository
	repo := storage.StoredRepo{
		FullName:    "facebook/react",
		Description: "A declarative, efficient, and flexible JavaScript library for building user interfaces",
		Topics:      []string{"javascript", "react", "frontend", "ui"},
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
			expectScore: false, // Technologies field removed, so no match expected
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
		FullName:    "facebook/react",
		Description: "A JavaScript library",
		Topics:      []string{"javascript", "frontend"},
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
			expectedFields: []string{"description", "topics"},
		},
		{
			name:           "multiple matches",
			queryTerms:     []string{"javascript", "component"},
			expectedFields: []string{"description", "topics"},
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
			t.Errorf(
				"Expected %s at position %d, got %s",
				expectedOrder[i],
				i,
				result.Repository.FullName,
			)
		}

		if result.Rank != i+1 {
			t.Errorf("Expected rank %d, got %d", i+1, result.Rank)
		}
	}
}

func TestRecencyDecay(t *testing.T) {
	engine := &SearchEngine{}
	baseScore := 0.5

	tests := []struct {
		name        string
		daysAgo     float64
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "updated today has factor ~1.0",
			daysAgo:     0,
			expectedMin: baseScore * 0.99,
			expectedMax: baseScore * 1.01,
		},
		{
			name:        "updated 182 days ago has factor ~0.9",
			daysAgo:     182.5,
			expectedMin: baseScore * 0.89,
			expectedMax: baseScore * 0.91,
		},
		{
			name:        "updated 365 days ago has factor ~0.8",
			daysAgo:     365,
			expectedMin: baseScore * 0.79,
			expectedMax: baseScore * 0.81,
		},
		{
			name:        "updated 730 days ago still clamped to factor ~0.8",
			daysAgo:     730,
			expectedMin: baseScore * 0.79,
			expectedMax: baseScore * 0.81,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := storage.StoredRepo{
				UpdatedAt: time.Now().Add(-time.Duration(tt.daysAgo*24) * time.Hour),
			}
			score := engine.applyRankingBoosts(repo, baseScore)

			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf(
					"Expected score in [%f, %f], got %f (recency factor ~%.4f)",
					tt.expectedMin, tt.expectedMax, score, score/baseScore,
				)
			}
		})
	}

	t.Run("zero UpdatedAt skips recency decay", func(t *testing.T) {
		repo := storage.StoredRepo{}
		score := engine.applyRankingBoosts(repo, baseScore)
		if score != baseScore {
			t.Errorf("Expected score %f with zero UpdatedAt, got %f", baseScore, score)
		}
	})
}

func TestStarBoost(t *testing.T) {
	engine := &SearchEngine{}
	baseScore := 0.5
	tolerance := 0.0001

	recentlyUpdated := time.Now()

	tests := []struct {
		name          string
		stars         int
		expectedBoost float64
	}{
		{
			name:          "0 stars gives boost of 1.0",
			stars:         0,
			expectedBoost: 1.0,
		},
		{
			name:          "100 stars gives small logarithmic boost",
			stars:         100,
			expectedBoost: 1.0 + (0.1 * math.Log10(101) / 6.0),
		},
		{
			name:          "10000 stars gives larger logarithmic boost",
			stars:         10000,
			expectedBoost: 1.0 + (0.1 * math.Log10(10001) / 6.0),
		},
		{
			name:          "1 star gives minimal boost",
			stars:         1,
			expectedBoost: 1.0 + (0.1 * math.Log10(2) / 6.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := storage.StoredRepo{
				StargazersCount: tt.stars,
				UpdatedAt:       recentlyUpdated,
			}
			score := engine.applyRankingBoosts(repo, baseScore)
			actualBoost := score / baseScore

			if math.Abs(actualBoost-tt.expectedBoost) > tolerance {
				t.Errorf(
					"Expected boost ~%f for %d stars, got %f (score=%f)",
					tt.expectedBoost, tt.stars, actualBoost, score,
				)
			}
		})
	}

	t.Run("star boost increases monotonically", func(t *testing.T) {
		starCounts := []int{0, 1, 10, 100, 1000, 10000, 100000}
		prevScore := 0.0
		for _, stars := range starCounts {
			repo := storage.StoredRepo{
				StargazersCount: stars,
				UpdatedAt:       recentlyUpdated,
			}
			score := engine.applyRankingBoosts(repo, baseScore)
			if score < prevScore {
				t.Errorf("Score decreased from %f to %f at %d stars", prevScore, score, stars)
			}
			prevScore = score
		}
	})
}

func TestScoreClamping(t *testing.T) {
	engine := &SearchEngine{}

	recentlyUpdated := time.Now()

	tests := []struct {
		name      string
		baseScore float64
		stars     int
	}{
		{
			name:      "high base score with high stars clamped to 1.0",
			baseScore: 0.99,
			stars:     100000,
		},
		{
			name:      "base score of 1.0 clamped after boost",
			baseScore: 1.0,
			stars:     10000,
		},
		{
			name:      "base score above 1.0 clamped",
			baseScore: 1.5,
			stars:     10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := storage.StoredRepo{
				StargazersCount: tt.stars,
				UpdatedAt:       recentlyUpdated,
			}
			boosted := engine.applyRankingBoosts(repo, tt.baseScore)

			clamped := boosted
			if clamped > 1.0 {
				clamped = 1.0
			}

			if clamped > 1.0 {
				t.Errorf("Clamped score should not exceed 1.0, got %f", clamped)
			}

			if tt.baseScore > 1.0 && boosted <= 1.0 {
				t.Errorf(
					"applyRankingBoosts does not clamp; expected boosted > 1.0 for base %f",
					tt.baseScore,
				)
			}
		})
	}

	t.Run("fuzzy search flow clamps scores", func(t *testing.T) {
		boosted := engine.applyRankingBoosts(storage.StoredRepo{
			StargazersCount: 100000,
			UpdatedAt:       recentlyUpdated,
		}, 0.98)

		if boosted > 1.0 {
			boosted = 1.0
		}
		if boosted > 1.0 {
			t.Errorf("Score after clamping must be <= 1.0, got %f", boosted)
		}
	})

	t.Run("zero base score remains zero", func(t *testing.T) {
		score := engine.applyRankingBoosts(storage.StoredRepo{
			StargazersCount: 100000,
			UpdatedAt:       recentlyUpdated,
		}, 0.0)
		if score != 0.0 {
			t.Errorf("Expected 0.0 for zero base score, got %f", score)
		}
	})

	t.Run("negative base score remains unchanged", func(t *testing.T) {
		score := engine.applyRankingBoosts(storage.StoredRepo{
			StargazersCount: 100000,
			UpdatedAt:       recentlyUpdated,
		}, -0.5)
		if score != -0.5 {
			t.Errorf("Expected -0.5 for negative base score, got %f", score)
		}
	})
}
