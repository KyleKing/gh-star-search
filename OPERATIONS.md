# Operations Guide

This document covers operational details that complement [DESIGN.md](DESIGN.md): schema management, configuration reference, caching behavior, rate limiting, and output formatting.

## Database Schema Management

### Current Approach

The database uses sequential SQL migrations stored in `internal/storage/migrations/`. Each migration is a numbered SQL file (e.g., `001_initial_schema.sql`) that runs once when the schema version is behind the code version.

Migrations are applied automatically on first run. The `schema_version` table tracks which migrations have been applied. For major schema changes or corruption, users can clear and re-sync:

```bash
gh star-search clear
gh star-search sync
```

### Schema

The `repositories` table stores one row per starred repo:

| Column                                          | Type              | Purpose                                          |
| ----------------------------------------------- | ----------------- | ------------------------------------------------ |
| `id`                                            | VARCHAR (UUID)    | Primary key                                      |
| `full_name`                                     | VARCHAR UNIQUE    | `owner/name` identifier                          |
| `description`, `homepage`, `language`           | TEXT/VARCHAR      | GitHub metadata                                  |
| `stargazers_count`, `forks_count`, `size_kb`    | INTEGER           | Numeric metrics                                  |
| `created_at`, `updated_at`, `last_synced`       | TIMESTAMP         | Time tracking                                    |
| `open_issues_open/total`, `open_prs_open/total` | INTEGER           | Issue/PR counts                                  |
| `commits_30d`, `commits_1y`, `commits_total`    | INTEGER           | Commit activity                                  |
| `topics_array`, `languages`, `contributors`     | JSON              | Structured metadata                              |
| `license_name`, `license_spdx_id`               | VARCHAR           | License info                                     |
| `content_hash`                                  | VARCHAR           | SHA256 for change detection                      |
| `purpose`                                       | TEXT              | AI-generated summary                             |
| `summary_generated_at`, `summary_version`       | TIMESTAMP/INTEGER | Summary tracking                                 |
| `topics_text`                                   | VARCHAR           | Space-joined topics for FTS indexing             |
| `contributors_text`                             | VARCHAR           | Space-joined contributor logins for FTS indexing |
| `repo_embedding`                                | JSON              | Float32 vector for semantic search               |

### Indexes

Standard indexes exist on: `language`, `updated_at`, `stargazers_count`, `full_name`, and `commits_total`.

A DuckDB FTS index is rebuilt after each sync via `PRAGMA create_fts_index`, covering: `full_name`, `description`, `purpose`, `topics_text`, and `contributors_text` (Porter stemmer, English stopwords). The FTS index does not auto-update -- it must be rebuilt after data changes.

### Adding Migrations

New migrations are added as numbered SQL files in `internal/storage/migrations/`. See `internal/storage/migrations/README.md` for detailed instructions.

Migrations are:

- Embedded in the binary via `//go:embed`
- Applied sequentially on app startup
- Idempotent (safe to run multiple times)
- Non-reversible (no rollback support)

## Embedding and Summarization

### Enabling Embeddings

Embeddings are opt-in. To enable:

1. Install Python 3 and sentence-transformers:

    ```bash
    pip install sentence-transformers
    ```

1. Enable via config file (`~/.config/gh-star-search/config.json`):

    ```json
    {
      "embedding": {
        "provider": "local",
        "model": "intfloat/e5-small-v2",
        "dimensions": 384,
        "enabled": true
      }
    }
    ```

    Or via environment variable:

    ```bash
    export GH_STAR_SEARCH_EMBEDDING_ENABLED=true
    ```

1. Run sync with the `--embed` flag:

    ```bash
    gh star-search sync --embed
    ```

### Embedding Models

| Model                            | Dimensions | RAM    | Quality                    |
| -------------------------------- | ---------- | ------ | -------------------------- |
| `intfloat/e5-small-v2` (default) | 384        | ~300MB | Excellent (118M params)    |
| `Qwen/Qwen3-Embedding-0.6B`      | 384-1024   | ~1GB   | Latest (configurable dims) |
| `all-mpnet-base-v2`              | 768        | ~600MB | Good                       |

The embedding input is built from: `full_name`, `purpose` (summary), `description`, and `topics`, joined with ". " separators.

### Summarization Pipeline

Summaries are generated via `scripts/summarize.py`, invoked as a Python subprocess. Two methods are available:

- **Heuristic** (no dependencies): Keyword-based extractive summarization using sentence scoring. ~10MB RAM.
- **Transformers** (requires `pip install transformers torch`): Uses DistilBART (`sshleifer/distilbart-cnn-12-6`). ~1.5GB RAM.

The script auto-detects available dependencies and falls back from transformers to heuristic. Run sync with `--summarize`:

```bash
gh star-search sync --summarize
```

### Fallback Behavior

| Scenario                              | Result                                             |
| ------------------------------------- | -------------------------------------------------- |
| Python not in PATH                    | Summarization/embedding skipped, warning logged    |
| `sentence-transformers` not installed | Embeddings skipped; vector search returns an error |
| `transformers` not installed          | Heuristic summarization used instead               |
| Script execution fails                | Warning logged, sync continues for remaining repos |
| Individual repo fails to embed        | Skipped, other repos continue                      |

