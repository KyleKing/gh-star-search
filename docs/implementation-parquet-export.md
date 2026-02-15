# Implementation Plan: Parquet Analytics Cache

Inspired by msgvault's denormalized Parquet files for fast aggregate queries (~3000x over SQLite JOINs), partitioned by year, with auto-build on TUI launch.

## Goal

Export a denormalized Parquet file from DuckDB for fast aggregate analytics. This provides a read-optimized format for the TUI's aggregate views (language counts, topic counts, org counts) and enables external analysis tools (DuckDB CLI, Python/pandas, Observable) to query starred repo data without touching the live database.

## Problem

The current DuckDB database stores repositories in a single normalized table with JSON columns for topics, languages, and contributors. Aggregate queries that unnest these JSON arrays are fast enough for CLI use but add unnecessary overhead in a TUI where the same aggregates are computed on every view switch. A denormalized Parquet file pre-computes the unnesting and allows millisecond aggregates.

## Design

### Parquet Schema

A single denormalized Parquet file with one row per repository, JSON arrays expanded to DuckDB list columns:

```sql
CREATE TABLE parquet_export AS
SELECT
    id,
    full_name,
    split_part(full_name, '/', 1) AS org,
    description,
    language AS primary_language,
    stargazers_count,
    forks_count,
    size_kb,
    created_at,
    updated_at,
    last_synced,
    open_issues_open,
    open_issues_total,
    open_prs_open,
    open_prs_total,
    commits_30d,
    commits_1y,
    commits_total,
    topics_array::VARCHAR[] AS topics,
    json_keys(languages) AS language_names,
    json_extract_string(languages, '$.*')::VARCHAR[] AS language_bytes,
    license_name,
    license_spdx_id,
    purpose,
    YEAR(created_at) AS created_year,
    YEAR(updated_at) AS updated_year
FROM repositories;
```

### File Layout

```
~/.config/gh-star-search/
  analytics/
    repositories.parquet          -- denormalized export
    _export_meta.json             -- export metadata
```

Metadata file:

```json
{
  "exported_at": "2026-02-14T10:30:00Z",
  "row_count": 1247,
  "source_db_path": "~/.config/gh-star-search/database.db",
  "version": 1
}
```

### No Partitioning

Unlike msgvault (which partitions by year across hundreds of thousands of emails), gh-star-search typically holds hundreds to low-thousands of repos. A single Parquet file is sufficient. Partitioning adds complexity without meaningful benefit at this scale.

### Build Command

```go
// cmd/build_cache.go
func buildCacheCommand() *cli.Command {
    return &cli.Command{
        Name:  "build-cache",
        Usage: "Build Parquet analytics cache from database",
        Flags: []cli.Flag{
            &cli.BoolFlag{
                Name:  "force",
                Usage: "Rebuild even if cache is fresh",
            },
        },
        Action: func(ctx context.Context, cmd *cli.Command) error {
            repo, err := initializeStorage(ctx, cmd)
            if err != nil { return err }
            defer repo.Close()

            return buildParquetCache(ctx, repo, cmd.Bool("force"))
        },
    }
}
```

### Build Logic

```go
// internal/storage/parquet.go

func (r *DuckDBRepository) ExportParquet(ctx context.Context, outputPath string) error {
    query := `
        COPY (
            SELECT
                id,
                full_name,
                split_part(full_name, '/', 1) AS org,
                description,
                language AS primary_language,
                stargazers_count,
                forks_count,
                size_kb,
                created_at,
                updated_at,
                last_synced,
                open_issues_open,
                open_issues_total,
                open_prs_open,
                open_prs_total,
                commits_30d,
                commits_1y,
                commits_total,
                topics_array::VARCHAR[] AS topics,
                license_name,
                license_spdx_id,
                purpose,
                YEAR(created_at) AS created_year,
                YEAR(updated_at) AS updated_year
            FROM repositories
        ) TO ? (FORMAT PARQUET, COMPRESSION ZSTD);
    `
    _, err := r.db.ExecContext(ctx, query, outputPath)
    return err
}
```

### Freshness Check

The cache is considered fresh if `_export_meta.json` exists and its `exported_at` is after the most recent `last_synced` in the database:

```go
func isCacheFresh(ctx context.Context, repo storage.Repository, metaPath string) bool {
    meta, err := loadExportMeta(metaPath)
    if err != nil { return false }

    stats, err := repo.GetStats(ctx)
    if err != nil { return false }

    return meta.ExportedAt.After(stats.LastSyncTime)
}
```

### Auto-Build on TUI Launch

The TUI command checks cache freshness on startup and rebuilds if stale:

```go
// cmd/tui.go (within Action)
analyticsDir := filepath.Join(configDir, "analytics")
metaPath := filepath.Join(analyticsDir, "_export_meta.json")
parquetPath := filepath.Join(analyticsDir, "repositories.parquet")

if !isCacheFresh(ctx, repo, metaPath) {
    fmt.Fprintln(os.Stderr, "Building analytics cache...")
    if err := repo.ExportParquet(ctx, parquetPath); err != nil {
        return fmt.Errorf("build cache: %w", err)
    }
    writeExportMeta(metaPath, rowCount)
}
```

### TUI Aggregate Queries Over Parquet

The TUI's aggregate queries read from the Parquet file directly instead of the live database:

```go
func (r *DuckDBRepository) GetLanguageCountsFromParquet(ctx context.Context, path string) ([]AggregateRow, error) {
    query := `
        SELECT primary_language AS name, COUNT(*) AS count, SUM(stargazers_count) AS total_stars
        FROM read_parquet(?)
        WHERE primary_language IS NOT NULL
        GROUP BY primary_language
        ORDER BY count DESC
    `
    return r.queryAggregateRows(ctx, query, path)
}

func (r *DuckDBRepository) GetTopicCountsFromParquet(ctx context.Context, path string) ([]AggregateRow, error) {
    query := `
        SELECT unnest(topics) AS name, COUNT(*) AS count, SUM(stargazers_count) AS total_stars
        FROM read_parquet(?)
        GROUP BY name
        ORDER BY count DESC
    `
    return r.queryAggregateRows(ctx, query, path)
}
```

DuckDB can query Parquet files directly via `read_parquet()` without loading them into memory, making this efficient even for large exports.

### External Tool Access

The Parquet file is a standard format queryable by external tools:

```bash
# DuckDB CLI
duckdb -c "SELECT primary_language, COUNT(*) FROM read_parquet('~/.config/gh-star-search/analytics/repositories.parquet') GROUP BY 1 ORDER BY 2 DESC LIMIT 10"

# Python
import duckdb
df = duckdb.sql("SELECT * FROM read_parquet('repositories.parquet')").df()
```

## Implementation Order

1. Add `ExportParquet()` method to `DuckDBRepository`
1. Add `_export_meta.json` read/write helpers
1. Add `isCacheFresh()` check
1. Create `cmd/build_cache.go` command
1. Wire into `main.go` command list
1. Add Parquet-backed aggregate query methods
1. Integrate auto-build into TUI startup (after TUI is implemented)
1. Tests: export produces valid Parquet, freshness check logic, aggregate queries return correct counts

## Scope Boundaries

- Single flat Parquet file, no partitioning
- No incremental export (full rebuild each time -- fast enough at this scale)
- No embeddings in Parquet (vectors are large and only needed for search, not aggregates)
- Languages JSON is not fully denormalized (would require a separate row-per-language table); only `primary_language` is exported as a column. Topic unnesting is handled at query time via DuckDB's `unnest()`
- Export does not include README content or raw text
