# Contributing to gh-star-search

## Architecture Overview

gh-star-search is a GitHub CLI extension built in Go that searches and discovers relationships across starred repositories. It follows a layered architecture:

```
CLI Layer (cmd/)  -->  Core Packages (internal/)  -->  DuckDB + GitHub API
```

### Design Principles

1. **Local-First**: All data stored in a local DuckDB database
2. **Minimal Content**: Only essential content ingested (no full git clones)
3. **Incremental Sync**: Staleness detection avoids unnecessary API calls
4. **Security-First**: Parameterized queries only
5. **Rate-Limited**: Built-in delays between API requests

## Project Structure

```
cmd/                      # CLI commands (sync, query, list, info, stats, clear, related, config)
internal/
  cache/                  # File-based caching with TTL
  config/                 # Configuration models & loading
  embedding/              # Embedding provider interface (local Python + remote)
  errors/                 # Structured error types
  formatter/              # Output formatting (long/short)
  github/                 # GitHub API client + VCR test helpers
  logging/                # Structured logging with slog
  processor/              # Content extraction & processing
  query/                  # Search engine (fuzzy + vector)
  related/                # Related repository engine (streaming batch)
  storage/                # DuckDB persistence layer
  summarizer/             # Python subprocess summarization
  types/                  # Shared type definitions
scripts/                  # Python helper scripts (summarize.py, embed.py)
```

## GitHub API Integration

| Endpoint | Purpose | Cache TTL |
|----------|---------|-----------|
| `GET /user/starred` | List starred repos | 14 days |
| `GET /repos/{owner}/{repo}` | Metadata | 14 days |
| `GET /repos/{owner}/{repo}/contributors` | Top contributors | 7 days |
| `GET /repos/{owner}/{repo}/topics` | Topics | 7 days |
| `GET /repos/{owner}/{repo}/languages` | Language stats | 7 days |
| `GET /repos/{owner}/{repo}/contents/{path}` | README content | Content-hash |
| `GET /repos/{owner}/{repo}/stats/participation` | Commit activity | 7 days |

**Rate Limiting**: 100ms delay per repository, 2s delay per batch (both configurable). GitHub allows 5,000 requests/hour for authenticated users.

**Authentication**: Uses `gh` CLI auth via `github.com/cli/go-gh`.

## Development

### Prerequisites

- Go 1.24+
- `gh` CLI installed and authenticated
- `mise` (optional, for task runner)

### Setup

```bash
go mod download
go test ./...
go build -o gh-star-search
```

### Using Mise

```bash
mise run ci        # lint, test, format
mise run test      # tests with coverage
mise run build     # build binary
mise run format    # format code
```

### Code Style

- Formatting: `gofmt` (enforced by git hooks)
- Linting: `golangci-lint` (30+ rules)
- SQL: `sqlfluff` validation
- Naming: `ls-lint` validation

## Testing

### Test Types

- **Unit** (`*_test.go`): Isolated function tests alongside source files
- **Integration** (`*_integration_test.go`): Component interactions, may need `gh` auth
- **Performance** (`*_performance_test.go`): Benchmarks for critical paths

### Running Tests

```bash
go test ./...              # all tests
go test -short ./...       # skip integration tests
go test -cover ./...       # with coverage
go test -race ./...        # with race detector
go test -bench=. ./...     # benchmarks
```

### Test Constants

Common constants in `internal/testing/constants.go` -- use these instead of magic numbers.

### HTTP Mocking

GitHub API tests use a mix of manual mocks and go-vcr v4 for HTTP recording/replay. See `internal/github/VCR_TESTING.md` for the VCR approach.

### Known Issues

- **DuckDB constraint violation** in update operations (workaround: delete + insert pattern)
- Integration tests skipped in `-short` mode (expected)

## Database Schema

Primary table: `repositories` with metadata, activity metrics, structured data (JSON topics/languages/contributors), license, content tracking, and embedding storage.

Indexes on: `language`, `updated_at`, `stargazers_count`, `full_name`.

The `content_chunks` table is deprecated and being phased out.

## Configuration

Sources (precedence order):
1. Command-line flags
2. Environment variables (`GH_STAR_SEARCH_` prefix)
3. Config file (`~/.config/gh-star-search/config.json`)
4. Defaults

Key settings: database path/pooling, cache directory/TTL, logging level, embedding provider, GitHub rate limiting delays.

## Making Changes

1. Create a feature branch
2. Write tests for new functionality
3. Ensure `go test ./...` passes
4. Run `golangci-lint run`
5. Commit with conventional format: `<type>(<scope>): <subject>`

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `perf`, `chore`
