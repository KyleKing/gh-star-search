# Design Document

## Overview

The `gh-star-search` GitHub CLI extension provides intelligent search, inspection, and related-repository discovery over a user's starred repositories. After simplification (per `new.md` guidance), the system now:

- Accepts a direct user query string (no natural language → SQL translation, no user-visible SQL)
- Supports two search modes: fuzzy (full‑text) and vector (semantic embeddings)
- Returns scored results with configurable limit (default 10)
- Does NOT (currently) support filtering by stars, language, topics, etc. (structured filtering is deferred)
- Provides short-form and long-form CLI output formats
- Offers a Related feature (same org, shared contributors, topic overlap, vector similarity)
- Generates summaries using a non-LLM (transformers-based) pipeline; LLM integration is temporarily removed
- Ingests only minimal content: GitHub Description, main README, `docs/README.md` (if present), and (optionally) text of a single external link referenced (e.g. project homepage) — BUT summary generation itself uses ONLY the main README + Description (per guidance)
- Uses a conservative caching & refresh policy (metadata refresh after staleness threshold; summaries only when forced)
- Minimizes file downloads (no full git clone)

LLM summarization, natural language query parsing, and extensive repository crawling are explicitly deferred (see Future Work).

## Architecture

### High-Level Architecture

```mermaid
graph TB
    CLI[CLI Interface]
    CLI --> Sync[Sync]
    CLI --> Query[Query]
    CLI --> Related[Related]
    CLI --> Mgmt[Mgmt Commands]

    Sync --> GH[GitHub API Client]
    Sync --> Proc[Processor]
    Sync --> Store[Storage]

    Query --> SearchCore[Search Engine]
    Query --> Store
    Query --> Format[Formatter]

    Related --> RelEngine[Related Engine]
    Related --> Store

    Proc --> Summ[Summary (transformers)]
    Proc --> Embed[Embedding Generator]

    Store --> DB[(DuckDB)]
    GH --> Cache[Local Cache]

    subgraph "External Services (Optional)"
        GHAPI[GitHub API]
        EmbedAPI[Embedding Provider]
    end

    GH --> GHAPI
    Embed --> EmbedAPI
```

### Component Architecture

Primary packages (updated):

