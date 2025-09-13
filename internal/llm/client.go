package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/username/gh-star-search/internal/query"
)

// Client implements the Service interface with multiple provider support
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient creates a new LLM client with the given configuration
func NewClient(config Config) *Client {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Configure updates the client configuration
func (c *Client) Configure(config Config) error {
	if config.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	
	if config.Model == "" {
		return fmt.Errorf("model is required")
	}
	
	// Validate provider-specific requirements
	switch config.Provider {
	case ProviderOpenAI:
		if config.APIKey == "" {
			return fmt.Errorf("API key is required for OpenAI provider")
		}
		if config.BaseURL == "" {
			config.BaseURL = "https://api.openai.com/v1"
		}
	case ProviderAnthropic:
		if config.APIKey == "" {
			return fmt.Errorf("API key is required for Anthropic provider")
		}
		if config.BaseURL == "" {
			config.BaseURL = "https://api.anthropic.com/v1"
		}
	case ProviderLocal, ProviderOllama:
		if config.BaseURL == "" {
			config.BaseURL = "http://localhost:11434"
		}
	default:
		return fmt.Errorf("unsupported provider: %s", config.Provider)
	}
	
	c.config = config
	return nil
}

// Summarize generates a summary of repository content using the configured LLM
func (c *Client) Summarize(ctx context.Context, prompt string, content string) (*SummaryResponse, error) {
	if c.config.Provider == "" {
		return nil, fmt.Errorf("LLM client not configured")
	}
	
	// Build the summarization prompt
	fullPrompt := c.buildSummarizationPrompt(prompt, content)
	
	// Make the API call based on provider
	switch c.config.Provider {
	case ProviderOpenAI:
		return c.summarizeOpenAI(ctx, fullPrompt)
	case ProviderAnthropic:
		return c.summarizeAnthropic(ctx, fullPrompt)
	case ProviderLocal, ProviderOllama:
		return c.summarizeOllama(ctx, fullPrompt)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", c.config.Provider)
	}
}

// ParseQuery converts natural language to SQL using the configured LLM
func (c *Client) ParseQuery(ctx context.Context, query string, schema query.Schema) (*QueryResponse, error) {
	if c.config.Provider == "" {
		return nil, fmt.Errorf("LLM client not configured")
	}
	
	// Build the query parsing prompt
	fullPrompt := c.buildQueryParsingPrompt(query, schema)
	
	// Make the API call based on provider
	switch c.config.Provider {
	case ProviderOpenAI:
		return c.parseQueryOpenAI(ctx, fullPrompt)
	case ProviderAnthropic:
		return c.parseQueryAnthropic(ctx, fullPrompt)
	case ProviderLocal, ProviderOllama:
		return c.parseQueryOllama(ctx, fullPrompt)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", c.config.Provider)
	}
}

// buildSummarizationPrompt creates a structured prompt for content summarization
func (c *Client) buildSummarizationPrompt(userPrompt, content string) string {
	systemPrompt := `You are an expert at analyzing software repositories and extracting key information. 
Your task is to analyze the provided repository content and generate a structured summary.

Please respond with a JSON object containing the following fields:
- purpose: A clear, concise description of what this repository does (1-2 sentences)
- technologies: An array of main programming languages, frameworks, and tools used
- use_cases: An array of primary use cases or applications for this software
- features: An array of key features or capabilities
- installation: Brief installation instructions if available
- usage: Brief usage instructions or examples if available
- confidence: A float between 0.0 and 1.0 indicating your confidence in the analysis

Focus on extracting factual information from the content. If information is not available, use empty strings or arrays.`

	if userPrompt != "" {
		systemPrompt += "\n\nAdditional instructions: " + userPrompt
	}

	return fmt.Sprintf("%s\n\nRepository content to analyze:\n\n%s", systemPrompt, content)
}

// buildQueryParsingPrompt creates a structured prompt for natural language query parsing
func (c *Client) buildQueryParsingPrompt(query string, schema query.Schema) string {
	systemPrompt := `You are an expert at converting natural language queries into DuckDB SQL queries.
Your task is to convert the user's natural language query into a valid DuckDB SQL query based on the provided database schema.

Please respond with a JSON object containing the following fields:
- sql: The generated DuckDB SQL query
- parameters: A map of parameter names to values (if using parameterized queries)
- explanation: A clear explanation of what the query does
- confidence: A float between 0.0 and 1.0 indicating your confidence in the query
- reasoning: Your reasoning process for generating this query

Guidelines:
1. Use proper DuckDB SQL syntax
2. Only query tables and columns that exist in the schema
3. Use appropriate WHERE clauses, JOINs, and ORDER BY as needed
4. Prefer LIMIT clauses for large result sets
5. Use full-text search capabilities when appropriate
6. Be conservative - if unsure, ask for clarification rather than guessing

Database Schema:
%s

User Query: %s`

	schemaStr := c.formatSchema(schema)
	return fmt.Sprintf(systemPrompt, schemaStr, query)
}

// formatSchema converts the schema to a readable string format
func (c *Client) formatSchema(schema query.Schema) string {
	var sb strings.Builder
	
	for tableName, table := range schema.Tables {
		sb.WriteString(fmt.Sprintf("Table: %s\n", tableName))
		sb.WriteString("Columns:\n")
		for _, column := range table.Columns {
			sb.WriteString(fmt.Sprintf("  - %s (%s)", column.Name, column.Type))
			if column.Description != "" {
				sb.WriteString(fmt.Sprintf(" - %s", column.Description))
			}
			sb.WriteString("\n")
		}
		if len(table.Indexes) > 0 {
			sb.WriteString("Indexes:\n")
			for _, index := range table.Indexes {
				sb.WriteString(fmt.Sprintf("  - %s on %s\n", index.Name, strings.Join(index.Columns, ", ")))
			}
		}
		sb.WriteString("\n")
	}
	
	return sb.String()
}

// OpenAI API structures
type openAIRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIMessage     `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	ResponseFormat *openAIResponseFormat `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Error   *openAIError   `json:"error,omitempty"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// summarizeOpenAI handles OpenAI API calls for summarization
func (c *Client) summarizeOpenAI(ctx context.Context, prompt string) (*SummaryResponse, error) {
	reqBody := openAIRequest{
		Model: c.config.Model,
		Messages: []openAIMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   2000,
		ResponseFormat: &openAIResponseFormat{Type: "json_object"},
	}
	
	respBody, err := c.makeOpenAIRequest(ctx, "/chat/completions", reqBody)
	if err != nil {
		return nil, err
	}
	
	var response openAIResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", response.Error.Message)
	}
	
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}
	
	// Parse the JSON response
	var summary SummaryResponse
	if err := json.Unmarshal([]byte(response.Choices[0].Message.Content), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse summary JSON: %w", err)
	}
	
	return &summary, nil
}

// parseQueryOpenAI handles OpenAI API calls for query parsing
func (c *Client) parseQueryOpenAI(ctx context.Context, prompt string) (*QueryResponse, error) {
	reqBody := openAIRequest{
		Model: c.config.Model,
		Messages: []openAIMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   1000,
		ResponseFormat: &openAIResponseFormat{Type: "json_object"},
	}
	
	respBody, err := c.makeOpenAIRequest(ctx, "/chat/completions", reqBody)
	if err != nil {
		return nil, err
	}
	
	var response openAIResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", response.Error.Message)
	}
	
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}
	
	// Parse the JSON response
	var queryResp QueryResponse
	if err := json.Unmarshal([]byte(response.Choices[0].Message.Content), &queryResp); err != nil {
		return nil, fmt.Errorf("failed to parse query JSON: %w", err)
	}
	
	return &queryResp, nil
}

// makeOpenAIRequest makes an HTTP request to the OpenAI API
func (c *Client) makeOpenAIRequest(ctx context.Context, endpoint string, reqBody interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	return body, nil
}

// Anthropic API structures
type anthropicRequest struct {
	Model     string            `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int               `json:"max_tokens"`
	System    string            `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Error   *anthropicError    `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// summarizeAnthropic handles Anthropic API calls for summarization
func (c *Client) summarizeAnthropic(ctx context.Context, prompt string) (*SummaryResponse, error) {
	reqBody := anthropicRequest{
		Model:     c.config.Model,
		MaxTokens: 2000,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}
	
	respBody, err := c.makeAnthropicRequest(ctx, "/messages", reqBody)
	if err != nil {
		return nil, err
	}
	
	var response anthropicResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("Anthropic API error: %s", response.Error.Message)
	}
	
	if len(response.Content) == 0 {
		return nil, fmt.Errorf("no response from Anthropic")
	}
	
	// Parse the JSON response
	var summary SummaryResponse
	if err := json.Unmarshal([]byte(response.Content[0].Text), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse summary JSON: %w", err)
	}
	
	return &summary, nil
}

