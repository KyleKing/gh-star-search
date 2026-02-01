# gh-star-search

A GitHub CLI extension to search, explore, and discover relationships across your starred repositories using a simple query string with fuzzy or vector (semantic) matching plus optional related‑repository insights.

![demo](.github/assets/demo.gif)

## Overview

`gh-star-search` ingests your starred repositories into a local DuckDB database, capturing structured metadata (stars, forks, issues, pull requests, commit activity, contributors, topics, languages, license) and minimal unstructured content (repository description + primary README). A lightweight summarization step (transformers / heuristic fallback) produces summary fields used for search and (optionally) embeddings.

Features:
- Fuzzy full-text and vector (semantic) search with relevance scoring (default 10, max 50 results)
- Related repository discovery (same org, shared contributors, topic overlap, vector similarity)
- Long-form and short-form output formats
- Incremental sync with staleness-based refresh (default 14 days)
- Non-LLM summarization (Python `transformers` / heuristic fallback)
- Minimal content ingestion (no full git clone)

## Installation

```bash
gh extension install kyleking/gh-star-search
```

## Usage

### Sync starred repositories
Fetch & (re)process starred repositories (incremental; respects staleness thresholds).
```bash
gh star-search sync
```

### Query (fuzzy or vector search)
```bash
gh star-search query "formatter javascript" --mode fuzzy --limit 10 --short
# Vector (semantic) mode
gh star-search query "terminal ui library" --mode vector --limit 5 --long
```
Flags:
- `--mode (fuzzy|vector)` default: fuzzy
- `--limit <n>` default: 10 (max 50)
- `--long` / `--short` force output format (query defaults to short)
- `--related` include related repositories section for each (optional)

### Related repositories (alternative explicit form)
(If implemented as a dedicated subcommand; otherwise use `query --related`.)
```bash
gh star-search related owner/repo
```
Returns up to 5 related repos with explanation (weights: Org 0.30, Topics 0.25, Shared Contributors 0.25, Vector 0.20; renormalized if components missing).

### List all repositories (short-form always)
```bash
gh star-search list
```

### Detailed repository info (long-form)
```bash
gh star-search info owner/repo
```

### Database statistics
```bash
gh star-search stats
```

### Clear the database
```bash
gh star-search clear
```

## Output Formats

### Long-form (per repository)
```
<org>/<name>  (link: https://github.com/<org>/<name>)
GitHub Description: <description or ->
GitHub External Description Link: <homepage or ->
Numbers: <open_issues>/<total_issues> open issues, <open_prs>/<total_prs> open PRs, <stars> stars, <forks> forks
Commits: <commits_30d> in last 30 days, <commits_1y> in last year, <commits_total> total
Age: <humanized (now - created_at)>
License: <license or ->
Top 10 Contributors: login1 (C1), login2 (C2), ...
GitHub Topics: topic1, topic2, ...
Languages: Lang1 (LOC/approx), Lang2 (...)
Related Stars: <count_same_org> in <org>, <count_shared_contrib> by top contributors
Last synced: <humanized (now - last_synced)>
Summary: <purpose/combined summary> (optional)
```

### Short-form
First two lines of long-form with condensed metadata and score, e.g.:
```
1. owner/repo  ⭐ 1234  Go  Updated 5d  Score:0.87
GitHub Description: High performance toolkit for ...
```

## Caching & Refresh Behavior
- Metadata refresh only if `last_synced` older than configurable threshold (default 14 days)
- Issues / PR counts, commit stats, languages, contributors may use shorter TTLs (e.g. 7–14 days)
- Summary regenerated ONLY if missing, version mismatch, or explicitly forced (`--force-summary` / config)
- Embeddings recalculated if summary changed or missing (when embeddings enabled)
- Minimal content download; temporary files removed after processing

## Minimal Content & Summarization
- Sources: Description, main README, optionally `docs/README.md`, optionally one homepage link text
- Summary input restricted to Description + main README (even if other sources fetched)
- Non‑LLM summarization via transformers model (e.g. DistilBART) or heuristic fallback if Python/`transformers` unavailable
- Summary fields: Purpose, Technologies, Use Cases, Features, Installation, Usage (+ generated timestamp, version, generator)

## Search Modes
- Fuzzy: Full‑text / BM25 style scoring across name, description, summary fields, topics, top contributor logins
- Vector: Cosine similarity over repository-level embedding of summary text
- Ranking boosts (internal, not filters): logarithmic stars, mild recency decay; final score capped at 1.0
- No structured filtering yet (stars/language/topic queries deferred)

## Related Repository Computation
Combines (weighted & renormalized): same org, topic overlap (Jaccard), shared top contributors (intersection of top 10), vector similarity (if embeddings enabled). Explanations list the contributing factors (e.g. `Shared org 'hashicorp' and 3 overlapping topics (terraform, cloud, plugin)`).

## Project Structure
```
gh-star-search/
├── cmd/                    # CLI commands (sync, query, list, info, stats, clear, related)
├── internal/
│   ├── cache/              # Local caching & freshness tracking
│   ├── config/             # Configuration models & defaults
│   ├── embedding/          # Embedding provider interface
│   ├── errors/             # Error categories / helpers
│   ├── formatter/          # Output formatting (long/short)
│   ├── github/             # GitHub API client
│   ├── logging/            # Structured logging utilities
│   ├── processor/          # Content extraction & processing
│   ├── query/              # Search engine (fuzzy + vector)
│   ├── related/            # Related repository engine
│   ├── storage/            # DuckDB persistence layer
│   ├── summarizer/         # Python-based summarization
│   └── types/              # Shared type definitions
├── scripts/                # Python helper scripts (summarize.py, embed.py)
├── main.go                 # Entry point
├── go.mod                  # Module definition
├── README.md               # Project documentation
└── CONTRIBUTING.md         # Architecture & contributor guide
```

## Development

### Building
```bash
mise run build
```

### Testing
```bash
mise run test
```

### Configuration
Configuration (JSON) includes: search defaults, embedding provider & dimensions, refresh thresholds, GitHub behavior. See `CONTRIBUTING.md` for details.

## Roadmap / Future Work
- Reintroduce optional LLM summarization (complement transformers)
- Structured filtering (stars, language, topic) & advanced grammar
- Chunk-level embeddings & hybrid BM25 + dense reranking
- Dependency / dependent metrics via GitHub dependency graph
- Background incremental refresh scheduling
- TUI (Bubble Tea) interactive interface
- Migration engine (golang-migrate) once schema stabilizes

## License

MIT License
