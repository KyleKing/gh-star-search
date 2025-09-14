package related

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/kyleking/gh-star-search/internal/storage"
)

// RelatedRepository represents a related repository with scoring details
type RelatedRepository struct {
	Repository  storage.StoredRepo `json:"repository"`
	Score       float64            `json:"score"`
	Explanation string             `json:"explanation"`
	Components  ScoreComponents    `json:"components"`
}

// ScoreComponents breaks down the related score by component
type ScoreComponents struct {
	SameOrg       float64 `json:"same_org"`
	TopicOverlap  float64 `json:"topic_overlap"`
	SharedContrib float64 `json:"shared_contrib"`
	VectorSim     float64 `json:"vector_sim"`
	FinalScore    float64 `json:"final_score"`
}

// Engine defines the related repository engine interface
type Engine interface {
	FindRelated(ctx context.Context, repoFullName string, limit int) ([]RelatedRepository, error)
}

// RelatedEngine implements the Engine interface
type RelatedEngine struct {
	repo storage.Repository
}

// NewRelatedEngine creates a new related repository engine
func NewRelatedEngine(repo storage.Repository) *RelatedEngine {
	return &RelatedEngine{
		repo: repo,
	}
}

// FindRelated finds repositories related to the given repository
func (e *RelatedEngine) FindRelated(ctx context.Context, repoFullName string, limit int) ([]RelatedRepository, error) {
	// Get the target repository
	targetRepo, err := e.repo.GetRepository(ctx, repoFullName)
	if err != nil {
		return nil, fmt.Errorf("failed to get target repository: %w", err)
	}

	// Get all repositories for comparison
	allRepos, err := e.repo.ListRepositories(ctx, 10000, 0) // Large limit to get all
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	var related []RelatedRepository

	for _, candidate := range allRepos {
		// Skip the target repository itself
		if candidate.FullName == targetRepo.FullName {
			continue
		}

		// Calculate component scores
		components := e.calculateComponents(*targetRepo, candidate)

		// Skip if no meaningful relationship
		if components.FinalScore < 0.25 {
			continue
		}

		// Generate explanation
		explanation := e.generateExplanation(components, *targetRepo, candidate)

		related = append(related, RelatedRepository{
			Repository:  candidate,
			Score:       components.FinalScore,
			Explanation: explanation,
			Components:  components,
		})
	}

	// Sort by score descending
	sort.Slice(related, func(i, j int) bool {
		return related[i].Score > related[j].Score
	})

	// Apply limit
	if limit > 0 && len(related) > limit {
		related = related[:limit]
	}

	return related, nil
}

// calculateComponents computes the weighted score components
func (e *RelatedEngine) calculateComponents(target, candidate storage.StoredRepo) ScoreComponents {
	var components ScoreComponents

	// Component weights (will be renormalized if some are missing)
	weights := map[string]float64{
		"same_org":       0.30,
		"topic_overlap":  0.25,
		"shared_contrib": 0.25,
		"vector_sim":     0.20,
	}

	availableComponents := make(map[string]float64)

	// 1. Same Organization Score
	sameOrgScore := e.calculateSameOrgScore(target, candidate)
	if sameOrgScore > 0 {
		availableComponents["same_org"] = sameOrgScore
		components.SameOrg = sameOrgScore
	}

	// 2. Topic Overlap Score (Jaccard similarity)
	topicScore := e.calculateTopicOverlapScore(target, candidate)
	if topicScore > 0 {
		availableComponents["topic_overlap"] = topicScore
		components.TopicOverlap = topicScore
	}

	// 3. Shared Contributors Score
	contribScore := e.calculateSharedContribScore(target, candidate)
	if contribScore > 0 {
		availableComponents["shared_contrib"] = contribScore
		components.SharedContrib = contribScore
	}

	// 4. Vector Similarity Score
	vectorScore := e.calculateVectorSimilarityScore(target, candidate)
	if vectorScore > 0 {
		availableComponents["vector_sim"] = vectorScore
		components.VectorSim = vectorScore
	}

	// Renormalize weights for available components
	if len(availableComponents) == 0 {
		components.FinalScore = 0.0
		return components
	}

	totalWeight := 0.0
	for component := range availableComponents {
		totalWeight += weights[component]
	}

	// Calculate weighted final score
	finalScore := 0.0

	for component, score := range availableComponents {
		normalizedWeight := weights[component] / totalWeight
		finalScore += score * normalizedWeight
	}

	components.FinalScore = finalScore

	return components
}

// calculateSameOrgScore returns 1.0 if same org, 0.0 otherwise
func (e *RelatedEngine) calculateSameOrgScore(target, candidate storage.StoredRepo) float64 {
	targetOrg := extractOrg(target.FullName)
	candidateOrg := extractOrg(candidate.FullName)

	// Only count as same org if it's actually an organization (not a user)
	// This is a heuristic - in practice you might want to check if it's actually an org
	if targetOrg != "" && candidateOrg != "" && targetOrg == candidateOrg {
		return 1.0
	}

	return 0.0
}

// calculateTopicOverlapScore calculates Jaccard similarity of topics
func (e *RelatedEngine) calculateTopicOverlapScore(target, candidate storage.StoredRepo) float64 {
	if len(target.Topics) == 0 || len(candidate.Topics) == 0 {
		return 0.0
	}

	// Convert to sets for easier intersection/union calculation
	targetSet := make(map[string]bool)
	for _, topic := range target.Topics {
		targetSet[strings.ToLower(topic)] = true
	}

	candidateSet := make(map[string]bool)
	for _, topic := range candidate.Topics {
		candidateSet[strings.ToLower(topic)] = true
	}

	// Calculate intersection
	intersection := 0

	for topic := range targetSet {
		if candidateSet[topic] {
			intersection++
		}
	}

	// Calculate union
	union := len(targetSet) + len(candidateSet) - intersection

	if union == 0 {
		return 0.0
	}

	// Jaccard similarity
	return float64(intersection) / float64(union)
}

