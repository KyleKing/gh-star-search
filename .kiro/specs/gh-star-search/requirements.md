# Requirements Document

## Introduction

This feature will create a GitHub CLI extension called `gh-star-search` that ingests and indexes all repositories starred by the currently logged-in user. The extension will collect both structured metadata (languages, stars, commits, issues, etc.) and unstructured content (README files, documentation) to enable search queries against a local DuckDB database. Users will be able to search their starred repositories using query strings with fuzzy or vector search, receive scored results, and view summarized results in the terminal with long or short form output.

## Requirements

### Requirement 1

**User Story:** As a GitHub user, I want to ingest all my starred repositories into a local database, so that I can search through them efficiently without making repeated API calls.

#### Acceptance Criteria

1. WHEN the user runs `gh star-search sync` THEN the system SHALL incrementally fetch (if not already cached locally) and process each repository that the authenticated GitHub user has starred
2. WHEN fetching repository data THEN the system SHALL collect structured metadata including languages, lines of code, repository size, star count, author, contributors, commit count, recent commit activity, issue counts, PR counts, release history, and other similar data
3. WHEN fetching repository data THEN the system SHALL collect unstructured content from the main README, GitHub Description, docs/README.md if found, and text from scraped URLs linked from GitHub or main READMEs to generate a summary
4. WHEN parsing the unstructured data THEN the system SHALL use the configured LLM or non-LLM summarization with transformers package, defaulting to non-LLM if no LLM is configured
5. WHEN a repository has been processed THEN the system SHALL append the extracted data into a local DuckDB database and remove downloaded files
6. WHEN fine-tuning the sync operation THEN the system SHALL allow for a specific repository to be synced and output the intermediary steps for manual review
7. WHEN fine-tuning the sync operation THEN the system SHALL allow for comparing different model backends and prompts to compare the quality of the summary
8. IF the sync process encounters API rate limits THEN the system SHALL handle them gracefully with appropriate exponential backoff and retry logic

### Requirement 2

**User Story:** As a GitHub user, I want to search my starred repositories using query strings, so that I can quickly find relevant repositories based on their purpose or characteristics.

#### Acceptance Criteria

1. WHEN the user runs `gh star-search query "<query>"` THEN the system SHALL accept a search query string and search against GitHub Description, Org and Name, Contributors, generated summary, and other chunked full text fields
2. WHEN providing the query THEN the system SHALL allow the user to select either fuzzy search or vector search
3. WHEN query results are found THEN the system SHALL return a configurable number of matches (default 10) with a score based on match quality
4. WHEN displaying results THEN the system SHALL show repositories in long-form or short-form format with structured and unstructured information
5. WHEN no results are found THEN the system SHALL display an appropriate "no results found" message
6. IF the local database doesn't exist THEN the system SHALL prompt the user to run sync first

### Requirement 4

**User Story:** As a GitHub user, I want the extension to follow GitHub CLI best practices, so that it integrates seamlessly with my existing GitHub workflow.

#### Acceptance Criteria

1. WHEN installing the extension THEN it SHALL be installable via `gh extension install`
2. WHEN using the extension THEN it SHALL use the existing GitHub CLI authentication
3. WHEN the extension runs THEN it SHALL follow GitHub CLI naming conventions and command structure
4. WHEN displaying output THEN it SHALL use consistent formatting with other GitHub CLI commands
5. WHEN errors occur THEN it SHALL provide helpful error messages following GitHub CLI patterns

### Requirement 5

**User Story:** As a GitHub user, I want to keep my starred repository data up-to-date, so that my searches reflect the current state of repositories.

#### Acceptance Criteria

1. WHEN the user runs `gh star-search sync` on an existing database THEN the system SHALL update existing repositories with new metadata only if last_synced is more than a configurable number of days (default 14 days)
    1. WHEN updating repository data THEN the system SHALL update the repository summary only when forced
2. WHEN syncing THEN the system SHALL detect newly starred repositories and add them to the database
3. WHEN syncing THEN the system SHALL detect unstarred repositories and remove them from the database
4. WHEN updating repository data THEN the system SHALL preserve historical data where appropriate
5. IF a repository is no longer accessible THEN the system SHALL handle the error gracefully and optionally remove it from the database

### Requirement 6

