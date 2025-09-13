package query

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/username/gh-star-search/internal/llm"
	"github.com/username/gh-star-search/internal/types"
)

// MockLLMService is a mock implementation of the LLM service
type MockLLMService struct {
	mock.Mock
}

func (m *MockLLMService) Summarize(ctx context.Context, prompt string, content string) (*llm.SummaryResponse, error) {
	args := m.Called(ctx, prompt, content)
	return args.Get(0).(*llm.SummaryResponse), args.Error(1)
}

func (m *MockLLMService) ParseQuery(ctx context.Context, query string, schema types.Schema) (*llm.QueryResponse, error) {
	args := m.Called(ctx, query, schema)
	return args.Get(0).(*llm.QueryResponse), args.Error(1)
}

func (m *MockLLMService) Configure(config llm.Config) error {
	args := m.Called(config)
	return args.Error(0)
}

func TestNewLLMParser(t *testing.T) {
	mockLLM := &MockLLMService{}
	parser := NewLLMParser(mockLLM, nil)
	
	assert.NotNil(t, parser)
	assert.Equal(t, mockLLM, parser.llmService)
	assert.NotNil(t, parser.schema)
	assert.Contains(t, parser.schema.Tables, "repositories")
	assert.Contains(t, parser.schema.Tables, "content_chunks")
}

func TestLLMParser_Parse(t *testing.T) {
	tests := []struct {
		name           string
		naturalQuery   string
		llmResponse    *llm.QueryResponse
		llmError       error
		expectedSQL    string
		expectedType   QueryType
		expectedError  string
	}{
		{
			name:         "successful search query",
			naturalQuery: "find javascript repositories",
			llmResponse: &llm.QueryResponse{
				SQL:         "SELECT * FROM repositories WHERE language = 'JavaScript'",
				Parameters:  map[string]string{},
				Explanation: "Search for repositories with JavaScript as primary language",
				Confidence:  0.9,
			},
			expectedSQL:  "SELECT * FROM repositories WHERE language = 'JavaScript'",
			expectedType: QueryTypeFilter,
		},
		{
			name:         "aggregation query",
			naturalQuery: "count repositories by language",
			llmResponse: &llm.QueryResponse{
				SQL:         "SELECT language, COUNT(*) FROM repositories GROUP BY language",
				Parameters:  map[string]string{},
				Explanation: "Count repositories grouped by programming language",
				Confidence:  0.95,
			},
			expectedSQL:  "SELECT language, COUNT(*) FROM repositories GROUP BY language",
			expectedType: QueryTypeAggregate,
		},
		{
			name:         "comparison query",
			naturalQuery: "repositories with more than 1000 stars",
			llmResponse: &llm.QueryResponse{
				SQL:         "SELECT * FROM repositories WHERE stargazers_count > 1000",
				Parameters:  map[string]string{},
				Explanation: "Find repositories with more than 1000 stars",
				Confidence:  0.85,
			},
			expectedSQL:  "SELECT * FROM repositories WHERE stargazers_count > 1000",
			expectedType: QueryTypeComparison,
		},
		{
			name:         "dangerous SQL injection attempt",
			naturalQuery: "show all repositories",
			llmResponse: &llm.QueryResponse{
				SQL:         "SELECT * FROM repositories; DROP TABLE repositories; --",
				Parameters:  map[string]string{},
				Explanation: "Malicious query attempt",
				Confidence:  0.1,
			},
			expectedError: "SQL contains suspicious pattern",
		},
		{
			name:         "non-SELECT statement",
			naturalQuery: "delete all repositories",
			llmResponse: &llm.QueryResponse{
				SQL:         "DELETE FROM repositories",
				Parameters:  map[string]string{},
				Explanation: "Delete operation",
				Confidence:  0.8,
			},
			expectedError: "only SELECT statements are allowed",
		},
		{
			name:         "invalid table reference",
			naturalQuery: "find users",
			llmResponse: &llm.QueryResponse{
				SQL:         "SELECT * FROM users",
				Parameters:  map[string]string{},
				Explanation: "Query invalid table",
				Confidence:  0.7,
			},
			expectedError: "SQL must reference valid tables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM := &MockLLMService{}
			parser := NewLLMParser(mockLLM, nil)

			if tt.llmError != nil {
				mockLLM.On("ParseQuery", mock.Anything, tt.naturalQuery, mock.Anything).Return((*llm.QueryResponse)(nil), tt.llmError)
			} else {
				mockLLM.On("ParseQuery", mock.Anything, tt.naturalQuery, mock.Anything).Return(tt.llmResponse, nil)
			}

			result, err := parser.Parse(context.Background(), tt.naturalQuery)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedSQL, result.SQL)
				assert.Equal(t, tt.expectedType, result.QueryType)
			}

			mockLLM.AssertExpectations(t)
		})
	}
}

