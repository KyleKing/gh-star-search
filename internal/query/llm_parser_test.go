package query

import (
	"context"
	"testing"

	"github.com/kyleking/gh-star-search/internal/llm"
	"github.com/kyleking/gh-star-search/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
		name          string
		naturalQuery  string
		llmResponse   *llm.QueryResponse
		llmError      error
		expectedSQL   string
		expectedType  Type
		expectedError string
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
			expectedType: TypeFilter,
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
			expectedType: TypeAggregate,
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
			expectedType: TypeComparison,
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
				assert.Equal(t, tt.expectedType, result.Type)
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
		expectedType Type
	}{
		{
			name:         "search query",
			sql:          "SELECT * FROM repositories",
			expectedType: TypeSearch,
		},
		{
			name:         "filter query with WHERE",
			sql:          "SELECT * FROM repositories WHERE language = 'Go'",
			expectedType: TypeFilter,
		},
		{
			name:         "filter query with LIMIT",
			sql:          "SELECT * FROM repositories LIMIT 10",
			expectedType: TypeFilter,
		},
		{
			name:         "aggregation query with COUNT",
			sql:          "SELECT COUNT(*) FROM repositories",
			expectedType: TypeAggregate,
		},
		{
			name:         "aggregation query with GROUP BY",
			sql:          "SELECT language, COUNT(*) FROM repositories GROUP BY language",
			expectedType: TypeAggregate,
		},
		{
			name:         "aggregation query with SUM",
			sql:          "SELECT SUM(stargazers_count) FROM repositories",
			expectedType: TypeAggregate,
		},
		{
			name:         "comparison query with greater than",
			sql:          "SELECT * FROM repositories WHERE stargazers_count > 1000",
			expectedType: TypeComparison,
		},
		{
			name:         "comparison query with BETWEEN",
			sql:          "SELECT * FROM repositories WHERE stargazers_count BETWEEN 100 AND 1000",
			expectedType: TypeComparison,
		},
		{
			name:         "comparison query with IN",
			sql:          "SELECT * FROM repositories WHERE language IN ('Go', 'Python')",
			expectedType: TypeComparison,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLLMParser(&MockLLMService{}, nil)
			queryType := parser.determineType(tt.sql)
			assert.Equal(t, tt.expectedType, queryType)
		})
	}
}

