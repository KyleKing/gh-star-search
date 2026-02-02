//go:build integration

package summarizer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/KyleKing/gh-star-search/internal/python"
)

var (
	testUVPath     string
	testProjectDir string
)

func TestMain(m *testing.M) {
	uvPath, err := python.FindUV()
	if err != nil {
		fmt.Fprintf(os.Stderr, "uv not installed, skipping integration tests\n")
		os.Exit(0)
	}
	testUVPath = uvPath

	cacheDir, err := os.MkdirTemp("", "summarizer-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(cacheDir)

	projectDir, err := python.EnsureEnvironment(context.Background(), uvPath, cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup Python env: %v\n", err)
		os.Exit(1)
	}
	testProjectDir = projectDir

	warmModel(uvPath, projectDir)
	os.Exit(m.Run())
}

func warmModel(uvPath, projectDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := python.RunScript(ctx, uvPath, projectDir, "summarize.py",
		"--method", "transformers", "--json")
	cmd.Stdin = strings.NewReader(
		"Pre-warm run to download the transformer model and populate the OS page cache. " +
			"This text is long enough to trigger actual model inference rather than a passthrough.")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "model pre-warm failed (tests may be slower): %v\n", err)
	}
}

func newIntegrationSummarizer() *Summarizer {
	s := New(testUVPath, testProjectDir)
	s.timeout = 120 * time.Second
	return s
}

func TestTransformersMethod(t *testing.T) {
	s := newIntegrationSummarizer()

	text := "This is a comprehensive library that provides multiple utilities for developers. " +
		"It includes features for data processing, workflow management, and API integration. " +
		"The library is designed to be flexible and extensible."

	result, err := s.Summarize(context.Background(), text, MethodAuto)
	if err != nil {
		t.Fatalf("Summarize(auto) error = %v", err)
	}

	if result.Summary == "" {
		t.Fatal("empty summary")
	}
	if result.Method != "transformers" {
		t.Errorf("expected method=transformers after pre-warm, got %s", result.Method)
	}

	t.Logf("Summary (%s): %s", result.Method, result.Summary)
}

func TestSummaryQuality(t *testing.T) {
	s := newIntegrationSummarizer()

	cases := []struct {
		name        string
		text        string
		wantMethod  string
		minKeywords []string
	}{
		{
			name: "repo_description",
			text: "React is a JavaScript library for building user interfaces. " +
				"It lets you compose complex UIs from small and isolated pieces of code called components. " +
				"React can be used as a base in the development of single-page, mobile, or server-rendered applications. " +
				"React is only concerned with state management and rendering that state to the DOM.",
			wantMethod:  "transformers",
			minKeywords: []string{"react"},
		},
		{
			name: "cli_tool_readme",
			text: "gh-star-search is a GitHub CLI extension for searching your starred repositories locally. " +
				"It syncs your starred repos into a local DuckDB database with full-text search and vector similarity. " +
				"Features include fuzzy BM25 search across names, descriptions, topics, and README content, " +
				"semantic vector search using sentence-transformer embeddings, " +
				"a related repository discovery engine, and formatted terminal output with multiple display modes.",
			wantMethod:  "transformers",
			minKeywords: []string{"search"},
		},
		{
			name: "technical_paragraph",
			text: "DuckDB is an in-process SQL OLAP database management system. " +
				"It is designed to support analytical query workloads while being efficient on single-machine deployments. " +
				"DuckDB provides native support for full-text search through its FTS extension, " +
				"which implements BM25 scoring for relevance ranking. " +
				"The database can handle both structured and semi-structured data including JSON columns.",
			wantMethod:  "transformers",
			minKeywords: []string{"duckdb"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := s.Summarize(context.Background(), tc.text, MethodAuto)
			if err != nil {
				t.Fatalf("error: %v", err)
			}

			if result.Method != tc.wantMethod {
				t.Errorf("method = %s, want %s", result.Method, tc.wantMethod)
			}

			if len(result.Summary) >= len(tc.text) {
				t.Errorf("summary (%d chars) not shorter than input (%d chars)",
					len(result.Summary), len(tc.text))
			}

			summaryLower := strings.ToLower(result.Summary)
			for _, kw := range tc.minKeywords {
				if !strings.Contains(summaryLower, kw) {
					t.Errorf("summary missing expected keyword %q: %s", kw, result.Summary)
				}
			}

			t.Logf("Input:   %s", tc.text[:80]+"...")
			t.Logf("Summary: %s", result.Summary)
		})
	}
}

func TestSummaryEdgeCases(t *testing.T) {
	s := newIntegrationSummarizer()

	t.Run("long_text", func(t *testing.T) {
		paragraph := "Kubernetes is an open-source container orchestration platform. " +
			"It automates deploying, scaling, and managing containerized applications. "
		text := strings.Repeat(paragraph, 20)

		result, err := s.Summarize(context.Background(), text, MethodAuto)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result.Summary == "" {
			t.Error("empty summary for long text")
		}
		if len(result.Summary) >= len(text) {
			t.Error("summary not shorter than repeated input")
		}

		t.Logf("Input length: %d, Summary length: %d", len(text), len(result.Summary))
	})

	t.Run("code_heavy_text", func(t *testing.T) {
		text := "This library provides a simple API for HTTP requests. " +
			"Usage: import requests; response = requests.get(url); print(response.json()). " +
			"The library handles connection pooling, retries, and timeout configuration. " +
			"It supports all HTTP methods including GET, POST, PUT, DELETE, and PATCH."

		result, err := s.Summarize(context.Background(), text, MethodAuto)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result.Summary == "" {
			t.Error("empty summary for code-heavy text")
		}

		t.Logf("Summary: %s", result.Summary)
	})
}

func BenchmarkHeuristic(b *testing.B) {
	s := newIntegrationSummarizer()
	text := "This is a comprehensive library that provides multiple utilities for developers. " +
		"It includes features for data processing, workflow management, and API integration. " +
		"The library is designed to be flexible and extensible."

	b.ResetTimer()
	for b.Loop() {
		_, err := s.Summarize(context.Background(), text, MethodHeuristic)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransformers(b *testing.B) {
	s := newIntegrationSummarizer()
	text := "This is a comprehensive library that provides multiple utilities for developers. " +
		"It includes features for data processing, workflow management, and API integration. " +
		"The library is designed to be flexible and extensible."

	b.ResetTimer()
	for b.Loop() {
		_, err := s.Summarize(context.Background(), text, MethodTransformers)
		if err != nil {
			b.Fatal(err)
		}
	}
}
