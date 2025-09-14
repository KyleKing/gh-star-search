# Requirement Changes

The original design was too vague and complicated. Instead, the search capability and interactions will be simplified as described below.

When making any edits, concisely document the differences in a separate "Changes" section. Make all other changes in-place for an LLM to implement

## CLI Search Changes

1. Instead of natural language to build DuckDB queries, accept a search query string. The user will not see nor edit a DuckDB query
1. The query string will be used to search against the users' starred repositories GitHub Description, Org and Name, Contributors, generated summary, and other reasonable chunked full text fields
1. The user can select either fuzzy search or vector search when providing the query string
1. The returned result will include a score based on the match quality
1. A configurable number of matches will be returned, which defaults to 10
1. Searching by stars, languages, or other structured data is not currently supported

## New "Related" CLI Feature

1. A user can request to see all of their starred repositories related to a given repository
2. There are several measures of "related" - same organization, shared contributors, by GitHub Topic, by Summary Vector Similarity, etc.
3. The similar repositories are displayed in short-form with an additional line of why there was a match (e.g. topic matches, shared contributor commits, etc.)

## LLM Changes

1. Support non-LLM summarization with the `transformers` package from Python (example: https://huggingface.co/docs/transformers/v4.56.1/en/tasks/summarization#summarization)
2. If no LLM is configured, the non-LLM summarization is the default

## Caching Changes

1. When re-running sync, metadata for the repository is only updated if the last_synced date is more than a configurable number of days (default 14 days)
1. When re-running sync, even when metadata is updated, the repository summary is only updated when forced
1. Only process the main README, the GitHub Description, docs/README.md if found, and the text of the scraped URL linked from GitHub or in the main READMEs instead of attempting to process the entire repo.
1. Instead of cloning the repo, only download the minimum number of files

1. Only process the main README file to generate a summary
1. Temporarily remove the LLM integration and just use the Summary for now

## CLI Long and Short Form Output

1. When the CLI returns a match for a repository, each should be shown in this long-form below, which needs more precise specification for how each number will be a calculated. There will also be an option for short form structure with only the first two lines of the long-form summary

    > [<org>/<name>](https://github.com/<org>/<name>)
    > GitHub Description: <...>
    > GitHub External Description Link: <...>
    > Numbers: <#>/<#> open issues, <#>/<#> open PRs, <#> stars, <#> forks
    > Commits: <#> in last 30 days, <#> in last year, <#> total
    > Age: <readable-time>
    > License: <...>
    > Top 10 Contributors: <Name (Commits)>, ...
    > GitHub Topics: <...>, ...
    > Languages: <Lang. (LOC)>, ...
    > Related Stars: <#> in <org>, <#> by <contributor> (*calculated by counting how many starred repositories also have at least one contribution by any of the top ten contributors to this one*)
    > Last synced: <readable-time>
    > Summary: <...> (*optional*)
    > (PLANNED: *Number of dependencies based on parsing the lock files or GitHub's Data)
    > (PLANNED: *Number of projects that depend on this one from GitHub's data)

## Documentation Changes

1. Add a CONTRIBUTING.md file with an overview of the architecture and implementation, the GitHub API endpoints used and their rate limits, etc.

## Long-term Planning

These are ideas added to the specification, but not in the task list because the current implementation is sufficient for now

1. Consider tracking code coverage with `go test -cover`
1. Consider using `gomock` for GitHub and LLM APIs
1. Consider configuring `golang-migrate` to support migrations before distributing to users. For now, the database can be deleted and recreated during development
1. Consider using Bubble Tea for a TUI interactive experience instead of only a CLI
1. Consider migrating from Cobra to `github.com/urfave/cli/v3` (https://cli.urfave.org/v3/getting-started)
1. Consider using subcommands to group some operations
1. Consider using <https://tmc.github.io/langchaingo/docs/how-to/configure-llm-providers> for the LLM integration
