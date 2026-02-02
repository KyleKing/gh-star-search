//go:build integration

package embedding

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/KyleKing/gh-star-search/internal/python"
	"github.com/KyleKing/gh-star-search/internal/related"
)

var (
	testProvider *LocalProvider
)

func TestMain(m *testing.M) {
	uvPath, err := python.FindUV()
	if err != nil {
		fmt.Fprintf(os.Stderr, "uv not installed, skipping integration tests\n")
		os.Exit(0)
	}

	cacheDir, err := os.MkdirTemp("", "embedding-integration-*")
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

	cfg := DefaultConfig()
	cfg.Enabled = true
	provider, err := NewLocalProvider(cfg, uvPath, projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create provider: %v\n", err)
		os.Exit(1)
	}
	testProvider = provider

	warmEmbeddings(provider)
	os.Exit(m.Run())
}

func warmEmbeddings(p *LocalProvider) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, err := p.GenerateEmbedding(ctx, "warm up the model cache")
	if err != nil {
		fmt.Fprintf(os.Stderr, "embedding pre-warm failed (tests may be slower): %v\n", err)
	}
}

func TestEmbeddingDeterminism(t *testing.T) {
	ctx := context.Background()
	text := "React is a JavaScript library for building user interfaces"

	emb1, err := testProvider.GenerateEmbedding(ctx, text)
	if err != nil {
		t.Fatalf("first embedding: %v", err)
	}

	emb2, err := testProvider.GenerateEmbedding(ctx, text)
	if err != nil {
		t.Fatalf("second embedding: %v", err)
	}

	if len(emb1) != len(emb2) {
		t.Fatalf("dimension mismatch: %d vs %d", len(emb1), len(emb2))
	}

	for i := range emb1 {
		if emb1[i] != emb2[i] {
			t.Fatalf("embeddings differ at index %d: %f vs %f", i, emb1[i], emb2[i])
		}
	}
}

func TestEmbeddingDimensionality(t *testing.T) {
	ctx := context.Background()

	texts := []string{
		"short",
		"A medium length sentence about programming languages and frameworks",
		"DuckDB is an in-process SQL OLAP database management system designed to support " +
			"analytical query workloads while being efficient on single-machine deployments. " +
			"It provides native support for full-text search through its FTS extension.",
	}

	for _, text := range texts {
		emb, err := testProvider.GenerateEmbedding(ctx, text)
		if err != nil {
			t.Fatalf("embedding for %q: %v", text[:20], err)
		}

		if len(emb) != defaultEmbeddingDimensions {
			t.Errorf("expected %d dimensions, got %d for text %q",
				defaultEmbeddingDimensions, len(emb), text[:20])
		}
	}
}

func TestEmbeddingSemanticSimilarity(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name      string
		anchor    string
		similar   string
		unrelated string
	}{
		{
			name:      "web frameworks",
			anchor:    "React is a JavaScript library for building user interfaces",
			similar:   "Vue.js is a progressive framework for building UIs",
			unrelated: "PostgreSQL is a powerful relational database system",
		},
		{
			name:      "databases",
			anchor:    "DuckDB is an in-process SQL OLAP database",
			similar:   "SQLite is an embedded SQL database engine",
			unrelated: "Kubernetes automates container orchestration and deployment",
		},
		{
			name:      "cli tools",
			anchor:    "gh-star-search searches your GitHub starred repositories locally",
			similar:   "gh is a command line tool for GitHub that brings pull requests and issues to your terminal",
			unrelated: "TensorFlow is an open source machine learning framework",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			anchorEmb, err := testProvider.GenerateEmbedding(ctx, tc.anchor)
			if err != nil {
				t.Fatalf("anchor embedding: %v", err)
			}

			similarEmb, err := testProvider.GenerateEmbedding(ctx, tc.similar)
			if err != nil {
				t.Fatalf("similar embedding: %v", err)
			}

			unrelatedEmb, err := testProvider.GenerateEmbedding(ctx, tc.unrelated)
			if err != nil {
				t.Fatalf("unrelated embedding: %v", err)
			}

			simScore := related.CosineSimilarity(anchorEmb, similarEmb)
			unrelScore := related.CosineSimilarity(anchorEmb, unrelatedEmb)

			if simScore <= unrelScore {
				t.Errorf("similar text should score higher than unrelated: similar=%.4f unrelated=%.4f",
					simScore, unrelScore)
			}

			t.Logf("similar=%.4f unrelated=%.4f delta=%.4f", simScore, unrelScore, simScore-unrelScore)
		})
	}
}

