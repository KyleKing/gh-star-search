# Next steps

Open work for gh-star-search. Everything here was re-verified against the repo on
2026-07-27; items that turned out to be done were removed. Reference material
lives in `DESIGN.md` and `OPERATIONS.md`; the append-only pass log is
`.freshen.md`.

## Release pipeline: cgo and native runners

Settled on 2026-08-01. `github.com/marcboeker/go-duckdb` only compiles with cgo,
so `CGO_ENABLED=0` made every target fail on `undefined: Conn` and no release
ever shipped a binary. `.goreleaser.yml` now sets `CGO_ENABLED=1` and builds only
what a runner can produce natively:

- darwin/arm64 and darwin/amd64, built by goreleaser on `macos-latest` (the
  universal macOS SDK covers the amd64 cross), which also publishes the release
  and the Homebrew cask
- linux/amd64, built with plain `go build` on `ubuntu-latest` in a separate job
  and attached to the same release afterwards, with its digest appended to
  `checksums.txt`

windows, freebsd, 386, and linux/arm64 were dropped rather than shipped broken.
Adding any of them back means a cgo cross toolchain (zig or osxcross) or another
native runner.

goreleaser stays inline in `bump_version.yml` because a tag pushed with
`GITHUB_TOKEN` does not trigger other workflows. goreleaser OSS has no
`--split`/`continue`, which is why the linux binary is attached by hand instead
of merged into one goreleaser run.

Still open:

- `mise run ci` passes while the release build fails, because `go test ./...`
  uses the host toolchain. `goreleaser` is now pinned in
  `.config/mise/conf.d/user.toml`; `goreleaser build --snapshot --clean` before
  tagging is the check that would have caught this. Wiring it into a mise task or
  the CI job is unfinished
- the same `CGO_ENABLED=0` default still ships in
  `my_go_template/go_template/.goreleaser.yml.jinja`, so any other child with a
  cgo dependency breaks identically. Worth an upstream copier question
- whether to keep DuckDB at all. A pure-Go store (modernc sqlite, or Parquet plus
  an in-process query layer) removes the cgo constraint and restores the full
  platform matrix
- v1.0.5 is the first release that ever carried binaries: three assets with three
  distinct hashes (darwin/arm64, darwin/amd64, linux/amd64) and a `checksums.txt`
  covering all three. The Homebrew cask reached `KyleKing/homebrew-tap` at the
  same time
- v1.0.0 and v1.0.1 stay as they are: two assetless releases. Deleting them
  rewrites the changelog for no gain
- v1.0.3 and v1.0.4 are tagged on `main` with no GitHub Release object, left by
  the two failed release runs. v1.0.2 was the same and was deleted on 2026-08-01
  (it was `97c84db`, restorable if wanted). Both can go the same way
- [issue #13](https://github.com/KyleKing/gh-star-search/issues/13)
  ("Installation Fails") can now be closed; v1.0.5 answers it

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

## Template sync

Updated to my_go_template v0.9.1 on 2026-08-01. What the update settled:

- `hk.pkl` gained `typos`, `shellcheck`, `check-json`, `copier-forbidden-files`,
  `byte-order-marker`, `check-executables-have-shebangs`, `forbid-new-submodules`,
  `python-debug-statements`, and `vcs-permalinks`, plus the `newlines` exclude for
  `.copier-answers.yml`. `ruff`, `mdformat`, and `prettier` stay deliberately
  absent; adopting ruff would need `internal/python/scripts/embed.py`'s ANN201 and
  D103 fixed first
- `Formula/gh-star-search.rb` and the `brew:sha` task were deleted by the
  template's `remove-if-found.txt`, and `CONTRIBUTING.md` now points at the
  goreleaser-generated cask instead
- `_typos.toml` was merged into the template's `.typos.toml` and deleted, so only
  one config is in play. The `ttest` allowlist entry (scipy's `stats.ttest_rel` in
  `internal/python/scripts/evaluate_embeddings.py`) carried over

Two template defects to back-port, because the next copier update reverts the
local fixes:

- `go_template/.github/workflows/ci.yml.jinja` runs both `jdx/mise-action` and
  `actions/setup-go` in the `ci` job. mise exports `GOROOT` for its own
  `go = "latest"` while setup-go supplies go.mod's version, so `go` drives the
  other tree's `compile` and every package fails with `compile: version "goX"
  does not match go tool version "goY"`. Latent since v0.8.0; it fired here the
  day Go 1.26.5 shipped. Setting `GOROOT: ""` on the step does not help because
  `mise run` re-exports it. Fixed locally by deleting setup-go from that job
- `go_template/.goreleaser.yml.jinja` sets `CGO_ENABLED=0`, which silently breaks
  any child with a cgo dependency. An opt-in copier question would surface it

Left over:

- `.pre-commit-config.yaml` is still in the tree because the template still ships
  it, but prek's git hooks are uninstalled and hk's config-based hooks
  (`hook.hk-*.command`, git 2.55) are what run. The template tracks removing it
- `AGENTS.md` is in the template's `_skip_if_exists`, so this repo still carries
  the pre-v0.6 shape (`### Package Guidelines`, `### File Organization`) and never
  received the v0.9.x rewrite. That rewrite adds the verification checklist and
  the "a release is verified by distinct hashes, not asset count" rule, both of
  which apply here. Worth hand-syncing while keeping this repo's package tree

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
