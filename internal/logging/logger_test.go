package logging

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyleking/gh-star-search/internal/config"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", DebugLevel},
		{"DEBUG", DebugLevel},
		{"info", InfoLevel},
		{"INFO", InfoLevel},
		{"warn", WarnLevel},
		{"WARN", WarnLevel},
		{"warning", WarnLevel},
		{"error", ErrorLevel},
		{"ERROR", ErrorLevel},
		{"invalid", InfoLevel}, // default
		{"", InfoLevel},        // default
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{LogLevel(999), "UNKNOWN"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestNewLoggerStdout(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	}
	
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
	
	assert.Equal(t, InfoLevel, logger.level)
	assert.Equal(t, "text", logger.format)
	assert.Equal(t, os.Stdout, logger.output)
	assert.Nil(t, logger.file)
}

func TestNewLoggerStderr(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stderr",
	}
	
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
	
	assert.Equal(t, DebugLevel, logger.level)
	assert.Equal(t, "json", logger.format)
	assert.Equal(t, os.Stderr, logger.output)
	assert.True(t, logger.showCaller)
}

func TestNewLoggerFile(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")
	
	cfg := config.LoggingConfig{
		Level:  "warn",
		Format: "text",
		Output: "file",
		File:   logFile,
	}
	
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
	
	assert.Equal(t, WarnLevel, logger.level)
	assert.Equal(t, "text", logger.format)
	assert.NotNil(t, logger.file)
	
	// Clean up
	logger.Close()
}

func TestNewLoggerFileInvalidPath(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "file",
		File:   "",
	}
	
	logger, err := NewLogger(cfg)
	assert.Error(t, err)
	assert.Nil(t, logger)
	assert.Contains(t, err.Error(), "log file path is required")
}

func TestNewLoggerInvalidOutput(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "invalid",
	}
	
	logger, err := NewLogger(cfg)
	assert.Error(t, err)
	assert.Nil(t, logger)
	assert.Contains(t, err.Error(), "invalid log output")
}

