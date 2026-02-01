# AI Agent Guidelines for gh-star-search

This document provides guidelines for AI coding assistants working on this Go project.

## Code Organization

### Package Structure

```
gh-star-search/

├── cmd/gh-star-search/  # CLI entry point

├── internal/         # Private packages (not importable by other modules)
│   ├── app/          # Application logic
│   └── ...

└── go.mod
```

### Package Guidelines

- One package = one purpose
- Package names: short, lowercase, no underscores (`httputil` not `http_util`)
- Avoid `util`, `common`, `misc` packages
- `internal/` prevents external imports at the compiler level

### File Organization

- Group related types, functions, and methods in the same file
- Name files after the primary type they contain (`user.go`, `user_test.go`)
- Keep `main.go` minimal

## Code Style

### Functional Composition

```go
func ValidateUser(u User) error {
    if err := validateEmail(u.Email); err != nil {
        return err
    }
    return validateAge(u.Age)
}
```

### Interfaces

- Define interfaces where they're used, not where they're implemented
- Keep interfaces small (1-3 methods)
- Avoid interface pollution

### Functional Options Pattern

```go
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{addr: addr, timeout: 30 * time.Second}
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

## Error Handling

- Errors are values; handle them explicitly
- Return errors, don't panic (except for truly unrecoverable states)
- Add context when wrapping: `fmt.Errorf("doing something: %w", err)`
- Use `errors.Is` and `errors.As` for checking errors
- Use custom error types for domain-specific errors

## Testing

### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -1, -1, -2},
        {"zero", 0, 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", tt.a, tt.b, got, tt.expected)
            }
        })
    }
}
```

### Test Files

- Use `_test` package suffix for black-box testing
- Place test files adjacent to the code they test

## Anti-Patterns to Avoid

- **Naked returns**: Always name what you're returning
- **Long functions**: If > 50 lines, consider breaking up
- **Deep nesting**: Use early returns to flatten
- **Interface pollution**: Don't define interfaces until needed
- **Ignoring errors**: `_ = doThing()` is almost always wrong
- **Global state**: Pass dependencies explicitly

## Tools

- Run `mise run ci` before committing
- Run `hk fix` to auto-fix linting issues
- Use `golangci-lint run` for detailed linting output

## Git Practices

- Do not stage, commit, or push without explicit instruction
- Use conventional commits (commitizen enforced)

## Project-Specific Guidelines

See [DESIGN.md](DESIGN.md) for project-specific architecture, patterns, and domain context.
