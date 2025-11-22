package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// LocalProvider implements embedding generation using local Python script
type LocalProvider struct {
	config     Config
	pythonPath string
	scriptPath string
	timeout    time.Duration
	dimensions int
}

// embeddingResult represents the JSON response from embed.py
type embeddingResult struct {
	Embeddings [][]float64 `json:"embeddings"`
	Model      string      `json:"model"`
	Dimension  int         `json:"dimension"`
	Count      int         `json:"count"`
}

// NewLocalProvider creates a new local embedding provider
func NewLocalProvider(config Config) (*LocalProvider, error) {
	// Find Python executable
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		// Try python as fallback
		pythonPath, err = exec.LookPath("python")
		if err != nil {
			return nil, fmt.Errorf("python not found in PATH: %w", err)
		}
	}

	// Find script path
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("failed to determine script path")
	}

	// Go up from internal/embedding/ to project root, then to scripts/
	projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..")
	scriptPath := filepath.Join(projectRoot, "scripts", "embed.py")

	provider := &LocalProvider{
		config:     config,
		pythonPath: pythonPath,
		scriptPath: scriptPath,
		timeout:    60 * time.Second, // Embeddings can take longer than summarization
		dimensions: config.Dimensions,
	}

	// Verify the provider is available
	if !provider.IsEnabled() {
		return nil, fmt.Errorf("local embedding provider not available: Python or script not found")
	}

	return provider, nil
}

// GenerateEmbedding generates an embedding for the given text
func (p *LocalProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return make([]float32, p.dimensions), nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Prepare input as JSON array
	input := []string{text}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Build command
	cmd := exec.CommandContext(
		ctx,
		p.pythonPath,
		p.scriptPath,
		"--model", p.config.Model,
		"--stdin",
	)

	// Set input
	cmd.Stdin = bytes.NewReader(inputJSON)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err = cmd.Run()
	if err != nil {
		// Check if it's a context timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("embedding generation timeout after %v", p.timeout)
		}

		// Return error with stderr for debugging
		return nil, fmt.Errorf("embedding generation failed: %w (stderr: %s)", err, stderr.String())
	}

	// Parse JSON output
	var result embeddingResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse embedding result: %w", err)
	}

	// Validate result
	if len(result.Embeddings) != 1 {
		return nil, fmt.Errorf("expected 1 embedding, got %d", len(result.Embeddings))
	}

	if result.Dimension != p.dimensions {
		return nil, fmt.Errorf("dimension mismatch: expected %d, got %d", p.dimensions, result.Dimension)
	}

	// Convert float64 to float32
	embedding := make([]float32, len(result.Embeddings[0]))
	for i, v := range result.Embeddings[0] {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// GenerateEmbeddings generates embeddings for multiple texts (batch operation)
func (p *LocalProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Prepare input as JSON array
	inputJSON, err := json.Marshal(texts)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Build command
	cmd := exec.CommandContext(
		ctx,
		p.pythonPath,
		p.scriptPath,
		"--model", p.config.Model,
		"--stdin",
	)

	// Set input
	cmd.Stdin = bytes.NewReader(inputJSON)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err = cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("embedding generation timeout after %v", p.timeout)
		}
		return nil, fmt.Errorf("embedding generation failed: %w (stderr: %s)", err, stderr.String())
	}

	// Parse JSON output
	var result embeddingResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse embedding result: %w", err)
	}

	// Validate result
	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}

	// Convert float64 to float32
	embeddings := make([][]float32, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		embeddings[i] = make([]float32, len(emb))
		for j, v := range emb {
			embeddings[i][j] = float32(v)
		}
	}

	return embeddings, nil
}

// GetDimensions returns the dimensionality of embeddings produced by this provider
func (p *LocalProvider) GetDimensions() int {
	return p.dimensions
}

// IsEnabled returns whether the provider is enabled and ready to use
func (p *LocalProvider) IsEnabled() bool {
	// Check if Python is available
	cmd := exec.Command(p.pythonPath, "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	// Check if script exists and runs
	cmd = exec.Command(p.pythonPath, p.scriptPath, "--help")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// GetName returns the provider name for identification
func (p *LocalProvider) GetName() string {
	return fmt.Sprintf("local:%s", p.config.Model)
}
