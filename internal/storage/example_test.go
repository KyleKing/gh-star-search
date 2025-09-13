package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/kyleking/gh-star-search/internal/github"
	"github.com/kyleking/gh-star-search/internal/processor"
)

// ExampleDuckDBRepository demonstrates basic usage of the DuckDB storage layer
func ExampleDuckDBRepository() {
	// Create a temporary database
	tempDir, _ := os.MkdirTemp("", "example_test")
	defer os.RemoveAll(tempDir)
	dbPath := filepath.Join(tempDir, "example.db")

	// Create repository instance
	repo, err := NewDuckDBRepository(dbPath)
	if err != nil {
		log.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Initialize the database schema
	err = repo.Initialize(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create sample repository data
	sampleRepo := processor.ProcessedRepo{
		Repository: github.Repository{
			FullName:        "example/awesome-project",
			Description:     "An awesome example project",
			Language:        "Go",
			StargazersCount: 1500,
			ForksCount:      200,
			Size:            2048,
			CreatedAt:       time.Now().Add(-365 * 24 * time.Hour),
			UpdatedAt:       time.Now().Add(-24 * time.Hour),
			Topics:          []string{"golang", "cli", "awesome"},
			License: &github.License{
				Name:   "MIT License",
				SPDXID: "MIT",
			},
		},
		Summary: processor.Summary{
			Purpose:      "A command-line tool for managing awesome projects",
			Technologies: []string{"Go", "Cobra", "DuckDB"},
			UseCases:     []string{"Project management", "CLI automation"},
			Features:     []string{"Fast search", "Local storage", "Cross-platform"},
			Installation: "go install github.com/example/awesome-project@latest",
			Usage:        "awesome-project [command] [flags]",
		},
		Chunks: []processor.ContentChunk{
			{
				Source:   "README.md",
				Type:     processor.ContentTypeReadme,
				Content:  "# Awesome Project\n\nThis is an awesome project for managing things.",
				Tokens:   20,
				Priority: processor.PriorityHigh,
			},
			{
				Source:   "main.go",
				Type:     processor.ContentTypeCode,
				Content:  "package main\n\nfunc main() {\n\t// Awesome code here\n}",
				Tokens:   15,
				Priority: processor.PriorityMedium,
			},
		},
		ProcessedAt: time.Now(),
		ContentHash: "example-hash-123",
	}

	// Store the repository
	err = repo.StoreRepository(ctx, sampleRepo)
	if err != nil {
		log.Fatalf("Failed to store repository: %v", err)
	}

	// Search for repositories
	results, err := repo.SearchRepositories(ctx, "awesome")
	if err != nil {
		log.Fatalf("Failed to search repositories: %v", err)
	}

	fmt.Printf("Found %d repositories matching 'awesome'\n", len(results))

	for _, result := range results {
		fmt.Printf("- %s: %s\n", result.Repository.FullName, result.Repository.Description)
	}

	// Get repository statistics
	stats, err := repo.GetStats(ctx)
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}

	fmt.Printf("Database contains %d repositories and %d content chunks\n",
		stats.TotalRepositories, stats.TotalContentChunks)

	// Output:
	// Applying migration 1: Initial schema creation
	// Found 1 repositories matching 'awesome'
	// - example/awesome-project: An awesome example project
	// Database contains 1 repositories and 2 content chunks
}
