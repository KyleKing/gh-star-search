# Embedding Cache Architecture

## Overview

The evaluation tool uses **model-versioned persistent caches** to minimize embedding computation. Each model gets its own database that stores embeddings incrementally, computing them only once per repository per model.

## Performance Impact

### Before Caching (Naive Implementation)

- **Time**: 30-40 minutes for 7,000 repos
- **Why slow**:
    - Subprocess overhead: ~438 subprocess calls (219 per model)
    - Model reloading: Potentially reloading model for each batch
    - Full regeneration: Recomputing all 7,000 repos for both models

### After Caching (Current Implementation)

- **First run**: 5-8 minutes (only new model needs embedding)

    - Current model (e5-small-v2): 0 seconds (already in main DB)
    - Candidate model: 5-8 minutes (one-time cost, in-process)
    - Queries: 30-60 seconds

- **Subsequent runs**: 1-2 minutes

    - Current model: 0 seconds (cached)
    - Candidate model: 0 seconds (cached)
    - Only new/changed repos: ~5-10 seconds
    - Queries: 30-60 seconds

**Speed improvement: 90-95% faster on subsequent runs**

## Architecture

### Directory Structure

```
~/.local/share/gh-star-search/
├── stars.db                                    # Main repository data
└── embeddings/
    ├── intfloat__e5-small-v2.db               # Model-specific cache
    ├── BAAI__bge-small-en-v1.5.db             # Another model
    ├── sentence-transformers__all-MiniLM-L6-v2.db
    └── metadata.json                           # Cache registry
```

### Cache Database Schema

Each model gets its own SQLite/DuckDB file:

```sql
-- embeddings table: stores vectors with content tracking
CREATE TABLE embeddings (
    repo_id VARCHAR PRIMARY KEY,
    embedding JSON NOT NULL,              -- Vector as JSON array
    content_hash VARCHAR NOT NULL,        -- For change detection
    embedded_at TIMESTAMP NOT NULL,
    model_id VARCHAR NOT NULL,
    model_dimensions INTEGER NOT NULL
);

-- cache_metadata table: tracks cache state
CREATE TABLE cache_metadata (
    model_id VARCHAR PRIMARY KEY,
    dimensions INTEGER NOT NULL,
    total_embeddings INTEGER NOT NULL,
    last_sync TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL
);
```

### Metadata Registry (`metadata.json`)

Tracks all cached models:

```json
{
  "models": {
    "intfloat/e5-small-v2": {
      "db_path": "intfloat__e5-small-v2.db",
      "dimensions": 384,
      "total_repos": 7234,
      "last_sync": "2026-02-01T21:30:00Z",
      "created": "2026-01-15T10:00:00Z"
    }
  }
}
```

## How It Works

### 1. Incremental Sync

When evaluating a model:

```python
# Find repos needing embeddings
repos_to_embed = """
    SELECT r.*
    FROM repositories r
    LEFT JOIN cache.embeddings e
        ON r.id = e.repo_id
        AND r.content_hash = e.content_hash
    WHERE e.repo_id IS NULL  -- Not in cache or content changed
"""

# Only compute what's needed
for batch in chunks(repos_to_embed, 128):
    embeddings = model.encode(batch_texts)
    cache.store(embeddings)
```

### 2. Content-Based Invalidation

Embeddings are invalidated when `content_hash` changes:

- Repository description updated
- README content changed
- Topics modified
- Purpose/summary regenerated

### 3. In-Process Embedding

**Before (subprocess approach):**

```python
# Each call spawns process + loads model
for batch in batches:
    subprocess.run(["uv", "run", "python", "embed.py", ...])
    # ~5 seconds per batch (overhead + embedding)
```

**After (in-process approach):**

```python
# Load model once, keep in memory
model = SentenceTransformer(model_id)

for batch in batches:
    embeddings = model.encode(batch_texts)
    # ~1.5 seconds per batch (just embedding)
```

### 4. View-Based Access

During evaluation, cached embeddings are accessed via views:

```sql
-- Attach cache database
ATTACH 'embeddings/intfloat__e5-small-v2.db' AS model_cache;

-- Create view for queries
CREATE VIEW eval_embeddings AS
SELECT repo_id, embedding
FROM model_cache.embeddings;

-- Run queries against view
SELECT r.*,
       array_cosine_similarity(e.embedding, query_embedding) AS score
FROM repositories r
JOIN eval_embeddings e ON r.id = e.repo_id
ORDER BY score DESC;
```

## Usage

### Basic Evaluation (With Caching - Default)

```bash
cd internal/python/scripts
uv run python evaluate_embeddings.py
```

First run output:

```
Using persistent embedding cache (incremental updates)

============================================================
Evaluating model: e5-small-v2
  Model ID: intfloat/e5-small-v2
  Dimensions: 384
============================================================

  Syncing embeddings with cache...
  All repos already cached  # ← Fast! Reuses existing

============================================================
Evaluating model: bge-small
  Model ID: BAAI/bge-small-en-v1.5
  Dimensions: 384
============================================================

  Syncing embeddings with cache...
  Loading model: BAAI/bge-small-en-v1.5
  Need to embed 7234 repos
  Embedded batch 57/57                    # ← One-time cost
  Embedded 7234 new/changed repos
```