// parseQueryAnthropic handles Anthropic API calls for query parsing
func (c *Client) parseQueryAnthropic(ctx context.Context, prompt string) (*QueryResponse, error) {
	reqBody := anthropicRequest{
		Model:     c.config.Model,
		MaxTokens: 1000,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}
	
	respBody, err := c.makeAnthropicRequest(ctx, "/messages", reqBody)
	if err != nil {
		return nil, err
	}
	
	var response anthropicResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}
	
	if response.Error != nil {
		return nil, fmt.Errorf("Anthropic API error: %s", response.Error.Message)
	}
	
	if len(response.Content) == 0 {
		return nil, fmt.Errorf("no response from Anthropic")
	}
	
	// Parse the JSON response
	var queryResp QueryResponse
	if err := json.Unmarshal([]byte(response.Content[0].Text), &queryResp); err != nil {
		return nil, fmt.Errorf("failed to parse query JSON: %w", err)
	}
	
	return &queryResp, nil
}

// makeAnthropicRequest makes an HTTP request to the Anthropic API
func (c *Client) makeAnthropicRequest(ctx context.Context, endpoint string, reqBody interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	return body, nil
}

// Ollama API structures
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// summarizeOllama handles Ollama API calls for summarization
func (c *Client) summarizeOllama(ctx context.Context, prompt string) (*SummaryResponse, error) {
	reqBody := ollamaRequest{
		Model:  c.config.Model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}
	
	respBody, err := c.makeOllamaRequest(ctx, "/api/generate", reqBody)
	if err != nil {
		return nil, err
	}
	
	var response ollamaResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}
	
	if response.Error != "" {
		return nil, fmt.Errorf("Ollama API error: %s", response.Error)
	}
	
	// Parse the JSON response
	var summary SummaryResponse
	if err := json.Unmarshal([]byte(response.Response), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse summary JSON: %w", err)
	}
	
	return &summary, nil
}

// parseQueryOllama handles Ollama API calls for query parsing
func (c *Client) parseQueryOllama(ctx context.Context, prompt string) (*QueryResponse, error) {
	reqBody := ollamaRequest{
		Model:  c.config.Model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}
	
	respBody, err := c.makeOllamaRequest(ctx, "/api/generate", reqBody)
	if err != nil {
		return nil, err
	}
	
	var response ollamaResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}
	
	if response.Error != "" {
		return nil, fmt.Errorf("Ollama API error: %s", response.Error)
	}
	
	// Parse the JSON response
	var queryResp QueryResponse
	if err := json.Unmarshal([]byte(response.Response), &queryResp); err != nil {
		return nil, fmt.Errorf("failed to parse query JSON: %w", err)
	}
	
	return &queryResp, nil
}

// makeOllamaRequest makes an HTTP request to the Ollama API
func (c *Client) makeOllamaRequest(ctx context.Context, endpoint string, reqBody interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	return body, nil
}