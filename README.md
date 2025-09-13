# gh-star-search

A GitHub CLI extension that enables intelligent search over your starred repositories using natural language queries.

## Overview

`gh-star-search` ingests and indexes all repositories starred by the currently logged-in user, collecting both structured metadata and unstructured content to enable natural language search queries against a local DuckDB database.

## Installation

```bash
gh extension install kyleking/gh-star-search
```

## Usage

### Sync your starred repositories

```bash
gh star-search sync
```

### Search using natural language

```bash
gh star-search query "javascript formatter updated in last month"
```

### List all repositories

```bash
gh star-search list
```

### Get repository information

```bash
gh star-search info owner/repo-name
```

### View database statistics

```bash
gh star-search stats
```

### Clear the database

```bash
gh star-search clear
```

## Project Structure

```
gh-star-search/
├── cmd/                    # CLI command implementations
│   ├── root.go            # Root command and CLI setup
│   ├── sync.go            # Sync command
│   ├── query.go           # Query command
│   ├── list.go            # List command
│   ├── info.go            # Info command
│   ├── stats.go           # Stats command
│   └── clear.go           # Clear command
├── internal/
│   ├── github/            # GitHub API client
│   │   └── client.go      # GitHub client interface and types
│   ├── processor/         # Content processing
│   │   └── service.go     # Content processor interface and types
│   ├── storage/           # Database operations
│   │   └── repository.go  # Storage interface and types
│   ├── query/             # Query parsing
│   │   └── parser.go      # Query parser interface and types
│   ├── llm/               # LLM service
│   │   └── service.go     # LLM service interface and types
│   ├── config/            # Configuration management
│   │   └── config.go      # Configuration types and defaults
│   └── cache/             # Local caching
│       └── cache.go       # Cache interface and types
├── main.go                # Application entry point
├── go.mod                 # Go module definition
└── README.md              # Project documentation
```

## Development

### Building

```bash
go build -o gh-star-search
```

### Testing

```bash
go test ./...
```

## License

MIT License
