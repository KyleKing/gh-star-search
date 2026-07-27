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

| Command | Description |
|---------|-------------|
| `mise run bench` | Run benchmarks |
| `mise run build` | Build binary |
| `mise run ci` | Full CI check (tests + build) |
| `mise run clean` | Clean build artifacts |
| `mise run demo` | Generate VHS demo recordings |
| `mise run format` | Auto-fix lint and formatting |
| `mise run hooks` | Run git hooks |
| `mise run lint` | Run linter |
| `mise dev` | Run from source (`go run`, always reflects current code) |
| `mise run test` | Run tests with coverage |
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
go run ./cmd/gh-start-search [args]
```

To test the actual `gh gh-start-search ...` extension invocation or a Homebrew install, use the released version rather than installing from this checkout:

```bash
gh extension install KyleKing/gh-start-search
# or
brew install --formula https://github.com/KyleKing/gh-start-search/raw/main/Formula/gh-start-search.rb
```


## Releases

Automated via goreleaser on tag push. **Note:** For GH CLI extensions, the first release is required before users can run `gh extension install KyleKing/gh-start-search`.

### Creating a Release

1. Tag and push:

   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

2. GitHub Actions will automatically:
   - Run tests and build
   - Create release with binaries for Linux, macOS, Windows, and FreeBSD (amd64/arm64)
   - Publish to GitHub Releases

3. Verify the release has properly named binaries:
   - `gh-start-search-linux-amd64`
   - `gh-start-search-darwin-arm64`
   - `gh-start-search-windows-amd64.exe`
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
