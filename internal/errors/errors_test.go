package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New(ErrTypeValidation, "test error message")

	assert.Equal(t, ErrTypeValidation, err.Type)
	assert.Equal(t, "test error message", err.Message)
	assert.NoError(t, err.Cause)
}

func TestNewf(t *testing.T) {
	err := Newf(ErrTypeDatabase, "failed to connect to %s", "database")

	assert.Equal(t, ErrTypeDatabase, err.Type)
	assert.Equal(t, "failed to connect to database", err.Message)
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := Wrap(originalErr, ErrTypeNetwork, "network operation failed")

	assert.Equal(t, ErrTypeNetwork, wrappedErr.Type)
	assert.Equal(t, "network operation failed", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
}

func TestWrapf(t *testing.T) {
	originalErr := errors.New("connection refused")
	wrappedErr := Wrapf(
		originalErr,
		ErrTypeNetwork,
		"failed to connect to %s:%d",
		"localhost",
		8080,
	)

	assert.Equal(t, ErrTypeNetwork, wrappedErr.Type)
	assert.Equal(t, "failed to connect to localhost:8080", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
}

func TestErrorString(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name: "error without cause",
			err: &Error{
				Type:    ErrTypeValidation,
				Message: "invalid input",
			},
			expected: "validation: invalid input",
		},
		{
			name: "error with cause",
			err: &Error{
				Type:    ErrTypeDatabase,
				Message: "query failed",
				Cause:   errors.New("connection timeout"),
			},
			expected: "database: query failed (caused by: connection timeout)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestUnwrap(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := Wrap(originalErr, ErrTypeNetwork, "wrapped error")

	assert.Equal(t, originalErr, wrappedErr.Unwrap())
}

func TestWithSuggestion(t *testing.T) {
	err := New(ErrTypeAuth, "authentication failed")
	err = err.WithSuggestion("Run 'gh auth login' to authenticate")
	err = err.WithSuggestion("Check your GitHub token permissions")

	assert.Len(t, err.Suggestions, 2)
	assert.Contains(t, err.Suggestions, "Run 'gh auth login' to authenticate")
	assert.Contains(t, err.Suggestions, "Check your GitHub token permissions")
}

func TestIsType(t *testing.T) {
	structErr := New(ErrTypeValidation, "validation error")
	regularErr := errors.New("regular error")

	assert.True(t, IsType(structErr, ErrTypeValidation))
	assert.False(t, IsType(structErr, ErrTypeDatabase))
	assert.False(t, IsType(regularErr, ErrTypeValidation))
}

func TestGetType(t *testing.T) {
	structErr := New(ErrTypeGitHubAPI, "API error")
	regularErr := errors.New("regular error")

	assert.Equal(t, ErrTypeGitHubAPI, GetType(structErr))
	assert.Equal(t, ErrTypeInternal, GetType(regularErr))
}

func TestNewConfigError(t *testing.T) {
	err := NewConfigError("invalid value", "log_level")

	assert.Equal(t, ErrTypeConfig, err.Type)
	assert.Contains(t, err.Message, "invalid value")
	assert.Contains(t, err.Message, "log_level")
	assert.Contains(t, err.Suggestions, "Check your configuration file syntax")
	assert.Contains(t, err.Suggestions, "Run with --help to see valid configuration options")
}

func TestNewConfigErrorEmptyField(t *testing.T) {
	err := NewConfigError("failed to load", "")

	assert.Equal(t, ErrTypeConfig, err.Type)
	assert.Equal(t, "failed to load", err.Message)
}

func TestErrorTypeString(t *testing.T) {
	tests := []struct {
		errType  ErrorType
		expected string
	}{
		{ErrTypeGitHubAPI, "github_api"},
		{ErrTypeDatabase, "database"},
		{ErrTypeValidation, "validation"},
		{ErrTypeRateLimit, "rate_limit"},
		{ErrTypeNotFound, "not_found"},
		{ErrTypeConfig, "config"},
		{ErrTypeNetwork, "network"},
		{ErrTypeAuth, "auth"},
		{ErrTypeFileSystem, "filesystem"},
		{ErrTypeInternal, "internal"},
	}

	for _, tt := range tests {
		t.Run(string(tt.errType), func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.errType))
		})
	}
}
