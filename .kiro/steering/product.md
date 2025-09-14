# Product Overview

This repository contains a GitHub CLI extension written in Go:

## gh-star-search
A GitHub CLI extension that enables fast, scored search over a user's starred repositories using a simple query string (no natural language to SQL translation). It ingests and indexes starred repositories into a local DuckDB database for fuzzy or vector similarity search.

**Key Features:**
- Query-string search (fuzzy or vector) with relevance score (default top 10 results)
- Related repositories discovery (org/topic/contributor/vector similarity) with match rationale
- Local DuckDB storage
- Incremental sync with selective metadata refresh & minimal file downloads
- Non-LLM summarization (Python `transformers`) as default; LLM integration optional & currently disabled
- Repository long-form and short-form output modes
- Repository metadata & selective content indexing (README, description, limited docs)

### Search
A single user-provided query string is matched against: repository name, organization, GitHub description, generated summary, contributor names, and other indexed text chunks. Structured field filtering (e.g., by stars, language) is not yet supported.

Users choose the search mode:
- Fuzzy: lightweight text relevance scoring
- Vector: embedding similarity over stored text representations

Results include a numeric score and are capped by a configurable limit (default 10).

### Related
Given a repository, the tool returns other starred repositories that are related via:
- Same organization
- Shared contributors
- Shared GitHub topics
- Summary vector similarity

Each related match includes a brief reason label (e.g., "topic: ai", "shared contributor: octocat").

### Output Formats
Two display modes:
- Short form: first two lines (link and GitHub description)
- Long form: standardized multi-line block including description, external link (if any), issue/PR metrics, commit activity, age, license, top contributors, topics, languages, related counts, last sync, optional summary, and planned future metrics (dependency counts, dependents).

### Sync & Caching
- Metadata refreshed only if `last_synced` older than configurable threshold (default 14 days)
- Summaries recomputed only when explicitly forced
- Only minimal files fetched (README, docs/README.md if present, external description link text); summary generation currently restricted to main README
- No full repository clone during sync

### Summarization
If no LLM is configured, a local non-LLM summarization (Python `transformers`) is used. LLM integration is temporarily disabled pending refinement; future reinstatement will be optional.

### Limitations
- No structured filtering (stars, languages, etc.) yet
- Summary limited to main README content (by design for speed)
- Vector search quality depends on chosen local embedding model (TBD in tech spec)

### Planned (Not Yet Implemented)
- Dependency & dependents metrics
- Optional LLM-enhanced summarization & query expansion
- Interactive TUI (Bubble Tea) exploration
- Advanced structured filters (languages, stars, activity ranges)

## Changes
- Replaced natural language query description with simple query-string search (fuzzy/vector)
- Added related repositories feature with rationale outputs
- Added scoring, result limit default (10), and explicit unsupported structured filters
- Documented sync & caching (age threshold, selective summary regeneration, minimal file fetch)
- Added non-LLM summarization default; noted temporary LLM removal
- Added long vs short output format specification
- Expanded key features list to align with new requirements
- Added limitations & planned sections reflecting new guidance