## GitHub API Rate Limiting

### Request Pacing

The sync command applies fixed delays to stay within GitHub's rate limits:

| Context                                 | Delay     |
| --------------------------------------- | --------- |
| Between paginated starred-repo pages    | 100ms     |
| Between individual repo content fetches | 50ms      |
| Between processing batches              | 2 seconds |
| Between repos within a batch            | 100ms     |

### Batch Processing

Repos are processed in configurable batches (default 10, flag `--batch-size`). Worker concurrency scales with batch size and CPU count:

| Batch Size | Workers      |
| ---------- | ------------ |
| 1-5        | 1            |
| 6-10       | min(2, CPUs) |
| 11-20      | min(3, CPUs) |
| 21+        | min(CPUs, 8) |

### Error Handling

- **HTTP 404**: Silently skipped for optional content (e.g., `docs/README.md`)
- **HTTP 202 Accepted**: GitHub stats endpoints return 202 when computing data asynchronously. The client returns empty data and the sync continues.
- **Rate limit errors**: Returned as structured `rate_limit` errors with the reset time in the error context. Currently there is no automatic retry/backoff -- the user is advised to wait and re-run sync.

### Reducing API Usage

- Sync is incremental: repos are skipped if `last_synced` is within the staleness threshold (default 14 days)
- Content is re-fetched only when the `content_hash` changes or metadata fields differ
- Use `--repo owner/name` to sync a single repository

## Cache Eviction Policy

The file cache (`~/.cache/gh-star-search/`) stores downloaded content with two eviction mechanisms:

### TTL Expiration

Each cache entry has an expiration timestamp set at write time (default TTL: 24 hours). Expired entries are removed:

- On read (lazy deletion when a `Get` encounters an expired entry)
- By a background goroutine that runs at a configurable frequency (default: 1 hour)

### Size-Based Eviction

When inserting a new entry would exceed the max cache size (default: 500MB), the oldest entries (by filesystem modification time) are removed until sufficient space is available. This is an LRU-like policy based on file mtime rather than access tracking.

### Cache File Layout

Each entry is stored as two files keyed by the first 16 characters of the SHA256 hash of the cache key:

```
~/.cache/gh-star-search/
  a1b2c3d4e5f6g7h8.data   # cached content
  a1b2c3d4e5f6g7h8.meta   # JSON metadata (key, created_at, expires_at, size)
```

### Configuration

| Setting            | Env Var                                    | Default                   |
| ------------------ | ------------------------------------------ | ------------------------- |
| Directory          | `GH_STAR_SEARCH_CACHE_DIR`                 | `~/.cache/gh-star-search` |
| Max size           | `GH_STAR_SEARCH_CACHE_MAX_SIZE_MB`         | 500 MB                    |
| TTL                | `GH_STAR_SEARCH_CACHE_TTL_HOURS`           | 24 hours                  |
| Cleanup frequency  | `GH_STAR_SEARCH_CACHE_CLEANUP_FREQ`        | 1h                        |
| Metadata staleness | `GH_STAR_SEARCH_CACHE_METADATA_STALE_DAYS` | 14 days                   |
| Stats staleness    | `GH_STAR_SEARCH_CACHE_STATS_STALE_DAYS`    | 7 days                    |

## Output Format Specification

### Long-Form

Displayed for `info`, `query --long`, and `list --long`. Fields appear in this order:

```
owner/repo  (link: https://github.com/owner/repo)
GitHub Description: <description or ->
GitHub External Description Link: <homepage or ->
Numbers: <open>/<total> open issues, <open>/<total> open PRs, <stars> stars, <forks> forks
Commits: <30d> in last 30 days, <1y> in last year, <total> total
Age: <humanized duration since created_at>
License: <SPDX ID, or license name, or ->
Top 10 Contributors: login1 (count), login2 (count), ...
GitHub Topics: topic1, topic2, ...
Languages: Lang1 (approx LOC), Lang2 (approx LOC), ...
Related Stars: <count> in <org>, <count> by top contributors
Last synced: <humanized duration since last_synced>
```

### Short-Form

Displayed for `query` (default), `list` (default). Shows the first two lines of long-form plus a summary line:

```
owner/repo  (link: https://github.com/owner/repo)
GitHub Description: <description or ->
<rank>. owner/repo (<description truncated to 80 chars>)  <stars>  <language>  Updated <age>  Score:<0.00-1.00>
```

### Formatting Rules

| Field               | Rule                                                              |
| ------------------- | ----------------------------------------------------------------- |
| Unknown integers    | Displayed as `?` (negative values)                                |
| Missing strings     | Displayed as `-`                                                  |
| Zero timestamps     | Displayed as `?`                                                  |
| Description (short) | Truncated to 80 characters with `...` suffix                      |
| Contributors        | Limited to top 10, sorted by contribution count descending        |
| Languages           | Sorted by byte count descending; LOC approximated as `bytes / 60` |
| Age                 | `today`, `N days ago`, `N months ago`, `N years ago`              |