- **cmd/**: CLI commands (`sync`, `query`, `list`, `info`, `stats`, `clear`, `related` or `query --related`)
- **internal/github/**: GitHub API client (metadata, topics, contributors, languages, commit stats, optional external link fetch)
- **internal/processor/**: Content extraction (minimal scope), summary generation (transformers), embedding generation
- **internal/storage/**: DuckDB persistence, search primitives, contributor/topic linkage
- **internal/query/**: Search engine (fuzzy + vector) & ranking (repurposed from previous NL parser)
- **internal/related/**: Related repository computation (may initially live inside `internal/query`)
- **internal/cache/**: Local caching + freshness timestamps & content hash tracking
- **internal/config/**: Configuration (search defaults, staleness thresholds, embedding provider switch)
- **internal/logging/**: Structured logging utilities

(Former **internal/llm/** module removed temporarily.)

## Components and Interfaces

### 1. CLI Interface Layer

Key flags (query):
- `--mode (fuzzy|vector)` (default `fuzzy` via config)
- `--limit <n>` (default 10, max 50)
- `--long` / `--short` (format override; `list` always short, `info` always long)
- `--related` (augment results with related section) or `related <repo>` subcommand

Validation: raw query length ≥ 2; limit bounds; mode enumeration; rejects unsupported structured filters (explicit user help text clarifies not yet supported).

### 2. GitHub API Client

Adds focused endpoints only when necessary (lazy fetch on demand / cached):
```go
type Client interface {
    GetStarredRepos(ctx context.Context) ([]Repository, error)      // basic metadata
    GetRepositoryReadme(ctx context.Context, fullName string) (Content, error)
    GetDocsReadme(ctx context.Context, fullName string) (Content, error) // optional
    GetContributors(ctx context.Context, fullName string, topN int) ([]Contributor, error)
    GetTopics(ctx context.Context, fullName string) ([]string, error)
    GetLanguages(ctx context.Context, fullName string) (map[string]int64, error) // bytes per language
    GetCommitActivity(ctx context.Context, fullName string) (*CommitActivity, error) // last 52 weeks
    GetPullCounts(ctx context.Context, fullName string) (open, total int, error) // aggregated via search/list
    GetIssueCounts(ctx context.Context, fullName string) (open, total int, error) // aggregated via search/list
    GetHomepageText(ctx context.Context, url string) (string, error) // single external link (optional)
}

type Contributor struct {
    Login        string
    Contributions int
}
```
Caching TTL distinctions:
- Metadata & topics: staleness threshold (default 14 days)
- Contributors / commit stats / counts: separate 7-day TTL (rate-limit sensitive)
- Languages: 14-day TTL

### 3. Processor (Extraction & Summarization)

Responsibilities:
1. Fetch minimal content (README, optional docs README, optional homepage link text)
2. Compute content hash (ordered concatenation of included sources)
3. If summary absent OR forced OR version mismatch → generate summary (only from README + Description)
4. Generate embeddings (optional; skipped if embedding disabled)

```go
type Processor interface {
    Extract(ctx context.Context, repo github.Repository) (*Extraction, error)
    Summarize(ctx context.Context, readme string, description string) (*Summary, error) // transformers/heuristic
    EmbedRepository(ctx context.Context, summary *Summary) ([]float32, error)          // repo-level vector
}

type Extraction struct {
    Readme       string
    DocsReadme   string // optional
    HomepageText string // optional
    Hash         string
}

type Summary struct {
    Purpose      string
    Technologies []string
    UseCases     []string
    Features     []string
    Installation string
    Usage        string
    GeneratedAt  time.Time
    Version      int
    Generator    string // e.g. "transformers:distilbart-cnn-12-6" / "heuristic"
}
```

### 4. Search Engine (`internal/query` repurposed)

Search fields (fuzzy index composition):
- Repository full_name (split owner + name)
- Description
- Summary fields (purpose, features, usage, installation)
- Topics (joined)
- Top contributor logins (joined; limited to top N=10)

Vector search uses repository-level embedding (summary text). (Chunk-level embeddings deferred unless needed.)

Interface:
```go
type Mode string
const (
    ModeFuzzy  Mode = "fuzzy"
    ModeVector Mode = "vector"
)

type Query struct { Raw string; Mode Mode }

type SearchOptions struct { Limit int; MinScore float64 }

type Result struct {
    RepoID      string
    Score       float64
    Rank        int
    MatchFields []string // best-matching logical fields
}

type Engine interface {
    Search(ctx context.Context, q Query, opts SearchOptions) ([]Result, error)
}
```
Ranking:
- BaseScore = BM25 normalized (fuzzy) OR cosine similarity (vector)
- Tie-break (optional, non-filter) lightweight boosts: star logarithmic + recency decay (documented as internal heuristics; user filtering still unsupported)
- Final Score capped to 1.0

### 5. Related Engine

Computes related repositories for a focus repo (via `related <repo>` or `--related` in query result expansion).

Components:
- Same Organization (binary ⇒ 1.0 else 0)
- Topic Overlap (Jaccard)
- Shared Contributors (normalized intersection of top 10 contributor sets)
- Vector Similarity (cosine; skipped if embeddings disabled)

Weights (available components renormalized): SameOrg 0.30, Topic 0.25, SharedContrib 0.25, Vector 0.20

Explanation template examples:
- "Shared org 'hashicorp' and 3 overlapping topics (terraform, cloud, plugin)"
- "2 shared top contributors (alice, bob) and high vector similarity 0.78"

### 6. Storage Layer

Simplified schema (repository + optional summary + embedding). Content chunks table retained ONLY if/when chunk-level embeddings added (deferred). Current design omits chunk table; embedding is repo-level.

```sql
CREATE TABLE repositories (
    id                  VARCHAR PRIMARY KEY,
    full_name           VARCHAR UNIQUE NOT NULL,
    description         TEXT,
    homepage            TEXT,
    stargazers_count    INTEGER,
    forks_count         INTEGER,
    open_issues_open    INTEGER,
    open_issues_total   INTEGER,
    open_prs_open       INTEGER,
    open_prs_total      INTEGER,
    commits_30d         INTEGER,
    commits_1y          INTEGER,
    commits_total       INTEGER,
    created_at          TIMESTAMP,
    updated_at          TIMESTAMP,
    last_synced         TIMESTAMP,
    topics              VARCHAR[],
    license_spdx_id     VARCHAR,
    license_name        VARCHAR,
    languages           JSON,         -- {"Go":12345,"Rust":2345} lines of code

    -- Summary
    purpose             TEXT,
    technologies        VARCHAR[],
    use_cases           VARCHAR[],
    features            VARCHAR[],
    installation        TEXT,
    usage               TEXT,
    summary_generated_at TIMESTAMP,
    summary_version     INTEGER,
    summary_generator   VARCHAR,

    -- Embedding
    repo_embedding      FLOAT[384],

    content_hash        VARCHAR
);

-- Fuzzy index (implementation-specific pseudo)
CREATE INDEX idx_repos_fts ON repositories USING fts(
  full_name, description, purpose, installation, usage, topics, technologies, features
);

CREATE INDEX idx_repositories_updated_at ON repositories(updated_at);
CREATE INDEX idx_repositories_stars ON repositories(stargazers_count);
```

### 7. Configuration Model

```go
type Config struct {
    Database   DatabaseConfig  `json:"database"`
    Search     SearchConfig    `json:"search"`
    Embeddings EmbeddingConfig `json:"embeddings"`
    Summary    SummaryConfig   `json:"summary"`
    Refresh    RefreshConfig   `json:"refresh"`
    GitHub     GitHubConfig    `json:"github"`
}

type SearchConfig struct { DefaultMode string; MinScore float64 }

type EmbeddingConfig struct { Provider string; Model string; Dimensions int; Enabled bool; Options map[string]string }

type SummaryConfig struct { Version int; TransformersModel string; Enable bool }

type RefreshConfig struct { MetadataStaleDays int; ForceSummary bool; StatsStaleDays int }
```

### 8. Output Formatting (Detailed Specification)

Long-form (each repository):
```
<org>/<name>  (link: https://github.com/<org>/<name>)
GitHub Description: <description or ->
GitHub External Description Link: <homepage or ->
Numbers: <open_issues>/<total_issues> open issues, <open_prs>/<total_prs> open PRs, <stars> stars, <forks> forks
Commits: <commits_30d> in last 30 days, <commits_1y> in last year, <commits_total> total
Age: <humanized (now - created_at)>
License: <license_spdx_id or license_name or ->
Top 10 Contributors: login1 (C1), login2 (C2), ...
GitHub Topics: topic1, topic2, ...
Languages: Lang1 (LOC/approx), Lang2 (...)
Related Stars: <count_same_org> in <org>, <count_shared_contrib> by top contributors
Last synced: <humanized (now - last_synced)>
Summary: <summary purpose/combined> (optional if available)
(PLANNED: dependencies count)
(PLANNED: 'used by' count)
```

Short-form: first two lines of long-form + truncated description (80 chars) + score: `<rank>. <full_name>  ⭐ <stars>  <language primary>  Updated <rel>  Score:<x.xx>`.

Derivations:
- `total_issues` = open issues + closed issues (queried via search API or pagination) cached 7d
- `total_prs` = open PRs + closed PRs (same method)
- `commits_30d` = sum of commit_activity weeks fully or partially within last 30 days
- `commits_1y` = sum last 365 days from weekly commit activity
- `commits_total` = sum all weeks + (optional) prior historical if available; fallback `-1` if stats endpoint not ready (GitHub may return 202); display `?` for unknown
- Languages LOC approximation: (bytes / average_bytes_per_line≈60) rounded; or display raw bytes if estimation disabled
- `count_same_org` = number of other starred repos with same owner (owner must be org, not user) excluding self
- `count_shared_contrib` = count of starred repos that share at least one of top 10 contributors (contributors intersection non-empty)

### 9. Caching & Refresh Strategy

- On `sync`: fetch basic metadata for all starred repos.
- Metadata refresh if `now - last_synced > metadata_stale_days`.
- Stats (issues/prs/commit activity/contributors/languages) refresh if `now - last_synced_stats > stats_stale_days` (stored implicitly via last_synced or per-field timestamps; simplified to reuse `last_synced` if separate columns avoided).
- Summary regenerated ONLY if (a) missing; (b) version mismatch; (c) `--force-summary` or config `ForceSummary`; content hash change alone does not force summary unless forced to minimize cost.
- Embeddings recalculated if summary changed or embedding missing and embeddings enabled.
- All heavy API calls (commit stats) parallelized with limited worker pool + backoff.

### 10. Related Computation Flow

1. Ensure contributor + topics + embedding (if enabled) present or fetch on demand.
2. Preselect candidate set: all other starred repos (optionally prune by same org OR topic overlap >0 OR shared contributor OR vector similarity baseline > threshold).
3. Compute component scores; renormalize missing.
4. Filter out final score <0.25; sort desc; take default limit (5).
5. Build explanation string from highest non-zero components.

### 11. Error Handling (Updated Taxonomy)

Categories: GitHubAPI, Storage, Search, Summary, Embedding, Configuration, Validation, Related.

Fallbacks:
- Embedding failure: fallback to fuzzy search; log warning.
- Commit stats incomplete (202): mark metrics unknown; proceed.
- Related partial data: omit missing components; renormalize.

### 12. Testing Strategy (Revised)

Unit:
- Search Engine: fuzzy score normalization, vector cosine, boost application
- Related Engine: weight renormalization, explanation formatting, thresholds
- Summary: transformer invocation adapter; heuristic fallback
- Refresh: staleness & force logic
- Formatting: long-form builder deterministic assembly

Integration:
- Sync end-to-end (with/without embeddings)
- Query (mode switching, score bounds [0,1])
- Related results determinism given frozen dataset
- Failure injection (simulate missing commit stats)

Performance:
- Sync 500 repos baseline (< target duration) with summary disabled/enabled
- Query latency (median & p95) for fuzzy vs vector

### 13. Performance Considerations

- Batch HTTP requests where APIs allow (topics/languages parallelized with limits)
- Cache commit stats & issue/PR counts (avoid per-query recomputation)
- Single-pass README preprocessing (strip badges) before summary & embedding
- Star/recency boost kept minimal to maintain user-intuitive ranking

### 14. Security & Privacy

- Only targeted file/content retrieval (README + optional docs README + homepage)
- All data local; embeddings provider may send summary text externally (explicit config warning)
- API keys never persisted in database tables

### 15. Future Work

- Reintroduce optional LLM summarization (replace or complement transformers)
- Structured filtering (stars>, language, topic) & advanced query grammar
- Chunk-level embeddings & hybrid BM25 + dense rerank pipeline
- Dependency / dependent metrics integration (GitHub dependency graph)
- Incremental background refresh & partial sync scheduling
- TUI interface (Bubble Tea) for interactive browsing
- Migration engine (golang-migrate) when schema stabilizes

### 16. Changes from Previous Design (Delta vs original & earlier simplified draft)

Referencing `new.md` guidance:
- Removed natural language → SQL parser & interfaces
- Removed temporary LLM integration; summaries now transformer/heuristic only
- Limited summary input strictly to main README + Description (other sources not fed into summary)
- Added explicit long-form output specification with metric definitions
- Added issue/PR counts, commit activity, contributor list, languages, related star metrics to schema
- Eliminated chunk-level embeddings (deferred) and broad file crawling
- Added contributor & language ingestion endpoints; refined caching TTLs
- Added explicit prohibition of structured filtering (only free-text ranking)
- Added Related engine design & scoring weights
- Added summary provenance fields & version gating
- Adjusted storage schema to include activity & count fields
- Clarified ranking heuristics are boosts (not user filters)
- Added precise derivations & placeholder handling for missing stats

## Rationale Summary

The redesign enforces a minimal, deterministic surface aligned with `new.md`: simpler ingestion, low overhead summarization, explicit formatting, and explainable related recommendations. Complexity (LLM parsing, wide crawling, advanced filters) is deferred, improving maintainability and reducing API & cost footprint while retaining meaningful semantic and relational discovery capabilities.
