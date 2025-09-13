package llm

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigManager_LoadConfig(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	cm := NewConfigManager(configPath)

	// Test loading non-existent config (should return default)
	config, err := cm.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.DefaultProvider != ProviderOpenAI {
		t.Errorf("Expected default provider OpenAI, got %s", config.DefaultProvider)
	}

	// Test saving and loading config
	config.DefaultProvider = ProviderAnthropic
	config.RetryAttempts = 5

	err = cm.SaveConfig(config)
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Load the saved config
	loadedConfig, err := cm.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loadedConfig.DefaultProvider != ProviderAnthropic {
		t.Errorf("Expected provider Anthropic, got %s", loadedConfig.DefaultProvider)
	}

	if loadedConfig.RetryAttempts != 5 {
		t.Errorf("Expected retry attempts 5, got %d", loadedConfig.RetryAttempts)
	}
}

func TestConfigManager_SaveConfig(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "subdir", "test-config.json")

	cm := NewConfigManager(configPath)
	config := DefaultManagerConfig()

	// Test saving config (should create directory)
	err := cm.SaveConfig(&config)
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestSetupDefaultProviders(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	err := SetupDefaultProviders(manager)
	if err != nil {
		t.Fatalf("SetupDefaultProviders() error = %v", err)
	}

	// Check that providers were registered
	providers := manager.GetAvailableProviders()
	expectedProviders := []string{ProviderOpenAI, ProviderAnthropic, ProviderOllama}

	if len(providers) != len(expectedProviders) {
		t.Errorf("Expected %d providers, got %d", len(expectedProviders), len(providers))
	}

	for _, expected := range expectedProviders {
		if !manager.IsProviderRegistered(expected) {
			t.Errorf("Provider %s not registered", expected)
		}
	}
}

func TestConfigureFromEnvironment(t *testing.T) {
	// Save original environment
	originalOpenAI := os.Getenv("OPENAI_API_KEY")
	originalAnthropic := os.Getenv("ANTHROPIC_API_KEY")
	originalOllama := os.Getenv("OLLAMA_BASE_URL")

	// Clean up after test
	defer func() {
		os.Setenv("OPENAI_API_KEY", originalOpenAI)
		os.Setenv("ANTHROPIC_API_KEY", originalAnthropic)
		os.Setenv("OLLAMA_BASE_URL", originalOllama)
	}()

	// Set test environment variables
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")

	manager := NewManager(DefaultManagerConfig())

	err := SetupDefaultProviders(manager)
	if err != nil {
		t.Fatalf("SetupDefaultProviders() error = %v", err)
	}

	err = ConfigureFromEnvironment(manager)
	if err != nil {
		t.Fatalf("ConfigureFromEnvironment() error = %v", err)
	}

	// Test that providers are registered (we can't easily test configuration without exposing internals)
	if !manager.IsProviderRegistered(ProviderOpenAI) {
		t.Error("OpenAI provider not registered")
	}

	if !manager.IsProviderRegistered(ProviderAnthropic) {
		t.Error("Anthropic provider not registered")
	}

	if !manager.IsProviderRegistered(ProviderOllama) {
		t.Error("Ollama provider not registered")
	}
}

func TestCreateManagerWithDefaults(t *testing.T) {
	manager, err := CreateManagerWithDefaults()
	if err != nil {
		t.Fatalf("CreateManagerWithDefaults() error = %v", err)
	}

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	// Check that default providers are available
	providers := manager.GetAvailableProviders()
	if len(providers) < 3 {
		t.Errorf("Expected at least 3 providers, got %d", len(providers))
	}
}

func TestManagerConfig_JSON(t *testing.T) {
	config := ManagerConfig{
		DefaultProvider:   ProviderOpenAI,
		FallbackProviders: []string{ProviderAnthropic},
		RetryAttempts:     3,
		RetryDelay:        time.Second * 2,
		Timeout:           time.Minute * 5,
		EnableFallback:    true,
	}

	// Test marshaling
	data, err := config.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	// Test unmarshaling
	var unmarshaled ManagerConfig

	err = unmarshaled.UnmarshalJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	// Check that values are preserved
	if unmarshaled.DefaultProvider != config.DefaultProvider {
		t.Errorf("DefaultProvider not preserved: got %s, want %s", unmarshaled.DefaultProvider, config.DefaultProvider)
	}

	if unmarshaled.RetryDelay != config.RetryDelay {
		t.Errorf("RetryDelay not preserved: got %v, want %v", unmarshaled.RetryDelay, config.RetryDelay)
	}

	if unmarshaled.Timeout != config.Timeout {
		t.Errorf("Timeout not preserved: got %v, want %v", unmarshaled.Timeout, config.Timeout)
	}
}

func TestGetDefaultConfigPath(t *testing.T) {
	path := GetDefaultConfigPath()

	if path == "" {
		t.Error("GetDefaultConfigPath() returned empty string")
	}

	// Should contain some expected components
	if !contains(path, "gh-star-search") {
		t.Errorf("Expected path to contain 'gh-star-search', got: %s", path)
	}
}
