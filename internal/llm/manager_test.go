package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kyleking/gh-star-search/internal/query"
)

// MockService implements the Service interface for testing
type MockService struct {
	summarizeFunc   func(ctx context.Context, prompt string, content string) (*SummaryResponse, error)
	parseQueryFunc  func(ctx context.Context, query string, schema query.Schema) (*QueryResponse, error)
	configureFunc   func(config Config) error
	shouldFail      bool
	failAfterCalls  int
	callCount       int
}

func (m *MockService) Summarize(ctx context.Context, prompt string, content string) (*SummaryResponse, error) {
	m.callCount++
	if m.shouldFail || (m.failAfterCalls > 0 && m.callCount <= m.failAfterCalls) {
		return nil, errors.New("mock service error")
	}
	if m.summarizeFunc != nil {
		return m.summarizeFunc(ctx, prompt, content)
	}
	return &SummaryResponse{
		Purpose:    "Mock summary",
		Confidence: 0.8,
	}, nil
}

func (m *MockService) ParseQuery(ctx context.Context, query string, schema query.Schema) (*QueryResponse, error) {
	m.callCount++
	if m.shouldFail || (m.failAfterCalls > 0 && m.callCount <= m.failAfterCalls) {
		return nil, errors.New("mock service error")
	}
	if m.parseQueryFunc != nil {
		return m.parseQueryFunc(ctx, query, schema)
	}
	return &QueryResponse{
		SQL:        "SELECT * FROM repositories",
		Confidence: 0.8,
	}, nil
}

func (m *MockService) Configure(config Config) error {
	if m.configureFunc != nil {
		return m.configureFunc(config)
	}
	return nil
}

func TestManager_RegisterProvider(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	tests := []struct {
		name        string
		providerName string
		service     Service
		wantErr     bool
	}{
		{
			name:        "valid provider",
			providerName: "test-provider",
			service:     &MockService{},
			wantErr:     false,
		},
		{
			name:        "empty name",
			providerName: "",
			service:     &MockService{},
			wantErr:     true,
		},
		{
			name:        "nil service",
			providerName: "test-provider",
			service:     nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.RegisterProvider(tt.providerName, tt.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterProvider() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && !manager.IsProviderRegistered(tt.providerName) {
				t.Errorf("Provider %s should be registered", tt.providerName)
			}
		})
	}
}

