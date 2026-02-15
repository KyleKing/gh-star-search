# Implementation Plan: Interactive TUI Browser

Inspired by msgvault's Bubble Tea TUI with drill-down analytics, selection, and search.

## Goal

Add a `tui` command that provides an interactive terminal interface for browsing starred repositories with drill-down navigation by language, topic, and organization. Replaces repeated `query`/`list`/`related` invocations with a single explorable view.

## Architecture

### New Package: `internal/tui/`

```
internal/tui/
  model.go        -- Bubble Tea model, state machine, Update()
  view.go         -- View rendering with lipgloss
  keys.go         -- Key bindings
  actions.go      -- Async data loading commands (tea.Cmd)
  format.go       -- Cell formatting helpers (truncation, alignment)
```

### Dependencies

```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles       -- textinput, viewport, table, help
github.com/charmbracelet/lipgloss
```

### Model State

```go
type viewKind int
const (
    viewLanguages viewKind = iota
    viewTopics
    viewOrgs
    viewRepoList
    viewRepoDetail
    viewSearch
)

type model struct {
    repo     storage.Repository   // database handle
    engine   query.Engine         // search engine

    view     viewKind
    stack    []viewState          // navigation history for back

    // Aggregate views
    items    []aggregateRow       // current list (language/topic/org rows)
    cursor   int
    offset   int                  // viewport scroll offset

    // Repo list (after drill-down)
    repos    []types.StoredRepo
    repoPage int
    repoSize int                  // page size

    // Search
    searchInput textinput.Model
    searchMode  string            // "fuzzy" or "vector"
    results     []query.Result

    // Layout
    width, height int

    // Async
    loading bool
    err     error
}

type viewState struct {
    view   viewKind
    filter string               // e.g., language="Go", topic="cli"
    cursor int
    offset int
}
```

### Navigation Model

Follows msgvault's drill-down pattern:

```
Languages ──Enter──> Repos filtered by language ──Enter──> Repo detail
Topics    ──Enter──> Repos filtered by topic    ──Enter──> Repo detail
Orgs      ──Enter──> Repos filtered by org      ──Enter──> Repo detail
Search    ──Enter──> Search results              ──Enter──> Repo detail
```

`Esc`/`Backspace` pops the navigation stack. `Tab` cycles between Languages, Topics, Orgs aggregate views. `/` opens the search input.

### Key Bindings

| Key                | Action                                             |
| ------------------ | -------------------------------------------------- |
| `j`/`k`, Up/Down   | Navigate rows                                      |
| `Enter`            | Drill down                                         |
| `Esc`, `Backspace` | Go back                                            |
| `Tab`              | Cycle aggregate view (Languages -> Topics -> Orgs) |
| `s`                | Cycle sort (Name -> Count -> Stars)                |
| `r`                | Reverse sort                                       |
| `/`                | Search                                             |
| `m`                | Toggle search mode (fuzzy/vector)                  |
| `q`                | Quit                                               |
| `?`                | Help                                               |

### Required Storage Additions

New methods on `storage.Repository` interface:

```go
// Aggregate queries for TUI views
GetLanguageCounts(ctx context.Context) ([]AggregateRow, error)
GetTopicCounts(ctx context.Context) ([]AggregateRow, error)
GetOrgCounts(ctx context.Context) ([]AggregateRow, error)
ListByLanguage(ctx context.Context, language string, limit, offset int) ([]StoredRepo, error)
ListByTopic(ctx context.Context, topic string, limit, offset int) ([]StoredRepo, error)
ListByOrg(ctx context.Context, org string, limit, offset int) ([]StoredRepo, error)
```

```go
type AggregateRow struct {
    Name       string
    Count      int
    TotalStars int
}
```

SQL for aggregates (DuckDB):

```sql
-- Language counts
SELECT language, COUNT(*) AS count, SUM(stargazers_count) AS total_stars
FROM repositories WHERE language IS NOT NULL
GROUP BY language ORDER BY count DESC;

-- Topic counts (unnest JSON array)
SELECT topic, COUNT(*) AS count, SUM(stargazers_count) AS total_stars
FROM (SELECT unnest(topics_array::VARCHAR[]) AS topic, stargazers_count FROM repositories)
GROUP BY topic ORDER BY count DESC;

-- Org counts
SELECT split_part(full_name, '/', 1) AS org, COUNT(*) AS count, SUM(stargazers_count) AS total_stars
FROM repositories
GROUP BY org ORDER BY count DESC;
```

### Async Data Loading

Follow msgvault's pattern of wrapping async operations with panic recovery:

```go
func loadAggregateCmd(repo storage.Repository, view viewKind) tea.Cmd {
    return func() tea.Msg {
        defer func() {
            if r := recover(); r != nil {
                // return error msg
            }
        }()
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        switch view {
        case viewLanguages:
            rows, err := repo.GetLanguageCounts(ctx)
            return aggregateLoadedMsg{rows: rows, err: err}
        // ...
        }
    }
}
```

### View Rendering

Follow the charm/bubbletea design philosophy from CLAUDE.md:

- Minimal color: white text, single background
- Borders for visual hierarchy (box around the table area)
- Color reserved for actionable elements (selected row highlight, search mode badge)

Aggregate view layout:

```
 gh-star-search ── Languages                          [Tab] cycle view
 ─────────────────────────────────────────────────────────────────────
  Go                          142 repos    1,245,302 stars
  Python                       98 repos      892,104 stars
  TypeScript                   67 repos      445,221 stars
  Rust                         45 repos      312,889 stars
 > JavaScript                  38 repos      201,445 stars  <── cursor
  ...
 ─────────────────────────────────────────────────────────────────────
  38 of 412 repos · [Enter] drill down · [/] search · [?] help
```

Repo detail view reuses `formatter.FormatLong` output, rendered inside a `viewport` bubble for scrolling.

### CLI Command

```go
// cmd/tui.go
func tuiCommand() *cli.Command {
    return &cli.Command{
        Name:  "tui",
        Usage: "Interactive browser for starred repositories",
        Action: func(ctx context.Context, cmd *cli.Command) error {
            repo, err := initializeStorage(ctx, cmd)
            if err != nil { return err }
            defer repo.Close()

            engine := query.NewSearchEngine(repo, embeddingProvider)
            m := tui.New(repo, engine)
            p := tea.NewProgram(m, tea.WithAltScreen())
            _, err = p.Run()
            return err
        },
    }
}
```

## Implementation Order

1. Add aggregate query methods to `storage.Repository` interface and `DuckDBRepository`
1. Create `internal/tui/keys.go` with key bindings
1. Create `internal/tui/model.go` with state machine, `Init()`, `Update()`
1. Create `internal/tui/view.go` with aggregate and list rendering
1. Create `internal/tui/actions.go` with async data loading commands
1. Create `internal/tui/format.go` with cell formatting helpers
1. Add `cmd/tui.go` command
1. Wire into `main.go` command list
1. Tests: model state transitions, aggregate SQL queries, key binding dispatch

## Scope Boundaries

- No deletion or mutation operations from TUI (read-only)
- No inline sync trigger (use `sync` command separately)
- No custom themes or color configuration
- Pagination for drill-down repo lists uses fixed page size (50), not infinite scroll
