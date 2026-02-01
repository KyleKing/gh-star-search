package errors

import (
	"errors"
	"fmt"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrTypeGitHubAPI   ErrorType = "github_api"
	ErrTypeDatabase    ErrorType = "database"
	ErrTypeValidation  ErrorType = "validation"
	ErrTypeRateLimit   ErrorType = "rate_limit"
	ErrTypeNotFound    ErrorType = "not_found"
	ErrTypeConfig      ErrorType = "config"
	ErrTypeNetwork     ErrorType = "network"
	ErrTypeAuth        ErrorType = "auth"
	ErrTypeFileSystem  ErrorType = "filesystem"
	ErrTypeInternal    ErrorType = "internal"
)

// Error represents a structured error with type and optional suggestions
type Error struct {
	Type        ErrorType
	Message     string
	Cause       error
	Suggestions []string
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}

	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// WithSuggestion adds a suggestion for resolving the error
func (e *Error) WithSuggestion(suggestion string) *Error {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// New creates a new structured error
func New(errType ErrorType, message string) *Error {
	return &Error{
		Type:    errType,
		Message: message,
	}
}

// Newf creates a new structured error with formatted message
func Newf(errType ErrorType, format string, args ...interface{}) *Error {
	return &Error{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errType ErrorType, message string) *Error {
	return &Error{
		Type:    errType,
		Message: message,
		Cause:   err,
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, errType ErrorType, format string, args ...interface{}) *Error {
	return &Error{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Cause:   err,
	}
}

// IsType checks if an error is of a specific type
func IsType(err error, errType ErrorType) bool {
	var structErr *Error
	if errors.As(err, &structErr) {
		return structErr.Type == errType
	}

	return false
}

// GetType returns the error type if it's a structured error
func GetType(err error) ErrorType {
	var structErr *Error
	if errors.As(err, &structErr) {
		return structErr.Type
	}

	return ErrTypeInternal
}

// NewConfigError creates a configuration error with suggestions
func NewConfigError(message, field string) *Error {
	err := New(ErrTypeConfig, message)
	if field != "" {
		err.Message = fmt.Sprintf("%s (field: %s)", message, field)
	}

	return err.
		WithSuggestion("Check your configuration file syntax").
		WithSuggestion("Run with --help to see valid configuration options")
}
