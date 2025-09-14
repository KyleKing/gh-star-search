package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New(ErrTypeValidation, "test error message")

	assert.Equal(t, ErrTypeValidation, err.Type)
	assert.Equal(t, "test error message", err.Message)
	assert.NotEmpty(t, err.Stack)
	assert.Nil(t, err.Cause)
}

func TestNewf(t *testing.T) {
	err := Newf(ErrTypeDatabase, "failed to connect to %s", "database")

	assert.Equal(t, ErrTypeDatabase, err.Type)
	assert.Equal(t, "failed to connect to database", err.Message)
	assert.NotEmpty(t, err.Stack)
}

func TestWrap(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	wrappedErr := Wrap(originalErr, ErrTypeNetwork, "network operation failed")

	assert.Equal(t, ErrTypeNetwork, wrappedErr.Type)
	assert.Equal(t, "network operation failed", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
	assert.NotEmpty(t, wrappedErr.Stack)
}

func TestWrapf(t *testing.T) {
	originalErr := fmt.Errorf("connection refused")
	wrappedErr := Wrapf(originalErr, ErrTypeNetwork, "failed to connect to %s:%d", "localhost", 8080)

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
				Cause:   fmt.Errorf("connection timeout"),
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
	originalErr := fmt.Errorf("original error")
	wrappedErr := Wrap(originalErr, ErrTypeNetwork, "wrapped error")

	assert.Equal(t, originalErr, wrappedErr.Unwrap())
}

func TestWithContext(t *testing.T) {
	err := New(ErrTypeGitHubAPI, "API request failed")
	err = err.WithContext("status_code", 404)
	err = err.WithContext("url", "https://api.github.com/user")

	assert.Equal(t, 404, err.Context["status_code"])
	assert.Equal(t, "https://api.github.com/user", err.Context["url"])
}

func TestWithSuggestion(t *testing.T) {
	err := New(ErrTypeAuth, "authentication failed")
	err = err.WithSuggestion("Run 'gh auth login' to authenticate")
	err = err.WithSuggestion("Check your GitHub token permissions")

	assert.Len(t, err.Suggestions, 2)
	assert.Contains(t, err.Suggestions, "Run 'gh auth login' to authenticate")
	assert.Contains(t, err.Suggestions, "Check your GitHub token permissions")
}

func TestWithStack(t *testing.T) {
	err := New(ErrTypeInternal, "internal error")
	err = err.WithStack()

	assert.NotEmpty(t, err.Stack)
	assert.Contains(t, err.Stack, "errors_test.go")
}

func TestIsType(t *testing.T) {
	structErr := New(ErrTypeValidation, "validation error")
	regularErr := fmt.Errorf("regular error")

	assert.True(t, IsType(structErr, ErrTypeValidation))
	assert.False(t, IsType(structErr, ErrTypeDatabase))
	assert.False(t, IsType(regularErr, ErrTypeValidation))
}

func TestGetType(t *testing.T) {
	structErr := New(ErrTypeGitHubAPI, "API error")
	regularErr := fmt.Errorf("regular error")

	assert.Equal(t, ErrTypeGitHubAPI, GetType(structErr))
	assert.Equal(t, ErrTypeInternal, GetType(regularErr))
}

func TestNewGitHubAPIError(t *testing.T) {
	err := NewGitHubAPIError("rate limit exceeded", 429)

	assert.Equal(t, ErrTypeGitHubAPI, err.Type)
	assert.Equal(t, "rate limit exceeded", err.Message)
	assert.Equal(t, 429, err.Context["status_code"])
	assert.Contains(t, err.Suggestions, "Check your GitHub authentication with 'gh auth status'")
	assert.Contains(t, err.Suggestions, "Verify your internet connection")
}

func TestNewDatabaseError(t *testing.T) {
	err := NewDatabaseError("table not found", "SELECT")

	assert.Equal(t, ErrTypeDatabase, err.Type)
	assert.Equal(t, "table not found", err.Message)
	assert.Equal(t, "SELECT", err.Context["operation"])
	assert.Contains(t, err.Suggestions, "Check database file permissions")
	assert.Contains(t, err.Suggestions, "Ensure sufficient disk space")
}

func TestNewConfigError(t *testing.T) {
	err := NewConfigError("invalid value", "log_level")

	assert.Equal(t, ErrTypeConfig, err.Type)
	assert.Equal(t, "invalid value", err.Message)
	assert.Equal(t, "log_level", err.Context["field"])
	assert.Contains(t, err.Suggestions, "Check your configuration file syntax")
	assert.Contains(t, err.Suggestions, "Run with --help to see valid configuration options")
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("invalid format", "2023-13-45")

	assert.Equal(t, ErrTypeValidation, err.Type)
	assert.Equal(t, "invalid format", err.Message)
	assert.Equal(t, "2023-13-45", err.Context["value"])
	assert.Contains(t, err.Suggestions, "Check the input format and try again")
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError("rate limit exceeded", "2023-12-01T12:00:00Z")

	assert.Equal(t, ErrTypeRateLimit, err.Type)
	assert.Equal(t, "rate limit exceeded", err.Message)
	assert.Equal(t, "2023-12-01T12:00:00Z", err.Context["reset_time"])
	assert.Contains(t, err.Suggestions, "Wait for the rate limit to reset")
	assert.Contains(t, err.Suggestions, "Consider using a GitHub token with higher rate limits")
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("repository", "owner/repo")

	assert.Equal(t, ErrTypeNotFound, err.Type)
	assert.Equal(t, "repository not found: owner/repo", err.Message)
	assert.Equal(t, "repository", err.Context["resource"])
	assert.Equal(t, "owner/repo", err.Context["identifier"])
	assert.Contains(t, err.Suggestions, "Check that the resource exists and you have access to it")
}

func TestNewAuthError(t *testing.T) {
	err := NewAuthError("token expired")

	assert.Equal(t, ErrTypeAuth, err.Type)
	assert.Equal(t, "token expired", err.Message)
	assert.Contains(t, err.Suggestions, "Run 'gh auth login' to authenticate with GitHub")
	assert.Contains(t, err.Suggestions, "Check that your GitHub token has the required permissions")
}

func TestNewNetworkError(t *testing.T) {
	err := NewNetworkError("connection timeout", "https://api.github.com")

	assert.Equal(t, ErrTypeNetwork, err.Type)
	assert.Equal(t, "connection timeout", err.Message)
	assert.Equal(t, "https://api.github.com", err.Context["url"])
	assert.Contains(t, err.Suggestions, "Check your internet connection")
	assert.Contains(t, err.Suggestions, "Verify that the service is accessible")
}

func TestNewFileSystemError(t *testing.T) {
	err := NewFileSystemError("permission denied", "/path/to/file")

	assert.Equal(t, ErrTypeFileSystem, err.Type)
	assert.Equal(t, "permission denied", err.Message)
	assert.Equal(t, "/path/to/file", err.Context["path"])
	assert.Contains(t, err.Suggestions, "Check file/directory permissions")
	assert.Contains(t, err.Suggestions, "Ensure the path exists and is accessible")
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