// calculateSharedContribScore calculates normalized intersection of top contributors
func (e *RelatedEngine) calculateSharedContribScore(target, candidate storage.StoredRepo) float64 {
	if len(target.Contributors) == 0 || len(candidate.Contributors) == 0 {
		return 0.0
	}

	// Get top 10 contributors from each
	targetContribs := getTopContributors(target.Contributors, 10)
	candidateContribs := getTopContributors(candidate.Contributors, 10)

	// Convert to sets
	targetSet := make(map[string]bool)
	for _, contrib := range targetContribs {
		targetSet[strings.ToLower(contrib.Login)] = true
	}

	candidateSet := make(map[string]bool)
	for _, contrib := range candidateContribs {
		candidateSet[strings.ToLower(contrib.Login)] = true
	}

	// Calculate intersection
	intersection := 0

	for login := range targetSet {
		if candidateSet[login] {
			intersection++
		}
	}

	if intersection == 0 {
		return 0.0
	}

	// Normalize by the smaller set size
	minSize := len(targetSet)
	if len(candidateSet) < minSize {
		minSize = len(candidateSet)
	}

	return float64(intersection) / float64(minSize)
}

// calculateVectorSimilarityScore calculates cosine similarity of embeddings
func (e *RelatedEngine) calculateVectorSimilarityScore(target, candidate storage.StoredRepo) float64 {
	// TODO: Implement when embeddings are available
	// For now, return 0.0 (will be excluded from scoring)
	if len(target.RepoEmbedding) == 0 || len(candidate.RepoEmbedding) == 0 {
		return 0.0
	}

	// Calculate cosine similarity
	return cosineSimilarity(target.RepoEmbedding, candidate.RepoEmbedding)
}

// generateExplanation creates a human-readable explanation of the relationship
func (e *RelatedEngine) generateExplanation(components ScoreComponents, target, candidate storage.StoredRepo) string {
	var explanations []string

	// Same org explanation
	if components.SameOrg > 0 {
		org := extractOrg(target.FullName)
		explanations = append(explanations, fmt.Sprintf("shared org '%s'", org))
	}

	// Topic overlap explanation
	if components.TopicOverlap > 0 {
		sharedTopics := getSharedTopics(target.Topics, candidate.Topics)
		if len(sharedTopics) > 0 {
			if len(sharedTopics) == 1 {
				explanations = append(explanations, fmt.Sprintf("shared topic (%s)", sharedTopics[0]))
			} else if len(sharedTopics) <= 3 {
				explanations = append(explanations, fmt.Sprintf("%d shared topics (%s)",
					len(sharedTopics), strings.Join(sharedTopics, ", ")))
			} else {
				explanations = append(explanations, fmt.Sprintf("%d shared topics (%s, ...)",
					len(sharedTopics), strings.Join(sharedTopics[:3], ", ")))
			}
		}
	}

	// Shared contributors explanation
	if components.SharedContrib > 0 {
		sharedContribs := getSharedContributors(target.Contributors, candidate.Contributors)
		if len(sharedContribs) > 0 {
			if len(sharedContribs) == 1 {
				explanations = append(explanations, fmt.Sprintf("shared contributor (%s)", sharedContribs[0]))
			} else if len(sharedContribs) <= 3 {
				explanations = append(explanations, fmt.Sprintf("%d shared contributors (%s)",
					len(sharedContribs), strings.Join(sharedContribs, ", ")))
			} else {
				explanations = append(explanations, fmt.Sprintf("%d shared contributors (%s, ...)",
					len(sharedContribs), strings.Join(sharedContribs[:3], ", ")))
			}
		}
	}

	// Vector similarity explanation
	if components.VectorSim > 0 {
		explanations = append(explanations, fmt.Sprintf("high vector similarity (%.2f)", components.VectorSim))
	}

	if len(explanations) == 0 {
		return "related"
	}

	return strings.Join(explanations, " and ")
}

// Helper functions

func extractOrg(fullName string) string {
	parts := strings.Split(fullName, "/")
	if len(parts) >= 2 {
		return parts[0]
	}

	return ""
}

func getTopContributors(contributors []storage.Contributor, limit int) []storage.Contributor {
	if len(contributors) <= limit {
		return contributors
	}

	// Sort by contributions descending
	sorted := make([]storage.Contributor, len(contributors))
	copy(sorted, contributors)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Contributions > sorted[j].Contributions
	})

	return sorted[:limit]
}

func getSharedTopics(topics1, topics2 []string) []string {
	set1 := make(map[string]bool)
	for _, topic := range topics1 {
		set1[strings.ToLower(topic)] = true
	}

	var shared []string

	for _, topic := range topics2 {
		if set1[strings.ToLower(topic)] {
			shared = append(shared, topic)
		}
	}

	return shared
}

func getSharedContributors(contribs1, contribs2 []storage.Contributor) []string {
	set1 := make(map[string]bool)
	for _, contrib := range contribs1 {
		set1[strings.ToLower(contrib.Login)] = true
	}

	var shared []string

	for _, contrib := range contribs2 {
		if set1[strings.ToLower(contrib.Login)] {
			shared = append(shared, contrib.Login)
		}
	}

	return shared
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64

	for i := range len(a) {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
