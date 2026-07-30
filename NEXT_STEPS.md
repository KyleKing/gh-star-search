# Next steps

Open work for gh-star-search. Everything here was re-verified against the repo on
2026-07-27; items that turned out to be done were removed. Reference material
lives in `DESIGN.md` and `OPERATIONS.md`; the append-only pass log is
`.freshen.md`.

## Blocking decision: DuckDB and cgo

This repo cannot build a release binary for any platform, and it will stay that
way until this is settled. Nothing in the release pipeline is worth touching
first.

`github.com/marcboeker/go-duckdb` only compiles with cgo, and `.goreleaser.yml`
sets `CGO_ENABLED=0`, inherited from `go_template/.goreleaser.yml.jinja`. All ten
configured targets fail identically:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/gh-star-search
# github.com/marcboeker/go-duckdb
transaction.go:6:5: undefined: Conn
```

Because the flag lives in the template, every my_go_template child with a cgo
dependency breaks the same way, so the answer probably belongs upstream too.

Three options:

- set `CGO_ENABLED=1` and ship linux/amd64 only, which builds natively on the
  ubuntu runner with no extra toolchain, and drop the other nine targets
- keep the full platform list and add zig or osxcross so goreleaser can
  cross-compile cgo to darwin and windows, which is more moving parts in CI
- drop goreleaser here and document `go install` in the README, which costs users
  a Go toolchain but needs no build matrix at all

The prior question of whether to keep DuckDB is still open and answering it first
may settle this one. A pure-Go store (sqlite via modernc, or Parquet plus an
in-process query layer) removes the cgo constraint outright.

Consequences to expect until it lands:

- v1.0.0 and v1.0.1 stay as they are: two assetless releases. Deleting them
  breaks any `_version` reference and rewrites the changelog for no gain, and
  back-filling binaries onto an old tag ships a build nobody cut
- v1.0.2 is tagged on `main` with no GitHub Release object at all, because
  goreleaser died before creating it. Leave the tag. Whichever option is taken,
  re-running goreleaser against it creates the release and attaches assets, so
  v1.0.2 can still be the first real one
- any new `fix:` or `feat:` commit trips Bump Version, which tags and then fails
  goreleaser at the build step. A fresh assetless release is the expected outcome
  of a normal bumpable commit right now, not a regression
- [issue #13](https://github.com/KyleKing/gh-star-search/issues/13)
  ("Installation Fails") stays open. It has been open since February and hueys is
  the only outside user any of these plugins has

## The two entrypoints disagree

Root `main.go` has the global flags (`--debug`, `--db-path`, `--cache-dir`) and
the config `Before` hook. `cmd/gh-star-search/main.go` has the `--version`
ldflags and neither. Goreleaser builds the second, so a release would ship a
binary missing the global flags and config initialization. Pick one entrypoint
and point goreleaser at it, or fold the version ldflags into the root `main.go`.
Both files still exist as of 2026-07-27.

## Ranking

From the eval work under `internal/python/scripts/` (plan of record:
`EVALUATION_IMPROVEMENT_PLAN.md`). `internal/query/engine.go:167` still applies
the fixed `1.0 + (0.1 * log10(stars+1) / 6.0)` boost:

- drop the star boost from 0.1 to 0.05; it over-weights popularity
- add a pure-semantic baseline mode so vector quality can be measured without the
  boosts
- consider an adaptive percentile star boost rather than a fixed weight

## Tests and coverage

- `mise run test:coverage-min` fails at 58.1% against the 70% floor, measured
  2026-07-27. It is wired into neither CI nor the git hooks, so it blocks
  nothing. Deliberately not closing the gap yet: the entrypoint and ranking work
  above will rewrite the code it would cover
- `TestEnsureEnvironment` in `internal/python/python_test.go:77` runs a real
  `uv sync` against the embedded lockfile inside the suite's 30s budget, so it
  panics on a cold uv cache and passes on a warm one. Give it the integration
  build tag, or skip unless the cache is already populated
- `internal/summarizer` fails the 30s timeout on cold torch startup and takes
  about 93s. Raise the timeout for the cold-start case or move it to a slow tier
  excluded from the default run
- The torch 2.13 / transformers 5.14 relock is still unverified: the eval scripts
  and the integration tier were never run, because that needs a ~2 GB download

## Dependencies

Two Dependabot PRs are open: #30 (go-dependencies group, 4 updates) and #31
(jdx/mise-action 4.2.0 to 4.2.3). #31 bumps a template-managed pin, so bump it in
my_go_template instead of merging here. Read any `urfave/cli` v3 bump carefully
before merging; the `Before` hook signature broke on a v3 upgrade once already.

## Pending the next copier update

This repo is pinned at my_go_template v0.7.0; the template is at v0.9.0.

- `hk.pkl` has no `typos`, `shellcheck`, `check-json`, or `copier-forbidden-files`
  step. Template v0.8.0 adds all of them, so the update closes this. `ruff`,
  `mdformat`, and `prettier` are deliberate template omissions and stay missing;
  adopting ruff here would need `internal/python/scripts/embed.py`'s ANN201 and
  D103 fixed first
- `hk.pkl`'s `newlines` step strips the trailing blank line copier writes into
  `.copier-answers.yml`, producing a spurious one-line diff on every update.
  Template 514e62b adds the exclude
- `Formula/gh-star-search.rb` and the `brew:sha` task are dead: the formula still
  carries `version "0.1.0"` and `REPLACE_WITH_SHA256_FOR_*` placeholders and has
  never been installable, and goreleaser has generated the cask since template
  v0.7.0. Template v0.8.0's `remove-if-found.txt` deletes the stub automatically,
  so nothing needs deleting by hand. The four `Formula/` references in
  `CONTRIBUTING.md` should go at the same time
- `.pre-commit-config.yaml` is still in the tree because the template ships it,
  but prek's git hooks are uninstalled and hk's config-based hooks
  (`hook.hk-*.command`, git 2.55) are what run. The template tracks removing it
- The update adds a CI `hooks` job running `hk check --all`. The one finding
  measured against this repo was `ttest` (scipy's `stats.ttest_rel`) in
  `internal/python/scripts/evaluate_embeddings.py:547`, allowlisted in
  `_typos.toml` on 2026-07-27. Watch the filename: this repo uses `_typos.toml`
  and the template ships `.typos.toml`, so the update lands a second config file
  and typos reads only the first it finds. Merge them into one

## Loose ends

- `[tool.tomlsort]` in `pyproject.toml` is dead config. toml-sort was dropped
  fleet-wide in favour of tombi, and `sort_inline_arrays = true` was the setting
  that reordered gci's import-group array in `.golangci.toml` and the `[tasks.ci]`
  `run` array. Delete the table
- `tombi-format` wants to reorder the `[[linters.exclusions.rules]]` tables in
  `.golangci.toml`. `hk check --all` reports it, and any commit touching that file
  will apply it
- `docs/` holds four unbuilt implementation plans (Bubble Tea TUI browser, Parquet
  analytics export, search-result caching, content-addressed README cache). All
  net-new, so they wait behind the release and entrypoint work

## Local state, do not lose

The populated index lives at `~/.config/gh-star-search/database.db` (about 38
repos, last written 2026-02-14). Do not run `clear`; rebuilding it means a full
re-sync. Summarization and embeddings run locally under `uv`, so there is no API
spend, only GitHub rate limit during `sync` and about 2 GB of local ML
dependencies on first `--embed`.
