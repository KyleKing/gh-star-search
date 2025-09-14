# Implementation Plan (Simplified Architecture Update 2025-09-14)

This plan supersedes the prior (legacy) task list to align with `new.md` and the updated design (`design.md`). Natural language → SQL parsing, LLM summarization, broad repository crawling, and chunk-level embeddings are removed or deferred. The focus now: minimal ingestion, deterministic transformers-based summaries, dual-mode (fuzzy/vector) search, and a Related engine with explainable scoring.

## Completed (Legacy Foundation)
These foundational capabilities remain valid and are retained (no rework unless noted):
- [x] Project structure & core interfaces (CLI skeleton, basic layering)
- [x] GitHub client (starred repos, basic metadata fetch, auth)
- [x] Initial DuckDB storage (baseline repo table)
- [x] Sync command (incremental detection, hashing, parallel fetch basics)
- [x] Repository management commands (`list`, `info`, `stats`, `clear`)
- [x] Configuration & logging framework, error handling baseline
- [x] Basic caching & performance primitives (parallelism, hashing)
- [x] Content extraction and processing (README, basic summarization)
- [x] Basic text search functionality (ILIKE-based search in DuckDB)

LLM, NL query parser, and wide content extraction pieces are now deprecated (see Changes section).

## Phase 2 – Simplification & New Core Features (Pending)
- [x] 1. Remove Legacy NL & LLM Code
    - Delete / archive `internal/llm/` and NL query parser code & references.
    - Strip interactive SQL editing & confidence scoring paths.
    - Ensure build passes after removal; add deprecation note in CHANGELOG (if present).

- [x] 2. Storage Schema Update
    - Add activity & metrics fields (issues/prs counts, commit activity, contributor/topic arrays, languages JSON, summary & embedding columns, related counts as derivable not stored).
    - Remove / do not recreate `content_chunks` table (unless future re-enabled).
    - Add `repo_embedding FLOAT[384]`, summary fields, `content_hash`.
    - Provide lightweight migration: detect old schema -> prompt auto rebuild (delete DB) or run additive ALTERs if feasible.

- [x] 3. Minimal Content Extraction
    - Fetch only: Description, main README, optional `docs/README.md`, optional single homepage URL text.
    - Compute deterministic `content_hash` (ordered sources) for change detection (not auto-forcing summary regeneration).

- [x] 4. Basic Summarization Pipeline (Heuristic)
    - Implement basic heuristic summarization in `Processor.GenerateSummary` that extracts purpose from README and technologies from package files.
    - Versioning & generator metadata recorded; marked as "heuristic" generator.
    - _Requirements: 1.4, 8.4_

- [ ] 5. Enhanced GitHub API Integration, Metrics Ingestion, and Caching
    - Implement missing GitHub API endpoints: GetContributors, GetTopics, GetLanguages, GetCommitActivity, GetPullCounts, GetIssueCounts.
    - Add GetHomepageText for optional external link scraping.
    - Issues / PR counts (open + total) cached 7d; commit activity aggregation (30d, 1y, total; handle 202 retry state with placeholders); top 10 contributors (login + contributions); topics; languages (bytes → LOC estimate or raw); related star counts (same org, shared contributor repos) computed on demand (not persisted).
    - Implement proper caching with TTL distinctions (metadata 14d, stats 7d); configurable `metadata_stale_days` & `stats_stale_days`; only refresh summaries when (missing | version mismatch | forced flag/config); recompute embeddings only if summary changed or embedding missing; parallel worker pool with backoff for rate-limited endpoints.
    - _Requirements: 1.2, 1.3, 5.1, 5.2, 5.4_

- [ ] 6. Dual-Mode Search and Related Engine Implementation
    - Create `cmd/query.go` with flags: `--mode (fuzzy|vector)`, `--limit`, `--long/--short`; implement query string validation (length ≥ 2, reject structured filters with helpful message); wire up to existing SearchRepositories with mode selection.
    - Enhance existing text search to implement proper BM25/FTS scoring for fuzzy mode; add vector search mode with cosine similarity (requires embeddings); implement ranking boosts: star logarithmic (+small), recency decay; clamp final score ≤1.0; track matched logical fields for explanation.
    - Single repository-level embedding over concatenated summary text (select key fields: purpose, features, usage); provider abstraction (local model / remote API) with `Enabled` flag & dimensionality validation; graceful fallback to fuzzy search if disabled or failure.
    - Create `internal/related/` package for related repository computation; compute weighted score: SameOrg(0.30), Topics(0.25), SharedContrib(0.25), Vector(0.20) with renormalization when components missing; explanation string assembly (top non-zero contributors); CLI integration via `related <repo>` command and `query --related` augmentation.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 8.4_

