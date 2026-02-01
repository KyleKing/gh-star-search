package testutil

import (
	"sync"
	"testing"
)

// RunConcurrent executes the given function concurrently n times.
// Waits for all goroutines to complete before returning.
// Any panics are captured and reported as test failures.
func RunConcurrent(t *testing.T, n int, fn func(workerID int)) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(n)

	for i := range n {
		go func(workerID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("worker %d panicked: %v", workerID, r)
				}
			}()
			fn(workerID)
		}(i)
	}

	wg.Wait()
}

// AssertNoRaces runs the given function multiple times concurrently
// to help detect potential race conditions. This should be used in conjunction
// with `go test -race` for full race detection.
func AssertNoRaces(t *testing.T, fn func(), iterations int) {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping race detection test in short mode")
	}

	RunConcurrent(t, iterations, func(_ int) {
		fn()
	})
}