Second run output:

```
============================================================
Evaluating model: bge-small
============================================================

  Syncing embeddings with cache...
  All repos already cached  # ← Instant!
```

### View Cache Statistics

```bash
uv run python evaluate_embeddings.py --cache-stats
```

Output:

```
Embedding Cache Statistics:
======================================================================

Model: intfloat/e5-small-v2
  Embeddings: 7,234
  Dimensions: 384
  Last sync: 2026-02-01T21:30:00
  DB size: 12.3 MB

Model: BAAI/bge-small-en-v1.5
  Embeddings: 7,234
  Dimensions: 384
  Last sync: 2026-02-01T21:32:00
  DB size: 12.3 MB
```

### Disable Caching (Force Regeneration)

```bash
uv run python evaluate_embeddings.py --no-cache
```

Use when:

- Testing embedding generation logic
- Debugging issues
- Verifying cache correctness

## Cache Management

### Clear Cache for Specific Model

```bash
rm ~/.local/share/gh-star-search/embeddings/BAAI__bge-small-en-v1.5.db
# Next run will regenerate
```

### Clear All Caches

```bash
rm -rf ~/.local/share/gh-star-search/embeddings/
# Keeps main database, regenerates all embeddings on next run
```

### Force Re-embedding After Content Changes

The cache automatically detects content changes via `content_hash`. If you want to force re-embedding:

1. Update content in main DB (e.g., via `gh star-search sync`)
1. Run evaluation - only changed repos will be re-embedded

### Cache Size Estimates

- **Per repository**: ~1.5 KB (384-dim float32 vector as JSON)
- **7,000 repos**: ~10-12 MB per model
- **10 models**: ~100-120 MB total

## Benefits

### 1. Minimal Computation

- Each repo embedded once per model
- Incremental updates for new/changed repos
- No redundant work across evaluation runs

### 2. Fast Model Comparison

- Try multiple candidate models quickly
- First model: 5-8 min (one-time)
- Each additional model: 5-8 min (one-time)
- All subsequent runs: 1-2 min

### 3. Development Velocity

- Iterate on queries without re-embedding
- Test metrics changes instantly
- Experiment with evaluation logic

### 4. Production Integration

- Cache can be shared with main application
- Same embeddings used in production queries
- Consistent results between eval and production

## Implementation Details

### Model Loading Strategy

```python
class EmbeddingEvaluator:
    def __init__(self, ...):
        self._loaded_models = {}  # In-memory model cache

    def _get_or_load_model(self, model_id: str):
        if model_id not in self._loaded_models:
            # Load once, keep in memory
            self._loaded_models[model_id] = SentenceTransformer(model_id)
        return self._loaded_models[model_id]
```

**Benefit**: Model loaded once per evaluation run, not once per batch.

### Batch Size Optimization

```python
# Before (subprocess): Small batches to minimize process overhead
batch_size = 32

# After (in-process): Larger batches for better throughput
batch_size = 128
```

**Impact**: 4x fewer iterations, better GPU utilization.

### Query Embedding Optimization

```python
# Queries also use in-process embedding
def _run_query(self, query_text, model_config, ...):
    query_emb = self._generate_embeddings_in_process(
        [query_text],
        model_config.model_id
    )[0]
    # No subprocess overhead per query
```

**Impact**: 30 queries × 2 models = 60 queries, ~5 seconds faster.

## Migration from Old Implementation

Old scripts don't need migration - caching is automatic:

1. First run: Generates cache (slower)
1. Subsequent runs: Uses cache (fast)

To force old behavior:

```bash
uv run python evaluate_embeddings.py --no-cache
```

## Troubleshooting

### Cache corruption

```bash
# Verify cache integrity
uv run python -c "
from embedding_cache import EmbeddingCache
cache = EmbeddingCache()
stats = cache.get_cache_stats()
print(stats)
"

# If issues, clear and regenerate
rm ~/.local/share/gh-star-search/embeddings/BAAI__bge-small-en-v1.5.db
```

### Embeddings seem wrong

```bash
# Check metadata
cat ~/.local/share/gh-star-search/embeddings/metadata.json

# Verify dimensions match
uv run python evaluate_embeddings.py --cache-stats
```

### Cache not updating after sync

```bash
# Check content_hash changed in main DB
sqlite3 ~/.local/share/gh-star-search/stars.db \
    "SELECT id, content_hash FROM repositories LIMIT 5"

# Force re-sync by clearing cache for that model
rm ~/.local/share/gh-star-search/embeddings/intfloat__e5-small-v2.db
```

## Future Enhancements

1. **Shared cache with production**: Use same cache for `gh star-search query --mode vector`
1. **Compression**: Store embeddings as binary instead of JSON (50% size reduction)
1. **Parallel embedding**: Use multiple GPUs/threads for faster initial embedding
1. **Remote caching**: Share caches across machines via cloud storage
1. **Automatic cleanup**: Remove caches for models not used in N days
