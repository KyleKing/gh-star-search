package query

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/username/gh-star-search/internal/llm"
)

// TestNaturalLanguageQueryExamples demonstrates various natural language queries
// and their expected SQL translations
func TestNaturalLanguageQueryExamples(t *testing.T) {
	examples := []struct {
		name            string
		naturalQuery    string
		expectedSQL     string
		expectedType    QueryType
		confidence      float64
		explanation     string
	}{
		{
			name:         "simple language filter",
			naturalQuery: "show me all Go repositories",
			expectedSQL:  "SELECT * FROM repositories WHERE language = 'Go' ORDER BY stargazers_count DESC",
			expectedType: QueryTypeFilter,
			confidence:   0.95,
			explanation:  "Filter repositories by Go programming language, ordered by popularity",
		},
		{
			name:         "popularity threshold",
			naturalQuery: "find popular JavaScript projects with more than 1000 stars",
			expectedSQL:  "SELECT * FROM repositories WHERE language = 'JavaScript' AND stargazers_count > 1000 ORDER BY stargazers_count DESC",
			expectedType: QueryTypeComparison,
			confidence:   0.9,
			explanation:  "Find JavaScript repositories with high star count",
		},
		{
			name:         "recent activity",
			naturalQuery: "repositories updated in the last month",
			expectedSQL:  "SELECT * FROM repositories WHERE updated_at >= DATE_SUB(CURRENT_DATE, INTERVAL 1 MONTH) ORDER BY updated_at DESC",
			expectedType: QueryTypeComparison,
			confidence:   0.85,
			explanation:  "Find recently updated repositories",
		},
		{
			name:         "language statistics",
			naturalQuery: "count repositories by programming language",
			expectedSQL:  "SELECT language, COUNT(*) as count FROM repositories WHERE language IS NOT NULL GROUP BY language ORDER BY count DESC",
			expectedType: QueryTypeAggregate,
			confidence:   0.95,
			explanation:  "Aggregate repository count by programming language",
		},
		{
			name:         "content search",
			naturalQuery: "find repositories about machine learning",
			expectedSQL:  "SELECT DISTINCT r.* FROM repositories r LEFT JOIN content_chunks c ON r.id = c.repository_id WHERE (r.description ILIKE '%machine learning%' OR r.purpose ILIKE '%machine learning%' OR c.content ILIKE '%machine learning%') ORDER BY r.stargazers_count DESC",
			expectedType: QueryTypeSearch,
			confidence:   0.8,
			explanation:  "Search for machine learning related repositories in descriptions and content",
		},
		{
			name:         "license filter",
			naturalQuery: "show MIT licensed repositories",
			expectedSQL:  "SELECT * FROM repositories WHERE license_spdx_id = 'MIT' OR license_name ILIKE '%MIT%' ORDER BY stargazers_count DESC",
			expectedType: QueryTypeSearch, // Changed because it contains ILIKE
			confidence:   0.9,
			explanation:  "Filter repositories by MIT license",
		},
		{
			name:         "size comparison",
			naturalQuery: "large repositories over 10MB",
			expectedSQL:  "SELECT * FROM repositories WHERE size_kb > 10240 ORDER BY size_kb DESC",
			expectedType: QueryTypeComparison,
			confidence:   0.85,
			explanation:  "Find repositories larger than 10MB (10240 KB)",
		},
		{
			name:         "topic search",
			naturalQuery: "web framework repositories",
			expectedSQL:  "SELECT * FROM repositories WHERE topics LIKE '%web%' OR topics LIKE '%framework%' OR description ILIKE '%web framework%' ORDER BY stargazers_count DESC",
			expectedType: QueryTypeSearch,
			confidence:   0.8,
			explanation:  "Search for web framework repositories using topics and descriptions",
		},
		{
			name:         "fork analysis",
			naturalQuery: "most forked Python repositories",
			expectedSQL:  "SELECT * FROM repositories WHERE language = 'Python' ORDER BY forks_count DESC LIMIT 20",
			expectedType: QueryTypeFilter,
			confidence:   0.9,
			explanation:  "Find Python repositories with the most forks",
		},
		{
			name:         "installation instructions",
			naturalQuery: "repositories with Docker installation",
			expectedSQL:  "SELECT * FROM repositories WHERE installation_instructions ILIKE '%docker%' ORDER BY stargazers_count DESC",
			expectedType: QueryTypeSearch,
			confidence:   0.85,
			explanation:  "Find repositories that mention Docker in installation instructions",
		},
		{
			name:         "complex aggregation",
			naturalQuery: "average stars per language for repositories with more than 100 stars",
			expectedSQL:  "SELECT language, AVG(stargazers_count) as avg_stars, COUNT(*) as repo_count FROM repositories WHERE stargazers_count > 100 AND language IS NOT NULL GROUP BY language HAVING COUNT(*) >= 5 ORDER BY avg_stars DESC",
			expectedType: QueryTypeAggregate,
			confidence:   0.8,
			explanation:  "Calculate average stars per language for popular repositories",
		},
		{
			name:         "date range query",
			naturalQuery: "repositories created between 2020 and 2022",
			expectedSQL:  "SELECT * FROM repositories WHERE created_at >= '2020-01-01' AND created_at < '2023-01-01' ORDER BY created_at DESC",
			expectedType: QueryTypeComparison,
			confidence:   0.9,
			explanation:  "Find repositories created in a specific date range",
		},
	}

	for _, example := range examples {
		t.Run(example.name, func(t *testing.T) {
			mockLLM := &MockLLMService{}
			parser := NewLLMParser(mockLLM, nil)

			// Mock the LLM response
			mockResponse := &llm.QueryResponse{
				SQL:         example.expectedSQL,
				Parameters:  map[string]string{},
				Explanation: example.explanation,
				Confidence:  example.confidence,
			}

			mockLLM.On("ParseQuery", mock.Anything, example.naturalQuery, mock.Anything).Return(mockResponse, nil)

			result, err := parser.Parse(context.Background(), example.naturalQuery)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, example.expectedSQL, result.SQL)
			assert.Equal(t, example.expectedType, result.QueryType)
			assert.Equal(t, example.confidence, result.Confidence)
			assert.Equal(t, example.explanation, result.Explanation)

			mockLLM.AssertExpectations(t)
		})
	}
}

