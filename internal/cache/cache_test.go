package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestFileCache_BasicOperations(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	
	cache, err := NewFileCache(tempDir, 10, time.Hour, time.Minute)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()
	
	ctx := context.Background()
	
	// Test Set and Get
	key := "test-key"
	data := []byte("test data")
	
	err = cache.Set(ctx, key, data, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set cache entry: %v", err)
	}
	
	retrieved, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get cache entry: %v", err)
	}
	
	if string(retrieved) != string(data) {
		t.Errorf("Retrieved data doesn't match. Expected: %s, Got: %s", string(data), string(retrieved))
	}
	
	// Test Delete
	err = cache.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Failed to delete cache entry: %v", err)
	}
	
	_, err = cache.Get(ctx, key)
	if err == nil {
		t.Error("Expected error when getting deleted key, but got none")
	}
}

func TestFileCache_TTL(t *testing.T) {
	tempDir := t.TempDir()
	
	cache, err := NewFileCache(tempDir, 10, time.Hour, time.Minute)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()
	
	ctx := context.Background()
	
	// Set entry with short TTL
	key := "ttl-test"
	data := []byte("ttl test data")
	
	err = cache.Set(ctx, key, data, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to set cache entry: %v", err)
	}
	
	// Should be available immediately
	_, err = cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get cache entry before expiration: %v", err)
	}
	
	// Wait for expiration
	time.Sleep(200 * time.Millisecond)
	
	// Should be expired now
	_, err = cache.Get(ctx, key)
	if err == nil {
		t.Error("Expected error when getting expired key, but got none")
	}
}

func TestFileCache_SizeLimit(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create cache with very small size limit (1MB)
	cache, err := NewFileCache(tempDir, 1, time.Hour, time.Minute)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()
	
	ctx := context.Background()
	
	// Add entries that exceed the size limit
	largeData := make([]byte, 512*1024) // 512KB
	for i := 0; i < len(largeData); i++ {
		largeData[i] = byte(i % 256)
	}
	
	// Add first entry
	err = cache.Set(ctx, "large1", largeData, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set first large entry: %v", err)
	}
	
	// Add second entry (should trigger cleanup)
	err = cache.Set(ctx, "large2", largeData, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set second large entry: %v", err)
	}
	
	// Add third entry (should trigger more cleanup)
	err = cache.Set(ctx, "large3", largeData, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set third large entry: %v", err)
	}
	
	// Check that cache size is within limits
	size, err := cache.Size(ctx)
	if err != nil {
		t.Fatalf("Failed to get cache size: %v", err)
	}
	
	maxSizeBytes := int64(1 * 1024 * 1024) // 1MB in bytes
	if size > maxSizeBytes {
		t.Errorf("Cache size %d exceeds limit %d", size, maxSizeBytes)
	}
}

func TestFileCache_Stats(t *testing.T) {
	tempDir := t.TempDir()
	
	cache, err := NewFileCache(tempDir, 10, time.Hour, time.Minute)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()
	
	ctx := context.Background()
	
	// Add some entries
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		data := []byte(fmt.Sprintf("data-%d", i))
		
		err = cache.Set(ctx, key, data, time.Hour)
		if err != nil {
			t.Fatalf("Failed to set entry %d: %v", i, err)
		}
	}
	
	// Get some entries (hits)
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, err = cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get entry %d: %v", i, err)
		}
	}
	
	// Try to get non-existent entries (misses)
	for i := 10; i < 12; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, err = cache.Get(ctx, key)
		if err == nil {
			t.Errorf("Expected miss for key %s, but got hit", key)
		}
	}
	
	// Check stats
	stats, err := cache.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}
	
	if stats.TotalEntries != 5 {
		t.Errorf("Expected 5 entries, got %d", stats.TotalEntries)
	}
	
	if stats.Hits != 3 {
		t.Errorf("Expected 3 hits, got %d", stats.Hits)
	}
	
	if stats.Misses != 2 {
		t.Errorf("Expected 2 misses, got %d", stats.Misses)
	}
	
	expectedHitRate := float64(3) / float64(5)
	if stats.HitRate != expectedHitRate {
		t.Errorf("Expected hit rate %.2f, got %.2f", expectedHitRate, stats.HitRate)
	}
}

func TestFileCache_Cleanup(t *testing.T) {
	tempDir := t.TempDir()
	
	cache, err := NewFileCache(tempDir, 10, time.Hour, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()
	
	ctx := context.Background()
	
	// Add entries with different TTLs
	shortTTL := 50 * time.Millisecond
	longTTL := time.Hour
	
	err = cache.Set(ctx, "short1", []byte("data1"), shortTTL)
	if err != nil {
		t.Fatalf("Failed to set short TTL entry: %v", err)
	}
	
	err = cache.Set(ctx, "short2", []byte("data2"), shortTTL)
	if err != nil {
		t.Fatalf("Failed to set short TTL entry: %v", err)
	}
	
	err = cache.Set(ctx, "long1", []byte("data3"), longTTL)
	if err != nil {
		t.Fatalf("Failed to set long TTL entry: %v", err)
	}
	
	// Wait for short TTL entries to expire
	time.Sleep(100 * time.Millisecond)
	
	// Manual cleanup
	err = cache.Cleanup(ctx)
	if err != nil {
		t.Fatalf("Failed to cleanup cache: %v", err)
	}
	
	// Check that expired entries are gone
	_, err = cache.Get(ctx, "short1")
	if err == nil {
		t.Error("Expected expired entry to be cleaned up")
	}
	
	_, err = cache.Get(ctx, "short2")
	if err == nil {
		t.Error("Expected expired entry to be cleaned up")
	}
	
	// Check that non-expired entry is still there
	_, err = cache.Get(ctx, "long1")
	if err != nil {
		t.Error("Expected non-expired entry to still be available")
	}
}

func BenchmarkFileCache_Set(b *testing.B) {
	tempDir := b.TempDir()
	
	cache, err := NewFileCache(tempDir, 100, time.Hour, time.Minute)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()
	
	ctx := context.Background()
	data := make([]byte, 1024) // 1KB data
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		err = cache.Set(ctx, key, data, time.Hour)
		if err != nil {
			b.Fatalf("Failed to set cache entry: %v", err)
		}
	}
}

func BenchmarkFileCache_Get(b *testing.B) {
	tempDir := b.TempDir()
	
	cache, err := NewFileCache(tempDir, 100, time.Hour, time.Minute)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()
	
	ctx := context.Background()
	data := make([]byte, 1024) // 1KB data
	
	// Pre-populate cache
	numEntries := 1000
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		err = cache.Set(ctx, key, data, time.Hour)
		if err != nil {
			b.Fatalf("Failed to set cache entry: %v", err)
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-key-%d", i%numEntries)
		_, err = cache.Get(ctx, key)
		if err != nil {
			b.Fatalf("Failed to get cache entry: %v", err)
		}
	}
}