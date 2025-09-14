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

- [ ] 3. Minimal Content Extraction
   - Fetch only: Description, main README, optional `docs/README.md`, optional single homepage URL text.
   - Compute deterministic `content_hash` (ordered sources) for change detection (not auto-forcing summary regeneration).

- [ ] 4. Transformers-Based Summarization Pipeline
   - Implement `Processor.Summarize` using configured Python `transformers` model (e.g., DistilBART) constrained to Description + README.
   - Heuristic fallback (section parsing) if Python unavailable or disabled.
   - Versioning & generator metadata recorded; gated by config.

- [ ] 5. Optional Embedding Generation
   - Single repository-level embedding over concatenated summary text (select key fields: purpose, features, usage).
   - Provider abstraction (local model / remote API) with `Enabled` flag & dimensionality validation.
   - Graceful fallback to fuzzy search if disabled or failure.

- [ ] 6. Dual-Mode Search Engine
   - Implement `Engine` with `ModeFuzzy` (BM25 / FTS) and `ModeVector` (cosine).
   - Ranking boosts: star logarithmic (+small), recency decay; clamp final score ≤1.0.
   - Track matched logical fields for explanation.
   - Add configuration for default mode & min score.

- [ ] 7. Related Engine
   - Compute weighted score: SameOrg(0.30), Topics(0.25), SharedContrib(0.25), Vector(0.20) with renormalization when components missing.
   - Explanation string assembly (top non-zero contributors).
   - CLI integration via `related <repo>` command and `query --related` augmentation.

- [ ] 8. Metrics Ingestion & Derivations
   - Issues / PR counts (open + total) cached 7d.
   - Commit activity aggregation (30d, 1y, total; handle 202 retry state with placeholders).
   - Top 10 contributors (login + contributions); topics; languages (bytes → LOC estimate or raw).
   - Related star counts (same org, shared contributor repos) computed on demand (not persisted).

- [ ] 9. Caching & Refresh Logic
   - Implement configurable `metadata_stale_days` & `stats_stale_days` (if separate) else unified logic.
   - Only refresh summaries when (missing | version mismatch | forced flag/config).
   - Recompute embeddings only if summary changed or embedding missing.
   - Parallel worker pool with backoff for rate-limited endpoints.

- [ ] 10. Output Formatting (Long & Short Forms)
   - Implement exact long-form spec (Lines: header link, Description, External link, Numbers, Commits, Age, License, Contributors, Topics, Languages, Related Stars, Last synced, Summary, planned placeholders).
   - Short-form = first two lines + score + truncated description (80 chars) + primary language.
   - Golden tests for deterministic formatting & unknown-value fallbacks ("?" / "-").

- [ ] 11. CLI Enhancements & Flags
   - `--mode`, `--limit`, `--long/--short`, `--related` integration; validation & helpful messages for unsupported structured filters.
   - Ensure default limit 10 (max 50) enforced uniformly.

- [ ] 12. Configuration Model Refactor
   - Add `SearchConfig`, `SummaryConfig`, `EmbeddingConfig`, `RefreshConfig` per design.
   - Validate dimensions vs embedding provider; emit clear error if mismatch.
   - Remove deprecated parser / LLM config fields.

- [ ] 13. Python Integration Layer
   - Detect Python & required `transformers` package; provide installation guidance on failure.
   - Timeout & memory guard; structured errors surfaced as Summary fallback warnings.

- [ ] 14. Testing Expansion
   - Unit: search scoring, vector similarity, related weighting & renorm, summary fallback, refresh gating, formatting builder.
   - Integration: sync (with & without embeddings), query (mode switch, score bounds), related deterministic outputs.
   - Failure injection: missing commit stats, embedding failure.
   - Performance: sync 500 repos (baseline target), query latency p50/p95 for both modes.

- [ ] 15. CONTRIBUTING.md
   - Architecture snapshot, endpoint usage & rate limits, caching strategy, summarization & embedding disclaimers, development workflow.

- [ ] 16. Logging & Error Taxonomy Alignment
   - Centralize error categories: GitHubAPI, Storage, Search, Summary, Embedding, Configuration, Validation, Related.
   - Add structured warning for partial data (e.g., commit stats unready, embedding skipped).

- [ ] 17. Packaging & Release (Carryover)
   - GitHub CLI extension manifest, cross-platform build, release workflow, install/update validation.

- [ ] 18. Performance & Resource Optimizations
   - Batch/lazy fetch where possible; concurrency tuning; measure memory footprint for large star sets.

- [ ] 19. Future Feature Gate Placeholders
   - Stubs / TODO comments for: structured filtering, chunk embeddings, dependency metrics, LLM summaries, export, TUI.

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
- Removed: Old Task 5 (LLM summarization) → replaced by new Task 4 (transformers) & Task 19 (future LLM reintroduction placeholder).
- Removed: Old Task 7 (natural language query parser) and related interactive SQL editing (superseded by direct search Tasks 6 & 11).
- Modified: Old Task 4 (broad content extraction & chunking) → narrowed to Task 3 (minimal extraction) with no chunk table.
- Modified: Old Task 8 (search & query execution) → split into Tasks 6 (dual-mode engine) & 11 (CLI flags) with simplified scope.
- Modified: Old Task 10 (caching/performance) → refined into Tasks 8 (metrics), 9 (refresh logic), 18 (performance tuning).
- Modified: Old Task 12 (test suite & docs) → expanded across Tasks 14 (testing) & 15 (CONTRIBUTING.md).
- Retained/Reframed: Old Task 13 (packaging) → Task 17; advanced polish elements of Old Task 14 moved to Deferred / Future Work or integrated into specific tasks (formatting, related, performance).
- Added: New tasks for related engine (Task 7), output formatting spec compliance (Task 10), embedding optionalization (Task 5), Python integration safeguards (Task 13), error taxonomy (Task 16).

## Progress Tracking Notes
Mark tasks as completed directly here as work proceeds. If schema changes break backward compatibility, document manual migration steps in CONTRIBUTING.md and increment summary version.
