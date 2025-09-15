package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	GetStats(ctx context.Context) (*Stats, error)
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
	TotalEntries int64   `json:"total_entries"`
	TotalSize    int64   `json:"total_size"`
	HitRate      float64 `json:"hit_rate"`
	MissRate     float64 `json:"miss_rate"`
	Hits         int64   `json:"hits"`
	Misses       int64   `json:"misses"`
}

// FileCache implements the Cache interface using the filesystem
type FileCache struct {
	directory   string
	maxSizeMB   int64
	defaultTTL  time.Duration
	cleanupFreq time.Duration
	mu          sync.RWMutex
	stats       Stats
	stopCleanup chan struct{}
	cleanupOnce sync.Once
}

// NewFileCache creates a new file-based cache
func NewFileCache(
	directory string,
	maxSizeMB int,
	defaultTTL, cleanupFreq time.Duration,
) (*FileCache, error) {
	// Expand path if it starts with ~
	if strings.HasPrefix(directory, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}

		directory = filepath.Join(home, directory[2:])
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &FileCache{
		directory:   directory,
		maxSizeMB:   int64(maxSizeMB) * 1024 * 1024, // Convert MB to bytes
		defaultTTL:  defaultTTL,
		cleanupFreq: cleanupFreq,
		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go cache.backgroundCleanup()

	return cache, nil
}

// Get retrieves data from cache
func (c *FileCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	filePath := c.getFilePath(key)
	metaPath := c.getMetaPath(key)

	// Check if files exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.stats.Misses++
		return nil, errors.New("cache miss: key not found")
	}

	// Read metadata
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		c.stats.Misses++
		return nil, fmt.Errorf("failed to read cache metadata: %w", err)
	}

	var entry Entry
	if err := json.Unmarshal(metaData, &entry); err != nil {
		c.stats.Misses++
		return nil, fmt.Errorf("failed to parse cache metadata: %w", err)
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		c.stats.Misses++
		// Clean up expired entry
		os.Remove(filePath)
		os.Remove(metaPath)

		return nil, errors.New("cache miss: entry expired")
	}

	// Read data
	data, err := os.ReadFile(filePath)
	if err != nil {
		c.stats.Misses++
		return nil, fmt.Errorf("failed to read cache data: %w", err)
	}

	c.stats.Hits++

	return data, nil
}

// Set stores data in cache with TTL
func (c *FileCache) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if ttl == 0 {
		ttl = c.defaultTTL
	}

	filePath := c.getFilePath(key)
	metaPath := c.getMetaPath(key)

	// Create entry metadata
	entry := Entry{
		Key:       key,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Size:      int64(len(data)),
	}

	// Check cache size limits before writing
	if err := c.enforceSize(entry.Size); err != nil {
		return fmt.Errorf("failed to enforce cache size: %w", err)
	}

	// Write data file
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache data: %w", err)
	}

	// Write metadata file
	metaData, err := json.Marshal(entry)
	if err != nil {
		os.Remove(filePath) // Clean up data file on error
		return fmt.Errorf("failed to marshal cache metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaData, 0600); err != nil {
		os.Remove(filePath) // Clean up data file on error
		return fmt.Errorf("failed to write cache metadata: %w", err)
	}

	return nil
}

// Delete removes an entry from cache
func (c *FileCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	filePath := c.getFilePath(key)
	metaPath := c.getMetaPath(key)

	// Remove both files, ignore errors if files don't exist
	os.Remove(filePath)
	os.Remove(metaPath)

	return nil
}

// Clear removes all entries from cache
func (c *FileCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Remove all files in cache directory
	entries, err := os.ReadDir(c.directory)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			os.Remove(filepath.Join(c.directory, entry.Name()))
		}
	}

	// Reset stats
	c.stats = Stats{}

	return nil
}

// Size returns the total size of cached data
func (c *FileCache) Size(ctx context.Context) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	return c.calculateSize()
}

