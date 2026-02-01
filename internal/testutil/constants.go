// Package testutil provides common constants and utilities for tests
package testutil

import "time"

const (
	// TestTimeout is the default timeout for test operations
	TestTimeout = 30 * time.Second

	// ShortTestTimeout is a shorter timeout for quick operations
	ShortTestTimeout = 5 * time.Second

	// LongTestTimeout is an extended timeout for long-running operations
	LongTestTimeout = 2 * time.Minute

	// TestBatchSize is the default batch size for test operations
	TestBatchSize = 5

	// TestRepoCount is a common number of test repositories to create
	TestRepoCount = 10

	// TestSmallRepoCount is a small number of test repositories
	TestSmallRepoCount = 3

	// TestLargeRepoCount is a large number of test repositories for performance tests
	TestLargeRepoCount = 100

	// TestStarCount is a typical star count for test repositories
	TestStarCount = 100

	// TestSleepMs is a common sleep duration for rate limiting in tests
	TestSleepMs = 10
)

// Common test strings
const (
	// TestOwner is a default test repository owner
	TestOwner = "testuser"

	// TestRepoName is a default test repository name
	TestRepoName = "testrepo"

	// TestFullName is a default full repository name
	TestFullName = TestOwner + "/" + TestRepoName

	// TestDescription is a default repository description
	TestDescription = "Test repository for unit tests"

	// TestLanguage is a default programming language
	TestLanguage = "Go"
)

// Coverage targets
const (
	// MinOverallCoverage is the minimum acceptable test coverage percentage across all packages
	MinOverallCoverage = 70.0

	// MinCriticalPathCoverage is the minimum coverage for critical data paths (transactions, concurrency)
	MinCriticalPathCoverage = 85.0

	// MinErrorHandlerCoverage is the minimum coverage for error handling code
	MinErrorHandlerCoverage = 60.0
)
