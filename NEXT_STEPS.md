# Next steps

## Open decision: DuckDB and cgo (noted 2026-07-29)

This repo cannot build a release binary for any platform, and it will stay that
way until I pick one of the options below. Nothing in the release pipeline is
worth touching before then.

The cause: `github.com/marcboeker/go-duckdb` only compiles with cgo, and
`.goreleaser.yml` sets `CGO_ENABLED=0`, which it inherits from the template's
`go_template/.goreleaser.yml.jinja`. All ten configured targets fail the same
way:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/gh-start-search
# github.com/marcboeker/go-duckdb
transaction.go:6:5: undefined: Conn
```

Because the flag lives in the template, every my_go_template child with a cgo
dependency breaks the same way, so whatever I choose here probably belongs
upstream too.

Three options:

- set `CGO_ENABLED=1` and ship linux/amd64 only, which builds natively on the
  ubuntu runner with no extra toolchain, and drop the other nine targets
- keep the full platform list and add zig or osxcross so goreleaser can
  cross-compile cgo to darwin and windows, which is more moving parts in CI
- drop goreleaser from this repo and document `go install` in the README, which
  costs users a Go toolchain but needs no build matrix at all

The prior question of whether to keep DuckDB is still open, and answering it
first may settle this one. A pure-Go store (sqlite via modernc, or Parquet plus
an in-process query layer) removes the cgo constraint outright.

Until this is settled, v1.0.0 and v1.0.1 stay as they are: two empty releases
with no assets. Deleting them would break any `_version` reference and rewrite
the changelog for no gain, and back-filling binaries onto an old tag ships a
build nobody cut. Let the first release that carries real binaries be the real
one.

## Follow-ups (noted 2026-07-27)

- `toml-sort-fix` alphabetized the `[tasks.ci]` `run` array in
  `.config/mise/conf.d/template.toml`, so the build now runs before the test.
  Harmless here, but it is the same inline-array hazard as the gci one below
- `tombi-format` wants to reorder the `[[linters.exclusions.rules]]` tables in
  `.golangci.toml`; `hk check --all` reports it, and a commit that touches that
  file will apply it
- The `toml-sort-fix` pre-commit hook rewrites `internal/python/scripts/uv.lock`
  after every `uv lock`, so the diff is 600 lines of reordering on top of the
  real change. Add a `exclude: uv\.lock` upstream in my_go_template
- `[tool.tomlsort]` in `pyproject.toml` sets `sort_inline_arrays = true`, which
  alphabetizes every inline array it touches. `.golangci.toml` is now excluded
  from the hook because that array is gci's import group order, but any other
  TOML where array order carries meaning has the same problem
- Dependabot PR #28 is partly superseded (`golang.org/x/net` is at 0.55.0 here),
  and it still carries `urfave/cli` v3.8.0, which is where the `Before` hook
  signature broke last time. Read that upgrade before merging
- The torch 2.13 / transformers 5.14 relock is unverified: the eval scripts and
  the integration test tier were not run, because that needs a ~2 GB download
- `TestEnsureEnvironment` runs a real `uv sync` against the embedded lockfile
  inside the suite's 30s budget, so it panics on any machine with a cold uv
  cache and passes on a warm one. Give it the integration build tag, or skip it
  unless the cache is already populated
- The template is already at v0.6.0; this repo landed on v0.5.1 (now on v0.6.4)

## Template catch-up (noted 2026-07-26, done 2026-07-27)

The repo now sits on my_go_template v0.5.1 with a green build and green gates.
The `_skip_if_exists` file `.config/mise.toml` holds settings only; the tool
pins live in `.config/mise/conf.d/template.toml`.


Findings from a review pass on 2026-07-21, building the repo and reading the entrypoints, release config, and ranking code. Each item names the file to change and a suggested approach. Line numbers are omitted because they drift; grep the symbol.

## Blocking: the build is broken on main

`go build ./...` fails. Two one-line fixes:

- `main.go` uses the old `urfave/cli/v3` `BeforeFunc` signature. v3.6.2 changed it to return `(context.Context, error)`. Update the `Before` hook to return `(ctx, err)`. A "build: update dependencies" commit introduced the break
- `cmd/sync_test.go` has an unused `strings` import that breaks that test binary

Nothing else can proceed until these land.

## The two entrypoints disagree

Root `main.go` has the global flags (`--debug`, `--db-path`, `--cache-dir`) and the config `Before` hook. `cmd/gh-start-search/main.go` has the `--version` ldflags and neither. Goreleaser builds the second, so a release today ships a binary missing the global flags and config initialization. Pick one entrypoint and make goreleaser build it, or fold the version ldflags into the root `main.go` and point goreleaser there.

The `gh-start-search` typo runs through the goreleaser `main:` path and `Formula/gh-start-search.rb`. Rename both to `gh-star-search` while resolving the entrypoint.

## Release config

`.goreleaser.yml` sets `CGO_ENABLED=0`, which is suspect for `marcboeker/go-duckdb`. Confirm the resulting binary can open a DuckDB database before tagging, or set `CGO_ENABLED=1` with the matching cross-compile toolchain.

Never released: no tags, and `Formula/gh-start-search.rb` carries `version "0.1.0"` with `REPLACE_WITH_SHA256_FOR_*` placeholders. Once the build and entrypoint are fixed, cut a v0.1.0 and close [issue #13](https://github.com/KyleKing/gh-star-search/issues/13). That reporter (hueys) is the only outside user any of these plugins has, and the issue has been open since February.

## Ranking, from the eval suite

The eval work under `internal/python/scripts/` (`NEXT_STEPS.md`, `EVALUATION_IMPROVEMENT_PLAN.md`) recommends:

- Drop the star boost from 0.1 to 0.05; it over-weights popularity
- Add a pure-semantic baseline mode so vector quality can be measured without the boosts
- Consider an adaptive percentile star boost rather than a fixed weight

## Tests

`internal/summarizer` fails a 30s timeout on cold torch startup, and that suite takes about 93s. Either raise the timeout for the cold-start case or mark it as a slow-tier test excluded from the default run.

## Unbuilt feature plans

`docs/` holds four implementation plans, none built: a Bubble Tea TUI browser, Parquet analytics export, search-result caching, and a content-addressed README cache. These are net-new, so they wait behind the build and release work.

## Template

On copier `my_go_template` v0.2.2, two minor versions behind v0.3.2. No bubbletea dependency, so no framework migration applies. Run the copier update after the build is green.

## Local state, do not lose

The populated index lives at `~/.config/gh-star-search/database.db` (about 38 repos, last written 2026-02-14). Do not run `clear`; rebuilding it means a full re-sync. Summarization and embeddings run locally under `uv`, so there is no API spend, only GitHub rate limit during `sync` and about 2 GB of local ML dependencies on first `--embed`.
