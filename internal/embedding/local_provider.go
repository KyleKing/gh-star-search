package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LocalProviderImpl implements local embedding generation using Python
type LocalProviderImpl struct {
	config     Config
	pythonPath string
	scriptPath string
	timeout    time.Duration
	dimensions int
}

// EmbeddingResult represents the result from the Python embedding script
type EmbeddingResult struct {
	Success     bool      `json:"success"`
	Embedding   []float32 `json:"embedding"`
	Dimensions  int       `json:"dimensions"`
	Model       string    `json:"model"`
	InputLength int       `json:"input_length"`
	Error       string    `json:"error,omitempty"`
}

// NewLocalProviderImpl creates a new local embedding provider
func NewLocalProviderImpl(cfg Config) (*LocalProviderImpl, error) {
	// Find Python executable
	pythonPath, err := findPythonExecutable()
	if err != nil {
		return nil, fmt.Errorf("failed to find Python: %w", err)
	}

	// Determine script path
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("failed to determine project root")
	}

	projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..")
	scriptPath := filepath.Join(projectRoot, "scripts", "embed.py")

	timeout := 30 * time.Second
	if val, ok := cfg.Options["timeout"]; ok {
		if d, err := time.ParseDuration(val); err == nil {
			timeout = d
		}
	}

	return &LocalProviderImpl{
		config:     cfg,
		pythonPath: pythonPath,
		scriptPath: scriptPath,
		timeout:    timeout,
		dimensions: cfg.Dimensions,
	}, nil
}

// GenerateEmbedding generates an embedding for the given text
func (p *LocalProviderImpl) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("empty input text")
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Prepare command
	cmd := exec.CommandContext(timeoutCtx, p.pythonPath, p.scriptPath, "--json")

	// Set up stdin with the text to embed
	cmd.Stdin = strings.NewReader(text)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()
	if err != nil {
		// Check if it was a timeout
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("embedding generation timed out after %v", p.timeout)
		}

		// Include stderr in error message
		stderrStr := stderr.String()
		if stderrStr != "" {
			return nil, fmt.Errorf("embedding generation failed: %w (stderr: %s)", err, stderrStr)
		}

		return nil, fmt.Errorf("embedding generation failed: %w", err)
	}

	// Parse JSON result
	var result EmbeddingResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse embedding result: %w (output: %s)", err, stdout.String())
	}

	if !result.Success {
		return nil, fmt.Errorf("embedding generation failed: %s", result.Error)
	}

	if len(result.Embedding) != p.dimensions {
		return nil, fmt.Errorf("dimension mismatch: expected %d, got %d", p.dimensions, len(result.Embedding))
	}

	return result.Embedding, nil
}

// GetDimensions returns the embedding dimensions
func (p *LocalProviderImpl) GetDimensions() int {
	return p.dimensions
}

// IsEnabled checks if the provider is enabled and ready
func (p *LocalProviderImpl) IsEnabled() bool {
	// Check if Python is available
	if p.pythonPath == "" {
		return false
	}

	// Check if script exists
	// Note: We don't check for sentence-transformers here to avoid
	// startup delays. The error will be caught when GenerateEmbedding is called.
	return true
}

// GetName returns the provider name
func (p *LocalProviderImpl) GetName() string {
	return "local:" + p.config.Model
}

// findPythonExecutable attempts to find a Python 3 executable
func findPythonExecutable() (string, error) {
	candidates := []string{"python3", "python"}

	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			// Verify it's Python 3
			cmd := exec.Command(path, "--version")
			output, err := cmd.CombinedOutput()
			if err == nil {
				version := string(output)
				if strings.Contains(version, "Python 3") {
					return path, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no suitable Python 3 executable found in PATH")
}