// TestComplexNaturalLanguageQueries tests more complex query scenarios
func TestComplexNaturalLanguageQueries(t *testing.T) {
	complexExamples := []struct {
		name         string
		naturalQuery string
		expectedSQL  string
		explanation  string
	}{
		{
			name:         "multi-condition search",
			naturalQuery: "find Go web frameworks with good documentation updated this year",
			expectedSQL: `SELECT DISTINCT r.* FROM repositories r 
				LEFT JOIN content_chunks c ON r.id = c.repository_id 
				WHERE r.language = 'Go' 
				AND (r.topics LIKE '%web%' OR r.topics LIKE '%framework%' OR r.description ILIKE '%web framework%')
				AND (r.description ILIKE '%documentation%' OR c.content ILIKE '%documentation%')
				AND r.updated_at >= '2024-01-01'
				ORDER BY r.stargazers_count DESC`,
			explanation: "Complex search combining language, topic, content, and date filters",
		},
		{
			name:         "comparative analysis",
			naturalQuery: "compare star counts between React and Vue repositories",
			expectedSQL: `SELECT 
				CASE 
					WHEN (topics LIKE '%react%' OR description ILIKE '%react%') THEN 'React'
					WHEN (topics LIKE '%vue%' OR description ILIKE '%vue%') THEN 'Vue'
				END as framework,
				COUNT(*) as repo_count,
				AVG(stargazers_count) as avg_stars,
				MAX(stargazers_count) as max_stars
				FROM repositories 
				WHERE (topics LIKE '%react%' OR description ILIKE '%react%' OR topics LIKE '%vue%' OR description ILIKE '%vue%')
				GROUP BY framework
				ORDER BY avg_stars DESC`,
			explanation: "Comparative analysis between React and Vue repositories",
		},
		{
			name:         "trend analysis",
			naturalQuery: "show repository creation trends by year for machine learning projects",
			expectedSQL: `SELECT 
				EXTRACT(YEAR FROM created_at) as year,
				COUNT(*) as repos_created
				FROM repositories 
				WHERE (description ILIKE '%machine learning%' OR topics LIKE '%machine-learning%' OR purpose ILIKE '%machine learning%')
				GROUP BY EXTRACT(YEAR FROM created_at)
				ORDER BY year DESC`,
			explanation: "Analyze creation trends for machine learning repositories by year",
		},
	}

	for _, example := range complexExamples {
		t.Run(example.name, func(t *testing.T) {
			mockLLM := &MockLLMService{}
			parser := NewLLMParser(mockLLM, nil)

			// Mock the LLM response
			mockResponse := &llm.QueryResponse{
				SQL:         example.expectedSQL,
				Parameters:  map[string]string{},
				Explanation: example.explanation,
				Confidence:  0.75, // Lower confidence for complex queries
			}

			mockLLM.On("ParseQuery", mock.Anything, example.naturalQuery, mock.Anything).Return(mockResponse, nil)

			result, err := parser.Parse(context.Background(), example.naturalQuery)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Contains(t, result.SQL, "SELECT") // Basic validation
			assert.True(t, result.Confidence > 0.0)
			assert.NotEmpty(t, result.Explanation)

			mockLLM.AssertExpectations(t)
		})
	}
}

