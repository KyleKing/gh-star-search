# DuckDB Constraint Workaround

## Problem Summary

DuckDB v1.8.x has a critical limitation where `UPDATE` and `DELETE+INSERT` operations fail with "duplicate key" constraint errors when executed within transactions on tables with `PRIMARY KEY` constraints. This particularly affects columns with JSON/LIST data types.

### Error Example
```
Constraint Error: Duplicate key "id: <uuid>" violates primary key constraint
```

## Root Cause

DuckDB performs over-eager constraint checking that evaluates PRIMARY KEY constraints **before DELETE operations complete** within the transaction scope. Even though the DELETE should remove the conflicting row before the INSERT happens, DuckDB's constraint validator sees both operations simultaneously and throws a false constraint violation.

### Affected Operations
- ✗ `UPDATE` statements on tables with PRIMARY KEY (internally rewritten as DELETE+INSERT)
- ✗ `DELETE...RETURNING` + `INSERT` within transactions
- ✗ Explicit `DELETE` + `INSERT` within transactions
- ✓ `DELETE` + `INSERT` **without** transactions (workaround)

## Solution Implemented

### For `UpdateRepository` and `UpdateRepositoryMetrics`

**Pattern**: Sequential DELETE+INSERT **without** transaction wrapper

```go
// 1. Read existing data to preserve fields
existingData := SELECT ... FROM repositories WHERE full_name = ?

// 2. Delete old row (fast)
DELETE FROM repositories WHERE id = ?

// 3. Insert updated row (fast)
INSERT INTO repositories VALUES (...)
```

### Trade-offs

| Aspect | With Transaction | Without Transaction (Current) |
|--------|-----------------|-------------------------------|
| Atomicity | ✓ ACID guaranteed | ✗ Not atomic |
| Concurrency Safety | ✗ Fails with constraint error | ✓ Works (last-write-wins) |
| Read Consistency | ✗ Readers fail during update | ✓ Sequential ops are fast enough |
| Metrics Preservation | N/A - Can't execute | ✓ Captured in step 1 |

**Risk mitigation**: Single-row operations execute in microseconds, making the non-atomic window negligible for practical purposes.

## References

- [Issue #11915](https://github.com/duckdb/duckdb/issues/11915): UPDATE with LIST columns fails
- [Issue #16520](https://github.com/duckdb/duckdb/issues/16520): DELETE+INSERT in transaction fails (even in v1.2.0+)
- [Issue #8764](https://github.com/duckdb/duckdb/issues/8764): UPDATE fails without changing PRIMARY KEY
- [DuckDB Indexes Documentation](https://duckdb.org/docs/stable/sql/indexes#index-limitations): Known constraint limitations

## Future Improvements

When DuckDB fixes the over-eager constraint checking:
1. Wrap operations in transactions for true atomicity
2. Add optimistic locking with version fields
3. Consider using DuckDB's `INSERT OR REPLACE` if available
