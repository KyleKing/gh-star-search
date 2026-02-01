package formatter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/KyleKing/gh-star-search/internal/query"
	"github.com/KyleKing/gh-star-search/internal/storage"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	FormatLong  OutputFormat = "long"
	FormatShort OutputFormat = "short"
)

// Formatter handles repository output formatting
type Formatter struct{}

// NewFormatter creates a new formatter instance
func NewFormatter() *Formatter {
	return &Formatter{}
}

// FormatResult formats a single search result
func (f *Formatter) FormatResult(result query.Result, format OutputFormat) string {
	switch format {
	case FormatLong:
		return f.formatLong(result.Repository)
	case FormatShort:
		return f.formatShort(result.Repository, result.Score, result.Rank)
	default:
		return f.formatShort(result.Repository, result.Score, result.Rank)
	}
}

// FormatRepository formats a repository without search context
func (f *Formatter) FormatRepository(repo storage.StoredRepo, format OutputFormat) string {
	switch format {
	case FormatLong:
		return f.formatLong(repo)
	case FormatShort:
		return f.formatShortBasic(repo)
	default:
		return f.formatShortBasic(repo)
	}
}

// formatLong implements the exact long-form specification from the design
func (f *Formatter) formatLong(repo storage.StoredRepo) string {
	var lines []string

	// Line 1: Header with link
	orgName := strings.Split(repo.FullName, "/")
	if len(orgName) == 2 {
		lines = append(lines, fmt.Sprintf("%s/%s  (link: https://github.com/%s)",
			orgName[0], orgName[1], repo.FullName))
	} else {
		lines = append(lines, fmt.Sprintf("%s  (link: https://github.com/%s)",
			repo.FullName, repo.FullName))
	}

	// Line 2: GitHub Description
	description := repo.Description
	if description == "" {
		description = "-"
	}

	lines = append(lines, "GitHub Description: "+description)

	// Line 3: External link
	homepage := repo.Homepage
	if homepage == "" {
		homepage = "-"
	}

	lines = append(lines, "GitHub External Description Link: "+homepage)

	// Line 4: Numbers (issues, PRs, stars, forks)
	openIssues := f.formatInt(repo.OpenIssuesOpen)
	totalIssues := f.formatInt(repo.OpenIssuesTotal)
	openPRs := f.formatInt(repo.OpenPRsOpen)
	totalPRs := f.formatInt(repo.OpenPRsTotal)
	stars := f.formatInt(repo.StargazersCount)
	forks := f.formatInt(repo.ForksCount)

	lines = append(
		lines,
		fmt.Sprintf("Numbers: %s/%s open issues, %s/%s open PRs, %s stars, %s forks",
			openIssues, totalIssues, openPRs, totalPRs, stars, forks),
	)

	// Line 5: Commits
	commits30d := f.formatInt(repo.Commits30d)
	commits1y := f.formatInt(repo.Commits1y)
	commitsTotal := f.formatInt(repo.CommitsTotal)

	lines = append(lines, fmt.Sprintf("Commits: %s in last 30 days, %s in last year, %s total",
		commits30d, commits1y, commitsTotal))

	// Line 6: Age
	age := f.humanizeAge(repo.CreatedAt)
	lines = append(lines, "Age: "+age)

	// Line 7: License
	license := repo.LicenseSPDXID
	if license == "" {
		license = repo.LicenseName
	}

	if license == "" {
		license = "-"
	}

	lines = append(lines, "License: "+license)

	// Line 8: Top 10 Contributors
	contributors := f.formatContributors(repo.Contributors)
	lines = append(lines, "Top 10 Contributors: "+contributors)

	// Line 9: GitHub Topics
	topics := strings.Join(repo.Topics, ", ")
	if topics == "" {
		topics = "-"
	}

	lines = append(lines, "GitHub Topics: "+topics)

	// Line 10: Languages
	languages := f.formatLanguages(repo.Languages)
	lines = append(lines, "Languages: "+languages)

	// Line 11: Related Stars
	relatedStars := f.formatRelatedStars(repo)
	lines = append(lines, "Related Stars: "+relatedStars)

	// Line 12: Last synced
	lastSynced := f.humanizeAge(repo.LastSynced)
	lines = append(lines, "Last synced: "+lastSynced)

	// Line 14-15: Planned placeholders
	lines = append(lines, "(PLANNED: dependencies count)")
	lines = append(lines, "(PLANNED: 'used by' count)")

	return strings.Join(lines, "\n")
}