// TestAmbiguousQueries tests how the parser handles ambiguous or unclear queries
func TestAmbiguousQueries(t *testing.T) {
	ambiguousExamples := []struct {
		name         string
		naturalQuery string
		expectedSQL  string
		confidence   float64
	}{
		{
			name:         "vague search term",
			naturalQuery: "find good repositories",
			expectedSQL:  "SELECT * FROM repositories ORDER BY stargazers_count DESC LIMIT 50",
			confidence:   0.3, // Low confidence due to ambiguity
		},
		{
			name:         "unclear time reference",
			naturalQuery: "recent repositories",
			expectedSQL:  "SELECT * FROM repositories ORDER BY updated_at DESC LIMIT 20",
			confidence:   0.4,
		},
		{
			name:         "ambiguous language reference",
			naturalQuery: "C repositories", // Could be C or C++
			expectedSQL:  "SELECT * FROM repositories WHERE language IN ('C', 'C++') ORDER BY stargazers_count DESC",
			confidence:   0.6,
		},
	}

	for _, example := range ambiguousExamples {
		t.Run(example.name, func(t *testing.T) {
			mockLLM := &MockLLMService{}
			parser := NewLLMParser(mockLLM, nil)

			mockResponse := &llm.QueryResponse{
				SQL:         example.expectedSQL,
				Parameters:  map[string]string{},
				Explanation: "Interpreted ambiguous query with best guess",
				Confidence:  example.confidence,
			}

			mockLLM.On("ParseQuery", mock.Anything, example.naturalQuery, mock.Anything).Return(mockResponse, nil)

			result, err := parser.Parse(context.Background(), example.naturalQuery)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, example.confidence, result.Confidence)
			assert.True(t, result.Confidence < 0.7) // Should have low confidence

			mockLLM.AssertExpectations(t)
		})
	}
}

// TestErrorHandling tests various error scenarios
func TestQueryParsingErrorHandling(t *testing.T) {
	errorExamples := []struct {
		name          string
		naturalQuery  string
		llmResponse   *llm.QueryResponse
		expectedError string
	}{
		{
			name:         "malicious query attempt",
			naturalQuery: "delete all my repositories",
			llmResponse: &llm.QueryResponse{
				SQL:         "DELETE FROM repositories",
				Confidence:  0.9,
			},
			expectedError: "only SELECT statements are allowed",
		},
		{
			name:         "injection attempt",
			naturalQuery: "show repositories; drop table repositories;",
			llmResponse: &llm.QueryResponse{
				SQL:         "SELECT * FROM repositories; DROP TABLE repositories;",
				Confidence:  0.1,
			},
			expectedError: "SQL contains suspicious pattern",
		},
		{
			name:         "invalid table reference",
			naturalQuery: "show all users",
			llmResponse: &llm.QueryResponse{
				SQL:         "SELECT * FROM users",
				Confidence:  0.8,
			},
			expectedError: "SQL must reference valid tables",
		},
	}

	for _, example := range errorExamples {
		t.Run(example.name, func(t *testing.T) {
			mockLLM := &MockLLMService{}
			parser := NewLLMParser(mockLLM, nil)

			mockLLM.On("ParseQuery", mock.Anything, example.naturalQuery, mock.Anything).Return(example.llmResponse, nil)

			result, err := parser.Parse(context.Background(), example.naturalQuery)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), example.expectedError)
			assert.Nil(t, result)

			mockLLM.AssertExpectations(t)
		})
	}
}