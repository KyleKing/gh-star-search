# Technology Stack

## Language & Runtime
- **Go 1.24**: Primary language
- **Python 3**: Optional, for summarization (`transformers`) and embeddings (`sentence-transformers`)
- **GitHub CLI Extension**: Executed as a `gh` extension

## Core Dependencies
- `github.com/urfave/cli/v3` for CLI command structure
- `github.com/cli/go-gh` for GitHub authentication
- DuckDB (via `github.com/marcboeker/go-duckdb`) for local storage and full-text querying
- `log/slog` for structured logging

## Data Ingestion & Sync
- GitHub REST API for repository metadata, topics, contributors, issues/PR counts, languages
- Minimal fetch: only README, docs/README.md (if present), external linked description page
- Configurable staleness window (default 14 days) before metadata refresh
- Summaries regenerated only when forced
- No full `git clone`; selective HTTP content retrieval

## Search
- **Fuzzy**: Token match with weighted field scoring across name, description, summary, topics, contributors
- **Vector**: Cosine similarity over embedding column (requires Python `sentence-transformers`)
- Ranking boosts: logarithmic star count, recency decay (partially implemented)
- Result limit configurable (`--limit`, default 10, max 50)

## Related Repository Discovery
- Weighted scoring: same org (30%), topic overlap (25%), shared contributors (25%), vector similarity (20%)
- Components renormalized when some are unavailable
- Streaming batch processing for memory efficiency

## Summarization
- Default: Python `transformers` (DistilBART) on primary README text
- Fallback: Heuristic keyword-based extractive summarization (no dependencies)
- Invoked via subprocess with JSON communication

## Embedding
- Default model: `sentence-transformers/all-MiniLM-L6-v2` (384 dimensions, ~80MB)
- Alternative: `all-mpnet-base-v2` (768 dimensions, ~420MB)
- Provider abstraction supports local (Python) and remote (API) backends

## Configuration
- JSON config file (`~/.config/gh-star-search/config.json`)
- Environment variables (`GH_STAR_SEARCH_` prefix)
- Command-line flag overrides

## Testing
- Unit tests (table-driven) across packages
- Integration tests for sync + query paths
- Performance benchmarks for critical paths
- go-vcr v4 for HTTP recording/replay (partial migration)

## Planned
- Dependency & 'used by' metrics
- Structured filters (language, stars range)
- TUI (Bubble Tea) interactive interface
- Optional LLM summarization reactivation
- Database migration engine (golang-migrate)
