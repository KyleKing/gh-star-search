# VCR Migration Plan: Replacing newMockRESTClient with go-vcr v4

## Overview

This plan outlines the steps to replace the complex newMockRESTClient implementation in client_test.go with go-vcr v4 for more realistic and maintainable HTTP interaction testing. The current mock requires manual setup of responses for each API endpoint, while VCR will record and replay actual HTTP interactions.

## Current State Analysis

• newMockRESTClient implements RESTClientInterface with manual response/error injection
• Tests manually set responses for specific paths like "user/starred?page=1&per_page=100"
• Complex pagination and error handling tests require extensive mock setup
• Call counting and verification logic is custom-built

## Migration Strategy

### 1. Add go-vcr v4 Dependency

go get gopkg.in/dnaeon/go-vcr.v4

### 2. Test Structure Changes

• Replace mockRESTClient with VCR recorder in each test
• Use recorder.New() to create cassettes in testdata/ directory
• Replace clientImpl{apiClient: mockClient} with VCR-wrapped client
• Remove manual response setting - let VCR handle recording/replay

### 3. Basic VCR Integration

```go
func TestGetStarredRepos_Success(t *testing.T) {
    r, err := recorder.New("testdata/get_starred_repos_success")
    if err != nil {
        t.Fatal(err)
    }
    defer r.Stop()

    // Create client with VCR's HTTP client
    client := &clientImpl{apiClient: &vcrRESTClient{httpClient: r.GetDefaultClient()}}

    // Test logic remains similar
    ctx := context.Background()
    repos, err := client.GetStarredRepos(ctx, "testuser")
    // ... assertions
}
```

### 4. VCR REST Client Wrapper

Create a new vcrRESTClient that implements RESTClientInterface using VCR's HTTP client:

```go
type vcrRESTClient struct {
    httpClient *http.Client
}

func (v *vcrRESTClient) Get(path string, response interface{}) error {
    // Convert GitHub API path to full URL and make request
    url := fmt.Sprintf("https://api.github.com/%s", path)
    resp, err := v.httpClient.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    return json.NewDecoder(resp.Body).Decode(response)
}
```

## Limiting Repository Matches

### 1. Reduce per_page Parameter

• Change default per_page=100 to per_page=5 in tests
• This reduces the number of repositories fetched and recorded
• Example: client.GetStarredRepos(ctx, "testuser", WithPerPage(5))

### 2. Use Test-Specific Users

• Create dedicated test users with limited starred repositories
• Record cassettes with minimal data sets
• Avoid recording large numbers of repositories

### 3. Pagination Control

• Test pagination with smaller page sizes
• Record only 1-2 pages instead of many pages
• Focus on pagination logic rather than large data sets

## Edge Case Testing

### 1. Error Scenarios

• Rate Limiting: Test 403 Forbidden responses
• Authentication Errors: Test 401 Unauthorized
• Not Found: Test 404 responses for missing repositories
• Server Errors: Test 500 Internal Server Error

### 2. Empty Results

• Test users with no starred repositories
• Test repositories with no contributors/releases
• Test empty search results

### 3. Network Issues

• Test timeout scenarios
• Test connection refused
• Test malformed responses

## VCR File Modifications for Edge Cases

### 1. Hook-Based Modifications

Use VCR hooks to modify recorded interactions for edge case testing:

```go
// Hook to simulate rate limiting
rateLimitHook := func(i *cassette.Interaction) error {
    if strings.Contains(i.Request.URL, "/user/starred") {
        i.Response.StatusCode = 403
        i.Response.Body = `{"message": "API rate limit exceeded"}`
    }
    return nil
}

opts := []recorder.Option{
    recorder.WithHook(rateLimitHook, recorder.BeforeResponseReplayHook),
}
```

### 2. Multiple Cassettes per Test

• Create separate cassette files for different scenarios
• Example: get_starred_repos_success.yaml, get_starred_repos_rate_limited.yaml
• Use test subtests to organize different cases

### 3. Dynamic Response Modification

• Use hooks to modify response bodies programmatically
• Change repository counts, add/remove items
• Simulate different pagination scenarios

## Implementation Steps

### Phase 1: Basic Setup

1. Add go-vcr dependency
1. Create vcrRESTClient wrapper
1. Convert one simple test (e.g., TestGetStarredRepos_Success)
1. Record initial cassette with real API call
1. Verify test passes in replay mode

### Phase 2: Pagination and Limits

1. Implement per_page limiting
1. Convert pagination test
1. Record cassettes with controlled data sizes
1. Test pagination logic with smaller datasets

### Phase 3: Error Cases

1. Create error scenario hooks
1. Convert error handling tests
1. Record/modify cassettes for different error conditions
1. Test error propagation and handling

### Phase 4: Edge Cases and Cleanup

1. Implement remaining edge case tests
1. Remove old newMockRESTClient code
1. Update test documentation
1. Optimize cassette sizes

## Benefits of VCR Approach

### 1. Realism

• Tests use actual HTTP interactions
• More accurate representation of real API behavior
• Catches integration issues missed by mocks

### 2. Maintainability

• No manual response setup required
• Automatic recording of new API changes
• Easier to update tests when API changes

### 3. Performance

• Fast replay mode for CI/CD
• No complex mock setup overhead
• Deterministic test execution

### 4. Flexibility

• Easy to add new test scenarios
• Hooks allow fine-grained control
• Supports complex interaction patterns

## Potential Challenges

### 1. API Rate Limits

• Recording requires API calls (may hit rate limits)
• Need to manage authentication tokens carefully
• Consider using GitHub App tokens for higher limits

### 2. Sensitive Data

• Ensure hooks remove sensitive information
• Use BeforeSaveHook to redact tokens/API keys
• Avoid committing sensitive data to repository

### 3. Cassette Management

• Large cassette files may be cumbersome
• Need to periodically refresh recordings
• Version control of YAML files

### 4. Test Determinism

• Ensure recordings are stable across environments
• Handle time-sensitive data appropriately
• Use hooks to normalize dynamic content

## Testing Strategy

### 1. Recording Mode

• Run tests in recording mode to capture interactions
• Use environment variable to control mode: VCR_MODE=record
• Record with minimal, controlled data sets

### 2. Replay Mode

• Default mode for CI/CD pipelines
• Fast, deterministic execution
• No external dependencies

### 3. Hybrid Approach

• Record core functionality once
• Use hooks for edge case variations
• Minimize API calls while maximizing coverage

## File Structure

```
internal/github/
├── client_test.go (updated with VCR)
├── testdata/
│   ├── get_starred_repos_success.yaml
│   ├── get_starred_repos_pagination.yaml
│   ├── get_starred_repos_error.yaml
│   └── ... (additional cassettes)
└── vcr_client.go (new VCR wrapper)
```

This migration will significantly simplify test maintenance while providing more realistic testing scenarios. \</xai:function_call">The plan has been written to vcr_migration_plan.md. It covers the complete strategy for replacing the mock with go-vcr v4, including implementation steps, benefits, challenges, and specific approaches for limiting repository matches and handling edge cases through VCR hooks and modifications. The plan emphasizes realistic testing while maintaining test performance and determinism.

The key insight is that VCR will eliminate the need for manual response setup while allowing us to use hooks to create edge case variations without making additional API calls. This approach provides better maintainability and more accurate testing compared to the current complex mock implementation.
