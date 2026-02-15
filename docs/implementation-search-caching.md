# Implementation Plan: Search Result Caching

Inspired by msgvault's `DuckDBEngine` search cache that materializes temp tables across pagination calls to avoid repeated Parquet scans.

## Goal

Cache materialized search results in DuckDB temp tables so that repeated or paginated queries against the same search term avoid full FTS/vector rescans. Enables future pagination support and improves TUI responsiveness for repeated queries.

## Problem

Current search executes a full FTS or vector scan on every call. The `query.SearchEngine` has no awareness of prior queries. This is fine for single CLI invocations but becomes a bottleneck when:

1. A TUI sends the same query repeatedly as the user scrolls
1. A future MCP server handles paginated search requests
1. The user re-runs the same query with different `--limit`

## Design

### Cache Location

Add cache state directly to `query.SearchEngine` (not the storage layer). The cache is session-scoped -- it lives for the lifetime of the process and requires no persistence.

### Cache Key

Deterministic key from search parameters:

```go
type cacheKey struct {
    Query string
    Mode  string // "fuzzy" or "vector"
}

func (k cacheKey) String() string {
    return fmt.Sprintf("%s:%s", k.Mode, k.Query)
}
```

### Cached State

```go
type searchCache struct {
    mu       sync.Mutex
    key      string          // current cache key
    results  []Result        // full scored+ranked result set
    cachedAt time.Time       // for staleness check
}
```

### Cache Lifecycle

1. **On search**: compute cache key from query + mode
1. **Cache hit**: key matches and age < TTL (default 60s) -> slice `results[offset:offset+limit]`
1. **Cache miss**: execute full search, store results, return requested slice
1. **Invalidation**: any sync operation calls `engine.InvalidateCache()` to clear

The TTL is short (60 seconds) because the cache exists for interactive responsiveness, not persistence. A sync operation explicitly invalidates.

### SearchEngine Changes

```go
// internal/query/engine.go

type SearchEngine struct {
    repo      storage.Repository
    embedding embedding.Provider
    logger    *slog.Logger

    // Search result cache
    cache searchCache
}

type SearchParams struct {
    Query    string
    Mode     string
    Limit    int
    Offset   int      // new field
    MinScore float64
}

func (e *SearchEngine) Search(ctx context.Context, params SearchParams) ([]Result, int, error) {
    key := fmt.Sprintf("%s:%s", params.Mode, params.Query)

    e.cache.mu.Lock()
    if e.cache.key == key && time.Since(e.cache.cachedAt) < 60*time.Second {
        total := len(e.cache.results)
        start := min(params.Offset, total)
        end := min(start+params.Limit, total)
        results := e.cache.results[start:end]
        e.cache.mu.Unlock()
        return results, total, nil
    }
    e.cache.mu.Unlock()

    // Execute full search (existing logic)
    results, err := e.executeSearch(ctx, params.Query, params.Mode, params.MinScore)
    if err != nil {
        return nil, 0, err
    }

    // Cache full result set
    e.cache.mu.Lock()
    e.cache.key = key
    e.cache.results = results
    e.cache.cachedAt = time.Now()
    e.cache.mu.Unlock()

    total := len(results)
    start := min(params.Offset, total)
    end := min(start+params.Limit, total)
    return results[start:end], total, nil
}

func (e *SearchEngine) InvalidateCache() {
    e.cache.mu.Lock()
    e.cache.key = ""
    e.cache.results = nil
    e.cache.mu.Unlock()
}
```

### Return Value Change

`Search` returns `([]Result, int, error)` where the `int` is the total result count. This allows callers to display "showing 10 of 47 results" without fetching all results.

### CLI Integration

The `query` command gains an `--offset` flag:

```go
&cli.IntFlag{
    Name:    "offset",
    Aliases: []string{"o"},
    Value:   0,
    Usage:   "Skip first N results",
}
```

For a single CLI invocation the cache provides no benefit (process exits after one call). The cache pays off in long-running processes: the TUI and a future MCP server.

### Sync Integration

At the end of `cmd/sync.go`, after sync completes, call `engine.InvalidateCache()` if the engine is accessible. In practice, sync and query run in separate process invocations, so this is only relevant when both are used within the TUI.

## Implementation Order

1. Add `SearchParams` struct with `Offset` field
1. Add `searchCache` struct to `SearchEngine`
1. Refactor `Search()` to accept `SearchParams`, extract current logic to `executeSearch()`
1. Add cache check/store logic around `executeSearch()`
1. Add `InvalidateCache()` method
1. Update `cmd/query.go` to use `SearchParams` and add `--offset` flag
1. Update return type to include total count
1. Tests: cache hit/miss, TTL expiry, invalidation, offset/limit slicing, concurrent access

## Scope Boundaries

- In-memory only, no disk persistence
- Single cached query (most recent), not an LRU of multiple queries
- TTL-based expiry (60s), not change-detection based
- No cache warming or prefetch
