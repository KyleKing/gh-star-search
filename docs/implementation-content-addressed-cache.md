# Implementation Plan: Content-Addressed README Cache

Inspired by msgvault's SHA-256 content-addressed attachment storage that deduplicates by hash.

## Goal

Avoid re-downloading and re-processing README content for repositories that haven't changed. Use content hashing to detect changes and serve cached content from disk, reducing GitHub API calls and processing time during sync.

## Problem

The current sync pipeline calls `processor.Service.ExtractContent()` for every repository on every sync, even when `content_hash` hasn't changed. The staleness check (`last_synced` vs. threshold) gates whether a repo is visited at all, but once visited, the full content extraction runs regardless. This means:

1. README fetches hit the GitHub API even when the repo hasn't been updated
1. Content chunking and hashing runs redundantly
1. Summarization and embedding are already gated by `content_hash` changes, but the extraction step is not

## Design

### Change Detection Flow

```
sync visits repo
  |
  v
Has repo.UpdatedAt changed since last_synced?
  |
  No --> skip entirely (existing behavior)
  |
  Yes --> fetch repo metadata from GitHub API
           |
           v
         Has GitHub updated_at changed since stored updated_at?
           |
           No --> touch last_synced, skip content extraction
           |
           Yes --> proceed with content extraction
                    |
                    v
                  Compute content_hash of extracted content
                    |
                    v
                  Has content_hash changed?
                    |
                    No --> update metadata only, skip summary/embed
                    |
                    Yes --> full pipeline (store, summarize, embed)
```

The key addition is the second check: comparing `updated_at` timestamps before downloading content. This avoids the expensive content extraction step entirely for repos whose metadata changed (e.g., star count) but whose content didn't.

### Cache Structure

Extend the existing `internal/cache/` file cache to store extracted content keyed by repository full name and content hash:

```
~/.cache/gh-star-search/
  content/
    {sha256-first-16}.data      -- serialized ProcessedContent
    {sha256-first-16}.meta      -- JSON metadata (repo, hash, created_at, expires_at)
```

### Cache Key Strategy

```go
func contentCacheKey(fullName string, updatedAt time.Time) string {
    raw := fmt.Sprintf("%s:%s", fullName, updatedAt.UTC().Format(time.RFC3339))
    h := sha256.Sum256([]byte(raw))
    return hex.EncodeToString(h[:])[:16]
}
```

The key incorporates `updated_at` so a cache hit means the content is guaranteed fresh for that version. No need for a separate staleness TTL -- the key itself encodes freshness.

### Processor Changes

```go
// internal/processor/service.go

type ProcessedContent struct {
    Chunks      []Chunk   `json:"chunks"`
    ContentHash string    `json:"content_hash"`
    ExtractedAt time.Time `json:"extracted_at"`
}

func (s *serviceImpl) ExtractContent(
    ctx context.Context,
    repo *github.Repository,
    cache cache.Cache,
) (*ProcessedContent, error) {
    key := contentCacheKey(repo.GetFullName(), repo.GetUpdatedAt().Time)

    // Check cache
    if cache != nil {
        if data, err := cache.Get(ctx, key); err == nil {
            var cached ProcessedContent
            if err := json.Unmarshal(data, &cached); err == nil {
                return &cached, nil
            }
        }
    }

    // Extract content (existing logic)
    content, err := s.extractContentFromGitHub(ctx, repo)
    if err != nil {
        return nil, err
    }

    // Store in cache
    if cache != nil {
        if data, err := json.Marshal(content); err == nil {
            _ = cache.Set(ctx, key, data, 30*24*time.Hour) // 30-day TTL
        }
    }

    return content, nil
}
```

### Sync Command Changes

In `cmd/sync.go`, the sync loop gains an early-exit path:

```go
// After fetching updated metadata from GitHub API
if storedRepo.ContentHash != "" && ghRepo.GetUpdatedAt().Time.Equal(storedRepo.UpdatedAt) {
    // Metadata unchanged -- update last_synced timestamp only
    repo.TouchLastSynced(ctx, fullName)
    continue
}

// Content may have changed -- extract and check hash
content, err := processor.ExtractContent(ctx, ghRepo, fileCache)
if err != nil { ... }

if content.ContentHash == storedRepo.ContentHash {
    // Content unchanged despite metadata update -- update metadata only
    repo.UpdateRepositoryMetadata(ctx, fullName, metadataFields)
    continue
}

// Content changed -- full pipeline
repo.UpdateRepository(ctx, fullName, ...)
repo.UpdateRepositorySummary(...)
repo.UpdateRepositoryEmbedding(...)
```

### Storage Additions

```go
// Touch last_synced without re-processing
TouchLastSynced(ctx context.Context, fullName string) error
```

### Cache Eviction

The existing `FileCache` already handles:

- Max size enforcement (500 MB default) with LRU eviction
- TTL expiration via `Cleanup()`
- Background cleanup

The 30-day TTL for content entries means cached READMEs survive multiple sync cycles but don't accumulate indefinitely.

### Size Estimation

Average README after chunking and JSON serialization: ~50 KB. For 1000 starred repos: ~50 MB of cache. Well within the 500 MB default limit.

## Implementation Order

1. Add `ProcessedContent` struct with JSON serialization
1. Add `contentCacheKey()` function
1. Modify `ExtractContent()` to accept a `cache.Cache` parameter and check/populate cache
1. Add `TouchLastSynced()` to storage interface and implementation
1. Update sync loop in `cmd/sync.go` with early-exit paths
1. Tests: cache hit avoids GitHub API call, cache miss proceeds normally, updated_at change triggers re-extraction, content_hash match skips summary/embed

## Scope Boundaries

- Cache stores processed chunks, not raw README bytes
- No deduplication across repos (different repos with identical READMEs are cached separately, since the key includes repo name)
- No compression of cached content (JSON is small enough at this scale)
- Existing `content_hash` field on `repositories` table is unchanged