func TestEmbeddingBatchConsistency(t *testing.T) {
	ctx := context.Background()

	texts := []string{
		"React is a JavaScript library for building user interfaces",
		"DuckDB is an in-process SQL OLAP database",
		"Kubernetes automates container orchestration",
	}

	batchEmbs, err := testProvider.GenerateEmbeddings(ctx, texts)
	if err != nil {
		t.Fatalf("batch embedding: %v", err)
	}

	if len(batchEmbs) != len(texts) {
		t.Fatalf("expected %d embeddings, got %d", len(texts), len(batchEmbs))
	}

	for i, text := range texts {
		singleEmb, err := testProvider.GenerateEmbedding(ctx, text)
		if err != nil {
			t.Fatalf("single embedding %d: %v", i, err)
		}

		sim := related.CosineSimilarity(batchEmbs[i], singleEmb)
		if sim < 0.999 {
			t.Errorf("batch[%d] vs single similarity=%.6f (expected >=0.999)", i, sim)
		}
	}
}

func TestEmbeddingEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("empty_text", func(t *testing.T) {
		emb, err := testProvider.GenerateEmbedding(ctx, "")
		if err != nil {
			t.Fatalf("empty text: %v", err)
		}
		if len(emb) != defaultEmbeddingDimensions {
			t.Errorf("expected %d dimensions for empty text, got %d",
				defaultEmbeddingDimensions, len(emb))
		}
	})

	t.Run("long_text", func(t *testing.T) {
		long := ""
		for range 500 {
			long += "This is a sentence about software development. "
		}

		emb, err := testProvider.GenerateEmbedding(ctx, long)
		if err != nil {
			t.Fatalf("long text: %v", err)
		}
		if len(emb) != defaultEmbeddingDimensions {
			t.Errorf("expected %d dimensions, got %d", defaultEmbeddingDimensions, len(emb))
		}
	})

	t.Run("special_characters", func(t *testing.T) {
		text := `func main() { fmt.Println("hello, 世界") } // @#$%^&*()`
		emb, err := testProvider.GenerateEmbedding(ctx, text)
		if err != nil {
			t.Fatalf("special chars: %v", err)
		}
		if len(emb) != defaultEmbeddingDimensions {
			t.Errorf("expected %d dimensions, got %d", defaultEmbeddingDimensions, len(emb))
		}
	})

	t.Run("unit_norm", func(t *testing.T) {
		emb, err := testProvider.GenerateEmbedding(ctx, "sentence-transformers produce unit-norm vectors")
		if err != nil {
			t.Fatalf("unit norm text: %v", err)
		}

		var norm float64
		for _, v := range emb {
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)

		if math.Abs(norm-1.0) > 0.01 {
			t.Errorf("expected unit norm (~1.0), got %.6f", norm)
		}
	})
}

func TestEmbeddingModelDrift(t *testing.T) {
	ctx := context.Background()

	knownText := "GitHub is a developer platform for building and scaling software"
	emb, err := testProvider.GenerateEmbedding(ctx, knownText)
	if err != nil {
		t.Fatalf("embedding: %v", err)
	}

	if len(emb) != defaultEmbeddingDimensions {
		t.Fatalf("expected %d dimensions, got %d", defaultEmbeddingDimensions, len(emb))
	}

	var norm float64
	var nonZero int
	for _, v := range emb {
		norm += float64(v) * float64(v)
		if v != 0 {
			nonZero++
		}
	}
	norm = math.Sqrt(norm)

	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("norm drift: expected ~1.0, got %.6f", norm)
	}

	if nonZero < defaultEmbeddingDimensions/2 {
		t.Errorf("too many zero components: %d of %d are non-zero", nonZero, defaultEmbeddingDimensions)
	}

	selfSim := related.CosineSimilarity(emb, emb)
	if math.Abs(selfSim-1.0) > 0.001 {
		t.Errorf("self-similarity should be ~1.0, got %.6f", selfSim)
	}

	t.Logf("norm=%.6f nonZero=%d/%d selfSim=%.6f", norm, nonZero, defaultEmbeddingDimensions, selfSim)
}

func BenchmarkEmbeddingSingle(b *testing.B) {
	ctx := context.Background()
	text := "React is a JavaScript library for building user interfaces"

	b.ResetTimer()
	for b.Loop() {
		_, err := testProvider.GenerateEmbedding(ctx, text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEmbeddingBatch(b *testing.B) {
	ctx := context.Background()
	texts := []string{
		"React is a JavaScript library for building user interfaces",
		"DuckDB is an in-process SQL OLAP database",
		"Kubernetes automates container orchestration",
		"Go is a statically typed compiled programming language",
		"Redis is an in-memory data structure store",
	}

	b.ResetTimer()
	for b.Loop() {
		_, err := testProvider.GenerateEmbeddings(ctx, texts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

