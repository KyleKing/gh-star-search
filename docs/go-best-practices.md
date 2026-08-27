# Go Best Practices

Worked examples for the conventions [AGENTS.md](../AGENTS.md) states as rules.

## Code Organization

### Package Guidelines

- One package, one purpose
- Package names: short, lowercase, no underscores (`httputil`, not `http_util`)
- Avoid `util`, `common`, and `misc` packages. Find a specific name or split the package
- `internal/` blocks external imports at the compiler level, so anything under it stays free to change
- Every exported symbol a downstream module can import is a compatibility promise, whether it lives under `pkg/` or at the top level
- Each entry point is its own main package under `cmd/<binary-name>/`

### File Organization

- Group related types, functions, and methods in the same file
- Name files after the primary type they contain (`user.go`, `user_test.go`)
- Keep `main.go` minimal and delegate to internal packages

## DRY Code

### Functional Composition

```go
// Compose small functions with single responsibilities
func ValidateUser(u User) error {
    if err := validateEmail(u.Email); err != nil {
        return err
    }
    return validateAge(u.Age)
}

func validateEmail(email string) error { /* ... */ }
func validateAge(age int) error { /* ... */ }
```

### Interfaces for Abstraction

```go
// Define interfaces where they're used, not where they're implemented
type UserStore interface {
    Get(id string) (*User, error)
    Save(u *User) error
}

// Consumers define the interface they need
func NewUserService(store UserStore) *UserService {
    return &UserService{store: store}
}
```

### Embedding for Composition

```go
type Logger struct{}
func (l *Logger) Log(msg string) { /* ... */ }

type Server struct {
    Logger  // Embedded: Server.Log() works directly
    addr string
}
```

### Generics (Go 1.18+)

```go
func Map[T, U any](items []T, fn func(T) U) []U {
    result := make([]U, len(items))
    for i, item := range items {
        result[i] = fn(item)
    }
    return result
}

func Filter[T any](items []T, predicate func(T) bool) []T {
    var result []T
    for _, item := range items {
        if predicate(item) {
            result = append(result, item)
        }
    }
    return result
}
```

### Functional Options Pattern

```go
type Server struct {
    addr    string
    timeout time.Duration
    logger  *log.Logger
}

type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func WithLogger(l *log.Logger) Option {
    return func(s *Server) { s.logger = l }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{addr: addr, timeout: 30 * time.Second}
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
srv := NewServer(":8080", WithTimeout(60*time.Second), WithLogger(logger))
```

## Error Handling

### Principles

- Errors are values, so handle them explicitly
- Return errors instead of panicking, except for truly unrecoverable states
- Add context when wrapping, and stop before over-wrapping

### Basic Pattern

```go
result, err := doSomething()
if err != nil {
    return fmt.Errorf("doing something: %w", err)
}
```

### Custom Error Types

```go
type NotFoundError struct {
    Resource string
    ID       string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
}

// Check with errors.As
var notFound *NotFoundError
if errors.As(err, &notFound) {
    // Handle not found case
}
```

### Sentinel Errors

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
)

// Check with errors.Is
if errors.Is(err, ErrNotFound) {
    // Handle not found
}
```

### Error Wrapping

```go
// Wrap with context using %w
if err := db.Query(q); err != nil {
    return fmt.Errorf("querying users: %w", err)
}

// Don't wrap if you're handling it
if err := doThing(); err != nil {
    log.Printf("warning: %v", err)
    return nil  // Handled, no wrap
}
```

### Defer for Cleanup

```go
func processFile(path string) (err error) {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("opening file: %w", err)
    }
    defer func() {
        if cerr := f.Close(); cerr != nil && err == nil {
            err = fmt.Errorf("closing file: %w", cerr)
        }
    }()
    // Process file...
    return nil
}
```

## Table-Driven Tests

### Basic Structure

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

### With Error Cases

```go
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Config
        wantErr bool
    }{
        {
            name:  "valid config",
            input: `{"port": 8080}`,
            want:  &Config{Port: 8080},
        },
        {
            name:    "invalid json",
            input:   `{invalid}`,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseConfig(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
            }
            if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %+v, want %+v", got, tt.want)
            }
        })
    }
}
```

### Using testify

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
    tests := []struct {
        name string
        user User
        want string
    }{
        {"full name", User{First: "John", Last: "Doe"}, "John Doe"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := tt.user.FullName()
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

Use `require` for error assertions so the test stops on failure, and `assert` for
value comparisons. `testifylint` rejects `assert.Error` and `assert.NoError`.

### Subtests with Parallel Execution

```go
func TestAPI(t *testing.T) {
    tests := []struct {
        name     string
        endpoint string
        status   int
    }{
        {"health", "/health", 200},
        {"users", "/users", 200},
        {"notfound", "/missing", 404},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            resp := httptest.NewRecorder()
            // Test logic...
        })
    }
}
```

Go 1.22+ scopes the range variable per iteration, so `tt := tt` is no longer needed.

## Additional Patterns

### Constructor Functions

```go
func NewUser(name string, age int) (*User, error) {
    if name == "" {
        return nil, errors.New("name required")
    }
    if age < 0 {
        return nil, errors.New("age must be non-negative")
    }
    return &User{name: name, age: age}, nil
}
```

### Context Usage

```go
func FetchData(ctx context.Context, id string) (*Data, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    // ...
}
```

### Avoiding nil Pointer Panics

```go
// Method on pointer receiver: check for nil
func (u *User) Name() string {
    if u == nil {
        return ""
    }
    return u.name
}

// Return empty slice, not nil
func GetUsers() []User {
    users := []User{}  // or make([]User, 0)
    // ...
    return users
}
```

### Struct Tags

```go
type User struct {
    ID        string    `json:"id" db:"id"`
    Email     string    `json:"email" db:"email" validate:"required,email"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}
```

## Anti-Patterns to Avoid

- Naked returns, which hide what a function actually returns
- Functions past ~50 lines, which usually hold two responsibilities
- Deep nesting, where an early return flattens the body
- Interface pollution: define an interface once a consumer needs the abstraction
- Ignored errors (`_ = doThing()` is almost always wrong)
- Global state, where passing dependencies explicitly is clearer
- Premature optimization: profile first, optimize second

## Resources

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
