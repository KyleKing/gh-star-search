// +build integration

package github

import (
	"context"
	"testing"
	"time"
)

// TestRealGitHubClient tests the client with real GitHub API
// Run with: go test -tags=integration ./internal/github
// Requires: gh auth login to be completed
func TestRealGitHubClient(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test fetching starred repositories (limit to first few)
	repos, err := client.GetStarredRepos(ctx, "")
	if err != nil {
		t.Fatalf("Failed to fetch starred repositories: %v", err)
	}

	t.Logf("Found %d starred repositories", len(repos))

	if len(repos) > 0 {
		repo := repos[0]
		t.Logf("First repository: %s", repo.FullName)
		t.Logf("Description: %s", repo.Description)
		t.Logf("Language: %s", repo.Language)
		t.Logf("Stars: %d", repo.StargazersCount)

		// Test fetching repository content
		content, err := client.GetRepositoryContent(ctx, repo, []string{"README.md"})
		if err != nil {
			t.Logf("Failed to fetch README for %s: %v", repo.FullName, err)
		} else if len(content) > 0 {
			t.Logf("README found for %s, size: %d bytes", repo.FullName, content[0].Size)
		}

		// Test fetching repository metadata
		metadata, err := client.GetRepositoryMetadata(ctx, repo)
		if err != nil {
			t.Logf("Failed to fetch metadata for %s: %v", repo.FullName, err)
		} else {
			t.Logf("Metadata for %s:", repo.FullName)
			t.Logf("  Commit count: %d", metadata.CommitCount)
			t.Logf("  Contributors: %v", metadata.Contributors)
			if metadata.LatestRelease != nil {
				t.Logf("  Latest release: %s", metadata.LatestRelease.TagName)
			}
		}
	}
}