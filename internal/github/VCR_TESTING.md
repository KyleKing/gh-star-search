# VCR Testing Guide

This document explains how to use go-vcr for testing GitHub API interactions in gh-star-search.

## Overview

We use [go-vcr v4](https://github.com/dnaeon/go-vcr) to record and replay HTTP interactions with the GitHub API. This provides:

- **Realistic tests**: Uses actual API responses
- **Fast CI**: Replays pre-recorded interactions
- **Deterministic**: Same results every time
- **No API limits**: No requests made during replay

## When to Use VCR vs Mock

### Use VCR for:
✅ Testing actual API integration
✅ Testing response parsing
✅ Testing pagination logic with real data
✅ Testing error responses (via hooks)
✅ New feature development (record once, replay forever)

### Use Mock for:
✅ Testing context cancellation
✅ Testing internal logic that doesn't need real API data
✅ Unit tests that don't involve external APIs
✅ Tests requiring precise control over response order

## Basic Usage

### 1. Simple Test with VCR

```go
func TestGetStarredRepos_Success(t *testing.T) {
    // Setup VCR recorder
    r, vcrClient := setupVCRRecorder(t, "get_starred_repos_success")
    defer cleanupRecorder(t, r)

    // Create client with VCR
    client := &clientImpl{apiClient: vcrClient}

    // Test as normal
    ctx := context.Background()
    repos, err := client.GetStarredRepos(ctx, "testuser")

    // Assertions
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
}
```

### 2. Test with Timeout

```go
func TestWithTimeout(t *testing.T) {
    r, vcrClient, cleanup := setupVCRWithTimeout(t, "my_test", 30*time.Second)
    defer cleanup()

    client := &clientImpl{apiClient: vcrClient}
    // ... test code
}
```

### 3. Error Simulation with Hooks

```go
func TestGetStarredRepos_RateLimit(t *testing.T) {
    // Use hook to simulate rate limiting
    r, vcrClient := setupVCRRecorder(t, "rate_limit_error", withRateLimitError())
    defer cleanupRecorder(t, r)

    client := &clientImpl{apiClient: vcrClient}
    ctx := context.Background()

    _, err := client.GetStarredRepos(ctx, "testuser")

    // Should get rate limit error
    if err == nil {
        t.Fatal("Expected rate limit error")
    }
}
```

## Recording Cassettes

### Initial Recording

1. **Authenticate with GitHub CLI**:
   ```bash
   gh auth login
   ```

2. **Run test in recording mode** (first time):
   ```bash
   go test -v -run TestGetStarredRepos_Success ./internal/github/
   ```

3. **Cassette is saved** to `testdata/get_starred_repos_success.yaml`

### Re-recording

To update an existing cassette:
```bash
rm internal/github/testdata/get_starred_repos_success.yaml
go test -v -run TestGetStarredRepos_Success ./internal/github/
```

## Managing Cassette Size

### Problem: Large Cassettes

Recording all starred repositories can create huge cassette files (90KB+).

### Solutions:

1. **Use Test Accounts with Few Repos**:
   - Create a GitHub account with only 5-10 starred repos
   - Use this account for recording

2. **Limit per_page Parameter**:
   ```go
   // In test configuration
   perPage := 5  // Small number for tests
   ```

3. **Edit Cassettes Manually**:
   - Remove unnecessary interactions
   - Keep only 1-2 pages of results
   - Trim large response bodies

4. **Use Hooks to Filter**:
   ```go
   recorder.WithHook(func(i *cassette.Interaction) error {
       // Modify response before saving
       return nil
   }, recorder.BeforeSaveHook)
   ```

## Available Hooks

### Error Simulation Hooks

| Hook | Purpose | HTTP Status |
|------|---------|-------------|
| `withRateLimitError()` | API rate limit exceeded | 403 |
| `withNotFoundError()` | Resource not found | 404 |
| `withServerError()` | Server error | 500 |

### Custom Hook Example

```go
func withCustomResponse() recorder.Option {
    return recorder.WithHook(func(i *cassette.Interaction) error {
        // Modify response
        i.Response.Body = `{"custom": "data"}`
        return nil
    }, recorder.BeforeResponseReplayHook)
}
```

## Testing Strategy

### 1. Happy Path (Success)
- Record with real API calls
- Use small datasets
- Verify response parsing

### 2. Error Cases (Failures)
- Use VCR hooks to simulate errors
- No need to trigger real errors
- Test error handling logic

### 3. Edge Cases
- Empty responses: Hook returns empty array
- Pagination: Record 2-3 pages max
- Timeouts: Use context cancellation (mock)

## Cassette File Structure

Cassettes are stored as YAML files in `testdata/`:

```
internal/github/testdata/
├── get_starred_repos_success.yaml
├── get_starred_repos_pagination.yaml
├── get_starred_repos_error.yaml
├── get_repository_content_success.yaml
└── ...
```

### Cassette Format

```yaml
---
version: 2
interactions:
  - id: 0
    request:
      proto: HTTP/1.1
      proto_major: 1
      proto_minor: 1
      method: GET
      url: https://api.github.com/user/starred?page=1&per_page=5
    response:
      proto: HTTP/1.1
      proto_major: 1
      proto_minor: 1
      status: 200 OK
      status_code: 200
      body: |
        [{"full_name": "owner/repo", ...}]
```

## Best Practices

### DO:
✅ Use descriptive cassette names
✅ Keep cassettes small (< 10KB when possible)
✅ Use hooks for error scenarios
✅ Clean up sensitive data with hooks
✅ Group related tests with subtests
✅ Add comments explaining hook usage

### DON'T:
❌ Commit large cassettes (> 100KB)
❌ Record with your personal account
❌ Include authentication tokens
❌ Record all starred repos (use limits)
❌ Record unnecessary API calls

## Troubleshooting

### "GitHub auth not available"

The test is skipped because `gh auth` is not configured:
```bash
gh auth login
```

### "Failed to create VCR recorder"

Check that the `testdata/` directory exists:
```bash
mkdir -p internal/github/testdata
```

### "Cassette file is huge"

Reduce the dataset size:
1. Use a test account with fewer starred repos
2. Manually edit the cassette file
3. Use `per_page=5` in the test

### "Test passes in recording mode but fails in replay"

Check for:
- Dynamic data (timestamps, UUIDs) that changes
- Missing matcher configuration
- Response headers not being ignored

## Migration Status

### ✅ Migrated to VCR:
- `TestGetStarredRepos_Success`

### 🔄 Partially Migrated:
- Error tests can use hooks instead of mocks

### ⏳ Still Using Mock:
- `TestGetStarredRepos_Pagination`
- `TestGetStarredRepos_Error`
- `TestGetStarredRepos_ContextCancellation` (mock is fine)
- `TestGetRepositoryContent_*`
- `TestGetRepositoryMetadata_*`
- `TestGetContributors_*`
- `TestGetTopics_*`
- `TestGetLanguages_*`
- `TestGetCommitActivity_*`
- `TestGetPullCounts_*`
- `TestGetIssueCounts_*`
- `TestGetHomepageText_*`

## Future Improvements

1. **Migrate remaining tests** incrementally
2. **Add more hooks** for common scenarios
3. **Optimize cassette sizes** with response filtering
4. **CI integration** with cassette validation
5. **Automated re-recording** for API changes

## References

- [go-vcr documentation](https://github.com/dnaeon/go-vcr)
- [VCR Migration Plan](../../../vcr_migration_plan.md)
- [GitHub API Documentation](https://docs.github.com/en/rest)
