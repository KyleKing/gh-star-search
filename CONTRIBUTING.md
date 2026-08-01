# Contributing to gh-star-search

## Setup

Prerequisites: Go (see `go.mod`), [mise](https://mise.jdx.dev/), [hk](https://hk.jdx.dev/), [uv](https://docs.astral.sh/uv/)

```bash
mise install
hk install --mise
mise run ci
```

## Tasks

Shared tasks live in `.config/mise/conf.d/template.toml` (managed by the copier template).
Project-specific tasks go in additional `.config/mise/conf.d/*.toml` files.

mise loads `conf.d/*.toml` files in alphabetical order, and a task defined in more
than one file resolves to whichever file loaded last. Name your project file so it
sorts after `template.toml` (`user.toml` works; `project.toml` does not, since
`p` < `t`) or a same-named task override will silently do nothing.

| Command | Description |
|---------|-------------|
| `mise run bench` | Run benchmarks |
| `mise run build` | Build binary |
| `mise run ci` | Full CI check (tests + build) |
| `mise run clean` | Clean build artifacts |
| `mise run demo` | Generate VHS demo recordings (needs [vhs](https://github.com/charmbracelet/vhs) on `PATH`; it is not pinned in `[tools]`) |
| `mise run dev` | Run from source (`go run`, always reflects current code) |
| `mise run format` | Auto-fix lint and formatting |
| `mise run hooks` | Run git hooks |
| `mise run lint` | Run linter |
| `mise run test` | Run tests with coverage |
| `mise run test:coverage-min` | Fail below the 70% coverage threshold |
| `mise run test:view-coverage` | Open the coverage report in a browser |
| `mise tasks` | List all available tasks |

## Code Guidelines

Follow [AGENTS.md](AGENTS.md) for code organization, testing patterns, and error handling.

Linting is configured in `.golangci.toml` with 40+ rules. Run `mise run format` to auto-fix.

## Git Workflow

Conventional commits enforced via [commitizen](https://commitizen-tools.github.io/commitizen/):

```
<type>(<scope>): <subject>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Git hooks run automatically via hk on commit and push.

## Development Install

Run straight from source with `go run`, which always reflects the current code, so there's no built binary or installed extension to go stale between edits:

```bash
go run ./cmd/gh-star-search [args]
```

Or through mise, which runs the same thing:

```bash
mise run dev [args]
```

To test the actual `gh star-search ...` invocation (the GitHub CLI strips the `gh-` prefix from the repository name) or a Homebrew install, use the released version rather than installing from this checkout:

```bash
gh extension install KyleKing/gh-star-search
# or
brew install --cask KyleKing/tap/gh-star-search
```


## Releases

Automated by the Bump Version workflow. **Note:** the first release is required before users can run `gh extension install KyleKing/gh-star-search`.

### Creating a Release

1. Land a `fix:` or `feat:` commit on `main`. Commit types commitizen does not bump (`docs:`, `build(deps):`) cut no tag and publish nothing.

2. GitHub Actions will automatically:
   - Bump the version, update CHANGELOG.md, and push a `bump:` commit
   - Tag the new version
   - Run goreleaser to build binaries for Linux, macOS, Windows, and FreeBSD (amd64/arm64) and publish the release

   goreleaser runs inside that same workflow because a tag pushed with `GITHUB_TOKEN` does not trigger any other workflow.

3. Verify the release by distinct hash, not by asset count. Every target is a separate build, so the checksums must all differ; a repeated hash means one binary was published under several names:

   ```bash
   gh release download <tag> -p checksums.txt -O - | awk '{print $1}' | sort -u | wc -l
   ```

   Expect one line per binary. Names should read `gh-star-search-linux-amd64`, `gh-star-search-darwin-arm64`, `gh-star-search-windows-amd64.exe`, and so on.

### Installing via Homebrew

goreleaser builds the cask and pushes it to `https://github.com/KyleKing/homebrew-tap` as part of the release, with the SHA256 values taken from the artifacts it just built:

```bash
brew install --cask KyleKing/tap/gh-star-search
```

The push needs a `TAP_DEPLOY_KEY` secret scoped to the tap repo; run `scripts/provision-tap-deploy-key.sh` to create it. Without the secret the release still publishes every binary and skips the cask with a warning.


## Troubleshooting

```bash
mise install --force   # Reinstall tools
hk install --mise --force  # Reinstall hooks
go test -v -run TestName ./package  # Debug specific test
go test ./... -update  # Refresh golden fixtures, where the project has them
```

Golden files are byte-exact snapshots, so `hk.pkl` excludes `**/*.golden` from every
whitespace fixer. Regenerate them with `-update` and review the diff; never hand-edit.
