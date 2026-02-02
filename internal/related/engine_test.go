package related

import (
	"testing"

	"github.com/KyleKing/gh-star-search/internal/storage"
)

func TestEngineImpl_CalculateSameOrgScore(t *testing.T) {
	engine := &EngineImpl{}

	tests := []struct {
		name          string
		targetRepo    string
		candidateRepo string
		expectedScore float64
	}{
		{
			name:          "same org",
			targetRepo:    "facebook/react",
			candidateRepo: "facebook/jest",
			expectedScore: 1.0,
		},
		{
			name:          "different org",
			targetRepo:    "facebook/react",
			candidateRepo: "google/angular",
			expectedScore: 0.0,
		},
		{
			name:          "user repos",
			targetRepo:    "john/project1",
			candidateRepo: "john/project2",
			expectedScore: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := storage.StoredRepo{FullName: tt.targetRepo}
			candidate := storage.StoredRepo{FullName: tt.candidateRepo}

			score := engine.calculateSameOrgScore(target, candidate)

			if score != tt.expectedScore {
				t.Errorf("Expected score %f, got %f", tt.expectedScore, score)
			}
		})
	}
}

func TestEngineImpl_CalculateTopicOverlapScore(t *testing.T) {
	engine := &EngineImpl{}

	tests := []struct {
		name            string
		targetTopics    []string
		candidateTopics []string
		expectedMin     float64 // Minimum expected score
		expectedMax     float64 // Maximum expected score
	}{
		{
			name:            "identical topics",
			targetTopics:    []string{"javascript", "react", "frontend"},
			candidateTopics: []string{"javascript", "react", "frontend"},
			expectedMin:     1.0,
			expectedMax:     1.0,
		},
		{
			name:            "partial overlap",
			targetTopics:    []string{"javascript", "react", "frontend"},
			candidateTopics: []string{"javascript", "vue", "frontend"},
			expectedMin:     0.4, // 2/5 = 0.4 (Jaccard)
			expectedMax:     0.6,
		},
		{
			name:            "no overlap",
			targetTopics:    []string{"javascript", "react"},
			candidateTopics: []string{"python", "django"},
			expectedMin:     0.0,
			expectedMax:     0.0,
		},
		{
			name:            "empty topics",
			targetTopics:    []string{},
			candidateTopics: []string{"javascript"},
			expectedMin:     0.0,
			expectedMax:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := storage.StoredRepo{Topics: tt.targetTopics}
			candidate := storage.StoredRepo{Topics: tt.candidateTopics}

			score := engine.calculateTopicOverlapScore(target, candidate)

			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf(
					"Expected score between %f and %f, got %f",
					tt.expectedMin,
					tt.expectedMax,
					score,
				)
			}
		})
	}
}

func TestEngineImpl_CalculateSharedContribScore(t *testing.T) {
	engine := &EngineImpl{}

	tests := []struct {
		name              string
		targetContribs    []storage.Contributor
		candidateContribs []storage.Contributor
		expectedMin       float64
		expectedMax       float64
	}{
		{
			name: "shared contributors",
			targetContribs: []storage.Contributor{
				{Login: "alice", Contributions: 100},
				{Login: "bob", Contributions: 50},
			},
			candidateContribs: []storage.Contributor{
				{Login: "alice", Contributions: 80},
				{Login: "charlie", Contributions: 30},
			},
			expectedMin: 0.4, // 1/2 = 0.5 (normalized by smaller set)
			expectedMax: 0.6,
		},
		{
			name: "no shared contributors",
			targetContribs: []storage.Contributor{
				{Login: "alice", Contributions: 100},
			},
			candidateContribs: []storage.Contributor{
				{Login: "bob", Contributions: 80},
			},
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
		{
			name:              "empty contributors",
			targetContribs:    []storage.Contributor{},
			candidateContribs: []storage.Contributor{{Login: "alice", Contributions: 100}},
			expectedMin:       0.0,
			expectedMax:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := storage.StoredRepo{Contributors: tt.targetContribs}
			candidate := storage.StoredRepo{Contributors: tt.candidateContribs}

			score := engine.calculateSharedContribScore(target, candidate)

			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf(
					"Expected score between %f and %f, got %f",
					tt.expectedMin,
					tt.expectedMax,
					score,
				)
			}
		})
	}
}

