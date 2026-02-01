-- Create repositories table with full schema
CREATE TABLE IF NOT EXISTS repositories (
    id VARCHAR PRIMARY KEY,
    full_name VARCHAR UNIQUE NOT NULL,
    description TEXT,
    homepage TEXT,
    language VARCHAR,
    stargazers_count INTEGER,
    forks_count INTEGER,
    size_kb INTEGER,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    last_synced TIMESTAMP,

    -- Activity & Metrics
    open_issues_open INTEGER DEFAULT 0,
    open_issues_total INTEGER DEFAULT 0,
    open_prs_open INTEGER DEFAULT 0,
    open_prs_total INTEGER DEFAULT 0,
    commits_30d INTEGER DEFAULT 0,
    commits_1y INTEGER DEFAULT 0,
    commits_total INTEGER DEFAULT 0,

    -- Metadata arrays and objects
    topics_array JSON DEFAULT '[]',
    languages JSON DEFAULT '{}',
    contributors JSON DEFAULT '[]',

    -- License
    license_name VARCHAR,
    license_spdx_id VARCHAR,

    -- Content tracking
    content_hash VARCHAR,

    -- Summarization (AI-generated summaries)
    purpose TEXT,
    summary_generated_at TIMESTAMP,
    summary_version INTEGER DEFAULT 0,

    -- Vector embeddings for semantic search
    repo_embedding JSON
);

-- Create indexes for query performance
-- Note: PRIMARY KEY and UNIQUE constraints automatically create ART indexes
-- See: https://duckdb.org/docs/stable/sql/indexes
CREATE INDEX IF NOT EXISTS idx_repositories_language ON repositories(language);
CREATE INDEX IF NOT EXISTS idx_repositories_updated_at ON repositories(updated_at);
CREATE INDEX IF NOT EXISTS idx_repositories_stargazers ON repositories(stargazers_count);
CREATE INDEX IF NOT EXISTS idx_repositories_full_name ON repositories(full_name);
CREATE INDEX IF NOT EXISTS idx_repositories_commits_total ON repositories(commits_total);
