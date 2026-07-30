## v1.0.2 (2026-07-30)

### Fix

- correct the gh-star-search name everywhere it was misspelled

## v1.0.1 (2026-07-27)

### Fix

- move the golangci exclude rules to the v2 linters.exclusions schema

## v1.0.0 (2026-07-27)

### Feat

- expand evaluation suite
- improve metrics and embeddings
- implement evaluation framework
- add evaluation tooling
- upgrade dependencies and optimize embedding model
- finish related counts and similarity
- prepare and document Python scripts as required
- switch custom bm25 to DuckDB FTS
- project cleanup
- **processor**: Implement selective file download with comprehensive filtering
- **query**: Implement vector search with cosine similarity
- **storage**: Add repo_embedding column for vector search
- **embedding**: Add vector embedding generation with Python/Go wrapper
- **sync**: Integrate AI summarization into sync command
- **summarizer**: Add RAM-efficient summarization with Python/Go wrapper
- **storage**: Add summarization fields to database schema
- **db**: Add query timeouts and extract connection pool constants
- **python**: Add RAM-efficient summarization and embedding support
- **config**: Make database connection pool fully configurable
- experiment with including commands to delete the db
- add config command
- minimize configuration
- replace custom environment logic with caarlos0/env
- replace Cobra with urfav/cli
- replace custom logger with slog
- remove schema compat
- switch to html-to-markdown library
- implement long-form output
- implement Task 6
- implement Task 5
- implement Task 2
- implement Task 11
- implement Task 10
- implement Task 9
- implement Task 8
- implement Task 7
- finish Task 6
- implement Task 6
- implement Task 5
- implement Task 4
- implement Task 3
- implement Task 2
- implement Task 1

### Fix

- **deps**: patch the eval script lockfile advisories
- **deps**: bump golang.org/x/net to 0.55.0
- restore .config/mise.toml as the settings-only skip-protected file
- enforce snake_case for Go filenames instead of camelCase
- update Before hook for urfave/cli v3.6.2 signature change
- cleanup new scripts
- resolve DuckDB issues
- use consistent KyleKing capitalization for all extensions
- resolve concurrency bugs
- **security**: Remove SQL injection vulnerability in SearchRepositories
- in-progress patches
- finish removing summary implementation
- remove complex force GC monitor
- resolve additional test failures
- resolve linter errors
- use kyleking for organization
- use UUIDs as PKs

### Refactor

- finish task list
- code cleanup of partial features
- removed unused chunks table, to be revisited
- Extract magic numbers to named constants
- remove summary implementation
- use reflect for merging config
- additional round of lint fixes
- remove LLM integration

### Perf

- **related**: Fix memory issue with streaming batch processing


- rework kiro documents based on simplified approach
