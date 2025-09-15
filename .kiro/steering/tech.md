# Technology Stack

## Language & Runtime
- **Go**: version 1.25
- **Python**: (optional) for non-LLM summarization via `transformers`
- **GitHub CLI Extension**: Executed as a `gh` extension

## Core Components
- `github.com/spf13/cobra` for CLI command structure
- DuckDB for local analytical & full-text (extension-assisted) querying
- Embedding / vector search layer (implementation TBD; likely local embedding model exposed via Python or Go wrapper)
- Fuzzy text search (basic scoring over tokenized fields)
- Optional (temporarily disabled) LLM integration for future summarization/query expansion

## Data Ingestion & Sync
- GitHub REST API for repository metadata, topics, contributors, issues/PR counts, languages
- Minimal fetch: only README, docs/README.md (if present), external linked description page
- Configurable staleness window (default 14 days) before metadata refresh
- Summaries regenerated only when forced; otherwise reused
- No full `git clone`; selective HTTP content retrieval

## Indexing & Storage
- DuckDB tables for: repositories, contributors, topics, languages, embeddings, summaries, sync metadata
- Text fields consolidated into searchable chunks for both fuzzy and vector indices
- Embedding generation deferred or batch-processed; missing embeddings fall back to fuzzy search

## Search Execution
- Input: single query string + mode flag (`--vector` or `--fuzzy`)
- Fuzzy: token match + simple scoring (e.g., weighted field frequency)
- Vector: cosine (or dot) similarity over embedding column
- Unified result schema includes: repo id, score, matched rationale (in related mode) and optional summary
- Result limit configurable (`--limit`, default 10)

## Related Repository Discovery
- Strategies (pluggable): same org, shared contributors (top N), shared topics, vector similarity threshold/top-K
- Each strategy emits (repo_id, reason, score?) tuples; results merged and deduplicated
- Display surfaces first reason (or prioritized ordering: contributor > topic > org > vector)

## Summarization
- Default: Python `transformers` summarization on primary README text only
- Pipeline invoked conditionally (no LLM configured) via subprocess or service boundary
- Future: Reinstate LLM path with abstraction preserved in `internal/llm`

## Output Formatting
- Long-form renderer composes metrics from cached tables (issues, PRs, stars, forks, commit activity, age, license, top contributors, topics, languages, related counts)
- Short-form: first two lines only (link + description)

## Configuration
- Environment / config file toggles: staleness days, summary force flag, default search mode, embedding backend enablement
- Safe defaults to minimize API usage and latency

## Observability & Performance
- Lightweight logging (structured where needed)
- Caching of API responses (ETag / conditional requests potential future optimization)
- Batch API requests to respect rate limits
- Potential memory monitoring hooks for large batch operations

## Deferred / Planned
- Dependency & 'used by' metrics ingestion
- Advanced structured filters (language, stars range)
- Interactive TUI (Bubble Tea) once core flows stabilized
- LLM integration reactivation with provider abstraction

## Testing
- Unit tests (table-driven) across packages
- Integration tests for sync + query paths (with fixtures)
- Performance tests for sync & search hot paths
- Mocking via interfaces; potential future adoption of `gomock`

## Changes
- Added Python `transformers` summarization and removed active LLM requirement (LLM now optional/disabled)
- Specified minimal file fetch strategy (no full clone) and selective README-only summarization
- Added configurable metadata staleness window & force-summary behavior
- Introduced fuzzy vs vector search modes and single query string input (replacing natural language to SQL)
- Added related repositories strategy layer & rationale output
- Clarified indexing schema (embeddings, text chunks) & search pipeline
- Defined long vs short output rendering responsibilities
- Added planned/deferred items aligned with new requirements
