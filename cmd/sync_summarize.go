package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/KyleKing/gh-star-search/internal/config"
	"github.com/KyleKing/gh-star-search/internal/python"
	"github.com/KyleKing/gh-star-search/internal/summarizer"
)

// generateSummaries generates AI summaries for repositories that need them
func (s *SyncService) generateSummaries(ctx context.Context, force bool) error {
	s.logVerbose("\nGenerating repository summaries...")

	// Get repositories that need summary updates
	repos, err := s.storage.GetRepositoriesNeedingSummaryUpdate(ctx, force)
	if err != nil {
		return fmt.Errorf("failed to get repositories needing summary: %w", err)
	}

	if len(repos) == 0 {
		fmt.Println("All repositories have summaries - no updates needed")
		return nil
	}

	fmt.Printf("\nGenerating summaries for %d repositories...\n", len(repos))

	// Prepare Python environment
	uvPath, err := python.FindUV()
	if err != nil {
		return fmt.Errorf("summarization requires uv: %w", err)
	}

	cacheDir := config.ExpandPath(s.config.Cache.Directory)
	projectDir, err := python.EnsureEnvironment(ctx, uvPath, cacheDir)
	if err != nil {
		return fmt.Errorf("failed to prepare Python environment: %w", err)
	}

	// Initialize summarizer
	sum := summarizer.New(uvPath, projectDir)

	// Track statistics
	successful := 0
	failed := 0

	// Process each repository
	for i, repoName := range repos {
		fmt.Printf("  [%d/%d] %s: ", i+1, len(repos), repoName)

		// Get repository details
		repo, err := s.storage.GetRepository(ctx, repoName)
		if err != nil {
			fmt.Printf("Failed to get repository: %v\n", err)
			failed++
			continue
		}

		// Build text to summarize from repository metadata
		text := buildSummaryInput(
			repo.FullName,
			repo.Description,
			repo.Homepage,
			repo.Topics,
			repo.Language,
		)

		// Generate summary
		result, err := sum.Summarize(ctx, text, summarizer.MethodAuto)
		if err != nil {
			fmt.Printf("Failed to generate summary: %v\n", err)
			failed++
			continue
		}

		if result.Error != "" {
			fmt.Printf("Summarization failed: %s\n", result.Error)
			failed++
			continue
		}

		// Store summary
		if err := s.storage.UpdateRepositorySummary(ctx, repoName, result.Summary); err != nil {
			fmt.Printf("Failed to store summary: %v\n", err)
			failed++
			continue
		}

		fmt.Printf("Summary generated (%s method)\n", result.Method)
		s.logVerbose(fmt.Sprintf("    Summary: %s", result.Summary))
		successful++
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("SUMMARIZATION COMPLETE")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total repositories: %d\n", len(repos))
	fmt.Printf("Successfully summarized: %d\n", successful)
	fmt.Printf("Failed: %d\n", failed)

	if failed > 0 {
		fmt.Printf("\n%d repositories failed to summarize\n", failed)
	} else {
		fmt.Println("\nAll repositories summarized successfully!")
	}

	return nil
}

// buildSummaryInput creates text input for summarization from repository metadata
func buildSummaryInput(
	fullName, description, homepage string,
	topics []string,
	language string,
) string {
	var parts []string

	// Add repository name
	parts = append(parts, fmt.Sprintf("Repository: %s", fullName))

	// Add description if available
	if description != "" {
		parts = append(parts, description)
	}

	// Add homepage if available and different from GitHub URL
	if homepage != "" && !strings.Contains(homepage, "github.com") {
		parts = append(parts, fmt.Sprintf("Homepage: %s", homepage))
	}

	// Add topics if available
	if len(topics) > 0 {
		parts = append(parts, fmt.Sprintf("Topics: %s", strings.Join(topics, ", ")))
	}

	// Add language if available
	if language != "" {
		parts = append(parts, fmt.Sprintf("Primary language: %s", language))
	}

	return strings.Join(parts, ". ")
}