func TestLLMParser_ValidateSQL(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedError string
	}{
		{
			name: "valid SELECT query",
			sql:  "SELECT * FROM repositories WHERE language = 'Go'",
		},
		{
			name:          "empty SQL",
			sql:           "",
			expectedError: "SQL query cannot be empty",
		},
		{
			name:          "DROP TABLE attempt",
			sql:           "DROP TABLE repositories",
			expectedError: "only SELECT statements are allowed",
		},
		{
			name:          "DELETE attempt",
			sql:           "DELETE FROM repositories WHERE id = 1",
			expectedError: "only SELECT statements are allowed",
		},
		{
			name:          "INSERT attempt",
			sql:           "INSERT INTO repositories VALUES (1, 'test')",
			expectedError: "only SELECT statements are allowed",
		},
		{
			name:          "UPDATE attempt",
			sql:           "UPDATE repositories SET name = 'test'",
			expectedError: "only SELECT statements are allowed",
		},
		{
			name:          "non-SELECT statement",
			sql:           "CREATE TABLE test (id INT)",
			expectedError: "only SELECT statements are allowed",
		},
		{
			name:          "SQL injection with comments",
			sql:           "SELECT * FROM repositories -- DROP TABLE repositories",
			expectedError: "SQL contains suspicious pattern: --",
		},
		{
			name:          "SQL injection with UNION",
			sql:           "SELECT * FROM repositories UNION SELECT * FROM users",
			expectedError: "SQL contains suspicious pattern: union",
		},
		{
			name:          "SQL injection with OR 1=1",
			sql:           "SELECT * FROM repositories WHERE name = 'test' OR 1=1",
			expectedError: "SQL contains suspicious pattern: or 1=1",
		},
		{
			name:          "invalid table reference",
			sql:           "SELECT * FROM invalid_table",
			expectedError: "SQL must reference valid tables",
		},
		{
			name: "valid query with content_chunks",
			sql:  "SELECT r.* FROM repositories r JOIN content_chunks c ON r.id = c.repository_id",
		},
		{
			name: "valid aggregation query",
			sql:  "SELECT language, COUNT(*) FROM repositories GROUP BY language ORDER BY COUNT(*) DESC",
		},
		{
			name: "valid search with LIKE",
			sql:  "SELECT * FROM repositories WHERE description LIKE '%web framework%'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLLMParser(&MockLLMService{}, nil)
			err := parser.ValidateSQL(tt.sql)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLLMParser_DetermineQueryType(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		expectedType QueryType
	}{
		{
			name:         "search query",
			sql:          "SELECT * FROM repositories",
			expectedType: QueryTypeSearch,
		},
		{
			name:         "filter query with WHERE",
			sql:          "SELECT * FROM repositories WHERE language = 'Go'",
			expectedType: QueryTypeFilter,
		},
		{
			name:         "filter query with LIMIT",
			sql:          "SELECT * FROM repositories LIMIT 10",
			expectedType: QueryTypeFilter,
		},
		{
			name:         "aggregation query with COUNT",
			sql:          "SELECT COUNT(*) FROM repositories",
			expectedType: QueryTypeAggregate,
		},
		{
			name:         "aggregation query with GROUP BY",
			sql:          "SELECT language, COUNT(*) FROM repositories GROUP BY language",
			expectedType: QueryTypeAggregate,
		},
		{
			name:         "aggregation query with SUM",
			sql:          "SELECT SUM(stargazers_count) FROM repositories",
			expectedType: QueryTypeAggregate,
		},
		{
			name:         "comparison query with greater than",
			sql:          "SELECT * FROM repositories WHERE stargazers_count > 1000",
			expectedType: QueryTypeComparison,
		},
		{
			name:         "comparison query with BETWEEN",
			sql:          "SELECT * FROM repositories WHERE stargazers_count BETWEEN 100 AND 1000",
			expectedType: QueryTypeComparison,
		},
		{
			name:         "comparison query with IN",
			sql:          "SELECT * FROM repositories WHERE language IN ('Go', 'Python')",
			expectedType: QueryTypeComparison,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLLMParser(&MockLLMService{}, nil)
			queryType := parser.determineQueryType(tt.sql)
			assert.Equal(t, tt.expectedType, queryType)
		})
	}
}

