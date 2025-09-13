package cache

import (
	"context"
	"time"
)

// Cache defines the interface for local file caching operations
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Size(ctx context.Context) (int64, error)
	Cleanup(ctx context.Context) error
}

// Entry represents a cache entry with metadata
type Entry struct {
	Key       string    `json:"key"`
	Data      []byte    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Size      int64     `json:"size"`
}

// Stats represents cache statistics
type Stats struct {
	TotalEntries int64 `json:"total_entries"`
	TotalSize    int64 `json:"total_size"`
	HitRate      float64 `json:"hit_rate"`
	MissRate     float64 `json:"miss_rate"`
}
