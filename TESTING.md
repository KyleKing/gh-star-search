# Testing Guide

This document provides information about testing in gh-star-search.

## Test Organization

Tests are organized into three categories:

### 1. Unit Tests (`*_test.go`)

Unit tests test individual functions and methods in isolation. They are located alongside the source files they test.

**Running unit tests:**
```bash
go test ./...
```

**Key packages with unit tests:**
- `internal/cache/`: 77.1% coverage
- `internal/config/`: 71.7% coverage
- `internal/errors/`: 97.9% coverage
- `internal/storage/`: Core database operations

### 2. Integration Tests (`*_integration_test.go`)

Integration tests verify that components work together correctly. They may require:
- Temporary databases
- GitHub authentication (via `gh` CLI)
- Network access

**Running integration tests:**
```bash
# Run all tests including integration tests
go test ./...

# Skip integration tests (short mode)
go test -short ./...
```

**Integration test files:**
- `cmd/query_integration_test.go`: End-to-end query testing
- `cmd/sync_integration_test.go`: Repository sync testing

### 3. Performance Tests (`*_performance_test.go`)

Performance tests benchmark critical operations to catch regressions.

**Running performance tests:**
```bash
go test -bench=. ./...
```

**Performance test files:**
- `cmd/sync_performance_test.go`: Batch processing benchmarks

## Test Constants

Common test constants are defined in `internal/testing/constants.go`:

```go
const (
    TestTimeout = 30 * time.Second       // Default test timeout
    TestBatchSize = 5                     // Default batch size for tests
    TestRepoCount = 10                    // Common number of test repos
    TestFullName = "testuser/testrepo"   // Default repository name
)
```

Use these constants instead of magic numbers to improve test maintainability.

## Known Test Issues

### Skipped Tests

Some tests are currently skipped due to known issues:

1. **`TestUpdateRepository`** (`internal/storage/duckdb_test.go:83`)
   - **Issue**: DuckDB constraint violation during update
   - **Status**: Known issue, being investigated
   - **Workaround**: Uses delete + insert pattern instead

2. **Integration tests in short mode**
   - Integration tests are skipped when running `go test -short`
   - This is expected behavior for CI/development speed

### Network-Dependent Tests

Some tests require network access and GitHub authentication:

- Tests in `cmd/sync_integration_test.go` require `gh` authentication
- Tests may fail in restricted network environments
- Use `-short` flag to skip these tests

## Testing Best Practices

### 1. Use Table-Driven Tests

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"basic", "input", "expected"},
        {"edge case", "", ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := function(tt.input)
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### 2. Use Test Fixtures

Create reusable test data:

```go
func createTestRepo() processor.ProcessedRepo {
    return processor.ProcessedRepo{
        Repository: github.Repository{
            FullName: testing.TestFullName,
            Description: testing.TestDescription,
            StargazersCount: testing.TestStarCount,
        },
    }
}
```

### 3. Clean Up Resources

Always use `defer` to clean up:

```go
func TestDatabase(t *testing.T) {
    repo, err := storage.NewDuckDBRepository(dbPath)
    if err != nil {
        t.Fatal(err)
    }
    defer repo.Close()  // Always clean up

    // Test logic...
}
```

### 4. Use Temporary Directories

Use `t.TempDir()` for test files:

```go
func TestFileOperation(t *testing.T) {
    tmpDir := t.TempDir()  // Automatically cleaned up
    dbPath := filepath.Join(tmpDir, "test.db")
    // Use dbPath...
}
```

## Test Coverage

### Current Coverage

Run tests with coverage:
```bash
go test -cover ./...
```

Generate HTML coverage report:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Coverage Goals

- Core packages (storage, query): **> 80%**
- Business logic (cmd): **> 70%**
- Utilities (errors, logging): **> 90%**

## Mocking Strategy

### Current Approach

Tests use manual mock implementations:
- `cmd/mock_test.go`: Mock GitHub client
- Manual response setup for each test case

### Planned Migration: go-vcr

The project is planning to migrate to `go-vcr` for HTTP request recording/replay.

**Benefits:**
- More realistic tests (actual HTTP interactions)
- Easier test maintenance
- Better edge case testing

**Status:**
- Migration plan documented in `vcr_migration_plan.md`
- Implementation pending

**How to contribute:**
See `vcr_migration_plan.md` for the complete migration strategy.

## Continuous Integration

Tests run automatically on push via GitHub Actions (`.github/workflows/ci.yaml`).

**CI runs:**
- Unit tests
- Integration tests (if credentials available)
- Linting (golangci-lint)
- Code formatting checks

## Troubleshooting Tests

### Tests Fail with "dial tcp: connection refused"

**Cause:** Tests trying to download Go modules but network is unavailable

**Solution:**
```bash
# Use GOPROXY=direct to bypass proxy
GOPROXY=direct go test ./...

# Or use offline mode if modules are cached
go test -mod=readonly ./...
```

### Tests Fail with "GitHub authentication required"

**Cause:** Integration tests need GitHub credentials

**Solution:**
```bash
# Authenticate with gh CLI
gh auth login

# Or skip integration tests
go test -short ./...
```

### Tests Timeout

**Cause:** Long-running operations or network issues

**Solution:**
```bash
# Increase timeout
go test -timeout 5m ./...

# Or skip slow tests
go test -short ./...
```

## Writing New Tests

When adding new tests:

1. **Choose the right test type:**
   - Unit test for isolated logic
   - Integration test for component interactions
   - Performance test for critical paths

2. **Use descriptive names:**
   ```go
   func TestStorageRepository_GetByFullName_NotFound(t *testing.T) {
       // Test logic...
   }
   ```

3. **Add test documentation:**
   ```go
   // TestCacheExpiration verifies that expired cache entries are properly
   // removed during cleanup operations.
   func TestCacheExpiration(t *testing.T) {
       // Test logic...
   }
   ```

4. **Test error cases:**
   - Invalid input
   - Missing data
   - Network failures
   - Resource exhaustion

5. **Use constants:**
   - Import `internal/testing` package
   - Use defined constants instead of magic numbers

## Resources

- [Go Testing Documentation](https://pkg.go.dev/testing)
- [Table-Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [go-vcr Documentation](https://github.com/dnaeon/go-vcr)
- Project-specific: `vcr_migration_plan.md`