func TestLLMParser_ParseOperation(t *testing.T) {
	tests := []struct {
		name              string
		planLine          string
		expectedOperation *QueryOperation
	}{
		{
			name:     "table scan operation",
			planLine: "SEQ_SCAN repositories (1000 rows)",
			expectedOperation: &QueryOperation{
				Type:          "SEQ_SCAN",
				Table:         "repositories",
				Condition:     "SEQ_SCAN repositories (1000 rows)",
				EstimatedRows: 1000,
				Cost:          100.0, // 1000 * 0.1
			},
		},
		{
			name:     "index scan operation",
			planLine: "INDEX_SCAN content_chunks (50 rows)",
			expectedOperation: &QueryOperation{
				Type:          "INDEX_SCAN",
				Table:         "content_chunks",
				Condition:     "INDEX_SCAN content_chunks (50 rows)",
				EstimatedRows: 50,
				Cost:          0.5, // 50 * 0.01
			},
		},
		{
			name:              "empty plan line",
			planLine:          "",
			expectedOperation: nil,
		},
		{
			name:     "filter operation",
			planLine: "FILTER language='Go' (100 rows)",
			expectedOperation: &QueryOperation{
				Type:          "FILTER",
				Table:         "",
				Condition:     "FILTER language='Go' (100 rows)",
				EstimatedRows: 100,
				Cost:          5.0, // 100 * 0.05
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLLMParser(&MockLLMService{}, nil)
			operation := parser.parseOperation(tt.planLine)

			if tt.expectedOperation == nil {
				assert.Nil(t, operation)
			} else {
				assert.NotNil(t, operation)
				assert.Equal(t, tt.expectedOperation.Type, operation.Type)
				assert.Equal(t, tt.expectedOperation.Table, operation.Table)
				assert.Equal(t, tt.expectedOperation.EstimatedRows, operation.EstimatedRows)
				assert.Equal(t, tt.expectedOperation.Cost, operation.Cost)
			}
		})
	}
}

func TestLLMParser_GenerateOptimizationTips(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		operations   []QueryOperation
		expectedTips []string
	}{
		{
			name: "query with sequential scan",
			sql:  "SELECT * FROM repositories WHERE language = 'Go'",
			operations: []QueryOperation{
				{Type: "SEQ_SCAN", Table: "repositories"},
			},
			expectedTips: []string{
				"Consider adding indexes on frequently filtered columns",
			},
		},
		{
			name: "query with LIKE pattern starting with %",
			sql:  "SELECT * FROM repositories WHERE name LIKE '%test%'",
			operations: []QueryOperation{
				{Type: "INDEX_SCAN", Table: "repositories"},
			},
			expectedTips: []string{
				"LIKE patterns starting with % cannot use indexes efficiently",
			},
		},
		{
			name: "query with OR condition",
			sql:  "SELECT * FROM repositories WHERE language = 'Go' OR language = 'Python'",
			operations: []QueryOperation{
				{Type: "INDEX_SCAN", Table: "repositories"},
			},
			expectedTips: []string{
				"OR conditions may prevent index usage - consider UNION instead",
			},
		},
		{
			name: "query without LIMIT",
			sql:  "SELECT * FROM repositories WHERE language = 'Go'",
			operations: []QueryOperation{
				{Type: "INDEX_SCAN", Table: "repositories"},
			},
			expectedTips: []string{
				"Consider adding LIMIT clause to prevent large result sets",
			},
		},
		{
			name: "query without WHERE clause",
			sql:  "SELECT * FROM repositories",
			operations: []QueryOperation{
				{Type: "SEQ_SCAN", Table: "repositories"},
			},
			expectedTips: []string{
				"Consider adding indexes on frequently filtered columns",
				"Consider adding LIMIT clause to prevent large result sets",
				"Consider adding WHERE clause to filter results",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLLMParser(&MockLLMService{}, nil)
			tips := parser.generateOptimizationTips(tt.sql, tt.operations)

			for _, expectedTip := range tt.expectedTips {
				assert.Contains(t, tips, expectedTip)
			}
		})
	}
}

