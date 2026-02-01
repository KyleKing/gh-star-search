# Path to v1.0

## Current State

Core features are implemented: sync, fuzzy search, related repository discovery, long/short output formatting, configuration management, and Python-based summarization/embedding pipelines.

## Remaining Work

### P1: Fix Failing Tests

- **VCR cassettes**: Only one cassette recorded (`get_starred_repos_success.yaml`). Remaining GitHub client tests still use manual mocks. Either re-record cassettes or migrate more tests incrementally.
- **Integration tests**: `cmd/sync_integration_test.go` and `cmd/query_integration_test.go` exist but may need attention after recent merges.

### P2: Complete Stubs

- **Related stars calculation** (`cmd/query.go:457`): `formatRelatedStars()` returns a placeholder string. Implement actual counting of same-org and shared-contributor repos, or display "N/A".
- **Debug flag** (`main.go:189,193`): Error cause/stack display is gated by hardcoded `false`. Wire up the existing `--debug` flag.
- **Recency calculation** (`internal/query/engine.go:323`): `recencyFactor` is hardcoded to `1.0`. Implement proper decay based on `UpdatedAt`.

### P3: Test Coverage

| Package | Action |
|---------|--------|
| related | Add edge case tests for streaming batch processing |
| storage | Add query/filter tests |
| cmd | Verify integration tests pass, add CLI flag tests |

### P4: Production Readiness

- [ ] All tests passing (`go test ./...`)
- [ ] Linter clean (`golangci-lint run`)
- [ ] No placeholder implementations in user-facing features
- [ ] Manual smoke test of core commands (sync, query, related, list, info, stats)
- [ ] GitHub CLI extension manifest and release workflow

## Deferred to v1.1+

- Structured filtering (stars, language, topic)
- Chunk-level embeddings and hybrid BM25 + dense reranking
- Dependency/dependent metrics
- Background incremental refresh
- TUI (Bubble Tea) interactive interface
- Database migration engine (golang-migrate)
