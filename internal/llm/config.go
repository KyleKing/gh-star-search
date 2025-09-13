package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ConfigManager handles loading and saving LLM configurations
type ConfigManager struct {
	configPath string
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath: configPath,
	}
}

// LoadConfig loads configuration from file or returns default
func (cm *ConfigManager) LoadConfig() (*ManagerConfig, error) {
	// Check if config file exists
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		defaultConfig := DefaultManagerConfig()
		return &defaultConfig, nil
	}

	// Read config file
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config ManagerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to file
func (cm *ConfigManager) SaveConfig(config *ManagerConfig) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".gh-star-search-llm.json"
	}
	return filepath.Join(homeDir, ".config", "gh-star-search", "llm.json")
}

// SetupDefaultProviders sets up common LLM providers with default configurations
func SetupDefaultProviders(manager *Manager) error {
	// Register OpenAI provider
	openAIClient := NewClient(Config{})
	if err := manager.RegisterProvider(ProviderOpenAI, openAIClient); err != nil {
		return fmt.Errorf("failed to register OpenAI provider: %w", err)
	}

	// Register Anthropic provider
	anthropicClient := NewClient(Config{})
	if err := manager.RegisterProvider(ProviderAnthropic, anthropicClient); err != nil {
		return fmt.Errorf("failed to register Anthropic provider: %w", err)
	}

	// Register Ollama provider
	ollamaClient := NewClient(Config{})
	if err := manager.RegisterProvider(ProviderOllama, ollamaClient); err != nil {
		return fmt.Errorf("failed to register Ollama provider: %w", err)
	}

	return nil
}

// ConfigureFromEnvironment configures providers from environment variables
func ConfigureFromEnvironment(manager *Manager) error {
	// Configure OpenAI if API key is available
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		config := Config{
			Provider: ProviderOpenAI,
			Model:    ModelGPT35Turbo,
			APIKey:   apiKey,
		}
		if err := manager.Configure(config); err != nil {
			return fmt.Errorf("failed to configure OpenAI: %w", err)
		}
	}

	// Configure Anthropic if API key is available
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		config := Config{
			Provider: ProviderAnthropic,
			Model:    ModelClaude3,
			APIKey:   apiKey,
		}
		if err := manager.Configure(config); err != nil {
			return fmt.Errorf("failed to configure Anthropic: %w", err)
		}
	}

	// Configure Ollama if base URL is available
	if baseURL := os.Getenv("OLLAMA_BASE_URL"); baseURL != "" {
		model := os.Getenv("OLLAMA_MODEL")
		if model == "" {
			model = ModelLlama2
		}

		config := Config{
			Provider: ProviderOllama,
			Model:    model,
			BaseURL:  baseURL,
		}
		if err := manager.Configure(config); err != nil {
			return fmt.Errorf("failed to configure Ollama: %w", err)
		}
	}

	return nil
}

// CreateManagerWithDefaults creates a manager with default providers and configuration
func CreateManagerWithDefaults() (*Manager, error) {
	// Load configuration
	configManager := NewConfigManager(GetDefaultConfigPath())
	config, err := configManager.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create manager
	manager := NewManager(*config)

	// Setup default providers
	if err := SetupDefaultProviders(manager); err != nil {
		return nil, fmt.Errorf("failed to setup providers: %w", err)
	}

	// Configure from environment
	if err := ConfigureFromEnvironment(manager); err != nil {
		return nil, fmt.Errorf("failed to configure from environment: %w", err)
	}

	return manager, nil
}

// MarshalJSON implements json.Marshaler for ManagerConfig
func (mc ManagerConfig) MarshalJSON() ([]byte, error) {
	type Alias ManagerConfig
	return json.Marshal(&struct {
		RetryDelay string `json:"retry_delay"`
		Timeout    string `json:"timeout"`
		*Alias
	}{
		RetryDelay: mc.RetryDelay.String(),
		Timeout:    mc.Timeout.String(),
		Alias:      (*Alias)(&mc),
	})
}

// UnmarshalJSON implements json.Unmarshaler for ManagerConfig
func (mc *ManagerConfig) UnmarshalJSON(data []byte) error {
	type Alias ManagerConfig
	aux := &struct {
		RetryDelay string `json:"retry_delay"`
		Timeout    string `json:"timeout"`
		*Alias
	}{
		Alias: (*Alias)(mc),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse duration strings
	if aux.RetryDelay != "" {
		duration, err := time.ParseDuration(aux.RetryDelay)
		if err != nil {
			return fmt.Errorf("invalid retry_delay: %w", err)
		}
		mc.RetryDelay = duration
	}

	if aux.Timeout != "" {
		duration, err := time.ParseDuration(aux.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
		mc.Timeout = duration
	}

	return nil
}
