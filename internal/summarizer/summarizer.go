package summarizer

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

// Method represents the summarization method used
type Method string

const (
	// MethodAuto automatically selects the best available method
	MethodAuto Method = "auto"
	// MethodHeuristic uses simple keyword extraction (no dependencies)
	MethodHeuristic Method = "heuristic"
	// MethodTransformers uses transformers library (requires Python packages)
	MethodTransformers Method = "transformers"
)

// Result represents a summarization result
type Result struct {
	Summary string `json:"summary"`
	Method  string `json:"method"`
	Error   string `json:"error,omitempty"`
}

// Summarizer handles text summarization
type Summarizer struct {
	pythonPath string
	scriptPath string
	timeout    time.Duration
}

// New creates a new Summarizer instance
func New() (*Summarizer, error) {
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

	// Go up from internal/summarizer/ to project root, then to scripts/
	projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..")
	scriptPath := filepath.Join(projectRoot, "scripts", "summarize.py")

	return &Summarizer{
		pythonPath: pythonPath,
		scriptPath: scriptPath,
		timeout:    30 * time.Second,
	}, nil
}

// Summarize summarizes the given text
func (s *Summarizer) Summarize(ctx context.Context, text string, method Method) (*Result, error) {
	if text == "" {
		return &Result{Summary: "", Method: "none"}, nil
	}

	// If text is very short, just return it
	if len(strings.TrimSpace(text)) < 100 {
		return &Result{Summary: strings.TrimSpace(text), Method: "passthrough"}, nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Build command
	cmd := exec.CommandContext(
		ctx,
		s.pythonPath,
		s.scriptPath,
		"--method", string(method),
		"--json",
	)

	// Set input
	cmd.Stdin = strings.NewReader(text)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()
	if err != nil {
		// Check if it's a context timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("summarization timeout after %v", s.timeout)
		}

		// Return error with stderr for debugging
		return nil, fmt.Errorf("summarization failed: %w (stderr: %s)", err, stderr.String())
	}

	// Parse JSON output
	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse summarization result: %w", err)
	}

	return &result, nil
}

// SummarizeSimple is a convenience method that uses auto method
func (s *Summarizer) SummarizeSimple(ctx context.Context, text string) (string, error) {
	result, err := s.Summarize(ctx, text, MethodAuto)
	if err != nil {
		return "", err
	}
	return result.Summary, nil
}

// IsAvailable checks if Python and the summarization script are available
func (s *Summarizer) IsAvailable() bool {
	// Check if Python is available
	cmd := exec.Command(s.pythonPath, "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	// Check if script exists
	cmd = exec.Command(s.pythonPath, s.scriptPath, "--help")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// GetCapabilities returns information about available summarization methods
func (s *Summarizer) GetCapabilities(ctx context.Context) (map[string]bool, error) {
	capabilities := map[string]bool{
		"heuristic":    true, // Always available in Python script
		"transformers": false,
	}

	// Test if transformers is available by trying to import it
	testCmd := exec.CommandContext(
		ctx,
		s.pythonPath,
		"-c",
		"import transformers; import torch",
	)

	if err := testCmd.Run(); err == nil {
		capabilities["transformers"] = true
	}

	return capabilities, nil
}
