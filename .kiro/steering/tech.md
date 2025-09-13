# Technology Stack

## Language & Runtime
- **Go**: versions 1.25
- **GitHub CLI Extensions**: Built as `gh` CLI extensions following GitHub's extension conventions

## gh-star-search
- `github.com/spf13/cobra` - CLI framework
- DuckDB integration
- LLM service integration

## Commands
```bash
# Build
go build -o gh-star-search

# Test
mise run test

# Install as GitHub CLI extension
gh extension install kyleking/gh-star-search

# Development testing
go run main.go [args]
```

## Architecture Patterns
- **Interface-based design**: Heavy use of interfaces for testability
- **Mock generation**: Uses `go:generate` directives for mock generation
- **Cobra CLI pattern**: Standard CLI structure with subcommands
- **Context propagation**: Consistent use of `context.Context` for cancellation
- **Concurrent processing**: Goroutines and channels for parallel operations
- **TUI patterns**: Bubble Tea architecture for interactive interfaces

## Testing
- Unit tests with `*_test.go` files
- Mock-based testing using generated mocks
- Fixture-based testing with test data in `fixtures/` directories
- Table-driven tests following Go conventions
