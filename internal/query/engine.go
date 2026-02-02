package query

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/KyleKing/gh-star-search/internal/embedding"
	"github.com/KyleKing/gh-star-search/internal/storage"
)

// Mode represents the search mode
type Mode string

const (
	ModeFuzzy  Mode = "fuzzy"
	ModeVector Mode = "vector"
)

// Query represents a search query
type Query struct {
	Raw  string
	Mode Mode
}

// SearchOptions represents search configuration options
type SearchOptions struct {
	Limit    int
	MinScore float64
}

// Result represents a search result with enhanced scoring
type Result struct {
	RepoID      string
	Score       float64
	Rank        int
	MatchFields []string // Fields that matched the query
	Repository  storage.StoredRepo
}

// Engine defines the search engine interface
type Engine interface {
	Search(ctx context.Context, q Query, opts SearchOptions) ([]Result, error)
}

// SearchEngine implements the Engine interface
type SearchEngine struct {
	repo       storage.Repository
	embManager *embedding.Manager
}

// NewSearchEngine creates a new search engine instance
func NewSearchEngine(repo storage.Repository, embManager *embedding.Manager) *SearchEngine {
	return &SearchEngine{
		repo:       repo,
		embManager: embManager,
	}
}

// Search executes a search query with the specified mode and options
func (e *SearchEngine) Search(ctx context.Context, q Query, opts SearchOptions) ([]Result, error) {
	switch q.Mode {
	case ModeFuzzy:
		return e.searchFuzzy(ctx, q.Raw, opts)
	case ModeVector:
		return e.searchVector(ctx, q.Raw, opts)
	default:
		return e.searchFuzzy(ctx, q.Raw, opts)
	}
}

// searchFuzzy performs FTS search with BM25 scoring from DuckDB
func (e *SearchEngine) searchFuzzy(
	ctx context.Context,
	query string,
	opts SearchOptions,
) ([]Result, error) {
	storageResults, err := e.repo.SearchRepositories(ctx, query)
	if err != nil {
		return nil, err
	}

	var results []Result
	queryTerms := tokenizeQuery(query)

	for _, sr := range storageResults {
		score := e.applyRankingBoosts(sr.Repository, sr.Score)

		if score < opts.MinScore {
			continue
		}

		matchFields := e.identifyMatchedFields(sr.Repository, queryTerms)

		results = append(results, Result{
			RepoID:      sr.Repository.ID,
			Score:       score,
			Repository:  sr.Repository,
			MatchFields: matchFields,
		})
	}

	normalizeScores(results)
	results = sortAndRankResults(results)

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// searchVector performs semantic search using pre-computed embeddings
func (e *SearchEngine) searchVector(
	ctx context.Context,
	query string,
	opts SearchOptions,
) ([]Result, error) {
	if e.embManager == nil || !e.embManager.IsEnabled() {
		return nil, fmt.Errorf("vector search requires embeddings to be enabled; run 'sync --embed' first")
	}

	queryEmbedding, err := e.embManager.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	storageResults, err := e.repo.SearchByEmbedding(ctx, queryEmbedding, limit, opts.MinScore)
	if err != nil {
		return nil, fmt.Errorf("embedding search failed: %w", err)
	}

	var results []Result
	for _, sr := range storageResults {
		score := e.applyRankingBoosts(sr.Repository, sr.Score)

		results = append(results, Result{
			RepoID:      sr.Repository.ID,
			Score:       score,
			Repository:  sr.Repository,
			MatchFields: []string{"embedding"},
		})
	}

	normalizeScores(results)
	results = sortAndRankResults(results)

	return results, nil
}

// applyRankingBoosts applies logarithmic star boost and recency decay
func (e *SearchEngine) applyRankingBoosts(repo storage.StoredRepo, baseScore float64) float64 {
	if baseScore <= 0 {
		return baseScore
	}

	starBoost := 1.0
	if repo.StargazersCount > 0 {
		starBoost = 1.0 + (0.1 * math.Log10(float64(repo.StargazersCount+1)) / 6.0)
	}

	recencyFactor := 1.0
	if !repo.UpdatedAt.IsZero() {
		daysSinceUpdate := time.Since(repo.UpdatedAt).Hours() / 24
		recencyFactor = 1.0 - 0.2*math.Min(1.0, daysSinceUpdate/365.0)
	}

	return baseScore * starBoost * recencyFactor
}

// identifyMatchedFields identifies which logical fields matched the query
func (e *SearchEngine) identifyMatchedFields(
	repo storage.StoredRepo,
	queryTerms []string,
) []string {
	var matchedFields []string

	fieldMap := map[string]string{
		"name":        repo.FullName,
		"description": repo.Description,
		"purpose":     repo.Purpose,
		"topics":      strings.Join(repo.Topics, " "),
	}

	for field, content := range fieldMap {
		if content == "" {
			continue
		}

		contentLower := strings.ToLower(content)
		for _, term := range queryTerms {
			if strings.Contains(contentLower, strings.ToLower(term)) {
				matchedFields = append(matchedFields, field)
				break
			}
		}
	}

	return matchedFields
}

// tokenizeQuery splits the query into search terms
func tokenizeQuery(query string) []string {
	parts := strings.Fields(strings.TrimSpace(query))

	var terms []string
	for _, part := range parts {
		if len(part) > 0 {
			terms = append(terms, part)
		}
	}

	return terms
}

// normalizeScores applies min-max normalization to map scores into [0, 1]
func normalizeScores(results []Result) {
	if len(results) == 0 {
		return
	}
	maxScore := results[0].Score
	for _, r := range results[1:] {
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}
	if maxScore <= 0 {
		return
	}
	for i := range results {
		results[i].Score = results[i].Score / maxScore
	}
}

// sortAndRankResults sorts results by score and assigns ranks
func sortAndRankResults(results []Result) []Result {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Repository.StargazersCount > results[j].Repository.StargazersCount
	})

	for i := range results {
		results[i].Rank = i + 1
	}

	return results
}
