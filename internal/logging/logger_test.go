package logging

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KyleKing/gh-star-search/internal/config"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"invalid", slog.LevelInfo}, // default
		{"", slog.LevelInfo},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetupLoggerStdout(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	}

	err := SetupLogger(cfg)
	require.NoError(t, err)

	// Test logging
	slog.Info("test message")
}

func TestSetupLoggerStderr(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stderr",
	}

	err := SetupLogger(cfg)
	require.NoError(t, err)

	// Test logging with fields
	slog.Info("test message", "key", "value")
}

func TestSetupLoggerFile(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	cfg := config.LoggingConfig{
		Level:  "warn",
		Format: "text",
		Output: "file",
		File:   logFile,
	}

	err := SetupLogger(cfg)
	require.NoError(t, err)

	// Test logging
	slog.Warn("test warning message")

	// Verify file was written
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test warning message")
}

func TestSetupLoggerFileInvalidPath(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "file",
		File:   "",
	}

	err := SetupLogger(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log file path is required")
}

func TestSetupLoggerInvalidOutput(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "invalid",
	}

	err := SetupLogger(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log output")
}

func TestSetupLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer

	// Create a logger that writes to our buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	// Test With()
	newLogger := logger.With("key", "value")
	newLogger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key=value")
}

func TestSetupLoggerLevels(t *testing.T) {
	var buf bytes.Buffer

	// Create logger with Warn level
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	logger := slog.New(handler)

	// These should not be logged (below threshold)
	logger.Debug("debug message")
	logger.Info("info message")

	// These should be logged
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should only have 2 lines (warn and error)
	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "warn message")
	assert.Contains(t, lines[1], "error message")
}

func TestSetupLoggerFormattedMessages(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	logger.Info("formatted message", "value1", "test", "value2", 42)

	output := buf.String()
	assert.Contains(t, output, "formatted message")
	assert.Contains(t, output, "value1=test")
	assert.Contains(t, output, "value2=42")
}

func TestSetupLoggerWithContext(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	ctx := context.Background()
	logger.InfoContext(ctx, "test message with context", "key", "value")

	output := buf.String()
	assert.Contains(t, output, "test message with context")
	assert.Contains(t, output, "key=value")
}

func TestSetupFallbackLogger(_ *testing.T) {
	SetupFallbackLogger()

	// Test that logging works
	slog.Info("fallback test message")
}

func TestLoggerMiddleware(t *testing.T) {
	var buf bytes.Buffer

	// Set up logger that writes to buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})
	slog.SetDefault(slog.New(handler))

	ctx := context.Background()

	// Test successful operation
	err := LoggerMiddleware(ctx, "test_operation", func(_ context.Context) error {
		return nil
	})

	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Starting operation")
	assert.Contains(t, output, "Operation completed successfully")
	assert.Contains(t, output, "test_operation")
}

func TestLoggerMiddlewareWithError(t *testing.T) {
	var buf bytes.Buffer

	// Set up logger that writes to buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))

	ctx := context.Background()
	testErr := assert.AnError

	// Test failed operation
	err := LoggerMiddleware(ctx, "test_operation", func(_ context.Context) error {
		return testErr
	})

	assert.Equal(t, testErr, err)

	output := buf.String()
	assert.Contains(t, output, "Starting operation")
	assert.Contains(t, output, "Operation failed")
	assert.Contains(t, output, testErr.Error())
}
