package query

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/kyleking/gh-star-search/internal/llm"
	"github.com/kyleking/gh-star-search/internal/types"
)

// LLMParser implements the Parser interface using LLM services
type LLMParser struct {
	llmService llm.Service
	db         *sql.DB
	schema     types.Schema
}

// NewLLMParser creates a new LLM-based query parser
func NewLLMParser(llmService llm.Service, db *sql.DB) *LLMParser {
	return &LLMParser{
		llmService: llmService,
		db:         db,
		schema:     getDefaultSchema(),
	}
}

// Parse converts a natural language query to SQL using LLM
func (p *LLMParser) Parse(ctx context.Context, naturalQuery string) (*ParsedQuery, error) {
	// Use the LLM service to parse the query
	response, err := p.llmService.ParseQuery(ctx, naturalQuery, p.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query with LLM: %w", err)
	}

	// Validate the generated SQL
	if err := p.ValidateSQL(response.SQL); err != nil {
		return nil, fmt.Errorf("generated SQL is invalid: %w", err)
	}

	// Determine query type
	queryType := p.determineQueryType(response.SQL)

	return &ParsedQuery{
		SQL:         response.SQL,
		Parameters:  response.Parameters,
		Explanation: response.Explanation,
		Confidence:  response.Confidence,
		QueryType:   queryType,
	}, nil
}

// ValidateSQL validates SQL for safety and correctness
func (p *LLMParser) ValidateSQL(sql string) error {
	if sql == "" {
		return fmt.Errorf("SQL query cannot be empty")
	}

	// Convert to lowercase for checking
	lowerSQL := strings.ToLower(strings.TrimSpace(sql))

	// Basic SQL injection protection - check for specific injection patterns first
	injectionPatterns := []string{
		"--", "/*", "*/", ";", 
		"or 1=1", "or '1'='1", "or \"1\"=\"1\"",
		"and 1=1", "and '1'='1", "and \"1\"=\"1\"",
		"' or '", "\" or \"", "' and '", "\" and \"",
		"union select", "union all select",
	}

	for _, pattern := range injectionPatterns {
		if strings.Contains(lowerSQL, pattern) {
			return fmt.Errorf("SQL contains suspicious pattern: %s", pattern)
		}
	}

	// Ensure it's a SELECT statement
	if !strings.HasPrefix(lowerSQL, "select") {
		return fmt.Errorf("only SELECT statements are allowed")
	}

	// Check for dangerous operations
	dangerousPatterns := []string{
		"drop table", "drop database", "delete from", "truncate",
		"alter table", "create table", "insert into", "update ",
		"grant ", "revoke ", "exec ", "execute ", "xp_",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerSQL, pattern) {
			return fmt.Errorf("SQL contains potentially dangerous operation: %s", pattern)
		}
	}

	// Check for valid table names
	validTables := []string{"repositories", "content_chunks"}
	hasValidTable := false
	for _, table := range validTables {
		if strings.Contains(lowerSQL, table) {
			hasValidTable = true
			break
		}
	}

	if !hasValidTable {
		return fmt.Errorf("SQL must reference valid tables: %v", validTables)
	}

	// Try to prepare the statement to check syntax
	if p.db != nil {
		stmt, err := p.db.Prepare(sql)
		if err != nil {
			return fmt.Errorf("SQL syntax error: %w", err)
		}
		stmt.Close()
	}

	return nil
}

// ExplainQuery provides execution plan information for a SQL query
func (p *LLMParser) ExplainQuery(sql string) (*QueryPlan, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database connection required for query explanation")
	}

	// DuckDB uses EXPLAIN for query plans
	explainSQL := "EXPLAIN " + sql
	rows, err := p.db.Query(explainSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to explain query: %w", err)
	}
	defer rows.Close()

	var operations []QueryOperation
	var estimatedRows int
	var estimatedCost float64

	// Parse the explain output (DuckDB specific format)
	for rows.Next() {
		var planLine string
		if err := rows.Scan(&planLine); err != nil {
			return nil, fmt.Errorf("failed to scan explain result: %w", err)
		}

		// Parse operation from plan line
		operation := p.parseOperation(planLine)
		if operation != nil {
			operations = append(operations, *operation)
			estimatedRows += operation.EstimatedRows
			estimatedCost += operation.Cost
		}
	}

	// Analyze for optimization tips
	tips := p.generateOptimizationTips(sql, operations)

	// Find indexes used
	indexesUsed := p.findIndexesUsed(operations)

	return &QueryPlan{
		EstimatedRows:    estimatedRows,
		EstimatedCost:    estimatedCost,
		IndexesUsed:      indexesUsed,
		Operations:       operations,
		OptimizationTips: tips,
	}, nil
}

