package llm

import (
	"context"

	"github.com/kyleking/gh-star-search/internal/types"
)

// Service defines the interface for LLM operations
type Service interface {
	Summarize(ctx context.Context, prompt string, content string) (*SummaryResponse, error)
	ParseQuery(ctx context.Context, query string, schema types.Schema) (*QueryResponse, error)
	Configure(config Config) error
}

// Config represents LLM service configuration
type Config struct {
	Provider string            `json:"provider"` // openai, anthropic, local
	Model    string            `json:"model"`
	APIKey   string            `json:"api_key,omitempty"`
	BaseURL  string            `json:"base_url,omitempty"`
	Options  map[string]string `json:"options,omitempty"`
}

// SummaryResponse represents the response from content summarization
type SummaryResponse struct {
	Purpose      string   `json:"purpose"`
	Technologies []string `json:"technologies"`
	UseCases     []string `json:"use_cases"`
	Features     []string `json:"features"`
	Installation string   `json:"installation"`
	Usage        string   `json:"usage"`
	Confidence   float64  `json:"confidence"`
}

// QueryResponse represents the response from query parsing
type QueryResponse struct {
	SQL         string            `json:"sql"`
	Parameters  map[string]string `json:"parameters"`
	Explanation string            `json:"explanation"`
	Confidence  float64           `json:"confidence"`
	Reasoning   string            `json:"reasoning"`
}

// Provider constants for different LLM providers
const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderLocal     = "local"
	ProviderOllama    = "ollama"
)

// Model constants for common models
const (
	ModelGPT4         = "gpt-4"
	ModelGPT35Turbo   = "gpt-3.5-turbo"
	ModelClaude3      = "claude-3-sonnet-20240229"
	ModelLlama2       = "llama2"
	ModelCodeLlama    = "codellama"
)
