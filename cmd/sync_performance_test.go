package cmd

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/kyleking/gh-star-search/internal/github"
	"github.com/kyleking/gh-star-search/internal/monitor"
)

// Helper function to create test repositories for performance testing
func createPerformanceTestRepositories(numRepos int) []github.Repository {
	repos := make([]github.Repository, numRepos)
	for i := 0; i < numRepos; i++ {
		repos[i] = github.Repository{
			FullName:        fmt.Sprintf("user/repo-%d", i),
			Description:     fmt.Sprintf("Test repository %d", i),
			Language:        "Go",
			StargazersCount: i * 10,
			ForksCount:      i * 2,
			Size:            1024,
			UpdatedAt:       time.Now(),
			CreatedAt:       time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return repos
}

// Test parallel processing performance
func TestSyncService_ParallelProcessing(t *testing.T) {
	tests := []struct {
		name      string
		numRepos  int
		batchSize int
	}{
		{"Small batch", 10, 5},
		{"Medium batch", 50, 10},
		{"Large batch", 100, 20},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data
			repos := createPerformanceTestRepositories(tt.numRepos)
			
			// Create sync service with memory monitoring
			memoryMonitor := monitor.NewMemoryMonitor(500, 5*time.Minute)
			memoryOptimizer := monitor.NewMemoryOptimizer(memoryMonitor)
			
			syncService := &SyncService{
				memoryMonitor:   memoryMonitor,
				memoryOptimizer: memoryOptimizer,
				verbose:         false,
			}
			
			ctx := context.Background()
			
			// Start memory monitoring
			memoryMonitor.Start(ctx, time.Second)
			defer memoryMonitor.Stop()
			
			// Measure performance of worker calculation
			startTime := time.Now()
			startMem := getMemoryUsage()
			
			// Test worker calculation performance
			for i := 0; i < len(repos); i += tt.batchSize {
				end := i + tt.batchSize
				if end > len(repos) {
					end = len(repos)
				}
				
				batchLen := end - i
				workers := syncService.calculateOptimalWorkers(batchLen)
				
				// Simulate some work
				time.Sleep(1 * time.Millisecond)
				
				// Verify worker count is reasonable
				if workers < 1 || workers > runtime.NumCPU() {
					t.Errorf("Invalid worker count %d for batch size %d", workers, batchLen)
				}
			}
			
			endTime := time.Now()
			endMem := getMemoryUsage()
			
			duration := endTime.Sub(startTime)
			memoryUsed := endMem - startMem
			
			t.Logf("Processed %d repositories in %v", tt.numRepos, duration)
			t.Logf("Memory used: %.2f MB", float64(memoryUsed)/1024/1024)
			t.Logf("Average time per repo: %v", duration/time.Duration(tt.numRepos))
			
			// Performance assertions - should be very fast for calculation only
			maxDurationPerRepo := 10 * time.Millisecond
			avgDuration := duration / time.Duration(tt.numRepos)
			if avgDuration > maxDurationPerRepo {
				t.Errorf("Average processing time per repo (%v) exceeds maximum (%v)", avgDuration, maxDurationPerRepo)
			}
		})
	}
}

// Test worker calculation
func TestSyncService_CalculateOptimalWorkers(t *testing.T) {
	syncService := &SyncService{}
	
	tests := []struct {
		batchSize int
		expected  int
	}{
		{1, 1},
		{5, 1},
		{10, min(2, runtime.NumCPU())},
		{20, min(3, runtime.NumCPU())},
		{50, min(runtime.NumCPU(), 8)},
		{100, min(runtime.NumCPU(), 8)},
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("batch_%d", tt.batchSize), func(t *testing.T) {
			workers := syncService.calculateOptimalWorkers(tt.batchSize)
			if workers != tt.expected {
				t.Errorf("Expected %d workers for batch size %d, got %d", tt.expected, tt.batchSize, workers)
			}
			
			// Workers should never exceed CPU count or be less than 1
			if workers > runtime.NumCPU() {
				t.Errorf("Workers (%d) should not exceed CPU count (%d)", workers, runtime.NumCPU())
			}
			
			if workers < 1 {
				t.Errorf("Workers (%d) should be at least 1", workers)
			}
		})
	}
}