// determineQueryType analyzes SQL to determine the query type
func (p *LLMParser) determineQueryType(sql string) QueryType {
	lowerSQL := strings.ToLower(sql)

	// Check for aggregation functions first (highest priority)
	aggregateFunctions := []string{"count(", "sum(", "avg(", "max(", "min(", "group by"}
	for _, fn := range aggregateFunctions {
		if strings.Contains(lowerSQL, fn) {
			return QueryTypeAggregate
		}
	}

	// Check for comparison operations (numeric/date comparisons)
	comparisonOps := []string{">", "<", ">=", "<=", "between", " in ("}
	for _, op := range comparisonOps {
		if strings.Contains(lowerSQL, op) {
			return QueryTypeComparison
		}
	}

	// Check for search patterns (text search operations)
	searchPatterns := []string{"ilike", "like", "full text", "match", "contains"}
	for _, pattern := range searchPatterns {
		if strings.Contains(lowerSQL, pattern) {
			return QueryTypeSearch
		}
	}

	// Check for filtering (simple equality, LIMIT clauses, etc.)
	if strings.Contains(lowerSQL, "where") || strings.Contains(lowerSQL, "having") || strings.Contains(lowerSQL, "limit") {
		return QueryTypeFilter
	}

	// Default to search for basic SELECT statements
	return QueryTypeSearch
}

// parseOperation parses a single operation from DuckDB explain output
func (p *LLMParser) parseOperation(planLine string) *QueryOperation {
	// This is a simplified parser for DuckDB explain output
	// In a real implementation, you'd need more sophisticated parsing
	
	planLine = strings.TrimSpace(planLine)
	if planLine == "" {
		return nil
	}

	// Extract operation type (first word)
	parts := strings.Fields(planLine)
	if len(parts) == 0 {
		return nil
	}

	opType := parts[0]
	
	// Try to extract table name and estimated rows
	var table string
	var estimatedRows int
	var cost float64

	// Look for table references
	if strings.Contains(planLine, "repositories") {
		table = "repositories"
	} else if strings.Contains(planLine, "content_chunks") {
		table = "content_chunks"
	}

	// Extract estimated rows (simplified - would need regex for real parsing)
	rowsRegex := regexp.MustCompile(`(\d+)\s+rows`)
	if matches := rowsRegex.FindStringSubmatch(planLine); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &estimatedRows)
	}

	// Estimate cost based on operation type
	switch strings.ToLower(opType) {
	case "seq_scan", "table_scan":
		cost = float64(estimatedRows) * 0.1
	case "index_scan":
		cost = float64(estimatedRows) * 0.01
	case "filter":
		cost = float64(estimatedRows) * 0.05
	case "projection":
		cost = float64(estimatedRows) * 0.02
	default:
		cost = float64(estimatedRows) * 0.1
	}

	return &QueryOperation{
		Type:          opType,
		Table:         table,
		Condition:     planLine,
		EstimatedRows: estimatedRows,
		Cost:          cost,
	}
}

// generateOptimizationTips provides suggestions for query optimization
func (p *LLMParser) generateOptimizationTips(sql string, operations []QueryOperation) []string {
	var tips []string
	lowerSQL := strings.ToLower(sql)

	// Check for missing indexes
	hasSeqScan := false
	for _, op := range operations {
		if strings.Contains(strings.ToLower(op.Type), "seq_scan") || 
		   strings.Contains(strings.ToLower(op.Type), "table_scan") {
			hasSeqScan = true
			break
		}
	}

	if hasSeqScan {
		tips = append(tips, "Consider adding indexes on frequently filtered columns")
	}

	// Check for inefficient patterns
	if strings.Contains(lowerSQL, "like '%") && strings.Contains(lowerSQL, "%'") {
		tips = append(tips, "LIKE patterns starting with % cannot use indexes efficiently")
	}

	if strings.Contains(lowerSQL, "or") {
		tips = append(tips, "OR conditions may prevent index usage - consider UNION instead")
	}

	if !strings.Contains(lowerSQL, "limit") {
		tips = append(tips, "Consider adding LIMIT clause to prevent large result sets")
	}

	// Check for missing WHERE clause on large tables
	if !strings.Contains(lowerSQL, "where") {
		tips = append(tips, "Consider adding WHERE clause to filter results")
	}

	return tips
}

