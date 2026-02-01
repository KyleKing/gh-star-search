# Incomplete Features

## Fuzzy scoring ignores 3 of 7 field weights (bug)

**Location:** `internal/query/engine.go:256-264` and `:365-383`

`calculateFuzzyScore` defines weights for 7 fields:
```
full_name:    1.0
description:  0.8
purpose:      0.9
technologies: 0.7
features:     0.6
topics:       0.5
contributors: 0.4
```

But `getFieldContent` only handles 4: `full_name`, `description`, `topics`, `contributors`. The remaining 3 fall through to `return ""` and never contribute to scores.

**Fix:**
- `purpose`: `StoredRepo` has a `Purpose` field. Add `case "purpose": return repo.Purpose` to `getFieldContent`.
- `technologies` and `features`: These fields don't exist on `StoredRepo`. Either remove these weights or define what data they would draw from. If they were intended to be parsed from `Purpose` or `Description`, that extraction logic doesn't exist yet.

**Recommendation:** Wire up `purpose`, remove `technologies` and `features` weights until the data source exists.

---

## Vector search is non-functional at scale

**Location:** `internal/query/engine.go:128-205`

Current problems:
1. Creates a new `embedding.Manager` per query call with hardcoded config (`:134-140`), ignoring any user configuration.
2. Never reads the `repo_embedding` column from storage. Instead, generates embeddings on-the-fly for up to 1000 repos at query time (`:157, 175`).
3. Silently falls back to fuzzy search on any error (`:143-153`), including when the local Python provider isn't installed.

**Plan:**
1. Read pre-computed embeddings from the `repo_embedding` column in storage.
2. Accept the embedding Manager as a dependency (inject via constructor) instead of creating one per call.
3. Only generate the query embedding on-the-fly; repo embeddings should already be stored from sync.
4. Return an explicit error when embeddings are unavailable instead of silently falling back.

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