func TestLLMParser_FindIndexesUsed(t *testing.T) {
	tests := []struct {
		name            string
		operations      []QueryOperation
		expectedIndexes []string
	}{
		{
			name: "operations with index usage",
			operations: []QueryOperation{
				{Type: "INDEX_SCAN", Condition: "using idx_repositories_language"},
				{Type: "INDEX_SCAN", Condition: "using idx_repositories_stargazers"},
			},
			expectedIndexes: []string{"idx_repositories_language", "idx_repositories_stargazers"},
		},
		{
			name: "operations without index usage",
			operations: []QueryOperation{
				{Type: "SEQ_SCAN", Condition: "full table scan"},
				{Type: "FILTER", Condition: "language = 'Go'"},
			},
			expectedIndexes: []string{},
		},
		{
			name:            "empty operations",
			operations:      []QueryOperation{},
			expectedIndexes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLLMParser(&MockLLMService{}, nil)
			indexes := parser.findIndexesUsed(tt.operations)

			assert.Equal(t, len(tt.expectedIndexes), len(indexes))
			for _, expectedIndex := range tt.expectedIndexes {
				assert.Contains(t, indexes, expectedIndex)
			}
		})
	}
}

func TestGetDefaultSchema(t *testing.T) {
	schema := getDefaultSchema()

	// Test repositories table
	assert.Contains(t, schema.Tables, "repositories")
	repoTable := schema.Tables["repositories"]
	assert.Equal(t, "repositories", repoTable.Name)
	assert.True(t, len(repoTable.Columns) > 0)
	assert.True(t, len(repoTable.Indexes) > 0)

	// Test content_chunks table
	assert.Contains(t, schema.Tables, "content_chunks")
	chunksTable := schema.Tables["content_chunks"]
	assert.Equal(t, "content_chunks", chunksTable.Name)
	assert.True(t, len(chunksTable.Columns) > 0)
	assert.True(t, len(chunksTable.Indexes) > 0)

	// Test specific columns exist
	repoColumns := make(map[string]bool)
	for _, col := range repoTable.Columns {
		repoColumns[col.Name] = true
	}
	
	expectedColumns := []string{
		"id", "full_name", "description", "language", "stargazers_count",
		"forks_count", "purpose", "technologies", "use_cases", "features",
	}
	
	for _, col := range expectedColumns {
		assert.True(t, repoColumns[col], "Column %s should exist in repositories table", col)
	}
}

// Integration test with real database (requires DuckDB)
func TestLLMParser_ExplainQuery_Integration(t *testing.T) {
	// Skip if no database available
	t.Skip("Integration test - requires DuckDB setup")

	// This would be an integration test that requires a real database
	// It's skipped by default but can be enabled for full testing
	
	/*
	db, err := sql.Open("duckdb", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create test schema
	_, err = db.Exec(`
		CREATE TABLE repositories (
			id VARCHAR PRIMARY KEY,
			full_name VARCHAR,
			language VARCHAR,
			stargazers_count INTEGER
		)
	`)
	require.NoError(t, err)

	parser := NewLLMParser(&MockLLMService{}, db)
	
	plan, err := parser.ExplainQuery("SELECT * FROM repositories WHERE language = 'Go'")
	assert.NoError(t, err)
	assert.NotNil(t, plan)
	*/
}

// Benchmark tests
func BenchmarkLLMParser_ValidateSQL(b *testing.B) {
	parser := NewLLMParser(&MockLLMService{}, nil)
	sql := "SELECT * FROM repositories WHERE language = 'Go' AND stargazers_count > 100 ORDER BY stargazers_count DESC LIMIT 10"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.ValidateSQL(sql)
	}
}

func BenchmarkLLMParser_DetermineQueryType(b *testing.B) {
	parser := NewLLMParser(&MockLLMService{}, nil)
	sql := "SELECT language, COUNT(*) FROM repositories WHERE stargazers_count > 100 GROUP BY language ORDER BY COUNT(*) DESC"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.determineQueryType(sql)
	}
}