# Project Structure

## gh-star-search (urfave/cli v3)
```
gh-star-search/
├── main.go              # Entry point
├── cmd/                 # CLI commands
│   ├── sync.go          # Sync command (incremental, minimal fetch)
│   ├── sync_summarize.go # Summary generation during sync
│   ├── sync_embed.go    # Embedding generation during sync
│   ├── query.go         # Query command (fuzzy/vector modes)
│   ├── list.go          # List command (short-form output)
│   ├── info.go          # Info command (long-form output)
│   ├── related.go       # Related repository discovery
│   ├── stats.go         # Stats command
│   ├── config.go        # Configuration management
│   └── clear.go         # Clear command
├── internal/
│   ├── cache/           # File-based caching with TTL
│   ├── config/          # Configuration models & loading
│   ├── embedding/       # Embedding provider interface (local/remote)
│   ├── errors/          # Structured error types
│   ├── formatter/       # Output formatting (long/short)
│   ├── github/          # GitHub API client + VCR test helpers
│   ├── logging/         # Structured logging with slog
│   ├── processor/       # Content extraction & processing
│   ├── query/           # Search engine (fuzzy + vector)
│   ├── related/         # Related repository engine (streaming batch)
│   ├── storage/         # DuckDB persistence layer
│   ├── summarizer/      # Python subprocess summarization
│   └── types/           # Shared type definitions
└── scripts/
    ├── summarize.py     # Heuristic + transformers summarization
    └── embed.py         # Sentence-transformer embeddings
```

## Architectural Notes
- Minimal ingestion: README + docs/README.md + external linked page (if present)
- Summaries limited to primary README; recomputed only when forced
- Single query-string interface for search; structured filters deferred
- Related engine uses streaming batch processing for memory efficiency
- Output renderer centralizes long vs short formatting logic
- Embedding generation optional; missing embeddings fall back to fuzzy search

## Testing Layout
- Unit tests (`*_test.go`) co-located with source
- Integration tests (`*_integration_test.go`) for sync/query
- Performance tests (`*_performance_test.go`) for benchmarks
- VCR cassettes in `internal/github/testdata/`
- Golden test files in `internal/formatter/testdata/`
