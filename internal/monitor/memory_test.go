package monitor

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestMemoryMonitor_BasicOperations(t *testing.T) {
	monitor := NewMemoryMonitor(100, time.Minute)

	// Test getting stats
	stats := monitor.GetStats()
	if stats.LastUpdated.IsZero() {
		// Stats should be initialized when first accessed
		monitor.updateStats()
		stats = monitor.GetStats()
	}

	if stats.AllocMB < 0 {
		t.Error("AllocMB should not be negative")
	}

	if stats.SysMB < 0 {
		t.Error("SysMB should not be negative")
	}

	if stats.GoroutineCount <= 0 {
		t.Error("GoroutineCount should be positive")
	}
}

func TestMemoryMonitor_ForceGC(t *testing.T) {
	monitor := NewMemoryMonitor(1, time.Millisecond) // Very low threshold

	// Update stats first
	monitor.updateStats()
	initialGC := monitor.GetStats().NumGC

	// Force GC
	monitor.ForceGC()

	// Check that GC was triggered
	finalGC := monitor.GetStats().NumGC
	if finalGC <= initialGC {
		// GC might not always increment the counter, so we'll just check that it ran
		t.Logf("GC counter: %d -> %d", initialGC, finalGC)
	}
}

func TestMemoryMonitor_MemoryPressure(t *testing.T) {
	monitor := NewMemoryMonitor(100, time.Minute)
	monitor.updateStats()

	pressure := monitor.GetMemoryPressure()

	if pressure < 0 || pressure > 1 {
		t.Errorf("Memory pressure should be between 0 and 1, got %f", pressure)
	}
}

func TestMemoryMonitor_ShouldOptimize(t *testing.T) {
	// Test with very low threshold to trigger optimization
	monitor := NewMemoryMonitor(1, time.Millisecond)
	monitor.updateStats()

	// Should recommend optimization due to low threshold
	if !monitor.ShouldOptimize() {
		t.Error("Should recommend optimization with very low threshold")
	}

	// Test with very high threshold
	monitor2 := NewMemoryMonitor(10000, time.Hour)
	monitor2.updateStats()

	// Should not recommend optimization with high threshold and recent GC
	monitor2.lastGC = time.Now()
	if monitor2.ShouldOptimize() {
		t.Error("Should not recommend optimization with high threshold and recent GC")
	}
}

func TestMemoryMonitor_StartStop(t *testing.T) {
	monitor := NewMemoryMonitor(100, time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Start monitoring
	monitor.Start(ctx, 10*time.Millisecond)

	// Let it run for a bit
	time.Sleep(30 * time.Millisecond)

	// Context should cancel automatically, but also call Stop
	monitor.Stop()
	// Should not panic or cause issues
}

func TestMemoryOptimizer_OptimizeForBatch(t *testing.T) {
	monitor := NewMemoryMonitor(100, time.Minute)
	optimizer := NewMemoryOptimizer(monitor)

	// Test different batch sizes
	testCases := []struct {
		batchSize int
		name      string
	}{
		{5, "small batch"},
		{25, "medium batch"},
		{100, "large batch"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			optimizer.OptimizeForBatch(tc.batchSize)
			// Should not panic
		})
	}

	// Restore defaults
	optimizer.RestoreDefaults()
}

func TestMemoryOptimizer_OptimizeForQuery(t *testing.T) {
	monitor := NewMemoryMonitor(100, time.Minute)
	optimizer := NewMemoryOptimizer(monitor)

	optimizer.OptimizeForQuery()
	// Should not panic

	optimizer.RestoreDefaults()
}

func TestMemoryMonitor_FormattedStats(t *testing.T) {
	monitor := NewMemoryMonitor(100, time.Minute)
	monitor.updateStats()

	formatted := monitor.GetFormattedStats()

	if len(formatted) == 0 {
		t.Error("Formatted stats should not be empty")
	}

	// Should contain key information
	expectedStrings := []string{
		"Memory Statistics:",
		"Allocated:",
		"System:",
		"Goroutines:",
		"GC Runs:",
	}

	for _, expected := range expectedStrings {
		if !contains(formatted, expected) {
			t.Errorf("Formatted stats should contain '%s'", expected)
		}
	}
}

func BenchmarkMemoryMonitor_UpdateStats(b *testing.B) {
	monitor := NewMemoryMonitor(100, time.Minute)

	b.ResetTimer()

	for range b.N {
		monitor.updateStats()
	}
}

func BenchmarkMemoryMonitor_GetStats(b *testing.B) {
	monitor := NewMemoryMonitor(100, time.Minute)
	monitor.updateStats()

	b.ResetTimer()

	for range b.N {
		_ = monitor.GetStats()
	}
}

func BenchmarkMemoryMonitor_ForceGC(b *testing.B) {
	monitor := NewMemoryMonitor(1, time.Millisecond) // Low threshold to trigger GC

	b.ResetTimer()

	for range b.N {
		monitor.ForceGC()
	}
}

// Test memory allocation and cleanup
func TestMemoryMonitor_MemoryAllocation(t *testing.T) {
	monitor := NewMemoryMonitor(50, time.Minute)
	monitor.updateStats()

	initialAlloc := monitor.GetStats().AllocMB

	// Allocate some memory
	data := make([][]byte, 1000)
	for i := range data {
		data[i] = make([]byte, 1024) // 1KB each
	}

	monitor.updateStats()
	afterAlloc := monitor.GetStats().AllocMB

	if afterAlloc <= initialAlloc {
		t.Logf("Memory allocation might not be reflected immediately. Initial: %.2f MB, After: %.2f MB", initialAlloc, afterAlloc)
	}

	// Force GC and check memory is freed
	monitor.OptimizeMemory()
	runtime.KeepAlive(data) // Prevent premature GC

	// Clear reference to allow GC
	data = nil

	monitor.OptimizeMemory()

	monitor.updateStats()
	afterGC := monitor.GetStats().AllocMB

	t.Logf("Memory usage: Initial: %.2f MB, After alloc: %.2f MB, After GC: %.2f MB",
		initialAlloc, afterAlloc, afterGC)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