// formatShort formats the short form with search context (score and rank)
func (f *Formatter) formatShort(repo storage.StoredRepo, score float64, rank int) string {
	// First two lines of long-form
	longForm := f.formatLong(repo)
	longLines := strings.Split(longForm, "\n")

	var firstTwoLines []string
	if len(longLines) >= 2 {
		firstTwoLines = longLines[:2]
	} else {
		firstTwoLines = longLines
	}

	// Add score, truncated description, and primary language
	description := repo.Description
	if len(description) > 80 {
		description = description[:77] + "..."
	}

	if description == "" {
		description = "-"
	}

	primaryLang := f.getPrimaryLanguage(repo.Languages)

	result := strings.Join(firstTwoLines, "\n")
	result += fmt.Sprintf("\n%d. %s (%s)  ⭐ %d  %s  Updated %s  Score:%.2f",
		rank, repo.FullName, description, repo.StargazersCount, primaryLang,
		f.humanizeAge(repo.UpdatedAt), score)

	return result
}

// formatShortBasic formats the short form without search context
func (f *Formatter) formatShortBasic(repo storage.StoredRepo) string {
	// First two lines of long-form
	longForm := f.formatLong(repo)
	longLines := strings.Split(longForm, "\n")

	if len(longLines) >= 2 {
		return strings.Join(longLines[:2], "\n")
	}

	return strings.Join(longLines, "\n")
}

// formatInt formats an integer, returning "?" for negative values (unknown)
func (f *Formatter) formatInt(value int) string {
	if value < 0 {
		return "?"
	}

	return strconv.Itoa(value)
}

// humanizeAge converts a time to a human-readable age string
func (f *Formatter) humanizeAge(t time.Time) string {
	if t.IsZero() {
		return "?"
	}

	now := time.Now()
	duration := now.Sub(t)

	days := int(duration.Hours() / 24)

	if days < 1 {
		return "today"
	} else if days == 1 {
		return "1 day ago"
	} else if days < 30 {
		return fmt.Sprintf("%d days ago", days)
	} else if days < 365 {
		months := days / 30
		if months == 1 {
			return "1 month ago"
		}

		return fmt.Sprintf("%d months ago", months)
	}

	years := days / 365
	if years == 1 {
		return "1 year ago"
	}

	return fmt.Sprintf("%d years ago", years)
}

// formatContributors formats the contributors list
func (f *Formatter) formatContributors(contributors []storage.Contributor) string {
	if len(contributors) == 0 {
		return "-"
	}

	// Limit to top 10 contributors
	limit := len(contributors)
	if limit > 10 {
		limit = 10
	}

	parts := make([]string, 0, limit)

	for i := range limit {
		contrib := contributors[i]
		parts = append(parts, fmt.Sprintf("%s (%d)", contrib.Login, contrib.Contributions))
	}

	return strings.Join(parts, ", ")
}

// formatLanguages formats the languages map with LOC approximation
func (f *Formatter) formatLanguages(languages map[string]int64) string {
	if len(languages) == 0 {
		return "-"
	}

	// Create a slice of language entries and sort by bytes (descending)
	type langEntry struct {
		name  string
		bytes int64
	}

	entries := make([]langEntry, 0, len(languages))
	for lang, bytes := range languages {
		entries = append(entries, langEntry{name: lang, bytes: bytes})
	}

	// Sort by bytes descending, then by name for deterministic output
	for i := range len(entries) - 1 {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].bytes < entries[j].bytes ||
				(entries[i].bytes == entries[j].bytes && entries[i].name > entries[j].name) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Convert bytes to approximate lines of code (60 bytes per line average)
	parts := make([]string, 0, len(entries))

	for _, entry := range entries {
		loc := entry.bytes / 60 // Approximate LOC
		parts = append(parts, fmt.Sprintf("%s (%d)", entry.name, loc))
	}

	return strings.Join(parts, ", ")
}

// getPrimaryLanguage returns the primary language (most bytes)
func (f *Formatter) getPrimaryLanguage(languages map[string]int64) string {
	if len(languages) == 0 {
		return "-"
	}

	var primaryLang string

	var maxBytes int64

	for lang, bytes := range languages {
		if bytes > maxBytes {
			maxBytes = bytes
			primaryLang = lang
		}
	}

	return primaryLang
}

// formatRelatedStars formats the related stars counts
func (f *Formatter) formatRelatedStars(repo storage.StoredRepo) string {
	// TODO: Implement actual related stars calculation
	// For now, return placeholder values
	orgName := strings.Split(repo.FullName, "/")[0]
	return fmt.Sprintf("? in %s, ? by top contributors", orgName)
}
