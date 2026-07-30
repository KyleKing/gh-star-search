# Contributing to gh-star-search

## Setup

Prerequisites: Go (see `go.mod`), [mise](https://mise.jdx.dev/), [hk](https://hk.jdx.dev/), [uv](https://docs.astral.sh/uv/)

```bash
mise install
hk install --mise
mise run ci
```

## Tasks

Shared tasks live in `.config/mise/conf.d/template.toml` (managed by the copier template). Project-specific tasks go in additional `.config/mise/conf.d/*.toml` files.

mise loads `conf.d/*.toml` files in alphabetical order, and a task defined in more than one file resolves to whichever file loaded last. Name your project file so it sorts after `template.toml` (`user.toml` works; `project.toml` does not, since `p` < `t`) or a same-named task override will silently do nothing.

| Command           | Description                                              |
| ----------------- | -------------------------------------------------------- |
| `mise run bench`  | Run benchmarks                                           |
| `mise run build`  | Build binary                                             |
| `mise run ci`     | Full CI check (tests + build)                            |
| `mise run clean`  | Clean build artifacts                                    |
| `mise run demo`   | Generate VHS demo recordings                             |
| `mise run format` | Auto-fix lint and formatting                             |
| `mise run hooks`  | Run git hooks                                            |
| `mise run lint`   | Run linter                                               |
| `mise dev`        | Run from source (`go run`, always reflects current code) |
| `mise run test`   | Run tests with coverage                                  |
| `mise tasks`      | List all available tasks                                 |

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

To test the actual `gh gh-star-search ...` extension invocation or a Homebrew install, use the released version rather than installing from this checkout:

```bash
gh extension install KyleKing/gh-star-search
# or
brew install --formula https://github.com/KyleKing/gh-star-search/raw/main/Formula/gh-star-search.rb
```

## Releases

Automated by the Bump Version workflow. **Note:** For GH CLI extensions, the first release is required before users can run `gh extension install KyleKing/gh-star-search`.

### Creating a Release

1. Land a `fix:` or `feat:` commit on `main`. Commit types commitizen does not bump (`docs:`, `build(deps):`) cut no tag and publish nothing.

1. GitHub Actions will automatically:

    - Bump the version, update CHANGELOG.md, and push a `bump:` commit
    - Tag the new version
    - Run goreleaser to build binaries for Linux, macOS, Windows, and FreeBSD (amd64/arm64) and publish the release

    goreleaser runs inside that same workflow because a tag pushed with `GITHUB_TOKEN` does not trigger any other workflow.

1. Verify the release has properly named binaries:

    - `gh-star-search-linux-amd64`
    - `gh-star-search-darwin-arm64`
    - `gh-star-search-windows-amd64.exe`
    - etc.

### Updating the Homebrew Formula

After a release, update `Formula/gh-star-search.rb`:

1. Download the release binaries from the GitHub release page

1. Generate SHA256 checksums:

    ```bash
    shasum -a 256 gh-star-search-darwin-arm64 gh-star-search-darwin-amd64 gh-star-search-linux-arm64 gh-star-search-linux-amd64
    ```

    Or run `mise run brew:sha` for a reminder of these steps.

1. Update the `version` and `sha256` values in `Formula/gh-star-search.rb`

1. Commit and push the formula changes

### Installing via Homebrew

Users can install directly from the repository formula:

```bash
brew install --formula https://github.com/KyleKing/gh-star-search/raw/main/Formula/gh-star-search.rb
```

Or from a local checkout:

```bash
brew install --formula ./Formula/gh-star-search.rb
```

To set up a [homebrew tap](https://docs.brew.sh/Taps) for `brew install KyleKing/tap/gh-star-search`, create a `homebrew-tap` repo at `https://github.com/KyleKing/homebrew-tap` and copy the formula there.

## Troubleshooting

```bash
mise install --force   # Reinstall tools
hk install --mise --force  # Reinstall hooks
go test -v -run TestName ./package  # Debug specific test
```