func TestLoggerWithField(t *testing.T) {
	var buf bytes.Buffer
	
	logger := &Logger{
		level:  InfoLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
	newLogger := logger.WithField("key", "value")
	newLogger.Info("test message")
	
	var entry LogEntry
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	
	assert.Equal(t, "test message", entry.Message)
	assert.Equal(t, "value", entry.Fields["key"])
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	
	logger := &Logger{
		level:  InfoLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}
	
	newLogger := logger.WithFields(fields)
	newLogger.Info("test message")
	
	var entry LogEntry
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	
	assert.Equal(t, "test message", entry.Message)
	assert.Equal(t, "value1", entry.Fields["key1"])
	assert.Equal(t, float64(42), entry.Fields["key2"]) // JSON unmarshals numbers as float64
	assert.Equal(t, true, entry.Fields["key3"])
}

func TestLoggerWithError(t *testing.T) {
	var buf bytes.Buffer
	
	logger := &Logger{
		level:  InfoLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
	testErr := assert.AnError
	newLogger := logger.WithError(testErr)
	newLogger.Info("test message")
	
	var entry LogEntry
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	
	assert.Equal(t, "test message", entry.Message)
	assert.Equal(t, testErr.Error(), entry.Fields["error"])
}

func TestLoggerWithErrorNil(t *testing.T) {
	var buf bytes.Buffer
	
	logger := &Logger{
		level:  InfoLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
	newLogger := logger.WithError(nil)
	assert.Equal(t, logger, newLogger) // Should return same logger when error is nil
}

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	
	logger := &Logger{
		level:  WarnLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
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
	
	// Check warn message
	var warnEntry LogEntry
	err := json.Unmarshal([]byte(lines[0]), &warnEntry)
	require.NoError(t, err)
	assert.Equal(t, "WARN", warnEntry.Level)
	assert.Equal(t, "warn message", warnEntry.Message)
	
	// Check error message
	var errorEntry LogEntry
	err = json.Unmarshal([]byte(lines[1]), &errorEntry)
	require.NoError(t, err)
	assert.Equal(t, "ERROR", errorEntry.Level)
	assert.Equal(t, "error message", errorEntry.Message)
}

func TestLoggerFormattedMessages(t *testing.T) {
	var buf bytes.Buffer
	
	logger := &Logger{
		level:  InfoLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
	logger.Infof("formatted message: %s %d", "test", 42)
	
	var entry LogEntry
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	
	assert.Equal(t, "formatted message: test 42", entry.Message)
}

func TestLoggerErrorWithErr(t *testing.T) {
	var buf bytes.Buffer
	
	logger := &Logger{
		level:  InfoLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
	testErr := assert.AnError
	logger.ErrorWithErr("operation failed", testErr)
	
	var entry LogEntry
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	
	assert.Equal(t, "operation failed", entry.Message)
	assert.Equal(t, testErr.Error(), entry.Error)
}

func TestLoggerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	
	logger := &Logger{
		level:      InfoLevel,
		format:     "text",
		output:     &buf,
		fields:     map[string]interface{}{"key": "value"},
		showCaller: false,
	}
	
	logger.Info("test message")
	
	output := buf.String()
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key=value")
}

func TestLoggerTextFormatWithCaller(t *testing.T) {
	var buf bytes.Buffer
	
	logger := &Logger{
		level:      InfoLevel,
		format:     "text",
		output:     &buf,
		fields:     make(map[string]interface{}),
		showCaller: true,
	}
	
	logger.Info("test message")
	
	output := buf.String()
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "logger_test.go:")
}

func TestLoggerClose(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")
	
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "file",
		File:   logFile,
	}
	
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	
	logger.Info("test message")
	
	err = logger.Close()
	assert.NoError(t, err)
	
	// Verify file was written
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test message")
}

func TestInitializeLogger(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stderr",
	}
	
	err := InitializeLogger(cfg)
	assert.NoError(t, err)
	
	// Test that global functions work
	Info("test global info")
	Debug("test global debug")
	
	// Clean up
	if logger := GetLogger(); logger != nil {
		logger.Close()
	}
}

func TestGlobalLoggingFunctions(t *testing.T) {
	var buf bytes.Buffer
	
	// Set up global logger
	globalLogger = &Logger{
		level:  InfoLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
	Info("info message")
	Warn("warn message")
	Error("error message")
	
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 3)
	
	// Verify each message
	for i, expectedLevel := range []string{"INFO", "WARN", "ERROR"} {
		var entry LogEntry
		err := json.Unmarshal([]byte(lines[i]), &entry)
		require.NoError(t, err)
		assert.Equal(t, expectedLevel, entry.Level)
	}
}

func TestLoggerMiddleware(t *testing.T) {
	var buf bytes.Buffer
	
	globalLogger = &Logger{
		level:  DebugLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
	// Test successful operation
	err := LoggerMiddleware("test_operation", func() error {
		time.Sleep(1 * time.Millisecond) // Small delay to test duration
		return nil
	})
	
	assert.NoError(t, err)
	
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 2) // Start and completion messages
	
	// Check start message
	var startEntry LogEntry
	err = json.Unmarshal([]byte(lines[0]), &startEntry)
	require.NoError(t, err)
	assert.Equal(t, "DEBUG", startEntry.Level)
	assert.Contains(t, startEntry.Message, "Starting operation")
	assert.Equal(t, "test_operation", startEntry.Fields["operation"])
	
	// Check completion message
	var endEntry LogEntry
	err = json.Unmarshal([]byte(lines[1]), &endEntry)
	require.NoError(t, err)
	assert.Equal(t, "DEBUG", endEntry.Level)
	assert.Contains(t, endEntry.Message, "Operation completed successfully")
	assert.NotNil(t, endEntry.Fields["duration"])
}

func TestLoggerMiddlewareWithError(t *testing.T) {
	var buf bytes.Buffer
	
	globalLogger = &Logger{
		level:  DebugLevel,
		format: "json",
		output: &buf,
		fields: make(map[string]interface{}),
	}
	
	testErr := assert.AnError
	
	// Test failed operation
	err := LoggerMiddleware("test_operation", func() error {
		return testErr
	})
	
	assert.Equal(t, testErr, err)
	
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 2) // Start and error messages
	
	// Check error message
	var errorEntry LogEntry
	err = json.Unmarshal([]byte(lines[1]), &errorEntry)
	require.NoError(t, err)
	assert.Equal(t, "ERROR", errorEntry.Level)
	assert.Contains(t, errorEntry.Message, "Operation failed")
	assert.Equal(t, testErr.Error(), errorEntry.Error)
}