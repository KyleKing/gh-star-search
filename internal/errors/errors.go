package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrTypeGitHubAPI ErrorType = "github_api"

	ErrTypeDatabase   ErrorType = "database"
	ErrTypeValidation ErrorType = "validation"
	ErrTypeRateLimit  ErrorType = "rate_limit"
	ErrTypeNotFound   ErrorType = "not_found"
	ErrTypeConfig     ErrorType = "config"
	ErrTypeNetwork    ErrorType = "network"
	ErrTypeAuth       ErrorType = "auth"
	ErrTypeFileSystem ErrorType = "filesystem"
	ErrTypeInternal   ErrorType = "internal"
)

// Error represents a structured error with context
type Error struct {
	Type        ErrorType              `json:"type"`
	Message     string                 `json:"message"`
	Code        string                 `json:"code"`
	Cause       error                  `json:"-"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Stack       string                 `json:"stack,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}

	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// WithContext adds context information to the error
func (e *Error) WithContext(key string, value interface{}) *Error {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}

	e.Context[key] = value

	return e
}

// WithSuggestion adds a suggestion for resolving the error
func (e *Error) WithSuggestion(suggestion string) *Error {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// WithStack captures the current stack trace
func (e *Error) WithStack() *Error {
	e.Stack = captureStack()
	return e
}

// New creates a new structured error
func New(errType ErrorType, message string) *Error {
	return &Error{
		Type:    errType,
		Message: message,
		Stack:   captureStack(),
	}
}

// Newf creates a new structured error with formatted message
func Newf(errType ErrorType, format string, args ...interface{}) *Error {
	return &Error{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Stack:   captureStack(),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errType ErrorType, message string) *Error {
	return &Error{
		Type:    errType,
		Message: message,
		Cause:   err,
		Stack:   captureStack(),
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, errType ErrorType, format string, args ...interface{}) *Error {
	return &Error{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Cause:   err,
		Stack:   captureStack(),
	}
}

// captureStack captures the current stack trace
func captureStack() string {
	const depth = 32

	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])

	var sb strings.Builder

	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "gh-star-search") {
			if !more {
				break
			}

			continue
		}

		sb.WriteString(fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function))

		if !more {
			break
		}
	}

	return sb.String()
}

// IsType checks if an error is of a specific type
func IsType(err error, errType ErrorType) bool {
	if structErr, ok := err.(*Error); ok {
		return structErr.Type == errType
	}

	return false
}

// GetType returns the error type if it's a structured error
func GetType(err error) ErrorType {
	if structErr, ok := err.(*Error); ok {
		return structErr.Type
	}

	return ErrTypeInternal
}

// Common error constructors for frequently used errors

// NewGitHubAPIError creates a GitHub API error
func NewGitHubAPIError(message string, statusCode int) *Error {
	return New(ErrTypeGitHubAPI, message).
		WithContext("status_code", statusCode).
		WithSuggestion("Check your GitHub authentication with 'gh auth status'").
		WithSuggestion("Verify your internet connection")
}

// NewDatabaseError creates a database error
func NewDatabaseError(message string, operation string) *Error {
	return New(ErrTypeDatabase, message).
		WithContext("operation", operation).
		WithSuggestion("Check database file permissions").
		WithSuggestion("Ensure sufficient disk space")
}

// NewConfigError creates a configuration error
func NewConfigError(message string, field string) *Error {
	return New(ErrTypeConfig, message).
		WithContext("field", field).
		WithSuggestion("Check your configuration file syntax").
		WithSuggestion("Run with --help to see valid configuration options")
}

// NewValidationError creates a validation error
func NewValidationError(message string, value interface{}) *Error {
	return New(ErrTypeValidation, message).
		WithContext("value", value).
		WithSuggestion("Check the input format and try again")
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(message string, resetTime string) *Error {
	return New(ErrTypeRateLimit, message).
		WithContext("reset_time", resetTime).
		WithSuggestion("Wait for the rate limit to reset").
		WithSuggestion("Consider using a GitHub token with higher rate limits")
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string, identifier string) *Error {
	return New(ErrTypeNotFound, fmt.Sprintf("%s not found: %s", resource, identifier)).
		WithContext("resource", resource).
		WithContext("identifier", identifier).
		WithSuggestion("Check that the resource exists and you have access to it")
}

// NewAuthError creates an authentication error
func NewAuthError(message string) *Error {
	return New(ErrTypeAuth, message).
		WithSuggestion("Run 'gh auth login' to authenticate with GitHub").
		WithSuggestion("Check that your GitHub token has the required permissions")
}

// NewNetworkError creates a network error
func NewNetworkError(message string, url string) *Error {
	return New(ErrTypeNetwork, message).
		WithContext("url", url).
		WithSuggestion("Check your internet connection").
		WithSuggestion("Verify that the service is accessible")
}

// NewFileSystemError creates a filesystem error
func NewFileSystemError(message string, path string) *Error {
	return New(ErrTypeFileSystem, message).
		WithContext("path", path).
		WithSuggestion("Check file/directory permissions").
		WithSuggestion("Ensure the path exists and is accessible")
}
