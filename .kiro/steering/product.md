# Product Overview

## gh-star-search

A GitHub CLI extension that enables scored search over a user's starred repositories. It ingests and indexes starred repositories into a local DuckDB database for fuzzy or vector similarity search.

**Key Features:**
- Query-string search (fuzzy or vector) with relevance scoring (default top 10 results)
- Related repository discovery (org/topic/contributor/vector similarity) with match rationale
- Local DuckDB storage with incremental sync and selective metadata refresh
- Non-LLM summarization (Python `transformers` / heuristic fallback)
- Long-form and short-form output modes

### Search
A query string is matched against: repository name, organization, description, generated summary, contributor names, and other indexed text. Structured field filtering (by stars, language) is not yet supported.

Modes:
- **Fuzzy**: Full-text relevance scoring
- **Vector**: Embedding similarity over stored text representations

### Related
Given a repository, returns other starred repositories related via same organization, shared contributors, shared topics, or summary vector similarity. Each match includes a reason label.

### Output Formats
- **Short form**: Condensed single-line with score, stars, language, update time
- **Long form**: Multi-line block with full metadata, metrics, contributors, topics, languages, related counts, optional summary

### Sync & Caching
- Metadata refreshed only if `last_synced` older than threshold (default 14 days)
- Summaries recomputed only when explicitly forced
- Minimal files fetched (README, optional docs/README.md, external link text)
- No full repository clone

### Limitations
- No structured filtering (stars, languages, etc.) yet
- Vector search quality depends on embedding model
- Summary limited to main README content

### Planned
- Dependency & 'used by' metrics
- Optional LLM-enhanced summarization
- TUI (Bubble Tea) interactive interface
- Structured filters (languages, stars, activity ranges)
