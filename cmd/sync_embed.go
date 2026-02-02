package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/KyleKing/gh-star-search/internal/config"
	"github.com/KyleKing/gh-star-search/internal/embedding"
	"github.com/KyleKing/gh-star-search/internal/python"
)

// generateEmbeddings generates vector embeddings for repositories
func (s *SyncService) generateEmbeddings(ctx context.Context, _ bool) error {
	s.logVerbose("\nGenerating repository embeddings...")

	// Get all repositories (we need to check which ones have embeddings)
	// TODO: Add a method to get repositories needing embeddings
	repos, err := s.storage.ListRepositories(ctx, 1000, 0)
	if err != nil {
		return fmt.Errorf("failed to get repositories: %w", err)
	}

	// Filter repositories that need embeddings
	var needEmbedding []string
	for _, repo := range repos {
		needEmbedding = append(needEmbedding, repo.FullName)
	}

	if len(needEmbedding) == 0 {
		fmt.Println("All repositories have embeddings - no updates needed")
		return nil
	}

	fmt.Printf("\nGenerating embeddings for %d repositories...\n", len(needEmbedding))

	// Prepare Python environment
	uvPath, err := python.FindUV()
	if err != nil {
		return fmt.Errorf("embedding generation requires uv: %w", err)
	}

	cacheDir := config.ExpandPath(s.config.Cache.Directory)
	projectDir, err := python.EnsureEnvironment(ctx, uvPath, cacheDir)
	if err != nil {
		return fmt.Errorf("failed to prepare Python environment: %w", err)
	}

	// Initialize embedding provider
	embConfig := embedding.Config{
		Provider:   "local",
		Model:      "intfloat/e5-small-v2",
		Dimensions: 384,
		Enabled:    true,
		Options:    make(map[string]string),
	}

	embProvider, err := embedding.NewProvider(embConfig, uvPath, projectDir)
	if err != nil {
		return fmt.Errorf("failed to initialize embedding provider: %w", err)
	}

	if !embProvider.IsEnabled() {
		return fmt.Errorf("embedding provider is not enabled")
	}

	// Track statistics
	successful := 0
	failed := 0

	// Process each repository
	for i, repoName := range needEmbedding {
		fmt.Printf("  [%d/%d] %s: ", i+1, len(needEmbedding), repoName)

		// Get repository details
		repo, err := s.storage.GetRepository(ctx, repoName)
		if err != nil {
			fmt.Printf("Failed to get repository: %v\n", err)
			failed++
			continue
		}

		// Build text to embed from repository metadata
		text := buildEmbeddingInput(repo.FullName, repo.Description, repo.Purpose, repo.Topics)

		// Generate embedding
		embVec, err := embProvider.GenerateEmbedding(ctx, text)
		if err != nil {
			fmt.Printf("Failed to generate embedding: %v\n", err)
			failed++
			continue
		}

		// Store embedding
		if err := s.storage.UpdateRepositoryEmbedding(ctx, repoName, embVec); err != nil {
			fmt.Printf("Failed to store embedding: %v\n", err)
			failed++
			continue
		}

		fmt.Printf("Embedding generated (%d dimensions)\n", len(embVec))
		successful++
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("EMBEDDING GENERATION COMPLETE")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total repositories: %d\n", len(needEmbedding))
	fmt.Printf("Successfully embedded: %d\n", successful)
	fmt.Printf("Failed: %d\n", failed)

	if failed > 0 {
		fmt.Printf("\n%d repositories failed to embed\n", failed)
	} else {
		fmt.Println("\nAll repositories embedded successfully!")
	}

	return nil
}

// buildEmbeddingInput creates text input for embedding from repository metadata
func buildEmbeddingInput(fullName, description, purpose string, topics []string) string {
	var parts []string

	// Add repository name (helps with similarity)
	parts = append(parts, fullName)

	// Add purpose/summary if available (high quality signal)
	if purpose != "" {
		parts = append(parts, purpose)
	}

	// Add description
	if description != "" {
		parts = append(parts, description)
	}

	// Add topics
	if len(topics) > 0 {
		parts = append(parts, strings.Join(topics, " "))
	}

	return strings.Join(parts, ". ")
}