**User Story:** As a GitHub user, I want to view and manage my local repository database, so that I can understand what data is stored and maintain the system.

#### Acceptance Criteria

1. WHEN the user runs `gh star-search list` THEN the system SHALL display all repositories in the local database in short-form (first two lines of long-form)
2. WHEN the user runs `gh star-search info <repo>` THEN the system SHALL display detailed information about a specific repository in long-form as specified
3. WHEN the user runs `gh star-search stats` THEN the system SHALL display statistics about the local database (total repos, last sync time, database size)
4. WHEN the user runs `gh star-search clear` THEN the system SHALL provide an option to clear the local database
5. IF the user confirms clearing THEN the system SHALL remove all data and the database file

### Requirement 7

**User Story:** As a GitHub user, I want the extension to handle large numbers of starred repositories efficiently, so that sync and search operations remain performant.

#### Acceptance Criteria

1. WHEN syncing repositories THEN the system SHALL process them in batches to avoid memory issues
2. WHEN making GitHub API calls THEN the system SHALL respect rate limits and use efficient API endpoints
3. WHEN searching the database THEN queries SHALL complete in under 5 seconds for databases with up to 10,000 repositories
4. WHEN storing unstructured content THEN the system SHALL implement reasonable size limits to prevent database bloat
5. WHEN processing THEN the system SHALL display progress indicators during operations

### Requirement 8

**User Story:** As a GitHub user, I want the system to intelligently extract and process unstructured content from repositories, so that I can search based on repository purpose, implementation details, and usage patterns.

#### Acceptance Criteria

1. WHEN processing unstructured content THEN the system SHALL prioritize the main README, GitHub Description, docs/README.md if found, and text from scraped URLs for content extraction
2. WHEN extracting code context THEN the system SHALL collect comments and docstrings from main entry points, configuration files, and primary source files to understand implementation approach
3. WHEN chunking content for processing THEN the system SHALL create logical chunks based on document sections, code blocks, and natural content boundaries with maximum chunk size of 2000 tokens
4. WHEN generating embeddings THEN the system SHALL extract key information including repository purpose, main technologies used, target use cases, installation/usage instructions, and notable features
5. WHEN storing processed content THEN the system SHALL maintain both the original text chunks and generated summaries for different search granularities
6. WHEN content exceeds size limits THEN the system SHALL prioritize README content, then documentation, then code comments in order of importance
7. IF a repository has no meaningful unstructured content THEN the system SHALL rely on structured metadata and repository description for search purposes

### Requirement 9

**User Story:** As a contributor, I want documentation on the architecture and implementation, so that I can understand and contribute to the project.

#### Acceptance Criteria

1. WHEN the project has a CONTRIBUTING.md file THEN it SHALL include an overview of the architecture and implementation
2. WHEN documenting THEN the system SHALL list the GitHub API endpoints used and their rate limits
3. WHEN documenting THEN the system SHALL include other relevant technical details for contributors

## Changes

This section summarizes the modifications made to the requirements document based on the new guidance:

- **Introduction**: Updated to reflect simplified search queries instead of natural language to DuckDB, added mention of fuzzy/vector search, scored results, and long/short form output.
- **Requirement 2**: Replaced natural language query parsing with direct search query string acceptance, added fuzzy/vector search options, scored results with configurable number (default 10), removed DuckDB query editing, updated display to long/short form, added note that structured data search is not supported.
- **Requirement 3**: Added new requirement for "Related" CLI feature to show related starred repositories based on organization, contributors, topics, and vector similarity, displayed in short-form with match explanations.
- **Requirements 4-9**: Renumbered due to insertion of new requirement.
- **Requirement 1**: Updated unstructured content collection to focus on main README, GitHub Description, docs/README.md, and scraped URLs; modified summarization to support both LLM and non-LLM (transformers) with non-LLM as default; changed file handling to download minimum files instead of cloning and remove them after processing.
- **Requirement 6**: Added caching logic to update metadata only after 14 days (configurable) and summaries only when forced.
- **Requirement 7**: Updated output for list to short-form and info to long-form as specified in the new guidance.
- **Requirement 8**: Updated content prioritization to main README, GitHub Description, docs/README.md, and scraped URLs.
- **Requirement 9**: Added new requirement for CONTRIBUTING.md documentation including architecture overview, GitHub API details, and rate limits.
