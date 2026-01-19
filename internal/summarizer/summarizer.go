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

// Method represents the summarization method to use
type Method string

const (
	// MethodHeuristic uses simple keyword extraction (~10MB RAM)
	MethodHeuristic Method = "heuristic"
	// MethodTransformers uses AI-based summarization (~1.5GB RAM)
	MethodTransformers Method = "transformers"
)

// Result represents the result of a summarization operation
type Result struct {
	Success      bool   `json:"success"`
	Summary      string `json:"summary"`
	Method       string `json:"method"`
	InputLength  int    `json:"input_length,omitempty"`
	OutputLength int    `json:"output_length,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Summarizer handles repository content summarization
type Summarizer struct {
	pythonPath string
	scriptPath string
	timeout    time.Duration
}

// Config holds configuration for the summarizer
type Config struct {
	// PythonPath is the path to the Python executable
	// If empty, will try to find python3 or python in PATH
	PythonPath string

	// ScriptPath is the path to the summarize.py script
	// If empty, will default to scripts/summarize.py relative to project root
	ScriptPath string

	// Timeout is the maximum duration for summarization
	// Default: 30 seconds
	Timeout time.Duration
}

// New creates a new Summarizer with the given configuration
func New(cfg Config) (*Summarizer, error) {
	pythonPath := cfg.PythonPath
	if pythonPath == "" {
		// Try to find Python in PATH
		var err error
		pythonPath, err = findPython()
		if err != nil {
			return nil, fmt.Errorf("failed to find Python executable: %w", err)
		}
	}

	scriptPath := cfg.ScriptPath
	if scriptPath == "" {
		// Default to scripts/summarize.py relative to project root
		// Get current file's directory and go up to project root
		_, currentFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("failed to determine project root")
		}

		projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..")
		scriptPath = filepath.Join(projectRoot, "scripts", "summarize.py")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Summarizer{
		pythonPath: pythonPath,
		scriptPath: scriptPath,
		timeout:    timeout,
	}, nil
}

// Summarize generates a summary of the given text using the specified method
func (s *Summarizer) Summarize(ctx context.Context, text string, method Method) (*Result, error) {
	if text == "" {
		return &Result{
			Success: false,
			Error:   "empty input text",
		}, nil
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Prepare command
	cmd := exec.CommandContext(timeoutCtx, s.pythonPath, s.scriptPath,
		"--method", string(method),
		"--json")

	// Set up stdin with the text to summarize
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
			return nil, fmt.Errorf("summarization timed out after %v", s.timeout)
		}

		// Include stderr in error message
		stderrStr := stderr.String()
		if stderrStr != "" {
			return nil, fmt.Errorf("summarization failed: %w (stderr: %s)", err, stderrStr)
		}

		return nil, fmt.Errorf("summarization failed: %w", err)
	}

	// Parse JSON result
	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse summarization result: %w (output: %s)", err, stdout.String())
	}

	return &result, nil
}

// SummarizeWithFallback attempts to summarize using the preferred method,
// falling back to heuristic if transformers fails
func (s *Summarizer) SummarizeWithFallback(ctx context.Context, text string) (*Result, error) {
	// Try transformers first (if available)
	result, err := s.Summarize(ctx, text, MethodTransformers)
	if err != nil || !result.Success {
		// Fall back to heuristic
		result, err = s.Summarize(ctx, text, MethodHeuristic)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// findPython attempts to find a Python executable in the system PATH
func findPython() (string, error) {
	// Try python3 first (preferred)
	pythonCandidates := []string{"python3", "python"}

	for _, candidate := range pythonCandidates {
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
