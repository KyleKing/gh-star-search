# AI Agent Guidelines for gh-star-search

How to work in this Go project. Task and release mechanics live in
[CONTRIBUTING.md](CONTRIBUTING.md), worked code examples live in
[docs/go-best-practices.md](docs/go-best-practices.md), and toolchain failures live
in [docs/troubleshooting.md](docs/troubleshooting.md). Architecture and domain
context, where the project keeps a DESIGN.md, live there. This file covers only
what those do not.

## Verify before you report

CI runs five jobs and `mise run ci` is only the first, so a green `mise run ci` is not
a green build. Reproduce all five locally:

```bash
hk check --all                            # every hook step; also re-runs `mise run ci`
mise exec -- actionlint                   # separate CI job; the validator for workflow YAML
mise exec -- golangci-lint run ./...      # separate CI job, not an hk step
mise exec -- golangci-lint config verify  # `run` accepts v1 schema keys silently
mise run bench                            # compiles and runs the benchmarks
```

Run linters to completion rather than stopping at the first hit: add
`--max-issues-per-linter=0 --max-same-issues=0`. A local run that passes while CI fails
means the two commands differ, so read the workflow step and match its flags.

Known false negatives, each of which has already cost a session:

- **Hooks look uninstalled when they are not.** `hk install --mise` on git 2.55+
  writes `hook.hk-*.command` into `.git/config` and creates no `.git/hooks/pre-commit`.
  Check with `git config --get-regexp '^hook\.'` (expect six entries) or
  `git hook list pre-commit` (expect `hk-pre-commit`). A file-existence check is wrong.
- **Never pipe or chain `git commit`.** `git commit … | tail` reports the pipe's exit
  code, so a failing hook looks like success and the follow-up push ships nothing.
  Commit as its own command, then confirm with `git log -1`. Hooks rewrite files here;
  when one fails that way, `git add -A` its edits and commit again.
- **A workflow that passes yamllint can still be broken.** `hk.pkl` excludes
  `.github/workflows/` from yamllint, and yamllint reads YAML syntax only. Runner
  labels, expression syntax, `workflow_call` inputs, and embedded shell are
  `actionlint`'s job, so edit a workflow and run `mise exec -- actionlint`.
- **A release is verified by distinct hashes, not asset count.** Ten assets can be one
  binary published ten times. Confirm with
  `gh release download <tag> -p checksums.txt -O - | awk '{print $1}' | sort -u | wc -l`
  and expect the same number as there are binaries.

When a check fails, fix the cause. Do not skip a test, widen a timeout, or disable a
linter to get to green. Three fix-and-push rounds with no new root cause means stop and
report what you found.

## Layout

The entry point lives in `cmd/gh-star-search/` and stays thin, delegating to
`internal/`. The compiler blocks imports of `internal/` from outside this module,
so anything under it can change freely.

One package, one purpose. Short lowercase names, no underscores (`httputil`, not
`http_util`), and no grab-bags (`util`, `common`, `misc`). Name a file after the
primary type it holds (`user.go`, `user_test.go`).

## Go conventions

- Define interfaces where they are consumed, not where implemented; keep them to 1-3
  methods and add one only once a consumer needs it
- Take `context.Context` as the first argument for cancellable or I/O-bound work
- MixedCaps naming with uppercase acronyms (`ServeHTTP`, `userID`, `GetHTTPClient`)
- Functional options for constructors with optional configuration (`WithTimeout(d)`)
- Return errors instead of panicking outside truly unrecoverable states, and wrap with
  context: `fmt.Errorf("loading config: %w", err)`
