# Incomplete Features

## ~~Fuzzy scoring ignores 3 of 7 field weights~~ (RESOLVED)

Replaced custom BM25-like scoring with DuckDB native FTS. The FTS index covers `full_name`, `description`, `purpose`, `topics_text`, and `contributors_text`. DuckDB handles BM25 scoring internally. The `technologies` and `features` fields were removed since no data source exists.

---

## ~~Vector search is non-functional at scale~~ (RESOLVED)

Fixed: `embedding.Manager` is injected via constructor. Query-time vector search reads pre-computed embeddings from the `repo_embedding` column using `array_cosine_similarity` in DuckDB SQL. Only the query embedding is generated on-the-fly. Returns an explicit error when embeddings are unavailable (no silent fallback to fuzzy).

---

## ~~`RemoteProvider` stub~~ (RESOLVED)

Deleted the `RemoteProvider` placeholder struct, `NewRemoteProvider`, and the `"remote"` case from `NewProvider`. Dead code removed.

---

## ~~Planned placeholders rendered in user output~~ (RESOLVED)

Removed the `(PLANNED: dependencies count)` and `(PLANNED: 'used by' count)` lines from `formatLong` output. Golden test files updated.

---

## ~~`formatRelatedStars` returns placeholder~~ (RESOLVED)

Implemented: `GetRelatedCounts` queries same-org repo count and shared-contributor repo count from the database. Transient fields `RelatedSameOrgCount` and `RelatedSharedContribCount` on `StoredRepo` are populated before display. Formatter now shows real counts.

---

## ~~`calculateVectorSimilarityScore` in related engine returns 0~~ (RESOLVED)

Implemented: reads `RepoEmbedding` from both target and candidate `StoredRepo` and computes cosine similarity. Negative scores are clamped to 0. Repos without embeddings return 0 and are excluded via weight renormalization.