// Cleanup removes expired entries
func (c *FileCache) Cleanup(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	now := time.Now()

	var removedCount int64

	entries, err := os.ReadDir(c.directory)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta") {
			continue
		}

		metaPath := filepath.Join(c.directory, entry.Name())

		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			continue // Skip files we can't read
		}

		var cacheEntry Entry
		if err := json.Unmarshal(metaData, &cacheEntry); err != nil {
			continue // Skip files we can't parse
		}

		// Remove expired entries
		if now.After(cacheEntry.ExpiresAt) {
			key := strings.TrimSuffix(entry.Name(), ".meta")
			filePath := filepath.Join(c.directory, key+".data")

			os.Remove(filePath)
			os.Remove(metaPath)

			removedCount++
		}
	}

	return nil
}

// GetStats returns cache statistics
func (c *FileCache) GetStats(ctx context.Context) (*Stats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Count current entries and size
	var totalEntries int64

	totalSize, _ := c.Size(ctx)

	entries, err := os.ReadDir(c.directory)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".data") {
				totalEntries++
			}
		}
	}

	stats := c.stats
	stats.TotalEntries = totalEntries
	stats.TotalSize = totalSize

	// Calculate hit/miss rates
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
		stats.MissRate = float64(stats.Misses) / float64(total)
	}

	return &stats, nil
}

// Close stops the background cleanup goroutine
func (c *FileCache) Close() error {
	c.cleanupOnce.Do(func() {
		close(c.stopCleanup)
	})

	return nil
}

// getFilePath returns the file path for a cache key
func (c *FileCache) getFilePath(key string) string {
	hash := c.hashKey(key)
	return filepath.Join(c.directory, hash+".data")
}

// getMetaPath returns the metadata file path for a cache key
func (c *FileCache) getMetaPath(key string) string {
	hash := c.hashKey(key)
	return filepath.Join(c.directory, hash+".meta")
}

// hashKey creates a safe filename from a cache key
func (c *FileCache) hashKey(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))

	return hex.EncodeToString(hasher.Sum(nil))[:16] // Use first 16 chars for shorter filenames
}

// enforceSize ensures cache doesn't exceed size limits
func (c *FileCache) enforceSize(newEntrySize int64) error {
	// Calculate current size without acquiring locks (we're already holding a write lock)
	currentSize, err := c.calculateSize()
	if err != nil {
		return err
	}

	if currentSize+newEntrySize <= c.maxSizeMB {
		return nil // Within limits
	}

	// Need to free up space - remove oldest entries first
	entries, err := os.ReadDir(c.directory)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	// Collect entries with their modification times
	type entryInfo struct {
		name    string
		modTime time.Time
		size    int64
	}

	var entryInfos []entryInfo

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Get corresponding data file size
		dataName := strings.TrimSuffix(entry.Name(), ".meta") + ".data"

		dataPath := filepath.Join(c.directory, dataName)
		if dataInfo, err := os.Stat(dataPath); err == nil {
			entryInfos = append(entryInfos, entryInfo{
				name:    strings.TrimSuffix(entry.Name(), ".meta"),
				modTime: info.ModTime(),
				size:    dataInfo.Size(),
			})
		}
	}

	// Sort by modification time (oldest first)
	for i := range len(entryInfos) - 1 {
		for j := i + 1; j < len(entryInfos); j++ {
			if entryInfos[i].modTime.After(entryInfos[j].modTime) {
				entryInfos[i], entryInfos[j] = entryInfos[j], entryInfos[i]
			}
		}
	}

	// Remove entries until we have enough space
	spaceNeeded := (currentSize + newEntrySize) - c.maxSizeMB

	var spaceFreed int64

	for _, info := range entryInfos {
		if spaceFreed >= spaceNeeded {
			break
		}

		// Remove entry
		dataPath := filepath.Join(c.directory, info.name+".data")
		metaPath := filepath.Join(c.directory, info.name+".meta")

		os.Remove(dataPath)
		os.Remove(metaPath)

		spaceFreed += info.size
	}

	return nil
}

// calculateSize calculates the total size without acquiring locks
func (c *FileCache) calculateSize() (int64, error) {
	var totalSize int64

	err := filepath.WalkDir(c.directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".data") {
			info, err := d.Info()
			if err != nil {
				return err
			}

			totalSize += info.Size()
		}

		return nil
	})

	return totalSize, err
}

// backgroundCleanup runs periodic cleanup of expired entries
func (c *FileCache) backgroundCleanup() {
	ticker := time.NewTicker(c.cleanupFreq)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = c.Cleanup(context.Background())
		case <-c.stopCleanup:
			return
		}
	}
}
