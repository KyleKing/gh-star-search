package query

import (
	"context"
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

// Schema represents the database schema for query generation
type Schema struct {
	Tables []Table `json:"tables"`
}

// Table represents a database table schema
type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	Indexes []Index  `json:"indexes"`
}

// Column represents a database column
type Column struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Searchable  bool   `json:"searchable"`
}

// Index represents a database index
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Type    string   `json:"type"` // btree, fts, etc.
}