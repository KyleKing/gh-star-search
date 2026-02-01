# Contributing to gh-star-search

## Setup

Prerequisites: Go (see `go.mod`), [mise](https://mise.jdx.dev/), [hk](https://hk.jdx.dev/)

```bash
mise install
hk install --mise
mise run ci
```

## Tasks

Shared tasks live in `.config/mise.template.toml` (managed by the copier template).
Project-specific tasks go in `.config/mise.project.toml` or other `mise.*.toml` files.

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


## Releases

Automated via goreleaser on tag push:

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions builds binaries for Linux, macOS, Windows, and FreeBSD (amd64/arm64).

### Updating the Homebrew Formula

After a release, update `Formula/gh-star-search.rb`:

1. Download the release binaries from the GitHub release page
2. Generate SHA256 checksums:

   ```bash
   shasum -a 256 gh-star-search-darwin-arm64 gh-star-search-darwin-amd64 gh-star-search-linux-arm64 gh-star-search-linux-amd64
   ```

   Or run `mise run brew:sha` for a reminder of these steps.

3. Update the `version` and `sha256` values in `Formula/gh-star-search.rb`
4. Commit and push the formula changes

### Installing via Homebrew

Users can install directly from the repository formula:

```bash
brew install --formula https://github.com/kyleking/gh-star-search/raw/main/Formula/gh-star-search.rb
```

Or from a local checkout:

```bash
brew install --formula ./Formula/gh-star-search.rb
```

To set up a [homebrew tap](https://docs.brew.sh/Taps) for `brew install kyleking/tap/gh-star-search`, create a `homebrew-tap` repo at `https://github.com/kyleking/homebrew-tap` and copy the formula there.


## Troubleshooting

```bash
mise install --force   # Reinstall tools
hk install --mise --force  # Reinstall hooks
go test -v -run TestName ./package  # Debug specific test
```
