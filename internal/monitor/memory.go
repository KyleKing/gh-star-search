package monitor

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// MemoryMonitor tracks memory usage and provides optimization features
type MemoryMonitor struct {
	mu                sync.RWMutex
	stats             MemoryStats
	gcThresholdMB     int64
	gcForceInterval   time.Duration
	lastGC            time.Time
	stopMonitoring    chan struct{}
	monitoringStarted bool
}

// MemoryStats represents memory usage statistics
type MemoryStats struct {
	AllocMB        float64   `json:"alloc_mb"`
	TotalAllocMB   float64   `json:"total_alloc_mb"`
	SysMB          float64   `json:"sys_mb"`
	NumGC          uint32    `json:"num_gc"`
	GCCPUFraction  float64   `json:"gc_cpu_fraction"`
	HeapObjectsMB  float64   `json:"heap_objects_mb"`
	StackInUseMB   float64   `json:"stack_in_use_mb"`
	LastUpdated    time.Time `json:"last_updated"`
	GoroutineCount int       `json:"goroutine_count"`
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor(gcThresholdMB int64, gcForceInterval time.Duration) *MemoryMonitor {
	return &MemoryMonitor{
		gcThresholdMB:   gcThresholdMB,
		gcForceInterval: gcForceInterval,
		stopMonitoring:  make(chan struct{}),
	}
}

// Start begins memory monitoring
func (m *MemoryMonitor) Start(ctx context.Context, interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.monitoringStarted {
		return
	}

	m.monitoringStarted = true
	go m.monitorLoop(ctx, interval)
}

// Stop stops memory monitoring
func (m *MemoryMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.monitoringStarted {
		return
	}

	select {
	case <-m.stopMonitoring:
		// Channel already closed
	default:
		close(m.stopMonitoring)
	}
	
	m.monitoringStarted = false
}

// GetStats returns current memory statistics
func (m *MemoryMonitor) GetStats() MemoryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// ForceGC forces garbage collection if conditions are met
func (m *MemoryMonitor) ForceGC() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	
	// Check if we should force GC based on memory usage or time
	shouldForceGC := m.stats.AllocMB > float64(m.gcThresholdMB) ||
		now.Sub(m.lastGC) > m.gcForceInterval

	if shouldForceGC {
		runtime.GC()
		debug.FreeOSMemory()
		m.lastGC = now
		
		// Update stats after GC
		m.updateStats()
	}
}

// OptimizeMemory performs various memory optimizations
func (m *MemoryMonitor) OptimizeMemory() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Force garbage collection
	runtime.GC()
	
	// Return memory to OS
	debug.FreeOSMemory()
	
	// Set GC target percentage for more aggressive collection
	debug.SetGCPercent(50) // More aggressive than default 100%
	
	m.lastGC = time.Now()
	m.updateStats()
}

// SetGCTarget sets the garbage collection target percentage
func (m *MemoryMonitor) SetGCTarget(percent int) {
	debug.SetGCPercent(percent)
}

// GetMemoryPressure returns a value from 0-1 indicating memory pressure
func (m *MemoryMonitor) GetMemoryPressure() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Calculate pressure based on allocated memory vs system memory
	if m.stats.SysMB == 0 {
		return 0
	}

	pressure := m.stats.AllocMB / m.stats.SysMB
	if pressure > 1.0 {
		pressure = 1.0
	}

	return pressure
}

// ShouldOptimize returns true if memory optimization is recommended
func (m *MemoryMonitor) ShouldOptimize() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Calculate pressure without calling GetMemoryPressure to avoid deadlock
	var pressure float64
	if m.stats.SysMB > 0 {
		pressure = m.stats.AllocMB / m.stats.SysMB
		if pressure > 1.0 {
			pressure = 1.0
		}
	}

	// Optimize if memory usage is high or it's been a while since last GC
	return m.stats.AllocMB > float64(m.gcThresholdMB) ||
		time.Since(m.lastGC) > m.gcForceInterval ||
		pressure > 0.8
}

// GetFormattedStats returns human-readable memory statistics
func (m *MemoryMonitor) GetFormattedStats() string {
	stats := m.GetStats()
	
	return fmt.Sprintf(`Memory Statistics:
  Allocated: %.2f MB
  Total Allocated: %.2f MB  
  System: %.2f MB
  Heap Objects: %.2f MB
  Stack In Use: %.2f MB
  Goroutines: %d
  GC Runs: %d
  GC CPU Fraction: %.4f
  Memory Pressure: %.2f
  Last Updated: %s`,
		stats.AllocMB,
		stats.TotalAllocMB,
		stats.SysMB,
		stats.HeapObjectsMB,
		stats.StackInUseMB,
		stats.GoroutineCount,
		stats.NumGC,
		stats.GCCPUFraction,
		m.GetMemoryPressure(),
		stats.LastUpdated.Format("15:04:05"),
	)
}

// monitorLoop runs the monitoring loop
func (m *MemoryMonitor) monitorLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			m.updateStats()
			
			// Check if we should force GC (avoid calling ShouldOptimize to prevent deadlock)
			shouldOptimize := m.stats.AllocMB > float64(m.gcThresholdMB) ||
				time.Since(m.lastGC) > m.gcForceInterval
			
			if shouldOptimize {
				m.ForceGC()
			}
			
			m.mu.Unlock()
			
		case <-m.stopMonitoring:
			return
		case <-ctx.Done():
			return
		}
	}
}

// updateStats updates the current memory statistics
func (m *MemoryMonitor) updateStats() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.stats = MemoryStats{
		AllocMB:        float64(memStats.Alloc) / 1024 / 1024,
		TotalAllocMB:   float64(memStats.TotalAlloc) / 1024 / 1024,
		SysMB:          float64(memStats.Sys) / 1024 / 1024,
		NumGC:          memStats.NumGC,
		GCCPUFraction:  memStats.GCCPUFraction,
		HeapObjectsMB:  float64(memStats.HeapObjects*8) / 1024 / 1024, // Approximate
		StackInUseMB:   float64(memStats.StackInuse) / 1024 / 1024,
		LastUpdated:    time.Now(),
		GoroutineCount: runtime.NumGoroutine(),
	}
}

// MemoryOptimizer provides memory optimization utilities
type MemoryOptimizer struct {
	monitor *MemoryMonitor
}

// NewMemoryOptimizer creates a new memory optimizer
func NewMemoryOptimizer(monitor *MemoryMonitor) *MemoryOptimizer {
	return &MemoryOptimizer{
		monitor: monitor,
	}
}

// OptimizeForBatch optimizes memory settings for batch processing
func (o *MemoryOptimizer) OptimizeForBatch(batchSize int) {
	// Set more aggressive GC for batch processing
	if batchSize > 50 {
		debug.SetGCPercent(25) // Very aggressive
	} else if batchSize > 20 {
		debug.SetGCPercent(50) // Moderately aggressive
	} else {
		debug.SetGCPercent(75) // Slightly aggressive
	}
	
	// Force initial cleanup
	o.monitor.OptimizeMemory()
}

// OptimizeForQuery optimizes memory settings for query processing
func (o *MemoryOptimizer) OptimizeForQuery() {
	// Set balanced GC for query processing
	debug.SetGCPercent(100) // Default setting
	
	// Clean up if memory pressure is high
	if o.monitor.GetMemoryPressure() > 0.7 {
		o.monitor.OptimizeMemory()
	}
}

// RestoreDefaults restores default memory settings
func (o *MemoryOptimizer) RestoreDefaults() {
	debug.SetGCPercent(100) // Go default
}