## Configuration Reference

Configuration is resolved in order: defaults -> JSON file -> environment variables -> CLI flags.

### Config File

Located at `~/.config/gh-star-search/config.json` (override with `GH_STAR_SEARCH_CONFIG` env var):

```json
{
  "database": {
    "path": "~/.config/gh-star-search/database.db",
    "max_connections": 10,
    "max_idle_conns": 5,
    "conn_max_lifetime": "30m",
    "conn_max_idle_time": "5m",
    "query_timeout": "30s"
  },
  "cache": {
    "directory": "~/.cache/gh-star-search",
    "max_size_mb": 500,
    "ttl_hours": 24,
    "cleanup_frequency": "1h",
    "metadata_stale_days": 14,
    "stats_stale_days": 7
  },
  "logging": {
    "level": "info",
    "format": "text",
    "output": "stdout",
    "file": "~/.config/gh-star-search/logs/app.log",
    "max_size_mb": 10,
    "max_backups": 5,
    "max_age_days": 30,
    "add_source": false
  },
  "debug": {
    "enabled": false,
    "profile_port": 6060,
    "metrics_port": 8080,
    "verbose": false,
    "trace_api": false
  }
}
```

### Environment Variables

All environment variables use the `GH_STAR_SEARCH_` prefix:

| Variable                            | Default                                | Description                          |
| ----------------------------------- | -------------------------------------- | ------------------------------------ |
| `GH_STAR_SEARCH_DB_PATH`            | `~/.config/gh-star-search/database.db` | Database file path                   |
| `GH_STAR_SEARCH_DB_MAX_CONNECTIONS` | `10`                                   | Max open DB connections              |
| `GH_STAR_SEARCH_DB_QUERY_TIMEOUT`   | `30s`                                  | Query timeout duration               |
| `GH_STAR_SEARCH_CACHE_DIR`          | `~/.cache/gh-star-search`              | Cache directory                      |
| `GH_STAR_SEARCH_CACHE_MAX_SIZE_MB`  | `500`                                  | Max cache size in MB                 |
| `GH_STAR_SEARCH_CACHE_TTL_HOURS`    | `24`                                   | Default cache entry TTL              |
| `GH_STAR_SEARCH_LOG_LEVEL`          | `info`                                 | Log level (debug/info/warn/error)    |
| `GH_STAR_SEARCH_LOG_FORMAT`         | `text`                                 | Log format (text/json)               |
| `GH_STAR_SEARCH_LOG_OUTPUT`         | `stdout`                               | Log destination (stdout/stderr/file) |
| `GH_STAR_SEARCH_DEBUG`              | `false`                                | Enable debug mode                    |
| `GH_STAR_SEARCH_VERBOSE`            | `false`                                | Enable verbose output                |
| `GH_STAR_SEARCH_EMBEDDING_ENABLED`  | `false`                                | Enable vector embeddings             |

### Validation

The configuration is validated at load time. Invalid values produce an error before any command runs:

- `level` must be one of: `debug`, `info`, `warn`, `error`
- `format` must be one of: `text`, `json`
- `output` must be one of: `stdout`, `stderr`, `file`
- Duration fields (`query_timeout`, `cleanup_frequency`, `conn_max_lifetime`) must parse as Go durations
- `max_connections` must be positive

### File Locations

| Purpose     | Path                                    |
| ----------- | --------------------------------------- |
| Config file | `~/.config/gh-star-search/config.json`  |
| Database    | `~/.config/gh-star-search/database.db`  |
| Cache       | `~/.cache/gh-star-search/`              |
| Logs        | `~/.config/gh-star-search/logs/app.log` |

All directories are auto-created on first use. Paths starting with `~` are expanded to the user's home directory.

## Structured Error Types

The application uses typed errors with context, suggestions, and filtered stack traces. Each error carries:

- **Type**: Category for programmatic handling (`github_api`, `database`, `validation`, `rate_limit`, `not_found`, `config`, `network`, `auth`, `filesystem`, `internal`)
- **Context**: Key-value pairs (e.g., `status_code`, `operation`, `path`)
- **Suggestions**: User-facing resolution steps
- **Stack**: Filtered to project frames only (excludes standard library and dependencies)

In debug mode (`--debug`), the full structured error including stack trace is printed. In normal mode, only the message and suggestions are shown.

### Common Error Patterns

| Error Type   | Typical Cause             | Suggestions                             |
| ------------ | ------------------------- | --------------------------------------- |
| `github_api` | API failure, bad response | Check `gh auth status`, verify internet |
| `rate_limit` | Too many API requests     | Wait for reset, upgrade token           |
| `database`   | DuckDB file issue         | Check permissions, check disk space     |
| `auth`       | Missing or invalid token  | Run `gh auth login`                     |
| `config`     | Invalid config value      | Check file syntax, run `--help`         |
| `not_found`  | Repo not in database      | Verify resource exists, check access    |