func TestLLMParser_ParseOperation(t *testing.T) {
	tests := []struct {
		name              string
		planLine          string
		expectedOperation *Operation
	}{
		{
			name:     "table scan operation",
			planLine: "SEQ_SCAN repositories (1000 rows)",
			expectedOperation: &Operation{
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
			expectedOperation: &Operation{
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
			expectedOperation: &Operation{
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
		operations   []Operation
		expectedTips []string
	}{
		{
			name: "query with sequential scan",
			sql:  "SELECT * FROM repositories WHERE language = 'Go'",
			operations: []Operation{
				{Type: "SEQ_SCAN", Table: "repositories"},
			},
			expectedTips: []string{
				"Consider adding indexes on frequently filtered columns",
			},
		},
		{
			name: "query with LIKE pattern starting with %",
			sql:  "SELECT * FROM repositories WHERE name LIKE '%test%'",
			operations: []Operation{
				{Type: "INDEX_SCAN", Table: "repositories"},
			},
			expectedTips: []string{
				"LIKE patterns starting with % cannot use indexes efficiently",
			},
		},
		{
			name: "query with OR condition",
			sql:  "SELECT * FROM repositories WHERE language = 'Go' OR language = 'Python'",
			operations: []Operation{
				{Type: "INDEX_SCAN", Table: "repositories"},
			},
			expectedTips: []string{
				"OR conditions may prevent index usage - consider UNION instead",
			},
		},
		{
			name: "query without LIMIT",
			sql:  "SELECT * FROM repositories WHERE language = 'Go'",
			operations: []Operation{
				{Type: "INDEX_SCAN", Table: "repositories"},
			},
			expectedTips: []string{
				"Consider adding LIMIT clause to prevent large result sets",
			},
		},
		{
			name: "query without WHERE clause",
			sql:  "SELECT * FROM repositories",
			operations: []Operation{
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
		operations      []Operation
		expectedIndexes []string
	}{
		{
			name: "operations with index usage",
			operations: []Operation{
				{Type: "INDEX_SCAN", Condition: "using idx_repositories_language"},
				{Type: "INDEX_SCAN", Condition: "using idx_repositories_stargazers"},
			},
			expectedIndexes: []string{"idx_repositories_language", "idx_repositories_stargazers"},
		},
		{
			name: "operations without index usage",
			operations: []Operation{
				{Type: "SEQ_SCAN", Condition: "full table scan"},
				{Type: "FILTER", Condition: "language = 'Go'"},
			},
			expectedIndexes: []string{},
		},
		{
			name:            "empty operations",
			operations:      []Operation{},
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
	assert.Positive(t, len(repoTable.Columns))
	assert.Positive(t, len(repoTable.Indexes))

	// Test content_chunks table
	assert.Contains(t, schema.Tables, "content_chunks")
	chunksTable := schema.Tables["content_chunks"]
	assert.Equal(t, "content_chunks", chunksTable.Name)
	assert.Positive(t, len(chunksTable.Columns))
	assert.Positive(t, len(chunksTable.Indexes))

	// Test specific columns exist in repositories table
	repoColumns := make(map[string]bool)
	for _, col := range repoTable.Columns {
		repoColumns[col.Name] = true
	}

	expectedRepoColumns := []string{
		"id", "full_name", "description", "language", "stargazers_count",
		"forks_count", "purpose", "technologies", "use_cases", "features",
	}

	for _, col := range expectedRepoColumns {
		assert.True(t, repoColumns[col], "Column %s should exist in repositories table", col)
	}

	// Test specific columns exist in content_chunks table
	chunksColumns := make(map[string]bool)
	for _, col := range chunksTable.Columns {
		chunksColumns[col.Name] = true
	}

	expectedChunksColumns := []string{
		"id", "repository_id", "source_path", "chunk_type", "content",
		"tokens", "priority", "created_at",
	}

	for _, col := range expectedChunksColumns {
		assert.True(t, chunksColumns[col], "Column %s should exist in content_chunks table", col)
	}

	// Test specific indexes exist in repositories table
	repoIndexes := make(map[string]bool)
	for _, idx := range repoTable.Indexes {
		repoIndexes[idx.Name] = true
	}

	expectedRepoIndexes := []string{
		"idx_repositories_language", "idx_repositories_updated_at",
		"idx_repositories_stargazers", "idx_repositories_full_name",
	}

	for _, idx := range expectedRepoIndexes {
		assert.True(t, repoIndexes[idx], "Index %s should exist in repositories table", idx)
	}

	// Test specific indexes exist in content_chunks table
	chunksIndexes := make(map[string]bool)
	for _, idx := range chunksTable.Indexes {
		chunksIndexes[idx.Name] = true
	}

	expectedChunksIndexes := []string{
		"idx_content_chunks_repo_type", "idx_content_chunks_repository_id",
	}

	for _, idx := range expectedChunksIndexes {
		assert.True(t, chunksIndexes[idx], "Index %s should exist in content_chunks table", idx)
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

	for range b.N {
		_ = parser.ValidateSQL(sql)
	}
}

func BenchmarkLLMParser_DetermineQueryType(b *testing.B) {
	parser := NewLLMParser(&MockLLMService{}, nil)
	sql := "SELECT language, COUNT(*) FROM repositories WHERE stargazers_count > 100 GROUP BY language ORDER BY COUNT(*) DESC"

	b.ResetTimer()

	for range b.N {
		_ = parser.determineType(sql)
	}
}
