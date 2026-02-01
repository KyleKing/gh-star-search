package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/KyleKing/gh-star-search/internal/related"
	"github.com/KyleKing/gh-star-search/internal/storage"
)

func TestValidateRepositoryName(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		wantErr  bool
	}{
		{
			name:     "valid repository name",
			repoName: "facebook/react",
			wantErr:  false,
		},
		{
			name:     "valid repository with numbers",
			repoName: "user123/repo456",
			wantErr:  false,
		},
		{
			name:     "valid repository with hyphens",
			repoName: "my-org/my-repo",
			wantErr:  false,
		},
		{
			name:     "empty repository name",
			repoName: "",
			wantErr:  true,
		},
		{
			name:     "missing owner",
			repoName: "/repo",
			wantErr:  true,
		},
		{
			name:     "missing repo name",
			repoName: "owner/",
			wantErr:  true,
		},
		{
			name:     "no slash separator",
			repoName: "owner-repo",
			wantErr:  true,
		},
		{
			name:     "multiple slashes",
			repoName: "owner/repo/extra",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepositoryName(tt.repoName)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"validateRepositoryName(%q) error = %v, wantErr %v",
					tt.repoName,
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestDisplayRelatedRepository(t *testing.T) {
	targetRepo := &storage.StoredRepo{
		FullName: "facebook/react",
		Language: "JavaScript",
		Topics:   []string{"react", "javascript", "frontend"},
	}

	tests := []struct {
		name     string
		rank     int
		rel      related.Repository
		contains []string
	}{
		{
			name: "repository with description",
			rank: 1,
			rel: related.Repository{
				Repository: storage.StoredRepo{
					FullName:        "vuejs/vue",
					Description:     "Vue.js - The Progressive JavaScript Framework",
					Language:        "JavaScript",
					StargazersCount: 195000,
				},
				Score:       0.85,
				Explanation: "Shared topics: javascript, frontend",
			},
			contains: []string{
				"1. vuejs/vue",
				"⭐ 195000",
				"JavaScript",
				"Score: 0.85",
				"Vue.js - The Progressive JavaScript Framework",
				"Related: Shared topics: javascript, frontend",
			},
		},
		{
			name: "repository with long description",
			rank: 2,
			rel: related.Repository{
				Repository: storage.StoredRepo{
					FullName:        "test/long-desc",
					Description:     strings.Repeat("A", 100), // Long description
					Language:        "Go",
					StargazersCount: 1000,
				},
				Score:       0.50,
				Explanation: "Same organization",
			},
			contains: []string{
				"2. test/long-desc",
				"⭐ 1000",
				"Go",
				"Score: 0.50",
				"...", // Truncation indicator
				"Related: Same organization",
			},
		},
		{
			name: "repository without language",
			rank: 3,
			rel: related.Repository{
				Repository: storage.StoredRepo{
					FullName:        "test/no-lang",
					Description:     "Test repository",
					Language:        "",
					StargazersCount: 500,
				},
				Score:       0.30,
				Explanation: "Shared contributors",
			},
			contains: []string{
				"3. test/no-lang",
				"Unknown",
				"Score: 0.30",
			},
		},
		{
			name: "repository without description",
			rank: 4,
			rel: related.Repository{
				Repository: storage.StoredRepo{
					FullName:        "test/no-desc",
					Description:     "",
					Language:        "Python",
					StargazersCount: 250,
				},
				Score:       0.20,
				Explanation: "Vector similarity",
			},
			contains: []string{
				"4. test/no-desc",
				"   -",
				"Python",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Display the repository
			displayRelatedRepository(tt.rank, tt.rel, targetRepo)

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			// Check output contains expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf(
						"displayRelatedRepository() output does not contain %q\nOutput: %s",
						expected,
						output,
					)
				}
			}
		})
	}
}
