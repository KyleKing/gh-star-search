package logging

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kyleking/gh-star-search/internal/config"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

const (
	// File permissions for log directories and files
	logDirPerm  = 0755
	logFilePerm = 0644

	// Magic numbers
	zeroFields  = 0
	callerSkip  = 3
	emptyString = ""
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Caller    string                 `json:"caller,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Logger provides structured logging capabilities
type Logger struct {
	level      LogLevel
	format     string
	output     io.Writer
	file       *os.File
	mu         sync.Mutex
	fields     map[string]interface{}
	showCaller bool
}

// Global logger instance
var globalLogger *Logger
var loggerOnce sync.Once

// InitializeLogger initializes the global logger with the given configuration
func InitializeLogger(cfg config.LoggingConfig) error {
	var err error

	loggerOnce.Do(func() {
		globalLogger, err = NewLogger(cfg)
	})

	return err
}

// NewLogger creates a new logger with the given configuration
func NewLogger(cfg config.LoggingConfig) (*Logger, error) {
	logger := &Logger{
		level:      parseLogLevel(cfg.Level),
		format:     cfg.Format,
		fields:     make(map[string]interface{}),
		showCaller: cfg.Level == "debug",
	}

	// Set up output
	switch strings.ToLower(cfg.Output) {
	case "stdout":
		logger.output = os.Stdout
	case "stderr":
		logger.output = os.Stderr
	case "file":
		if cfg.File == "" {
			return nil, errors.New("log file path is required when output is 'file'")
		}

		// Ensure log directory exists
		if err := os.MkdirAll(filepath.Dir(cfg.File), logDirPerm); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFilePerm)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		logger.file = file
		logger.output = file
	default:
		return nil, fmt.Errorf("invalid log output: %s", cfg.Output)
	}

	return logger, nil
}

// parseLogLevel parses a string log level into LogLevel
func parseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn", "warning":
		return WarnLevel
	case "error":
		return ErrorLevel
	default:
		return InfoLevel
	}
}

// WithField adds a field to the logger context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:      l.level,
		format:     l.format,
		output:     l.output,
		file:       l.file,
		fields:     make(map[string]interface{}),
		showCaller: l.showCaller,
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new field
	newLogger.fields[key] = value

	return newLogger
}

// WithFields adds multiple fields to the logger context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:      l.level,
		format:     l.format,
		output:     l.output,
		file:       l.file,
		fields:     make(map[string]interface{}),
		showCaller: l.showCaller,
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithError adds an error to the logger context
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}

	return l.WithField("error", err.Error())
}

// log writes a log entry at the specified level
func (l *Logger) log(level LogLevel, message string, err error) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level.String(),
		Message:   message,
		Fields:    l.fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	if l.showCaller {
		entry.Caller = getCaller()
	}

	var output string

	if l.format == "json" {
		data, _ := json.Marshal(entry)
		output = string(data)
	} else {
		output = l.formatText(entry)
	}

	_, _ = fmt.Fprintln(l.output, output)
}

// formatText formats a log entry as human-readable text
func (_ *Logger) formatText(entry LogEntry) string {
	var parts []string

	// Timestamp and level
	parts = append(parts, fmt.Sprintf("[%s] %s", entry.Timestamp, entry.Level))

	// Caller information
	if entry.Caller != "" {
		parts = append(parts, fmt.Sprintf("(%s)", entry.Caller))
	}

	// Message
	parts = append(parts, entry.Message)

	// Fields
	if len(entry.Fields) > zeroFields {
		var fieldParts []string
		for k, v := range entry.Fields {
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%v", k, v))
		}

		parts = append(parts, fmt.Sprintf("{%s}", strings.Join(fieldParts, " ")))
	}

	// Error
	if entry.Error != emptyString {
		parts = append(parts, "error="+entry.Error)
	}

	return strings.Join(parts, " ")
}

// getCaller returns information about the calling function
func getCaller() string {
	_, file, line, ok := runtime.Caller(callerSkip)
	if !ok {
		return "unknown"
	}

	// Get just the filename, not the full path
	filename := filepath.Base(file)

	return fmt.Sprintf("%s:%d", filename, line)
}

// Debug logs a debug message
func (l *Logger) Debug(message string) {
	l.log(DebugLevel, message, nil)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

// Info logs an info message
func (l *Logger) Info(message string) {
	l.log(InfoLevel, message, nil)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

// Warn logs a warning message
func (l *Logger) Warn(message string) {
	l.log(WarnLevel, message, nil)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

// Error logs an error message
func (l *Logger) Error(message string) {
	l.log(ErrorLevel, message, nil)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

// ErrorWithErr logs an error message with an associated error
func (l *Logger) ErrorWithErr(message string, err error) {
	l.log(ErrorLevel, message, err)
}

// Close closes the logger and any associated resources
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}

	return nil
}

// Global logging functions that use the global logger

// Debug logs a debug message using the global logger
func Debug(message string) {
	if globalLogger != nil {
		globalLogger.Debug(message)
	}
}

// Debugf logs a formatted debug message using the global logger
func Debugf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debugf(format, args...)
	}
}

// Info logs an info message using the global logger
func Info(message string) {
	if globalLogger != nil {
		globalLogger.Info(message)
	}
}

// Infof logs a formatted info message using the global logger
func Infof(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Infof(format, args...)
	}
}

// Warn logs a warning message using the global logger
func Warn(message string) {
	if globalLogger != nil {
		globalLogger.Warn(message)
	}
}

// Warnf logs a formatted warning message using the global logger
func Warnf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warnf(format, args...)
	}
}

// Error logs an error message using the global logger
func Error(message string) {
	if globalLogger != nil {
		globalLogger.Error(message)
	}
}

// Errorf logs a formatted error message using the global logger
func Errorf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Errorf(format, args...)
	}
}

// ErrorWithErr logs an error message with an associated error using the global logger
func ErrorWithErr(message string, err error) {
	if globalLogger != nil {
		globalLogger.ErrorWithErr(message, err)
	}
}

// WithField adds a field to the global logger context
func WithField(key string, value interface{}) *Logger {
	if globalLogger != nil {
		return globalLogger.WithField(key, value)
	}

	return nil
}

// WithFields adds multiple fields to the global logger context
func WithFields(fields map[string]interface{}) *Logger {
	if globalLogger != nil {
		return globalLogger.WithFields(fields)
	}

	return nil
}

// WithError adds an error to the global logger context
func WithError(err error) *Logger {
	if globalLogger != nil {
		return globalLogger.WithError(err)
	}

	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	return globalLogger
}

// SetupFallbackLogger sets up a basic logger for cases where configuration fails
func SetupFallbackLogger() {
	globalLogger = &Logger{
		level:      InfoLevel,
		format:     "text",
		output:     os.Stderr,
		fields:     make(map[string]interface{}),
		showCaller: false,
	}
}

// LoggerMiddleware provides a way to wrap functions with logging
func LoggerMiddleware(operation string, fn func() error) error {
	logger := WithField("operation", operation)
	logger.Debug("Starting operation")

	start := time.Now()
	err := fn()
	duration := time.Since(start)

	if err != nil {
		logger.WithField("duration", duration).ErrorWithErr("Operation failed", err)
	} else {
		logger.WithField("duration", duration).Debug("Operation completed successfully")
	}

	return err
}
