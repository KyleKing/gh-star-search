# Path to v1.0

## Current State Summary

**Overall Assessment: 7.5/10** - Solid foundation with clear path to MVP completion

| Metric | Value |
|--------|-------|
| Test Coverage | ~60% average |
| Failing Tests | 4 (VCR cassette mismatch, cmd integration) |
| Outstanding TODOs | 11 |
| Core Features | Complete (sync, search, related, output formatting) |
| Blocking Issues | VCR migration incomplete |

---

## Priority 1: Fix Failing Tests

**Goal:** Green test suite

### 1.1 Fix VCR Cassette Mismatch

The VCR tests fail because recorded cassettes use `per_page=100` but code now uses `per_page=50`.

**Files:**
- `internal/github/client_test.go:151` - `TestGetStarredRepos_Success`
- `internal/github/client_test.go:202` - `TestGetStarredRepos_Pagination`

**Options:**

A. Re-record cassettes (recommended):
```bash
# Set environment to record mode and run tests
VCR_MODE=record go test ./internal/github/... -run TestGetStarredRepos -v
```

B. Update code to match cassettes:
```go
// internal/github/client.go - change per_page back to 100
// OR update test to use per_page=100 explicitly
```

C. Temporarily skip VCR tests:
```go
// Add at top of failing tests
t.Skip("VCR cassettes need re-recording")
```

### 1.2 Fix cmd Integration Tests

**Files:**
- `cmd/sync_integration_test.go`
- `cmd/query_integration_test.go`

**Action:** Investigate after VCR fixes - likely cascading from GitHub client issues.

---

## Priority 2: Remove Placeholder Code

**Goal:** No misleading stubs or incomplete implementations

### 2.1 Related Stars Calculation

Currently returns placeholder string. Either implement or display "N/A".

**Files:**
- `cmd/query.go:447` - `formatRelatedStars()`
- `internal/formatter/formatter.go:323`

**Implementation approach:**
```go
func (e *Engine) CountRelatedStars(ctx context.Context, repo storage.StoredRepo) (orgCount, contributorCount int, err error) {
    // Count repos in same org
    orgCount, err = e.repo.CountByOrg(ctx, repo.Owner)
    if err != nil {
        return 0, 0, err
    }

    // Count repos with shared top contributors
    contributorCount, err = e.repo.CountByContributors(ctx, repo.TopContributors)
    if err != nil {
        return 0, 0, err
    }

    return orgCount, contributorCount, nil
}
```

### 2.2 Vector Search Decision

Currently stubs that fall back to fuzzy search. Choose one:

A. **Remove vector search option** (recommended for v1):
   - Remove `--mode vector` flag
   - Update help text to indicate fuzzy-only
   - Document vector search as planned for v1.1

B. **Implement basic vector search**:
   - Requires embedding provider implementation
   - Significant scope increase

**Files:**
- `internal/query/engine.go:131`
- `internal/embedding/provider.go:144-192`
- `internal/related/engine.go:267`

### 2.3 Debug Flag TODOs

**Files:**
- `main.go:189,193`

**Action:** Wire up existing `--debug` flag to show error cause and stack:
```go
if err.Cause != nil && cfg.Debug {
    fmt.Fprintf(os.Stderr, "Cause: %v\n", err.Cause)
}
```

---

## Priority 3: Code Quality Improvements

**Goal:** Production-ready code

### 3.1 Extract Magic Numbers

**File:** `cmd/query.go`

```go
// Add constants at package level
const (
    MinQueryLimit     = 1
    MaxQueryLimit     = 50
    DefaultQueryLimit = 10
    MinQueryLength    = 2
)

// Update validation
if queryLimit < MinQueryLimit || queryLimit > MaxQueryLimit {
    return errors.New(errors.ErrTypeValidation,
        fmt.Sprintf("limit must be between %d and %d", MinQueryLimit, MaxQueryLimit))
}
```

### 3.2 Recency Calculation

**File:** `internal/query/engine.go:214`

```go
func calculateRecencyBoost(updatedAt time.Time) float64 {
    daysSinceUpdate := time.Since(updatedAt).Hours() / 24
    switch {
    case daysSinceUpdate < 30:
        return 0.1
    case daysSinceUpdate < 90:
        return 0.05
    case daysSinceUpdate < 365:
        return 0.02
    default:
        return 0.0
    }
}
```

---

## Priority 4: Test Coverage Improvements

**Goal:** >70% coverage across all packages

### 4.1 Low Coverage Packages

| Package | Current | Target | Action |
|---------|---------|--------|--------|
| related | 48.7% | 70% | Add engine edge case tests |
| storage | 53.0% | 70% | Add query/filter tests |
| cmd | 49.8% | 60% | Fix integration tests, add CLI flag tests |
| embedding | 0.0% | - | Remove or stub with skip |

### 4.2 Add CLI Argument Tests

**File:** `cmd/query_test.go` (new or extend)

```go
func TestQueryValidation(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        wantErr string
    }{
        {"empty query", []string{""}, "at least 2 characters"},
        {"limit too high", []string{"--limit", "100", "test"}, "between 1 and 50"},
        {"invalid mode", []string{"--mode", "invalid", "test"}, "invalid mode"},
    }
    // ...
}
```

---

## Priority 5: Documentation

**Goal:** Clear user and contributor documentation

### 5.1 Update README

- Remove references to vector search (if removing for v1)
- Add "Known Limitations" section
- Update feature list to match actual implementation

### 5.2 Add CONTRIBUTING.md

Per `new.md` requirements:
- Architecture overview
- GitHub API endpoints used
- Rate limit considerations
- Development setup

---

## Deferred to v1.1+

These features are documented in `new.md` but not required for v1:

| Feature | Complexity | Notes |
|---------|------------|-------|
| Vector/semantic search | High | Requires embedding infrastructure |
| Search across README content | Medium | Currently description-only |
| Structured filtering (stars, language) | Medium | Query syntax extension |
| TUI with Bubble Tea | High | Major UI addition |
| LLM summarization | High | Recently removed |
| Dependency metrics | Medium | GitHub API integration |
| Background refresh | Medium | Scheduler/daemon mode |

---

## Checklist for v1.0 Release

### Blocking
- [ ] All tests passing
- [ ] VCR cassettes re-recorded or tests fixed
- [ ] No placeholder implementations in user-facing features

### Required
- [ ] Related stars calculation implemented or marked N/A
- [ ] Vector search removed from CLI or implemented
- [ ] Debug flag wired up for error details
- [ ] Magic numbers extracted to constants

### Recommended
- [ ] Test coverage >60% overall
- [ ] README updated to match implementation
- [ ] CONTRIBUTING.md added
- [ ] No critical TODOs remaining in code

### Pre-release
- [ ] `go build` succeeds
- [ ] `go test ./...` passes
- [ ] `golangci-lint run` passes
- [ ] Manual smoke test of core commands:
  - `gh star-search sync`
  - `gh star-search query "golang"`
  - `gh star-search related owner/repo`
  - `gh star-search list`
  - `gh star-search info owner/repo`
  - `gh star-search stats`

---

## Estimated Effort

| Priority | Tasks | Estimate |
|----------|-------|----------|
| P1: Fix tests | VCR cassettes, integration tests | Small |
| P2: Remove placeholders | Related stars, vector decision | Medium |
| P3: Code quality | Constants, recency calc, debug flag | Small |
| P4: Test coverage | CLI tests, edge cases | Medium |
| P5: Documentation | README, CONTRIBUTING | Small |

**Total for minimum viable v1:** P1 + P2 (partial) + P3
**Total for polished v1:** All priorities
