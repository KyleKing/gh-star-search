package github_test

import (
	"context"
	"fmt"
	"log"

	"github.com/KyleKing/gh-star-search/internal/github"
)

func ExampleClient_GetStarredRepos() {
	// Create a new GitHub client using existing GitHub CLI authentication
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	ctx := context.Background()

	// Fetch all starred repositories for the authenticated user
	repos, err := client.GetStarredRepos(ctx, "")
	if err != nil {
		log.Fatalf("Failed to fetch starred repositories: %v", err)
	}

	fmt.Printf("Found %d starred repositories\n", len(repos))

	// Print details of the first few repositories
	for i, repo := range repos {
		if i >= 3 { // Limit output for example
			break
		}

		fmt.Printf("Repository: %s\n", repo.FullName)
		fmt.Printf("  Description: %s\n", repo.Description)
		fmt.Printf("  Language: %s\n", repo.Language)
		fmt.Printf("  Stars: %d\n", repo.StargazersCount)
		fmt.Printf("  Updated: %s\n", repo.UpdatedAt.Format("2006-01-02"))
		fmt.Println()
	}
}

func ExampleClient_GetRepositoryContent() {
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Example repository
	repo := github.Repository{
		FullName:      "owner/repository",
		DefaultBranch: "main",
	}

	ctx := context.Background()

	// Fetch specific files from the repository
	paths := []string{"README.md", "LICENSE", "package.json"}

	contents, err := client.GetRepositoryContent(ctx, repo, paths)
	if err != nil {
		log.Fatalf("Failed to fetch repository content: %v", err)
	}

	fmt.Printf("Found %d files\n", len(contents))

	for _, content := range contents {
		fmt.Printf("File: %s\n", content.Path)
		fmt.Printf("  Type: %s\n", content.Type)
		fmt.Printf("  Size: %d bytes\n", content.Size)
		fmt.Printf("  Encoding: %s\n", content.Encoding)
		fmt.Println()
	}
}

func ExampleClient_GetRepositoryMetadata() {
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Example repository
	repo := github.Repository{
		FullName:      "owner/repository",
		DefaultBranch: "main",
	}

	ctx := context.Background()

	// Fetch additional metadata
	metadata, err := client.GetRepositoryMetadata(ctx, repo)
	if err != nil {
		log.Fatalf("Failed to fetch repository metadata: %v", err)
	}

	fmt.Printf("Repository Metadata:\n")
	fmt.Printf("  Commit count: %d\n", metadata.CommitCount)
	fmt.Printf("  Contributors: %v\n", metadata.Contributors)
	fmt.Printf("  Last commit: %s\n", metadata.LastCommitDate.Format("2006-01-02"))

	if metadata.LatestRelease != nil {
		fmt.Printf("  Latest release: %s (%s)\n",
			metadata.LatestRelease.TagName,
			metadata.LatestRelease.PublishedAt.Format("2006-01-02"))
	}

	fmt.Printf("  Total releases: %d\n", metadata.ReleaseCount)
}