// findIndexesUsed extracts index names from operations
func (p *LLMParser) findIndexesUsed(operations []QueryOperation) []string {
	var indexes []string
	indexMap := make(map[string]bool)

	for _, op := range operations {
		if strings.Contains(strings.ToLower(op.Type), "index") {
			// Extract index name from condition (simplified)
			if strings.Contains(op.Condition, "idx_") {
				// This would need more sophisticated parsing in practice
				parts := strings.Fields(op.Condition)
				for _, part := range parts {
					if strings.HasPrefix(part, "idx_") {
						if !indexMap[part] {
							indexes = append(indexes, part)
							indexMap[part] = true
						}
					}
				}
			}
		}
	}

	return indexes
}

// getDefaultSchema returns the default database schema
func getDefaultSchema() types.Schema {
	return types.Schema{
		Tables: map[string]types.Table{
			"repositories": {
				Name: "repositories",
				Columns: []types.Column{
					{Name: "id", Type: "VARCHAR", Description: "Unique repository identifier", Searchable: false},
					{Name: "full_name", Type: "VARCHAR", Description: "Repository full name (owner/repo)", Searchable: true},
					{Name: "description", Type: "TEXT", Description: "Repository description", Searchable: true},
					{Name: "language", Type: "VARCHAR", Description: "Primary programming language", Searchable: true},
					{Name: "stargazers_count", Type: "INTEGER", Description: "Number of stars", Searchable: false},
					{Name: "forks_count", Type: "INTEGER", Description: "Number of forks", Searchable: false},
					{Name: "size_kb", Type: "INTEGER", Description: "Repository size in KB", Searchable: false},
					{Name: "created_at", Type: "TIMESTAMP", Description: "Repository creation date", Searchable: false},
					{Name: "updated_at", Type: "TIMESTAMP", Description: "Last update date", Searchable: false},
					{Name: "last_synced", Type: "TIMESTAMP", Description: "Last sync date", Searchable: false},
					{Name: "topics", Type: "VARCHAR", Description: "Repository topics (JSON array)", Searchable: true},
					{Name: "license_name", Type: "VARCHAR", Description: "License name", Searchable: true},
					{Name: "license_spdx_id", Type: "VARCHAR", Description: "SPDX license identifier", Searchable: true},
					{Name: "purpose", Type: "TEXT", Description: "Repository purpose summary", Searchable: true},
					{Name: "technologies", Type: "VARCHAR", Description: "Technologies used (JSON array)", Searchable: true},
					{Name: "use_cases", Type: "VARCHAR", Description: "Use cases (JSON array)", Searchable: true},
					{Name: "features", Type: "VARCHAR", Description: "Key features (JSON array)", Searchable: true},
					{Name: "installation_instructions", Type: "TEXT", Description: "Installation instructions", Searchable: true},
					{Name: "usage_instructions", Type: "TEXT", Description: "Usage instructions", Searchable: true},
					{Name: "content_hash", Type: "VARCHAR", Description: "Content hash for change detection", Searchable: false},
				},
				Indexes: []types.Index{
					{Name: "idx_repositories_language", Columns: []string{"language"}, Type: "btree"},
					{Name: "idx_repositories_updated_at", Columns: []string{"updated_at"}, Type: "btree"},
					{Name: "idx_repositories_stargazers", Columns: []string{"stargazers_count"}, Type: "btree"},
					{Name: "idx_repositories_full_name", Columns: []string{"full_name"}, Type: "btree"},
				},
			},
			"content_chunks": {
				Name: "content_chunks",
				Columns: []types.Column{
					{Name: "id", Type: "VARCHAR", Description: "Unique chunk identifier", Searchable: false},
					{Name: "repository_id", Type: "VARCHAR", Description: "Reference to repository", Searchable: false},
					{Name: "source_path", Type: "VARCHAR", Description: "Source file path", Searchable: true},
					{Name: "chunk_type", Type: "VARCHAR", Description: "Type of content (readme, code, docs)", Searchable: true},
					{Name: "content", Type: "TEXT", Description: "Actual content text", Searchable: true},
					{Name: "tokens", Type: "INTEGER", Description: "Number of tokens", Searchable: false},
					{Name: "priority", Type: "INTEGER", Description: "Content priority", Searchable: false},
					{Name: "created_at", Type: "TIMESTAMP", Description: "Chunk creation date", Searchable: false},
				},
				Indexes: []types.Index{
					{Name: "idx_content_chunks_repo_type", Columns: []string{"repository_id", "chunk_type"}, Type: "btree"},
					{Name: "idx_content_chunks_repository_id", Columns: []string{"repository_id"}, Type: "btree"},
				},
			},
		},
	}
}