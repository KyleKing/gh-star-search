# Database Migrations

Simple sequential SQL migrations for schema evolution.

## How It Works

1. Migrations are numbered SQL files: `001_description.sql`, `002_description.sql`, etc.
1. On startup, the system checks current schema version
1. Executes any migrations with version > current version
1. Each migration runs in a transaction and is recorded in `schema_version` table

## Adding a New Migration

1. **Create a new SQL file** with the next sequential number:

    ```bash
    # If latest is 001_initial_schema.sql, create:
    touch internal/storage/migrations/002_add_archived_field.sql
    ```

1. **Write your SQL** (DDL only, no DOWN migrations):

    ```sql
    -- Add archived column to repositories
    ALTER TABLE repositories ADD COLUMN IF NOT EXISTS archived BOOLEAN DEFAULT false;

    -- Add index for archived repositories
    CREATE INDEX IF NOT EXISTS idx_repositories_archived ON repositories(archived);
    ```

1. **Test locally**:

    ```bash
    # Migrations run automatically on app start
    go run cmd/gh-star-search/main.go stats

    # Or run tests
    go test ./internal/storage -v
    ```

1. **Commit** - migrations are embedded in the binary via `//go:embed`

## File Naming Convention

- Format: `NNN_description.sql`
- `NNN` = three-digit version number (001, 002, 003, ...)
- `description` = snake_case description
- Examples:
    - `001_initial_schema.sql`
    - `002_add_embeddings.sql`
    - `003_add_user_tags.sql`

## Best Practices

- **Idempotent operations**: Use `IF NOT EXISTS`, `IF EXISTS`, `COALESCE`, etc.
- **One migration per change**: Don't bundle unrelated changes
- **Test with fresh database**: Ensure migration works from scratch
- **Test with existing data**: Ensure migration works on populated database
- **No rollbacks**: This system doesn't support DOWN migrations
- **Comments**: Explain why, not what (SQL is self-documenting)

## Example: Adding a New Table

```sql
-- internal/storage/migrations/003_add_user_tags.sql
CREATE TABLE IF NOT EXISTS user_tags (
    id VARCHAR PRIMARY KEY,
    repository_id VARCHAR NOT NULL,
    tag VARCHAR NOT NULL,
    color VARCHAR,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_tags_repository ON user_tags(repository_id);
CREATE INDEX IF NOT EXISTS idx_user_tags_tag ON user_tags(tag);
```

## Example: Adding a Column

```sql
-- internal/storage/migrations/004_add_archived_flag.sql
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS archived BOOLEAN DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_repositories_archived ON repositories(archived);
```

## Checking Schema Version

The `schema_version` table tracks applied migrations:

```sql
SELECT version, name, applied_at
FROM schema_version
ORDER BY version;
```

## Troubleshooting

**Migration fails to apply:**

- Check SQL syntax with DuckDB CLI
- Ensure migration is idempotent
- Check logs for error details

**Version mismatch:**

- If schema is corrupted, clear and re-sync:
    ```bash
    gh star-search clear
    gh star-search sync
    ```

**Adding migration between versions:**

- Don't renumber! Skip the number if needed
- Example: If you have 001 and 003, that's fine
- System sorts by number, not gaps
