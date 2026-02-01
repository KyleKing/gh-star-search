# Overengineered Areas

## Config fields that configure nothing

### Database connection pooling (`internal/config/config.go:26-32`)

`DatabaseConfig` has `MaxConnections`, `MaxIdleConns`, `ConnMaxLifetime`, `ConnMaxIdleTime`. DuckDB is an embedded database -- connection pooling parameters have no effect.

**Suggestion:** Remove `MaxConnections`, `MaxIdleConns`, `ConnMaxLifetime`, `ConnMaxIdleTime`. Keep `Path` and `QueryTimeout`.

### Debug ports (`internal/config/config.go:57-63`)

`DebugConfig` has `ProfilePort` and `MetricsPort` but no pprof or metrics server exists anywhere in the codebase.

**Suggestion:** Remove `ProfilePort` and `MetricsPort`. If pprof is added later, add the config field at that time.

### Log rotation fields (`internal/config/config.go:46-54`)

`LoggingConfig` has `MaxSizeMB`, `MaxBackups`, `MaxAgeDays` but `SetupLogger` opens a file with `O_APPEND` and does no rotation. These fields are read by `cmd/config.go` display output but have no functional effect.

**Suggestion:** Remove `MaxSizeMB`, `MaxBackups`, `MaxAgeDays` from `LoggingConfig`. If log rotation is needed later, add lumberjack or similar and reintroduce the config.

---

## `internal/errors` package

Full structured error system with:
- Stack traces captured via `runtime.Callers` on every `New`/`Wrap` call
- 10 error type constants
- JSON tags on all fields
- Context maps (`map[string]interface{}`)
- Suggestion string arrays
- 8 specialized constructors (`NewGitHubAPIError`, `NewDatabaseError`, etc.)

For a CLI tool that prints errors to stderr and exits, this is heavy. The stack trace overhead is paid on every error creation even though stacks are only useful during development.

**Suggestion:** Consider simplifying to `fmt.Errorf` with sentinel error types. If structured errors are valuable for user-facing suggestions, keep the type + message + suggestions but drop stack traces, JSON tags, and the context map.

---

## `embedding.Manager` abstraction (`internal/embedding/provider.go`)

Manager wraps a Provider interface, but only one provider works (LocalProvider). Manager.GenerateEmbedding checks IsEnabled then delegates. Manager.IsEnabled checks both config and provider. The Manager layer adds almost no logic.

**Suggestion:** Remove the Manager wrapper. Call the Provider interface directly. When a second provider is added, evaluate whether a manager is needed.

---

## File cache background goroutine (`internal/cache/cache.go`)

`NewFileCache` unconditionally spawns a background cleanup ticker goroutine. For a CLI that runs one command and exits, this goroutine runs the entire process lifetime with no benefit (cleanup frequency is typically 1 hour but the process lives seconds).

**Suggestion:** Make background cleanup opt-in, or run cleanup once at cache creation time instead of on a timer.

---

## Duplicated `cosineSimilarity`

Identical implementations at:
- `internal/query/engine.go:208-226`
- `internal/related/engine.go:440-458`

**Suggestion:** Extract to a shared utility, or delete from `query/engine.go` since vector search is non-functional there anyway.

---

## Bubble sort in hot paths

Three locations use O(n^2) selection sort where `sort.Slice` (O(n log n)) is already used elsewhere:
- `internal/query/engine.go:404` (`sortAndRankResults`)
- `internal/formatter/formatter.go:281` (`formatLanguages`)
- `internal/cache/cache.go:436` (`enforceSize`)

**Suggestion:** Replace with `sort.Slice`. For `sortAndRankResults` with potentially thousands of starred repos, this matters.
