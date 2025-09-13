package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/username/gh-star-search/internal/query"
)

func TestClient_Configure(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid OpenAI config",
			config: Config{
				Provider: ProviderOpenAI,
				Model:    ModelGPT35Turbo,
				APIKey:   "test-key",
			},
			wantErr: false,
		},
		{
			name: "valid Anthropic config",
			config: Config{
				Provider: ProviderAnthropic,
				Model:    ModelClaude3,
				APIKey:   "test-key",
			},
			wantErr: false,
		},
		{
			name: "valid Ollama config",
			config: Config{
				Provider: ProviderOllama,
				Model:    ModelLlama2,
				BaseURL:  "http://localhost:11434",
			},
			wantErr: false,
		},
		{
			name: "missing provider",
			config: Config{
				Model:  ModelGPT35Turbo,
				APIKey: "test-key",
			},
			wantErr: true,
		},
		{
			name: "missing model",
			config: Config{
				Provider: ProviderOpenAI,
				APIKey:   "test-key",
			},
			wantErr: true,
		},
		{
			name: "missing API key for OpenAI",
			config: Config{
				Provider: ProviderOpenAI,
				Model:    ModelGPT35Turbo,
			},
			wantErr: true,
		},
		{
			name: "unsupported provider",
			config: Config{
				Provider: "unsupported",
				Model:    "test-model",
				APIKey:   "test-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(Config{})
			err := client.Configure(tt.config)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if !tt.wantErr && client.config.Provider != tt.config.Provider {
				t.Errorf("Configure() did not set provider correctly")
			}
		})
	}
}

func TestClient_SummarizeOpenAI(t *testing.T) {
	// Mock OpenAI API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Expected path /chat/completions, got %s", r.URL.Path)
		}
		
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Authorization header with Bearer token")
		}
		
		response := openAIResponse{
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Content: `{
							"purpose": "A test repository for unit testing",
							"technologies": ["Go", "Testing"],
							"use_cases": ["Unit Testing", "Development"],
							"features": ["Test Framework", "Mocking"],
							"installation": "go get example.com/test",
							"usage": "go test ./...",
							"confidence": 0.9
						}`,
					},
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{})
	err := client.Configure(Config{
		Provider: ProviderOpenAI,
		Model:    ModelGPT35Turbo,
		APIKey:   "test-key",
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to configure client: %v", err)
	}

	ctx := context.Background()
	summary, err := client.Summarize(ctx, "", "test repository content")
	
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}
	
	if summary.Purpose != "A test repository for unit testing" {
		t.Errorf("Expected purpose 'A test repository for unit testing', got '%s'", summary.Purpose)
	}
	
	if len(summary.Technologies) != 2 {
		t.Errorf("Expected 2 technologies, got %d", len(summary.Technologies))
	}
	
	if summary.Confidence != 0.9 {
		t.Errorf("Expected confidence 0.9, got %f", summary.Confidence)
	}
}

func TestClient_SummarizeOpenAI_Error(t *testing.T) {
	// Mock OpenAI API server with error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := openAIResponse{
			Error: &openAIError{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{})
	err := client.Configure(Config{
		Provider: ProviderOpenAI,
		Model:    ModelGPT35Turbo,
		APIKey:   "invalid-key",
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to configure client: %v", err)
	}

	ctx := context.Background()
	_, err = client.Summarize(ctx, "", "test content")
	
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	
	if !contains(err.Error(), "Invalid API key") {
		t.Errorf("Expected error to contain 'Invalid API key', got: %v", err)
	}
}

func TestClient_ParseQueryOpenAI(t *testing.T) {
	// Mock OpenAI API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := openAIResponse{
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Content: `{
							"sql": "SELECT full_name, description FROM repositories WHERE language = 'Go' LIMIT 10",
							"parameters": {},
							"explanation": "This query finds Go repositories",
							"confidence": 0.95,
							"reasoning": "User asked for Go repositories, filtered by language column"
						}`,
					},
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{})
	err := client.Configure(Config{
		Provider: ProviderOpenAI,
		Model:    ModelGPT35Turbo,
		APIKey:   "test-key",
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to configure client: %v", err)
	}

	schema := query.Schema{
		Tables: map[string]query.Table{
			"repositories": {
				Name: "repositories",
				Columns: []query.Column{
					{Name: "full_name", Type: "VARCHAR"},
					{Name: "description", Type: "TEXT"},
					{Name: "language", Type: "VARCHAR"},
				},
			},
		},
	}

	ctx := context.Background()
	queryResp, err := client.ParseQuery(ctx, "show me Go repositories", schema)
	
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	
	expectedSQL := "SELECT full_name, description FROM repositories WHERE language = 'Go' LIMIT 10"
	if queryResp.SQL != expectedSQL {
		t.Errorf("Expected SQL '%s', got '%s'", expectedSQL, queryResp.SQL)
	}
	
	if queryResp.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", queryResp.Confidence)
	}
}