func TestManager_Configure(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())
	mockService := &MockService{}

	// Register a provider
	err := manager.RegisterProvider("test-provider", mockService)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Provider: "test-provider",
				Model:    "test-model",
			},
			wantErr: false,
		},
		{
			name: "unregistered provider",
			config: Config{
				Provider: "unknown-provider",
				Model:    "test-model",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.Configure(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_SummarizeWithFallback(t *testing.T) {
	config := ManagerConfig{
		DefaultProvider:   "primary",
		FallbackProviders: []string{"secondary"},
		RetryAttempts:     1,
		RetryDelay:        time.Millisecond * 10,
		Timeout:           time.Second * 5,
		EnableFallback:    true,
	}
	manager := NewManager(config)

	// Register providers
	primaryService := &MockService{shouldFail: true}
	secondaryService := &MockService{shouldFail: false}

	err := manager.RegisterProvider("primary", primaryService)
	if err != nil {
		t.Fatalf("Failed to register primary provider: %v", err)
	}

	err = manager.RegisterProvider("secondary", secondaryService)
	if err != nil {
		t.Fatalf("Failed to register secondary provider: %v", err)
	}

	ctx := context.Background()
	summary, err := manager.Summarize(ctx, "", "test content")

	if err != nil {
		t.Fatalf("Summarize() should succeed with fallback, got error: %v", err)
	}

	if summary.Purpose != "Mock summary" {
		t.Errorf("Expected mock summary, got: %s", summary.Purpose)
	}
}

func TestManager_SummarizeWithRuleFallback(t *testing.T) {
	config := ManagerConfig{
		DefaultProvider:   "primary",
		FallbackProviders: []string{},
		RetryAttempts:     0,
		EnableFallback:    true,
	}
	manager := NewManager(config)

	// Register failing provider
	primaryService := &MockService{shouldFail: true}
	err := manager.RegisterProvider("primary", primaryService)
	if err != nil {
		t.Fatalf("Failed to register primary provider: %v", err)
	}

	ctx := context.Background()
	summary, err := manager.Summarize(ctx, "", "This is a JavaScript library for web development")

	if err != nil {
		t.Fatalf("Summarize() should succeed with rule-based fallback, got error: %v", err)
	}

	// Should use rule-based fallback with low confidence
	if summary.Confidence >= 0.5 {
		t.Errorf("Expected low confidence from rule-based fallback, got: %f", summary.Confidence)
	}
}

func TestManager_SummarizeAllProvidersFail(t *testing.T) {
	config := ManagerConfig{
		DefaultProvider:   "primary",
		FallbackProviders: []string{"secondary"},
		RetryAttempts:     0,
		EnableFallback:    false, // Disable rule-based fallback
	}
	manager := NewManager(config)

	// Register failing providers
	primaryService := &MockService{shouldFail: true}
	secondaryService := &MockService{shouldFail: true}

	err := manager.RegisterProvider("primary", primaryService)
	if err != nil {
		t.Fatalf("Failed to register primary provider: %v", err)
	}

	err = manager.RegisterProvider("secondary", secondaryService)
	if err != nil {
		t.Fatalf("Failed to register secondary provider: %v", err)
	}

	ctx := context.Background()
	_, err = manager.Summarize(ctx, "", "test content")

	if err == nil {
		t.Fatal("Expected error when all providers fail and fallback is disabled")
	}

	if !contains(err.Error(), "all LLM providers failed") {
		t.Errorf("Expected error about all providers failing, got: %v", err)
	}
}

func TestManager_ParseQueryWithRetry(t *testing.T) {
	config := ManagerConfig{
		DefaultProvider: "primary",
		RetryAttempts:   2,
		RetryDelay:      time.Millisecond * 10,
		EnableFallback:  false,
	}
	manager := NewManager(config)

	// Provider that fails first 2 calls, then succeeds
	primaryService := &MockService{failAfterCalls: 2}
	err := manager.RegisterProvider("primary", primaryService)
	if err != nil {
		t.Fatalf("Failed to register primary provider: %v", err)
	}

	ctx := context.Background()
	schema := query.Schema{Tables: map[string]query.Table{}}
	
	queryResp, err := manager.ParseQuery(ctx, "test query", schema)

	if err != nil {
		t.Fatalf("ParseQuery() should succeed after retries, got error: %v", err)
	}

	if queryResp.SQL != "SELECT * FROM repositories" {
		t.Errorf("Expected mock SQL, got: %s", queryResp.SQL)
	}

	// Should have made 3 calls (initial + 2 retries)
	if primaryService.callCount != 3 {
		t.Errorf("Expected 3 calls (with retries), got: %d", primaryService.callCount)
	}
}

func TestManager_TimeoutHandling(t *testing.T) {
	config := ManagerConfig{
		DefaultProvider: "primary",
		Timeout:         time.Millisecond * 50, // Very short timeout
		EnableFallback:  false,
	}
	manager := NewManager(config)

	// Provider that takes too long
	primaryService := &MockService{
		summarizeFunc: func(ctx context.Context, prompt string, content string) (*SummaryResponse, error) {
			select {
			case <-time.After(time.Millisecond * 100): // Longer than timeout
				return &SummaryResponse{}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	err := manager.RegisterProvider("primary", primaryService)
	if err != nil {
		t.Fatalf("Failed to register primary provider: %v", err)
	}

	ctx := context.Background()
	_, err = manager.Summarize(ctx, "", "test content")

	if err == nil {
		t.Fatal("Expected timeout error")
	}

	if !contains(err.Error(), "context deadline exceeded") && !contains(err.Error(), "all LLM providers failed") {
		t.Errorf("Expected timeout-related error, got: %v", err)
	}
}

func TestManager_GetAvailableProviders(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	// Initially no providers
	providers := manager.GetAvailableProviders()
	if len(providers) != 0 {
		t.Errorf("Expected 0 providers initially, got %d", len(providers))
	}

	// Register some providers
	err := manager.RegisterProvider("provider1", &MockService{})
	if err != nil {
		t.Fatalf("Failed to register provider1: %v", err)
	}

	err = manager.RegisterProvider("provider2", &MockService{})
	if err != nil {
		t.Fatalf("Failed to register provider2: %v", err)
	}

	providers = manager.GetAvailableProviders()
	if len(providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers))
	}

	// Check that both providers are in the list
	providerMap := make(map[string]bool)
	for _, p := range providers {
		providerMap[p] = true
	}

	if !providerMap["provider1"] || !providerMap["provider2"] {
		t.Errorf("Expected both provider1 and provider2 in list, got: %v", providers)
	}
}

func TestManager_IsProviderRegistered(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	// Initially no providers registered
	if manager.IsProviderRegistered("test") {
		t.Error("Expected provider 'test' to not be registered")
	}

	// Register a provider
	err := manager.RegisterProvider("test", &MockService{})
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Now it should be registered
	if !manager.IsProviderRegistered("test") {
		t.Error("Expected provider 'test' to be registered")
	}

	// Other providers should not be registered
	if manager.IsProviderRegistered("other") {
		t.Error("Expected provider 'other' to not be registered")
	}
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	if config.DefaultProvider != ProviderOpenAI {
		t.Errorf("Expected default provider to be OpenAI, got: %s", config.DefaultProvider)
	}

	if len(config.FallbackProviders) == 0 {
		t.Error("Expected fallback providers to be configured")
	}

	if config.RetryAttempts <= 0 {
		t.Errorf("Expected positive retry attempts, got: %d", config.RetryAttempts)
	}

	if config.RetryDelay <= 0 {
		t.Errorf("Expected positive retry delay, got: %v", config.RetryDelay)
	}

	if config.Timeout <= 0 {
		t.Errorf("Expected positive timeout, got: %v", config.Timeout)
	}

	if !config.EnableFallback {
		t.Error("Expected fallback to be enabled by default")
	}
}

func TestManager_ContextCancellation(t *testing.T) {
	config := ManagerConfig{
		DefaultProvider: "primary",
		EnableFallback:  false, // Disable fallback to ensure we get the cancellation error
	}
	manager := NewManager(config)

	// Provider that checks for context cancellation
	primaryService := &MockService{
		summarizeFunc: func(ctx context.Context, prompt string, content string) (*SummaryResponse, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Millisecond * 100):
				return &SummaryResponse{}, nil
			}
		},
	}

	err := manager.RegisterProvider("primary", primaryService)
	if err != nil {
		t.Fatalf("Failed to register primary provider: %v", err)
	}

	// Create context that gets cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = manager.Summarize(ctx, "", "test content")

	if err == nil {
		t.Fatal("Expected error due to context cancellation")
	}

	if !contains(err.Error(), "context canceled") && !contains(err.Error(), "all LLM providers failed") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}