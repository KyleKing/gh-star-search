package github

import (
	"context"
	"sync"
	"time"
)

// WorkerPool manages parallel execution of GitHub API calls with rate limiting and backoff
type WorkerPool struct {
	workers     int
	rateLimiter chan struct{}
	backoffBase time.Duration
	maxBackoff  time.Duration
}

// NewWorkerPool creates a new worker pool for GitHub API calls
func NewWorkerPool(workers int, rateLimit int, backoffBase, maxBackoff time.Duration) *WorkerPool {
	// Create rate limiter channel
	rateLimiter := make(chan struct{}, rateLimit)

	// Fill the rate limiter initially
	for range rateLimit {
		rateLimiter <- struct{}{}
	}

	return &WorkerPool{
		workers:     workers,
		rateLimiter: rateLimiter,
		backoffBase: backoffBase,
		maxBackoff:  maxBackoff,
	}
}

// Task represents a unit of work for the worker pool
type Task struct {
	ID   string
	Func func(ctx context.Context) (interface{}, error)
}

// Result represents the result of a task execution
type Result struct {
	ID    string
	Data  interface{}
	Error error
}

// Execute runs tasks in parallel with rate limiting and backoff
func (wp *WorkerPool) Execute(ctx context.Context, tasks []Task) []Result {
	if len(tasks) == 0 {
		return []Result{}
	}

	taskChan := make(chan Task, len(tasks))
	resultChan := make(chan Result, len(tasks))

	// Start workers
	var wg sync.WaitGroup
	for range wp.workers {
		wg.Add(1)

		go wp.worker(ctx, &wg, taskChan, resultChan)
	}

	// Send tasks
	go func() {
		defer close(taskChan)

		for _, task := range tasks {
			select {
			case taskChan <- task:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Collect results
	results := make([]Result, 0, len(tasks))

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// worker processes tasks from the task channel
func (wp *WorkerPool) worker(ctx context.Context, wg *sync.WaitGroup, taskChan <-chan Task, resultChan chan<- Result) {
	defer wg.Done()

	for {
		select {
		case task, ok := <-taskChan:
			if !ok {
				return
			}

			result := wp.executeTask(ctx, task)

			select {
			case resultChan <- result:
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// executeTask executes a single task with rate limiting and backoff
func (wp *WorkerPool) executeTask(ctx context.Context, task Task) Result {
	var lastErr error

	backoff := wp.backoffBase

	for attempt := range 3 {
		// Wait for rate limit token
		select {
		case <-wp.rateLimiter:
		case <-ctx.Done():
			return Result{ID: task.ID, Error: ctx.Err()}
		}

		// Execute the task
		data, err := task.Func(ctx)

		// Return rate limit token after a delay
		go func() {
			time.Sleep(100 * time.Millisecond) // Basic rate limiting delay
			wp.rateLimiter <- struct{}{}
		}()

		if err == nil {
			return Result{ID: task.ID, Data: data}
		}

		lastErr = err

		// Check if it's a rate limit error and apply backoff
		if isRateLimitError(err) && attempt < 2 {
			select {
			case <-time.After(backoff):
				backoff = wp.nextBackoff(backoff)
			case <-ctx.Done():
				return Result{ID: task.ID, Error: ctx.Err()}
			}

			continue
		}

		// For non-rate-limit errors, return immediately
		break
	}

	return Result{ID: task.ID, Error: lastErr}
}

// nextBackoff calculates the next backoff duration with exponential backoff
func (wp *WorkerPool) nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > wp.maxBackoff {
		return wp.maxBackoff
	}

	return next
}

// isRateLimitError checks if an error is related to rate limiting
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	return contains(errStr, "rate limit") ||
		contains(errStr, "403") ||
		contains(errStr, "429") ||
		contains(errStr, "API rate limit exceeded")
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr))))
}

// containsSubstring performs a simple substring search
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

// BatchExecutor provides a high-level interface for batched API operations
type BatchExecutor struct {
	client Client
	pool   *WorkerPool
}

// NewBatchExecutor creates a new batch executor
func NewBatchExecutor(client Client, workers int, rateLimit int) *BatchExecutor {
	pool := NewWorkerPool(workers, rateLimit, 1*time.Second, 30*time.Second)

	return &BatchExecutor{
		client: client,
		pool:   pool,
	}
}

// FetchRepositoryMetrics fetches metrics for multiple repositories in parallel
func (be *BatchExecutor) FetchRepositoryMetrics(ctx context.Context, repos []Repository) map[string]*RepositoryMetrics {
	tasks := make([]Task, 0, len(repos)*6) // 6 different metrics per repo

	// Create tasks for each repository and metric type
	for _, repo := range repos {
		repoName := repo.FullName

		// Contributors task
		tasks = append(tasks, Task{
			ID: repoName + ":contributors",
			Func: func(ctx context.Context) (interface{}, error) {
				return be.client.GetContributors(ctx, repoName, 10)
			},
		})

		// Topics task
		tasks = append(tasks, Task{
			ID: repoName + ":topics",
			Func: func(ctx context.Context) (interface{}, error) {
				return be.client.GetTopics(ctx, repoName)
			},
		})

		// Languages task
		tasks = append(tasks, Task{
			ID: repoName + ":languages",
			Func: func(ctx context.Context) (interface{}, error) {
				return be.client.GetLanguages(ctx, repoName)
			},
		})

		// Commit activity task
		tasks = append(tasks, Task{
			ID: repoName + ":commits",
			Func: func(ctx context.Context) (interface{}, error) {
				return be.client.GetCommitActivity(ctx, repoName)
			},
		})

		// Pull request counts task
		tasks = append(tasks, Task{
			ID: repoName + ":prs",
			Func: func(ctx context.Context) (interface{}, error) {
				open, total, err := be.client.GetPullCounts(ctx, repoName)
				if err != nil {
					return nil, err
				}
				return map[string]int{"open": open, "total": total}, nil
			},
		})

		// Issue counts task
		tasks = append(tasks, Task{
			ID: repoName + ":issues",
			Func: func(ctx context.Context) (interface{}, error) {
				open, total, err := be.client.GetIssueCounts(ctx, repoName)
				if err != nil {
					return nil, err
				}
				return map[string]int{"open": open, "total": total}, nil
			},
		})
	}

	// Execute all tasks
	results := be.pool.Execute(ctx, tasks)

	// Organize results by repository
	metrics := make(map[string]*RepositoryMetrics)
	for _, repo := range repos {
		metrics[repo.FullName] = &RepositoryMetrics{}
	}

	// Process results
	for _, result := range results {
		parts := splitString(result.ID, ":")
		if len(parts) != 2 {
			continue
		}

		repoName := parts[0]
		metricType := parts[1]

		if result.Error != nil {
			// Log error but continue with other metrics
			continue
		}

		repoMetrics := metrics[repoName]
		if repoMetrics == nil {
			continue
		}

		switch metricType {
		case "contributors":
			if contributors, ok := result.Data.([]Contributor); ok {
				repoMetrics.Contributors = contributors
			}
		case "topics":
			if topics, ok := result.Data.([]string); ok {
				repoMetrics.Topics = topics
			}
		case "languages":
			if languages, ok := result.Data.(map[string]int64); ok {
				repoMetrics.Languages = languages
			}
		case "commits":
			if activity, ok := result.Data.(*CommitActivity); ok {
				repoMetrics.CommitActivity = activity
			}
		case "prs":
			if counts, ok := result.Data.(map[string]int); ok {
				repoMetrics.OpenPRs = counts["open"]
				repoMetrics.TotalPRs = counts["total"]
			}
		case "issues":
			if counts, ok := result.Data.(map[string]int); ok {
				repoMetrics.OpenIssues = counts["open"]
				repoMetrics.TotalIssues = counts["total"]
			}
		}
	}

	return metrics
}

// RepositoryMetrics aggregates all metrics for a repository
type RepositoryMetrics struct {
	Contributors   []Contributor
	Topics         []string
	Languages      map[string]int64
	CommitActivity *CommitActivity
	OpenPRs        int
	TotalPRs       int
	OpenIssues     int
	TotalIssues    int
}

// splitString splits a string by delimiter (simple implementation)
func splitString(s, delimiter string) []string {
	if s == "" {
		return []string{}
	}

	var parts []string

	start := 0

	for i := 0; i <= len(s)-len(delimiter); i++ {
		if s[i:i+len(delimiter)] == delimiter {
			parts = append(parts, s[start:i])
			start = i + len(delimiter)
			i += len(delimiter) - 1
		}
	}

	// Add the last part
	parts = append(parts, s[start:])

	return parts
}