func TestEngineImpl_GenerateExplanation(t *testing.T) {
	engine := &EngineImpl{}

	tests := []struct {
		name           string
		components     ScoreComponents
		target         storage.StoredRepo
		candidate      storage.StoredRepo
		expectContains []string // Strings that should be in the explanation
	}{
		{
			name: "same org explanation",
			components: ScoreComponents{
				SameOrg: 1.0,
			},
			target:         storage.StoredRepo{FullName: "facebook/react"},
			candidate:      storage.StoredRepo{FullName: "facebook/jest"},
			expectContains: []string{"shared org", "facebook"},
		},
		{
			name: "topic overlap explanation",
			components: ScoreComponents{
				TopicOverlap: 0.5,
			},
			target: storage.StoredRepo{
				FullName: "facebook/react",
				Topics:   []string{"javascript", "react"},
			},
			candidate: storage.StoredRepo{
				FullName: "vuejs/vue",
				Topics:   []string{"javascript", "vue"},
			},
			expectContains: []string{"shared topic", "javascript"},
		},
		{
			name: "multiple components",
			components: ScoreComponents{
				SameOrg:      1.0,
				TopicOverlap: 0.5,
			},
			target: storage.StoredRepo{
				FullName: "facebook/react",
				Topics:   []string{"javascript", "react"},
			},
			candidate: storage.StoredRepo{
				FullName: "facebook/jest",
				Topics:   []string{"javascript", "testing"},
			},
			expectContains: []string{"shared org", "facebook", "and", "shared topic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			explanation := engine.generateExplanation(tt.components, tt.target, tt.candidate)

			for _, expected := range tt.expectContains {
				if !contains(explanation, expected) {
					t.Errorf("Expected explanation to contain '%s', got: %s", expected, explanation)
				}
			}
		})
	}
}

func TestExtractOrg(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		expected string
	}{
		{
			name:     "normal repo",
			fullName: "facebook/react",
			expected: "facebook",
		},
		{
			name:     "nested path",
			fullName: "org/subproject/repo",
			expected: "org",
		},
		{
			name:     "no slash",
			fullName: "repo",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractOrg(tt.fullName)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetSharedTopics(t *testing.T) {
	tests := []struct {
		name     string
		topics1  []string
		topics2  []string
		expected []string
	}{
		{
			name:     "shared topics",
			topics1:  []string{"javascript", "react", "frontend"},
			topics2:  []string{"javascript", "vue", "frontend"},
			expected: []string{"javascript", "frontend"},
		},
		{
			name:     "no shared topics",
			topics1:  []string{"javascript", "react"},
			topics2:  []string{"python", "django"},
			expected: []string{},
		},
		{
			name:     "case insensitive",
			topics1:  []string{"JavaScript", "React"},
			topics2:  []string{"javascript", "Vue"},
			expected: []string{"javascript"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSharedTopics(tt.topics1, tt.topics2)

			if len(result) != len(tt.expected) {
				t.Errorf(
					"Expected %d shared topics, got %d: %v",
					len(tt.expected),
					len(result),
					result,
				)

				return
			}

			// Convert to map for easier comparison
			resultMap := make(map[string]bool)
			for _, topic := range result {
				resultMap[topic] = true
			}

			for _, expected := range tt.expected {
				if !resultMap[expected] {
					t.Errorf("Expected shared topic %s not found in result: %v", expected, result)
				}
			}
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name      string
		a         []float32
		b         []float32
		expected  float64
		tolerance float64
	}{
		{
			name:      "identical vectors",
			a:         []float32{1.0, 0.0, 0.0},
			b:         []float32{1.0, 0.0, 0.0},
			expected:  1.0,
			tolerance: 0.001,
		},
		{
			name:      "orthogonal vectors",
			a:         []float32{1.0, 0.0},
			b:         []float32{0.0, 1.0},
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "opposite vectors",
			a:         []float32{1.0, 0.0},
			b:         []float32{-1.0, 0.0},
			expected:  -1.0,
			tolerance: 0.001,
		},
		{
			name:      "different lengths",
			a:         []float32{1.0, 0.0},
			b:         []float32{1.0},
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "zero vectors",
			a:         []float32{0.0, 0.0},
			b:         []float32{0.0, 0.0},
			expected:  0.0,
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CosineSimilarity(tt.a, tt.b)

			if abs(result-tt.expected) > tt.tolerance {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestEngineImpl_CalculateVectorSimilarityScore(t *testing.T) {
	engine := &EngineImpl{}

	tests := []struct {
		name        string
		target      storage.StoredRepo
		candidate   storage.StoredRepo
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "identical vectors",
			target:      storage.StoredRepo{RepoEmbedding: []float32{1.0, 0.0, 0.0}},
			candidate:   storage.StoredRepo{RepoEmbedding: []float32{1.0, 0.0, 0.0}},
			expectedMin: 0.99,
			expectedMax: 1.01,
		},
		{
			name:        "orthogonal vectors",
			target:      storage.StoredRepo{RepoEmbedding: []float32{1.0, 0.0}},
			candidate:   storage.StoredRepo{RepoEmbedding: []float32{0.0, 1.0}},
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
		{
			name:        "nil target embedding",
			target:      storage.StoredRepo{RepoEmbedding: nil},
			candidate:   storage.StoredRepo{RepoEmbedding: []float32{1.0, 0.0}},
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
		{
			name:        "nil candidate embedding",
			target:      storage.StoredRepo{RepoEmbedding: []float32{1.0, 0.0}},
			candidate:   storage.StoredRepo{RepoEmbedding: nil},
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
		{
			name:        "opposite vectors clamped to zero",
			target:      storage.StoredRepo{RepoEmbedding: []float32{1.0, 0.0}},
			candidate:   storage.StoredRepo{RepoEmbedding: []float32{-1.0, 0.0}},
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := engine.calculateVectorSimilarityScore(tt.target, tt.candidate)

			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("Expected score between %f and %f, got %f",
					tt.expectedMin, tt.expectedMax, score)
			}
		})
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}

	return x
}
