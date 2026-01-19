# Contributing to gh-star-search

Thank you for your interest in contributing to gh-star-search! This document provides an overview of the project architecture, development workflow, and guidelines for contributors.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Project Structure](#project-structure)
- [Key Components](#key-components)
- [GitHub API Integration](#github-api-integration)
- [Development Workflow](#development-workflow)
- [Testing Strategy](#testing-strategy)
- [Caching Strategy](#caching-strategy)
- [Database Schema](#database-schema)

## Architecture Overview

gh-star-search is a GitHub CLI extension built in Go that enables powerful search and discovery across starred repositories. The application follows a clean architecture pattern with clear separation of concerns:

```
┌─────────────┐
│  CLI Layer  │  (cmd/)       - User-facing commands
└──────┬──────┘
       │
┌──────▼──────┐
│   Service   │  (cmd/)       - Business logic for commands
│    Layer    │
└──────┬──────┘
       │
┌──────▼──────────────────────────────────┐
│        Core Internal Packages            │
├──────────────┬───────────┬──────────────┤
│   Storage    │   Query   │   Related    │  (internal/)
│   (DuckDB)   │  (Search) │  (Discovery) │
├──────────────┼───────────┼──────────────┤
│   GitHub     │ Processor │   Config     │
│    Client    │ (Content) │   Caching    │
└──────────────┴───────────┴──────────────┘
```

### Core Design Principles

1. **Local-First**: All data is stored in a local DuckDB database
2. **Minimal Content**: Only essential content is ingested (no full git clones)
3. **Incremental Sync**: Smart staleness detection to avoid unnecessary API calls
4. **Security-First**: Parameterized queries only, no SQL injection vectors
5. **Rate-Limited**: Respectful API usage with built-in delays

## Project Structure

```
gh-star-search/
├── cmd/                      # CLI command implementations
│   ├── sync.go              # Repository synchronization
│   ├── query.go             # Search functionality
│   ├── list.go              # List all repositories
│   ├── info.go              # Repository details
│   ├── stats.go             # Database statistics
│   ├── clear.go             # Database management
│   ├── related.go           # Related repository discovery
│   └── config.go            # Configuration management
│
├── internal/                 # Core application logic
│   ├── cache/               # File-based caching with TTL
│   ├── config/              # Configuration models & loading
│   ├── embedding/           # Embedding provider interface
│   ├── errors/              # Structured error types
│   ├── formatter/           # Output formatting (long/short)
│   ├── github/              # GitHub API client
│   ├── logging/             # Structured logging with slog
│   ├── processor/           # Content extraction & processing
│   ├── query/               # Search engine (fuzzy + vector)
│   ├── related/             # Related repository engine
│   ├── storage/             # DuckDB persistence layer
│   └── types/               # Type definitions
│
├── .config/                  # Build and task configuration
│   ├── mise.toml            # Task definitions (test, build, format)
│   └── mise.hk.toml         # Mise hook configuration
│
├── .github/workflows/        # CI/CD pipelines
│   └── ci.yaml              # GitHub Actions workflow
│
├── .kiro/                    # Project specifications
│   ├── specs/               # Requirements and design docs
│   └── steering/            # Architecture decisions
│
├── main.go                   # Application entry point
├── go.mod                    # Go module definition
├── hk.pkl                    # Git hooks configuration
└── README.md                 # User documentation
```

## Key Components

### Storage Layer (`internal/storage/`)

The storage layer uses DuckDB as an embedded database for fast, efficient querying.

**Key Files:**
- `duckdb.go`: Main repository implementation with connection pooling
- `repository.go`: Repository interface defining storage operations
- `migrations.go`: Schema management and versioning

**Configuration:**
- Default query timeout: 30 seconds
- Max open connections: 10
- Max idle connections: 5
- Connection max lifetime: 30 minutes
- Connection max idle time: 5 minutes

**Security Features:**
- All queries use parameterized statements
- SQL injection protection via keyword validation
- Query timeouts to prevent resource exhaustion

### GitHub Client (`internal/github/`)

**Key Files:**
- `client.go`: REST API client for GitHub
- `cached_client.go`: Wrapper with caching layer
- `worker_pool.go`: Concurrent request processing

**Rate Limiting:**
- 100ms delay between repositories (configurable: `RepositoryRateLimitMs`)
- 2s delay between batches (configurable: `BatchDelaySeconds`)
- Connection pooling to avoid overwhelming the API

### Search Engine (`internal/query/`)

Implements dual-mode search:

1. **Fuzzy Mode** (default): Full-text BM25-style scoring
2. **Vector Mode**: Semantic similarity using embeddings (infrastructure ready)

**Ranking Factors:**
- Star count (logarithmic boost)
- Recency (recent updates ranked higher)
- Field matches (name > description > topics > content)

**Query Limits:**
- Minimum: 1 result (`MinQueryLimit`)
- Maximum: 50 results (`MaxQueryLimit`)
- Default: 10 results (`DefaultQueryLimit`)
- Minimum query length: 2 characters (`MinQueryLength`)

### Related Repository Engine (`internal/related/`)

Finds related repositories using weighted scoring:

**Score Components:**
- Same organization: 30% weight
- Topic overlap (Jaccard similarity): 25% weight
- Shared top contributors: 25% weight
- Vector similarity: 20% weight (when embeddings available)

**Configuration:**
- Minimum score threshold: 0.25 (`MinRelatedScoreThreshold`)
- Maximum repositories loaded: 10,000 (`MaxRepositoryFetchLimit`)
  - ⚠️ **Known Issue**: Loads all repos into memory (see Phase 4 improvements)

### Configuration System (`internal/config/`)

Configuration sources (in order of precedence):
1. Command-line flags
2. Environment variables (prefix: `GH_STAR_SEARCH_`)
3. Config file (`~/.config/gh-star-search/config.json`)
4. Default values

**Key Configuration Options:**
```json
{
  "database": {
    "path": "~/.config/gh-star-search/database.db",
    "max_connections": 10,
    "query_timeout": "30s"
  },
  "cache": {
    "directory": "~/.cache/gh-star-search",
    "metadata_stale_days": 14,
    "ttl_hours": 24
  },
  "logging": {
    "level": "info",
    "format": "text"
  }
}
```

## GitHub API Integration

### API Endpoints Used

| Endpoint | Purpose | Rate Limit | Caching |
|----------|---------|------------|---------|
| `GET /user/starred` | List starred repositories | 5,000/hour | 14 days |
| `GET /repos/{owner}/{repo}` | Repository metadata | 5,000/hour | 14 days |
| `GET /repos/{owner}/{repo}/contributors` | Top contributors | 5,000/hour | 7 days |
| `GET /repos/{owner}/{repo}/topics` | Repository topics | 5,000/hour | 7 days |
| `GET /repos/{owner}/{repo}/languages` | Language statistics | 5,000/hour | 7 days |
| `GET /repos/{owner}/{repo}/contents/{path}` | File contents (README) | 5,000/hour | Never* |
| `GET /repos/{owner}/{repo}/stats/participation` | Commit activity | 5,000/hour | 7 days |

\* Content is cached via content hash - only re-fetched if hash changes

### Rate Limit Handling

GitHub's REST API has the following limits for authenticated users:
- **5,000 requests per hour** for most endpoints
- Rate limit resets hourly
- Current limit available via response headers: `X-RateLimit-Remaining`

**Our Rate Limiting Strategy:**
1. Built-in delays between requests (100ms per repo, 2s per batch)
2. Incremental sync with staleness detection (default: 14 days)
3. Content hash tracking to avoid re-processing unchanged content
4. Cached API responses to minimize redundant calls

**Authentication:**
- Uses `gh` CLI authentication (via `github.com/cli/go-gh`)
- Supports both personal access tokens and OAuth
- No additional authentication setup required

## Development Workflow

### Prerequisites

- Go 1.24.0 or later
- `gh` CLI installed and authenticated
- `mise` (optional, for task runner)
- Git hooks via `hk` (optional)

### Setup

```bash
# Clone the repository
git clone https://github.com/KyleKing/gh-star-search.git
cd gh-star-search

# Install dependencies
go mod download

# Run tests
go test ./...

# Build the binary
go build -o gh-star-search
```

### Using Mise Tasks

```bash
# Run all checks (lint, test, format)
mise run ci

# Run tests with coverage
mise run test

# Build the binary
mise run build

# Format code
mise run format

# Run git hooks manually
mise run hooks
```

### Code Style

- **Formatting**: Use `gofmt` (enforced by git hooks)
- **Linting**: Comprehensive linting with `golangci-lint` (30+ rules enabled)
- **SQL Linting**: SQL queries validated with `sqlfluff`
- **Naming Conventions**: File/directory names validated with `ls-lint`

### Git Hooks

Pre-commit hooks automatically run:
- `gofmt` formatting
- `golangci-lint` checks
- SQL query validation
- File naming validation

### Making Changes

1. Create a feature branch: `git checkout -b feature/your-feature-name`
2. Make your changes with clear, atomic commits
3. Write tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Run linters: `golangci-lint run`
6. Commit with descriptive messages
7. Push and create a pull request

### Commit Message Guidelines

Follow conventional commit format:

```
<type>(<scope>): <subject>

<body>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code refactoring
- `docs`: Documentation changes
- `test`: Test additions/changes
- `perf`: Performance improvements
- `chore`: Maintenance tasks

**Examples:**
```
feat(search): Add vector similarity search mode
fix(security): Remove SQL injection vulnerability
refactor(storage): Extract connection pool constants
docs(contributing): Add architecture overview
```

## Testing Strategy

### Test Types

1. **Unit Tests**: Test individual functions in isolation
   - Located alongside source files (`*_test.go`)
   - Use table-driven tests where appropriate
   - Mock external dependencies

2. **Integration Tests**: Test component interactions
   - `*_integration_test.go` files
   - Use temporary databases
   - May require GitHub authentication (skipped in CI if not available)

3. **Performance Tests**: Benchmark critical operations
   - `*_performance_test.go` files
   - Focus on database queries and batch processing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detector
go test -race ./...

# Run specific package tests
go test ./internal/storage/

# Run tests matching pattern
go test -run TestDuckDBRepository ./internal/storage/
```

### Test Coverage

Current coverage by package:
- `internal/cache`: 77.1%
- `internal/config`: 71.7%
- `internal/errors`: 97.9%

**Coverage Goals:**
- Core packages (storage, query): > 80%
- Business logic (cmd): > 70%
- Utilities (errors, logging): > 90%

### Mocking Strategy

- **Planned**: Migration to `go-vcr` for HTTP request mocking
- **Current**: Manual mock implementations for GitHub client
- See `vcr_migration_plan.md` for migration details

## Caching Strategy

### Cache Types

1. **File Cache** (`internal/cache/`):
   - Stores GitHub API responses
   - TTL-based expiration (default: 24 hours)
   - Automatic cleanup on initialization
   - Location: `~/.cache/gh-star-search/`

2. **Content Hash Cache**:
   - Tracks README content changes
   - Stored in database alongside repository
   - Only re-processes when hash changes

3. **Staleness-Based Refresh**:
   - Metadata refresh after N days (default: 14)
   - Stats refresh after N days (default: 7)
   - Configurable via `metadata_stale_days` setting

### Cache Invalidation

- Automatic: Files older than TTL are removed on cleanup
- Manual: Use `--force` flag to bypass cache
- Database: Clear entire database with `gh star-search clear`

## Database Schema

### Tables

#### `repositories`

Primary table storing repository metadata:

```sql
CREATE TABLE repositories (
    id TEXT PRIMARY KEY,
    full_name TEXT UNIQUE NOT NULL,
    description TEXT,
    homepage TEXT,
    language TEXT,
    stargazers_count INTEGER,
    forks_count INTEGER,
    size_kb INTEGER,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    last_synced TIMESTAMP,

    -- Activity metrics
    open_issues_open INTEGER,
    open_issues_total INTEGER,
    open_prs_open INTEGER,
    open_prs_total INTEGER,
    commits_30d INTEGER,
    commits_1y INTEGER,
    commits_total INTEGER,

    -- Structured data (JSON)
    topics_array TEXT,
    languages TEXT,
    contributors TEXT,

    -- License info
    license_name TEXT,
    license_spdx_id TEXT,

    -- Content tracking
    content_hash TEXT,

    -- Future: Embeddings
    repo_embedding FLOAT[]
);
```

**Indexes:**
```sql
CREATE INDEX idx_repositories_language ON repositories(language);
CREATE INDEX idx_repositories_updated_at ON repositories(updated_at);
CREATE INDEX idx_repositories_stargazers ON repositories(stargazers_count);
CREATE INDEX idx_repositories_full_name ON repositories(full_name);
```

#### `content_chunks` (Deprecated)

Stores content chunks for future LLM processing:

```sql
CREATE TABLE content_chunks (
    id TEXT PRIMARY KEY,
    repository_id TEXT REFERENCES repositories(id),
    source_path TEXT,
    chunk_type TEXT,
    content TEXT,
    tokens INTEGER,
    priority INTEGER
);
```

**Note**: This table is being phased out in favor of simpler content storage.

### Migration Strategy

- Schema versioning via `internal/storage/migrations.go`
- Migrations run automatically on `Initialize()`
- For development: Safe to delete database and resync

---

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/KyleKing/gh-star-search/issues)
- **Discussions**: Use GitHub Discussions for questions
- **Documentation**: See README.md for user-facing docs

## License

This project is licensed under the MIT License - see the LICENSE file for details.
