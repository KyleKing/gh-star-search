# Requirements Document

## Introduction

This feature will create a GitHub CLI extension called `gh-star-search` that ingests and indexes all repositories starred by the currently logged-in user. The extension will collect both structured metadata (languages, stars, commits, issues, etc.) and unstructured content (README files, documentation, code comments) to enable natural language search queries against a local DuckDB database. Users will be able to search their starred repositories using natural language queries like "javascript formatter updated in last month," review and modify the generated DuckDB query before running, and then receive summarized results in the terminal.

## Requirements

### Requirement 1

**User Story:** As a GitHub user, I want to ingest all my starred repositories into a local database, so that I can search through them efficiently without making repeated API calls.

#### Acceptance Criteria

1. WHEN the user runs `gh star-search sync` THEN the system SHALL incrementally fetch (if not already cached locally) and process each repository that the authenticated GitHub user has starred
2. WHEN fetching repository data THEN the system SHALL collect structured metadata including languages, lines of code, repository size, star count, author, contributors, commit count, recent commit activity, issue counts, PR counts, release history, and other similar data
3. WHEN fetching repository data THEN the system SHALL collect unstructured content from README files, documentation directories, package manifests, and key source files to generate a high-level summary about the purpose of the repository, the implementation, and usage
4. WHEN parsing the unstructured data THEN the system SHALL use the configured local or remote LLM to summarize the content
5. WHEN a repository has been processed THEN the system SHALL append the extracted data into a local DuckDB database and optionally remove any local files
6. WHEN fine-tuning the sync operation THEN the system SHALL allow for a specific repository to be synced and output the intermediary steps for manual review
7. WHEN fine-tuning the sync operation THEN the system SHALL allow for comparing different model backends and prompts to compare the quality of the summary
8. IF the sync process encounters API rate limits THEN the system SHALL handle them gracefully with appropriate exponential backoff and retry logic

### Requirement 2

**User Story:** As a GitHub user, I want to search my starred repositories using natural language queries, so that I can quickly find relevant repositories based on their purpose or characteristics.

#### Acceptance Criteria

1. WHEN the user runs `gh star-search query "javascript formatter updated in last month"` THEN the system SHALL parse the natural language query to produce a valid DuckDB Query
2. WHEN a DuckDB Query is produced THEN the system SHALL allow a user to review and edit the query before running
3. WHEN processing a DuckDB query THEN the system SHALL search both structured and unstructured data in the local database
4. WHEN query results are found THEN the system SHALL display a summary of matching repositories with relevant structured and unstructured information
5. WHEN no results are found THEN the system SHALL display an appropriate "no results found" message
6. IF the local database doesn't exist THEN the system SHALL prompt the user to run sync first

### Requirement 3

**User Story:** As a GitHub user, I want the extension to follow GitHub CLI best practices, so that it integrates seamlessly with my existing GitHub workflow.

#### Acceptance Criteria

1. WHEN installing the extension THEN it SHALL be installable via `gh extension install`
2. WHEN using the extension THEN it SHALL use the existing GitHub CLI authentication
3. WHEN the extension runs THEN it SHALL follow GitHub CLI naming conventions and command structure
4. WHEN displaying output THEN it SHALL use consistent formatting with other GitHub CLI commands
5. WHEN errors occur THEN it SHALL provide helpful error messages following GitHub CLI patterns

### Requirement 4

**User Story:** As a GitHub user, I want to keep my starred repository data up-to-date, so that my searches reflect the current state of repositories.

#### Acceptance Criteria

1. WHEN the user runs `gh star-search sync` on an existing database THEN the system SHALL update existing repositories with new data
2. WHEN syncing THEN the system SHALL detect newly starred repositories and add them to the database
3. WHEN syncing THEN the system SHALL detect unstarred repositories and remove them from the database
4. WHEN updating repository data THEN the system SHALL preserve historical data where appropriate
5. IF a repository is no longer accessible THEN the system SHALL handle the error gracefully and optionally remove it from the database

### Requirement 5

**User Story:** As a GitHub user, I want to view and manage my local repository database, so that I can understand what data is stored and maintain the system.

#### Acceptance Criteria

1. WHEN the user runs `gh star-search list` THEN the system SHALL display all repositories in the local database with basic information
2. WHEN the user runs `gh star-search info <repo>` THEN the system SHALL display detailed information about a specific repository
3. WHEN the user runs `gh star-search stats` THEN the system SHALL display statistics about the local database (total repos, last sync time, database size)
4. WHEN the user runs `gh star-search clear` THEN the system SHALL provide an option to clear the local database
5. IF the user confirms clearing THEN the system SHALL remove all data and the database file

### Requirement 6

**User Story:** As a GitHub user, I want the extension to handle large numbers of starred repositories efficiently, so that sync and search operations remain performant.

#### Acceptance Criteria

1. WHEN syncing repositories THEN the system SHALL process them in batches to avoid memory issues
2. WHEN making GitHub API calls THEN the system SHALL respect rate limits and use efficient API endpoints
3. WHEN searching the database THEN queries SHALL complete in under 5 seconds for databases with up to 10,000 repositories
4. WHEN storing unstructured content THEN the system SHALL implement reasonable size limits to prevent database bloat
5. WHEN processing THEN the system SHALL display progress indicators during operations

### Requirement 7

**User Story:** As a GitHub user, I want the system to intelligently extract and process unstructured content from repositories, so that I can search based on repository purpose, implementation details, and usage patterns.

#### Acceptance Criteria

1. WHEN processing unstructured content THEN the system SHALL prioritize README files, CHANGELOG files, documentation directories (docs/, wiki/), package.json description fields, and license files for content extraction
2. WHEN extracting code context THEN the system SHALL collect comments and docstrings from main entry points, configuration files, and primary source files to understand implementation approach
3. WHEN chunking content for processing THEN the system SHALL create logical chunks based on document sections, code blocks, and natural content boundaries with maximum chunk size of 2000 tokens
4. WHEN generating embeddings THEN the system SHALL extract key information including repository purpose, main technologies used, target use cases, installation/usage instructions, and notable features
5. WHEN storing processed content THEN the system SHALL maintain both the original text chunks and generated summaries for different search granularities
6. WHEN content exceeds size limits THEN the system SHALL prioritize README content, then documentation, then code comments in order of importance
7. IF a repository has no meaningful unstructured content THEN the system SHALL rely on structured metadata and repository description for search purposes
