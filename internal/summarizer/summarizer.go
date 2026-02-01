package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/KyleKing/gh-star-search/internal/python"
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
	uvPath     string
	projectDir string
	timeout    time.Duration
}

// New creates a new Summarizer instance
func New(uvPath, projectDir string) *Summarizer {
	return &Summarizer{
		uvPath:     uvPath,
		projectDir: projectDir,
		timeout:    30 * time.Second,
	}
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
	cmd := python.RunScript(
		ctx,
		s.uvPath,
		s.projectDir,
		"summarize.py",
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

// IsAvailable checks if the summarizer is ready to use
func (s *Summarizer) IsAvailable() bool {
	return s.uvPath != "" && s.projectDir != ""
}

// GetCapabilities returns information about available summarization methods
func (s *Summarizer) GetCapabilities(_ context.Context) map[string]bool {
	return map[string]bool{
		"heuristic":    true,
		"transformers": true,
	}
}
