package query

import (
	"context"
	"database/sql"

	"github.com/kyleking/gh-star-search/internal/llm"
)

// Parser defines the interface for natural language query parsing
type Parser interface {
	Parse(ctx context.Context, naturalQuery string) (*ParsedQuery, error)
	ValidateSQL(sql string) error
	ExplainQuery(sql string) (*QueryPlan, error)
}

// ParsedQuery represents a parsed natural language query
type ParsedQuery struct {
	SQL         string            `json:"sql"`
	Parameters  map[string]string `json:"parameters"`
	Explanation string            `json:"explanation"`
	Confidence  float64           `json:"confidence"`
	QueryType   QueryType         `json:"query_type"`
}

// QueryType represents different types of queries
type QueryType string

const (
	QueryTypeSearch     QueryType = "search"
	QueryTypeFilter     QueryType = "filter"
	QueryTypeAggregate  QueryType = "aggregate"
	QueryTypeComparison QueryType = "comparison"
)

// QueryPlan represents the execution plan for a SQL query
type QueryPlan struct {
	EstimatedRows    int                    `json:"estimated_rows"`
	EstimatedCost    float64                `json:"estimated_cost"`
	IndexesUsed      []string               `json:"indexes_used"`
	Operations       []QueryOperation       `json:"operations"`
	OptimizationTips []string               `json:"optimization_tips"`
}

// QueryOperation represents a single operation in the query plan
type QueryOperation struct {
	Type        string  `json:"type"`
	Table       string  `json:"table"`
	Condition   string  `json:"condition"`
	EstimatedRows int   `json:"estimated_rows"`
	Cost        float64 `json:"cost"`
}

// Schema, Table, Column, and Index types are now in the types package
// to avoid import cycles

// NewParser creates a new query parser with the specified LLM service and database
func NewParser(llmService llm.Service, db *sql.DB) Parser {
	return NewLLMParser(llmService, db)
}