func TestClient_SummarizeAnthropic(t *testing.T) {
	// Mock Anthropic API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("Expected path /messages, got %s", r.URL.Path)
		}
		
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("Expected x-api-key header with test-key")
		}
		
		response := anthropicResponse{
			Content: []anthropicContent{
				{
					Type: "text",
					Text: `{
						"purpose": "A test repository for Anthropic testing",
						"technologies": ["Python", "AI"],
						"use_cases": ["AI Development"],
						"features": ["Claude Integration"],
						"installation": "pip install anthropic",
						"usage": "import anthropic",
						"confidence": 0.85
					}`,
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{})
	err := client.Configure(Config{
		Provider: ProviderAnthropic,
		Model:    ModelClaude3,
		APIKey:   "test-key",
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to configure client: %v", err)
	}

	ctx := context.Background()
	summary, err := client.Summarize(ctx, "", "test repository content")
	
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}
	
	if summary.Purpose != "A test repository for Anthropic testing" {
		t.Errorf("Expected purpose 'A test repository for Anthropic testing', got '%s'", summary.Purpose)
	}
	
	if summary.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", summary.Confidence)
	}
}

func TestClient_SummarizeOllama(t *testing.T) {
	// Mock Ollama API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("Expected path /api/generate, got %s", r.URL.Path)
		}
		
		response := ollamaResponse{
			Response: `{
				"purpose": "A test repository for Ollama testing",
				"technologies": ["Go", "Local AI"],
				"use_cases": ["Local Development"],
				"features": ["Offline Processing"],
				"installation": "ollama pull llama2",
				"usage": "ollama run llama2",
				"confidence": 0.7
			}`,
			Done: true,
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{})
	err := client.Configure(Config{
		Provider: ProviderOllama,
		Model:    ModelLlama2,
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to configure client: %v", err)
	}

	ctx := context.Background()
	summary, err := client.Summarize(ctx, "", "test repository content")
	
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}
	
	if summary.Purpose != "A test repository for Ollama testing" {
		t.Errorf("Expected purpose 'A test repository for Ollama testing', got '%s'", summary.Purpose)
	}
	
	if summary.Confidence != 0.7 {
		t.Errorf("Expected confidence 0.7, got %f", summary.Confidence)
	}
}

func TestClient_UnconfiguredError(t *testing.T) {
	client := NewClient(Config{})
	
	ctx := context.Background()
	
	// Test summarize with unconfigured client
	_, err := client.Summarize(ctx, "", "test content")
	if err == nil {
		t.Error("Expected error for unconfigured client, got nil")
	}
	
	// Test parse query with unconfigured client
	schema := query.Schema{Tables: map[string]query.Table{}}
	_, err = client.ParseQuery(ctx, "test query", schema)
	if err == nil {
		t.Error("Expected error for unconfigured client, got nil")
	}
}

func TestClient_HTTPError(t *testing.T) {
	// Mock server that returns HTTP error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewClient(Config{})
	err := client.Configure(Config{
		Provider: ProviderOpenAI,
		Model:    ModelGPT35Turbo,
		APIKey:   "test-key",
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to configure client: %v", err)
	}

	ctx := context.Background()
	_, err = client.Summarize(ctx, "", "test content")
	
	if err == nil {
		t.Fatal("Expected error for HTTP 500, got nil")
	}
	
	if !contains(err.Error(), "500") {
		t.Errorf("Expected error to contain '500', got: %v", err)
	}
}

func TestBuildSummarizationPrompt(t *testing.T) {
	client := NewClient(Config{})
	
	prompt := client.buildSummarizationPrompt("", "test content")
	
	if !contains(prompt, "test content") {
		t.Error("Prompt should contain the provided content")
	}
	
	if !contains(prompt, "JSON object") {
		t.Error("Prompt should mention JSON object format")
	}
	
	// Test with custom prompt
	customPrompt := client.buildSummarizationPrompt("Focus on security", "test content")
	
	if !contains(customPrompt, "Focus on security") {
		t.Error("Prompt should contain custom instructions")
	}
}

func TestBuildQueryParsingPrompt(t *testing.T) {
	client := NewClient(Config{})
	
	schema := query.Schema{
		Tables: map[string]query.Table{
			"repositories": {
				Name: "repositories",
				Columns: []query.Column{
					{Name: "full_name", Type: "VARCHAR", Description: "Repository full name"},
					{Name: "language", Type: "VARCHAR", Description: "Primary language"},
				},
			},
		},
	}
	
	prompt := client.buildQueryParsingPrompt("show Go repositories", schema)
	
	if !contains(prompt, "show Go repositories") {
		t.Error("Prompt should contain the user query")
	}
	
	if !contains(prompt, "repositories") {
		t.Error("Prompt should contain table name")
	}
	
	if !contains(prompt, "full_name") {
		t.Error("Prompt should contain column names")
	}
	
	if !contains(prompt, "Repository full name") {
		t.Error("Prompt should contain column descriptions")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 1; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}