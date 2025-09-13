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
	ExplainQuery(sql string) (*Plan, error)
}

// ParsedQuery represents a parsed natural language query
type ParsedQuery struct {
	SQL         string            `json:"sql"`
	Parameters  map[string]string `json:"parameters"`
	Explanation string            `json:"explanation"`
	Confidence  float64           `json:"confidence"`
	Type        Type              `json:"query_type"`
}

// Type represents different types of queries
type Type string

const (
	TypeSearch     Type = "search"
	TypeFilter     Type = "filter"
	TypeAggregate  Type = "aggregate"
	TypeComparison Type = "comparison"
)

// Plan represents the execution plan for a SQL query
type Plan struct {
	EstimatedRows    int         `json:"estimated_rows"`
	EstimatedCost    float64     `json:"estimated_cost"`
	IndexesUsed      []string    `json:"indexes_used"`
	Operations       []Operation `json:"operations"`
	OptimizationTips []string    `json:"optimization_tips"`
}

// Operation represents a single operation in the query plan
type Operation struct {
	Type          string  `json:"type"`
	Table         string  `json:"table"`
	Condition     string  `json:"condition"`
	EstimatedRows int     `json:"estimated_rows"`
	Cost          float64 `json:"cost"`
}

// NewParser creates a new query parser with the specified LLM service and database
func NewParser(llmService llm.Service, db *sql.DB) Parser {
	return NewLLMParser(llmService, db)
}
