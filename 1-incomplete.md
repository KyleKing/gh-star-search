# Incomplete Features

## ~~Fuzzy scoring ignores 3 of 7 field weights~~ (RESOLVED)

Replaced custom BM25-like scoring with DuckDB native FTS. The FTS index covers `full_name`, `description`, `purpose`, `topics_text`, and `contributors_text`. DuckDB handles BM25 scoring internally. The `technologies` and `features` fields were removed since no data source exists.

---

## ~~Vector search is non-functional at scale~~ (RESOLVED)

Fixed: `embedding.Manager` is injected via constructor. Query-time vector search reads pre-computed embeddings from the `repo_embedding` column using `array_cosine_similarity` in DuckDB SQL. Only the query embedding is generated on-the-fly. Returns an explicit error when embeddings are unavailable (no silent fallback to fuzzy).

---

## `RemoteProvider` stub

**Location:** `internal/embedding/provider.go:140-170`

`RemoteProvider` is a complete stub:
- `GenerateEmbedding` returns a zero vector
- `IsEnabled` returns `false`
- No API key validation

**Plan:** If a remote provider is desired (OpenAI, Cohere, etc.):
1. Accept an API key via config
2. Implement HTTP client with rate limiting
3. Wire `IsEnabled` to check key presence and validity
4. Otherwise, delete the stub to avoid dead code

---

## Planned placeholders rendered in user output

**Location:** `internal/formatter/formatter.go:146-148`

`formatLong` appends literal strings to output:
```
(PLANNED: dependencies count)
(PLANNED: 'used by' count)
```

These are visible to end users.

**Fix:** Remove these lines. If/when dependency data is available, add them back with real values.

---

## `formatRelatedStars` returns placeholder

**Location:** `internal/formatter/formatter.go:322-327`

Always returns `"? in {org}, ? by top contributors"` regardless of data.

**Plan:** This needs a query that counts how many other starred repos share the same org or top contributors. The data is available in the database -- the `related` engine already computes these signals. Wire the formatter to accept pre-computed related counts, or remove the field from output until it's implemented.

---

## `calculateVectorSimilarityScore` in related engine returns 0

**Location:** `internal/related/engine.go:307-311`

Always returns `0.0` with a TODO comment. The related scoring weights allocate 20% to vector similarity (`:156`) but it's always zero, so the remaining components are renormalized to fill 100%.

**Plan:** Once embeddings are stored in the database, read the `repo_embedding` for both target and candidate repos and compute cosine similarity. The `cosineSimilarity` function already exists in the same file (`:440`).
