# Next steps

## Template catch-up (noted 2026-07-26)

This repo is on my_go_template v0.2.2 while the template is at v0.5.0, the widest
drift in the fleet. The jump crosses the hk 1.53 restructure, the MISE_ENV
removal, the golangci v2 schema fix, the run-to-dev task rename, and the move of
tool pins into `.config/mise/conf.d/template.toml`. Expect a heavy
`copier update --UNSAFE --conflict=rej --defaults` with many .rej files, and
hand-sync `.config/mise.toml` afterward because it is in `_skip_if_exists`. Fix
the broken build below first so update conflicts land on a compiling baseline.


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