- Inspect with `errors.Is` / `errors.As`; define types for domain-specific errors
- Validate at boundaries and trust internal code (parse, don't validate)
- Doc-comment exported symbols, starting with the symbol name, describing non-obvious
  behavior and invariants rather than restating the types

Avoid: naked returns, functions past ~50 lines, deep nesting (return early), ignored
errors (`_ = doThing()` is almost always wrong), and shared global state (pass
dependencies explicitly).

[docs/go-best-practices.md](docs/go-best-practices.md) has a worked example of each
pattern above, plus the package layout rules and the anti-pattern list in full.

## Testing

Table-driven tests with subtests via `t.Run`, placed next to the code they cover, in a
`_test` package for black-box coverage. `mise run test:coverage-min` enforces the 70%
floor.

Golden fixtures are byte-exact: regenerate with `go test ./... -update` and review the
diff, never hand-edit. `hk.pkl` excludes `**/*.golden` from the whitespace fixers, so a
fixture under any other name (`testdata/golden_*.txt`) will be silently rewritten on
commit; either rename it or add its glob to that exclude.

## TUI testing

A subprocess pipe is not a terminal, so the program detects a non-tty and never renders.
Exploratory checks need a real PTY: run it under `tmux new -d -x 80 -y 24 <cmd>`, drive
it with `tmux send-keys`, and read what actually rendered with `tmux capture-pane -p`
(`-e` keeps ANSI codes). A tab in captured output is not proof the program emitted one:
`tmux capture-pane` re-emits a tab wherever the cursor was advanced across cells that
were never written. Check which column the text actually lands on before chasing it as
a rendering bug. For scripted tests prefer
`github.com/charmbracelet/x/exp/teatest`, which drives the `tea.Model` directly and
diffs golden frames; on Bubble Tea v2 that import is
`github.com/charmbracelet/x/exp/teatest/v2`, because the unsuffixed module targets v1.
Fall back to `github.com/creack/pty` for non-Bubble Tea binaries.

Exercise deliberately, because each of these renders fine in the happy path and breaks
elsewhere: resize mid-session (`SIGWINCH`) and the minimum supported size; every quit
path independently (`q`, `ctrl-c`, `esc`) restoring cursor and alt-screen state; empty
and single-item states; and piped non-tty stdout degrading instead of hanging.

Turn every bug found this way into a test named for its trigger
(`TestQuit_RestoresCursorOnCtrlC`), parametrized over terminal size rather than
hardcoding one.

## Template-managed files

This project is generated from
[my_go_template](https://github.com/KyleKing/my_go_template). Edit the template, not the
render, for anything under `.github/workflows/`, `.golangci.toml`, `hk.pkl`,
`.goreleaser.yml`, or `.config/mise/conf.d/template.toml` — a `copier update` overwrites
local edits there. Project-specific mise tasks belong in a sibling conf.d file
(CONTRIBUTING.md explains the load order that makes the filename matter).

After `copier update --UNSAFE --conflict=rej --defaults`, re-apply real local content
from each `.rej`, discard hunks the new template supersedes, and delete the files (a
hook blocks committing them). Three specifics:

- `.cz.toml` is re-rendered with `version = "0.0.0"`. Restore the real version by hand
  or the next release cuts `v0.0.1`.
- Scaffolding the project then implements (the `cmd/` entry point, the `bindings/` cgo
  shim, its Python wrapper) conflicts on every update, because the template keeps
  rendering its starting version. Re-apply from the `.rej` and move on
- **The same file conflicting on two consecutive updates means an answer is wrong, not
  that the patch needs re-applying.** Read `.copier-answers.yml` first and fix the
  answer; a typo there gets faithfully re-rendered every pass.

Resolve conflicts rather than avoiding them. Adding a file to `_skip_if_exists` looks
like it removes the friction, and instead it makes the file stop receiving every later
template fix, silently and forever. A conflict is loud and usually a few lines to
re-apply, whereas a skipped file drifts without ever saying so. `_skip_if_exists` is
for files the template has nothing further to say about after the first render, which
is why it holds `README.md`, `DESIGN.md`, `go.mod`, and `.config/mise.toml` and nothing
that carries template-maintained content.

Resolving a conflict is half the work. The other half is deciding which side was
better and backporting it. A `.rej` is a diff between what the template renders and
what this project actually needed, so it names the gap directly:

- The template's hunk is better, so take it and delete the local version
- The local hunk is better and generalizes, so take it locally AND open the change
  against [my_go_template](https://github.com/KyleKing/my_go_template) so the next
  render starts from it
- The local hunk is better and is specific to this project, so take it locally and
  leave the template alone

The second case is the one to watch for, because skipping it means re-resolving the
same conflict on every future update. If a file clobbers the same way twice, the
template is rendering the wrong thing and the fix belongs upstream.

This file is template-owned and `copier update` keeps it current. Put project-specific
guidance in `AGENTS.local.md` (loaded below when present) or in a nested `AGENTS.md`
scoped to its directory.

@AGENTS.local.md
