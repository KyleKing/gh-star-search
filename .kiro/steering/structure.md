# Project Structure

## gh-star-search (Cobra-based)
```
gh-star-search/
├── main.go              # Entry point
├── cmd/                 # CLI commands
│   ├── root.go          # Root command setup
│   ├── sync.go          # Sync command (incremental, minimal fetch)
│   ├── query.go         # Query command (fuzzy/vector modes)
│   ├── list.go          # List command (formatted output)
│   ├── info.go          # Info command (long/short forms)
│   ├── stats.go         # Stats command
│   └── clear.go         # Clear command
└── internal/
    ├── github/          # GitHub API client (metadata, topics, contributors)
    ├── processor/       # Content processing (README extraction, summarization)
    ├── storage/         # DuckDB repository (tables, embeddings)
    ├── query/           # Query execution (fuzzy & vector strategies)
    ├── llm/             # (Disabled) future LLM integration
    ├── config/          # Configuration management
    ├── cache/           # Local caching & staleness checks
    └── logging/         # Logging utilities
```

## Architectural Notes
- Minimal ingestion: README + docs/README.md + external linked page (if present)
- Summaries limited to primary README; recomputed only when forced
- Single query-string interface for search; structured filters deferred
- Pluggable related strategies (org, topic, contributor, vector)
- Output renderer centralizes long vs short formatting logic

## Package Adjustments
- Removed references to unused generic layers (`conn/`, `fixtures/` as global concept)
- Emphasis on `processor/` + `query/` for summarization and search strategies
- `llm/` retained but currently dormant

## Testing Layout
- Standard `*_test.go` co-located tests
- Integration & performance tests for sync/query in existing `*_integration_test.go` and `*_performance_test.go`
- Test data housed under package `testdata/` directories (e.g., `processor/testdata`)

## Configuration & Caching
- Configurable sync staleness window (default 14 days)
- Force flag for summary regeneration
- Embedding generation optional; absence falls back to fuzzy mode

## Changes
- Added vector vs fuzzy query distinction & related strategies
- Documented minimal file fetch + summary scope
- Clarified dormant LLM package status
- Simplified package list (removed unused `conn/`, `fixtures/` bullets)
- Added logging package mention
- Added config & caching behavioral notes
- Added long vs short output renderer description
