# Project Structure

## gh-star-search (Cobra-based)
```
gh-star-search/
├── main.go              # Simple entry point
├── cmd/                 # CLI commands (Cobra pattern)
│   ├── root.go          # Root command setup
│   ├── sync.go          # Sync command
│   ├── query.go         # Query command
│   ├── list.go          # List command
│   ├── info.go          # Info command
│   ├── stats.go         # Stats command
│   └── clear.go         # Clear command
└── internal/            # Internal packages
    ├── github/          # GitHub API client
    ├── processor/       # Content processing
    ├── storage/         # Database operations
    ├── query/           # Query parsing
    ├── llm/             # LLM service
    ├── config/          # Configuration
    └── cache/           # Local caching
```

## Architectural Patterns

### Package Organization
- **cmd/**: CLI command implementations and main business logic
- **internal/**: Private packages not meant for external use
- **conn/**: External system integration layer
- **mocks/**: Generated test mocks
- **fixtures/**: Test data and mock responses

### File Naming Conventions
- `main.go`: Application entry point
- `*_test.go`: Test files alongside source files
- `*.go`: Implementation files named after their primary type/function
- Interface files often named after the main interface (e.g., `connection.go`)

### Testing Structure
- Tests co-located with source files
- Fixture data in dedicated `fixtures/` directories
- Mock generation using `//go:generate` directives
- Separate test packages for integration tests

### Configuration Files
- `go.mod`/`go.sum`: Go module definitions
- `.gitignore`: Git ignore patterns
- `.github/`: GitHub Actions and templates (where present)
- `README.md`: Project documentation
- `LICENSE`: License files

## Development Workflow
Each project is independently buildable and testable. The repository supports working on individual projects without affecting others, while sharing common patterns and conventions across all three tools.