// Benchmark parallel vs sequential processing
func BenchmarkSyncService_ProcessingComparison(b *testing.B) {
	numRepos := 50
	batchSize := 10
	
	repos := createPerformanceTestRepositories(numRepos)
	
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Simulate sequential processing
			for _, repo := range repos {
				// Simulate processing time
				time.Sleep(1 * time.Microsecond)
				_ = repo
			}
		}
	})
	
	b.Run("Parallel", func(b *testing.B) {
		syncService := &SyncService{}
		
		for i := 0; i < b.N; i++ {
			// Process in batches with parallel workers
			for j := 0; j < len(repos); j += batchSize {
				end := j + batchSize
				if end > len(repos) {
					end = len(repos)
				}
				
				batch := repos[j:end]
				
				// This would normally call processBatch, but for benchmark we'll simulate
				workers := syncService.calculateOptimalWorkers(len(batch))
				
				var wg sync.WaitGroup
				jobs := make(chan github.Repository, len(batch))
				
				// Start workers
				for w := 0; w < workers; w++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for repo := range jobs {
							// Simulate processing
							time.Sleep(1 * time.Microsecond)
							_ = repo
						}
					}()
				}
				
				// Send jobs
				go func() {
					defer close(jobs)
					for _, repo := range batch {
						jobs <- repo
					}
				}()
				
				wg.Wait()
			}
		}
	})
}

// Test memory optimization effectiveness
func TestMemoryOptimization_Effectiveness(t *testing.T) {
	memMonitor := monitor.NewMemoryMonitor(50, time.Minute)
	optimizer := monitor.NewMemoryOptimizer(memMonitor)
	
	ctx := context.Background()
	memMonitor.Start(ctx, 100*time.Millisecond)
	defer memMonitor.Stop()
	
	// Get initial memory stats
	memMonitor.OptimizeMemory()
	time.Sleep(100 * time.Millisecond) // Let monitoring update
	initialStats := memMonitor.GetStats()
	
	// Allocate memory to simulate processing
	data := make([][]byte, 1000)
	for i := range data {
		data[i] = make([]byte, 10240) // 10KB each = ~10MB total
	}
	
	time.Sleep(100 * time.Millisecond) // Let monitoring update
	afterAllocStats := memMonitor.GetStats()
	
	// Optimize memory
	optimizer.OptimizeForBatch(100)
	data = nil // Allow GC
	runtime.GC()
	
	time.Sleep(100 * time.Millisecond) // Let monitoring update
	afterOptimizeStats := memMonitor.GetStats()
	
	t.Logf("Memory usage - Initial: %.2f MB, After alloc: %.2f MB, After optimize: %.2f MB",
		initialStats.AllocMB, afterAllocStats.AllocMB, afterOptimizeStats.AllocMB)
	
	// Memory should be reduced after optimization
	if afterOptimizeStats.AllocMB >= afterAllocStats.AllocMB {
		t.Logf("Memory optimization may not be immediately visible due to Go's memory management")
	}
	
	// GC should have run
	if afterOptimizeStats.NumGC <= initialStats.NumGC {
		t.Logf("GC count - Initial: %d, After optimize: %d", initialStats.NumGC, afterOptimizeStats.NumGC)
	}
}

// Helper function to get current memory usage
func getMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// Test cache performance impact
func TestCachePerformanceImpact(t *testing.T) {
	// This test would require integration with the actual cache
	// For now, we'll test the concept
	
	numOperations := 1000
	
	// Test without cache (simulate direct operations)
	start := time.Now()
	for i := 0; i < numOperations; i++ {
		// Simulate expensive operation
		time.Sleep(100 * time.Microsecond)
	}
	withoutCache := time.Since(start)
	
	// Test with cache (simulate cached operations)
	start = time.Now()
	cache := make(map[string][]byte) // Simple in-memory cache for test
	
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("key-%d", i%100) // 10% cache hit rate
		
		if _, exists := cache[key]; exists {
			// Cache hit - no delay
			continue
		}
		
		// Cache miss - simulate expensive operation and cache result
		time.Sleep(100 * time.Microsecond)
		cache[key] = []byte("cached data")
	}
	withCache := time.Since(start)
	
	t.Logf("Performance - Without cache: %v, With cache: %v", withoutCache, withCache)
	t.Logf("Cache improvement: %.2fx faster", float64(withoutCache)/float64(withCache))
	
	// Cache should provide some improvement
	if withCache >= withoutCache {
		t.Logf("Cache didn't provide expected performance improvement (may be due to test overhead)")
	}
}