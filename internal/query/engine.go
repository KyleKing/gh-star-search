package query

import (
	"context"
	"math"
	"strings"

	"github.com/kyleking/gh-star-search/internal/storage"
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
	repo storage.Repository
}

// NewSearchEngine creates a new search engine instance
func NewSearchEngine(repo storage.Repository) *SearchEngine {
	return &SearchEngine{
		repo: repo,
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
		return e.searchFuzzy(ctx, q.Raw, opts) // Default to fuzzy
	}
}

// searchFuzzy performs fuzzy text search with BM25-like scoring
func (e *SearchEngine) searchFuzzy(
	ctx context.Context,
	query string,
	opts SearchOptions,
) ([]Result, error) {
	// Get raw search results from storage layer
	storageResults, err := e.repo.SearchRepositories(ctx, query)
	if err != nil {
		return nil, err
	}

	// Convert to enhanced results with improved scoring
	var results []Result

	queryTerms := tokenizeQuery(query)

	for _, sr := range storageResults {
		// Calculate enhanced BM25-like score
		score := e.calculateFuzzyScore(sr.Repository, queryTerms)

		// Apply ranking boosts
		score = e.applyRankingBoosts(sr.Repository, score)

		// Clamp score to 1.0
		if score > 1.0 {
			score = 1.0
		}

		// Skip results below minimum score
		if score < opts.MinScore {
			continue
		}

		// Identify matched fields
		matchFields := e.identifyMatchedFields(sr.Repository, queryTerms)

		results = append(results, Result{
			RepoID:      sr.Repository.ID,
			Score:       score,
			Repository:  sr.Repository,
			MatchFields: matchFields,
		})
	}

	// Sort by score (descending) and assign ranks
	results = sortAndRankResults(results)

	// Apply limit
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// searchVector performs vector similarity search
func (e *SearchEngine) searchVector(
	ctx context.Context,
	query string,
	opts SearchOptions,
) ([]Result, error) {
	// TODO: Implement vector search with cosine similarity
	// For now, fall back to fuzzy search
	// This will be implemented when embedding functionality is added
	return e.searchFuzzy(ctx, query, opts)
}

// calculateFuzzyScore computes a BM25-like relevance score
func (e *SearchEngine) calculateFuzzyScore(repo storage.StoredRepo, queryTerms []string) float64 {
	if len(queryTerms) == 0 {
		return 0.0
	}

	// Define field weights (higher = more important)
	fieldWeights := map[string]float64{
		"full_name":    1.0,
		"description":  0.8,
		"purpose":      0.9,
		"technologies": 0.7,
		"features":     0.6,
		"topics":       0.5,
		"contributors": 0.4,
	}

	totalScore := 0.0
	matchedTerms := 0

	for _, term := range queryTerms {
		termLower := strings.ToLower(term)
		termScore := 0.0

		// Check each field for matches
		for field, weight := range fieldWeights {
			fieldContent := e.getFieldContent(repo, field)
			if fieldContent == "" {
				continue
			}

			fieldContentLower := strings.ToLower(fieldContent)

			// Simple term frequency calculation
			tf := float64(strings.Count(fieldContentLower, termLower))
			if tf > 0 {
				// Apply BM25-like scoring with field weight
				// Simplified: tf / (tf + k1) * weight
				k1 := 1.2 // BM25 parameter
				fieldScore := (tf / (tf + k1)) * weight
				termScore += fieldScore
			}
		}

		if termScore > 0 {
			matchedTerms++
			totalScore += termScore
		}
	}

	// Normalize by number of query terms and apply coverage bonus
	if matchedTerms > 0 {
		avgScore := totalScore / float64(len(queryTerms))
		coverageBonus := float64(matchedTerms) / float64(len(queryTerms))

		return avgScore * (0.7 + 0.3*coverageBonus) // Base score + coverage bonus
	}

	return 0.0
}

// applyRankingBoosts applies logarithmic star boost and recency decay
func (e *SearchEngine) applyRankingBoosts(repo storage.StoredRepo, baseScore float64) float64 {
	if baseScore <= 0 {
		return baseScore
	}

	// Logarithmic star boost (small boost)
	starBoost := 1.0
	if repo.StargazersCount > 0 {
		// Small logarithmic boost: 1 + 0.1 * log10(stars+1) / 6
		// This gives ~0.05 boost for 100 stars, ~0.1 boost for 10k stars
		starBoost = 1.0 + (0.1 * math.Log10(float64(repo.StargazersCount+1)) / 6.0)
	}

	// Recency decay (small penalty for very old repos)
	recencyFactor := 1.0
	// TODO: Implement proper recency calculation based on repo.UpdatedAt
	// For now, use a placeholder that doesn't change the score

	return baseScore * starBoost * recencyFactor
}

// identifyMatchedFields identifies which logical fields matched the query
func (e *SearchEngine) identifyMatchedFields(
	repo storage.StoredRepo,
	queryTerms []string,
) []string {
	var matchedFields []string

	fieldMap := map[string]string{
		"name":         repo.FullName,
		"description":  repo.Description,
		"purpose":      repo.Purpose,
		"technologies": strings.Join(repo.Technologies, " "),
		"features":     strings.Join(repo.Features, " "),
		"topics":       strings.Join(repo.Topics, " "),
	}

	for field, content := range fieldMap {
		if content == "" {
			continue
		}

		contentLower := strings.ToLower(content)
		for _, term := range queryTerms {
			if strings.Contains(contentLower, strings.ToLower(term)) {
				matchedFields = append(matchedFields, field)
				break // Only add field once
			}
		}
	}

	return matchedFields
}

// getFieldContent extracts content from a specific field
func (e *SearchEngine) getFieldContent(repo storage.StoredRepo, field string) string {
	switch field {
	case "full_name":
		return repo.FullName
	case "description":
		return repo.Description
	case "purpose":
		return repo.Purpose
	case "technologies":
		return strings.Join(repo.Technologies, " ")
	case "features":
		return strings.Join(repo.Features, " ")
	case "topics":
		return strings.Join(repo.Topics, " ")
	case "contributors":
		var names []string
		for _, contrib := range repo.Contributors {
			names = append(names, contrib.Login)
		}

		return strings.Join(names, " ")
	default:
		return ""
	}
}

// tokenizeQuery splits the query into search terms
func tokenizeQuery(query string) []string {
	// Simple tokenization: split by whitespace and remove empty strings
	parts := strings.Fields(strings.TrimSpace(query))

	var terms []string

	for _, part := range parts {
		if len(part) > 0 {
			terms = append(terms, part)
		}
	}

	return terms
}

// sortAndRankResults sorts results by score and assigns ranks
func sortAndRankResults(results []Result) []Result {
	// Sort by score descending, then by stars descending as tiebreaker
	for i := range len(results) - 1 {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score ||
				(results[i].Score == results[j].Score &&
					results[i].Repository.StargazersCount < results[j].Repository.StargazersCount) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Assign ranks
	for i := range results {
		results[i].Rank = i + 1
	}

	return results
}