- [ ] 7. Output Formatting (Long & Short Forms)
    - Create `internal/formatter/` package for output rendering.
    - Implement exact long-form spec (Lines: header link, Description, External link, Numbers, Commits, Age, License, Contributors, Topics, Languages, Related Stars, Last synced, Summary, planned placeholders).
    - Short-form = first two lines of long-form + score + truncated description (80 chars) + primary language.
    - Golden tests for deterministic formatting & unknown-value fallbacks ("?" / "-").
    - _Requirements: 2.4, 6.2, 6.3_

- [ ] 8. Transformers-Based Summarization Pipeline
    - Detect Python & required `transformers` package; provide installation guidance on failure.
    - Implement `Processor.Summarize` using configured Python `transformers` model (e.g., DistilBART) constrained to Description + README.
    - Timeout & memory guard; structured errors surfaced as Summary fallback warnings.
    - Versioning & generator metadata recorded; gated by config.
    - _Requirements: 1.4, 8.4_

- [ ] 9. Configuration Model Refactor
    - Add `SearchConfig`, `SummaryConfig`, `EmbeddingConfig`, `RefreshConfig` per design.
    - Validate dimensions vs embedding provider; emit clear error if mismatch.
    - Remove deprecated parser / LLM config fields.
    - _Requirements: 7.3_

- [ ] 10. Testing Expansion
    - Unit: search scoring, vector similarity, related weighting & renorm, summary fallback, refresh gating, formatting builder.
    - Integration: sync (with & without embeddings), query (mode switch, score bounds), related deterministic outputs.
    - Failure injection: missing commit stats, embedding failure.
    - Performance: sync 500 repos (baseline target), query latency p50/p95 for both modes.
    - _Requirements: 7.3_

- [ ] 11. Documentation, Logging & Error Taxonomy
    - Create CONTRIBUTING.md with architecture snapshot, endpoint usage & rate limits, caching strategy, summarization & embedding disclaimers, development workflow.
    - Centralize error categories: GitHubAPI, Storage, Search, Summary, Embedding, Configuration, Validation, Related; add structured warning for partial data (e.g., commit stats unready, embedding skipped).
    - _Requirements: 7.1, 7.2, 9.1, 9.2, 9.3_

- [ ] 12. Packaging & Release (Carryover)
    - GitHub CLI extension manifest, cross-platform build, release workflow, install/update validation.
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

- [ ] 13. Performance & Resource Optimizations
    - Batch/lazy fetch where possible; concurrency tuning; measure memory footprint for large star sets.
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

- [ ] 14. Future Feature Gate Placeholders
    - Stubs / TODO comments for: structured filtering, chunk embeddings, dependency metrics, LLM summaries, export, TUI.
    - _Requirements: Deferred features_

## Deferred / Future Work (Not in Current Scope)
- Reintroduce optional LLM summarization.
- Structured filters (stars, language, topic queries) & advanced grammar.
- Chunk-level embeddings & hybrid reranking pipeline.
- Dependency & dependent metrics (GitHub dependency graph).
- Background incremental refresh scheduler.
- TUI (Bubble Tea) interactive browser.
- Migration engine (golang-migrate) once schema stabilizes.
- Export functionality & advanced analytics.

## Changes (vs Previous tasks.md)
- Removed: Old Task 5 (LLM summarization) → replaced by new Task 4 (transformers) & Task 14 (future LLM reintroduction placeholder).
- Removed: Old Task 7 (natural language query parser) and related interactive SQL editing (superseded by direct search Tasks 6 & 11).
- Modified: Old Task 4 (broad content extraction & chunking) → narrowed to Task 3 (minimal extraction) with no chunk table.
- Modified: Old Task 8 (search & query execution) → split into Tasks 6 (dual-mode engine) & 11 (CLI flags) with simplified scope.
- Modified: Old Task 10 (caching/performance) → refined into Tasks 5 (metrics & caching), 13 (performance tuning).
- Modified: Old Task 12 (test suite & docs) → expanded across Tasks 10 (testing) & 11 (CONTRIBUTING.md & logging).
- Retained/Reframed: Old Task 13 (packaging) → Task 12; advanced polish elements of Old Task 14 moved to Deferred / Future Work or integrated into specific tasks (formatting, related, performance).
- Added: New tasks for related engine (Task 6), output formatting spec compliance (Task 7), embedding optionalization (Task 6), Python integration safeguards (Task 8), error taxonomy (Task 11).
- Merged: Tasks 5,10,11 into new Task 5; Tasks 6,7,8,9 into new Task 6; Tasks 16,17 into new Task 11; Tasks 12-20 renumbered accordingly.

## Progress Tracking Notes
Mark tasks as completed directly here as work proceeds. If schema changes break backward compatibility, document manual migration steps in CONTRIBUTING.md and increment summary version.
