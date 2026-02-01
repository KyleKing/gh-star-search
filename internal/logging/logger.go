package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/kyleking/gh-star-search/internal/config"
)

const (
	// File permissions for log directories and files
	logDirPerm  = 0o755
	logFilePerm = 0o644
)

// parseLogLevel parses a string log level into slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// SetupLogger initializes the global slog logger with the given configuration
func SetupLogger(cfg config.LoggingConfig) error {
	// Set up output writer
	var writer io.Writer

	var file *os.File

	switch strings.ToLower(cfg.Output) {
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	case "file":
		if cfg.File == "" {
			return errors.New("log file path is required when output is 'file'")
		}

		// Ensure log directory exists
		if err := os.MkdirAll(filepath.Dir(cfg.File), logDirPerm); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		var err error

		file, err = os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFilePerm)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		writer = file
	default:
		return fmt.Errorf("invalid log output: %s", cfg.Output)
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     parseLogLevel(cfg.Level),
		AddSource: cfg.AddSource || cfg.Level == "debug",
	}

	// Create handler based on format
	var handler slog.Handler

	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	// Set as default logger
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Close file if it was opened (will be closed when program exits)
	if file != nil {
		// Note: In a real application, you might want to keep track of the file
		// and close it properly on shutdown
		_ = file
	}

	return nil
}

// SetupFallbackLogger sets up a basic logger for cases where configuration fails
func SetupFallbackLogger() {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// LoggerMiddleware provides a way to wrap functions with logging using slog
func LoggerMiddleware(ctx context.Context, operation string, fn func(context.Context) error) error {
	logger := slog.With("operation", operation)
	logger.DebugContext(ctx, "Starting operation")

	err := fn(ctx)

	if err != nil {
		logger.ErrorContext(ctx, "Operation failed", "error", err)
	} else {
		logger.DebugContext(ctx, "Operation completed successfully")
	}

	return err
}
