package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/kyleking/gh-star-search/internal/storage"
	"github.com/urfave/cli/v3"
)

func StatsCommand() *cli.Command {
	return &cli.Command{
		Name:        "stats",
		Usage:       "Display database statistics",
		Description: `Show statistics about the local database including total repositories, last sync time, and database size.`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runStats(ctx)
		},
	}
}

func runStats(ctx context.Context) error {
	return runStatsWithStorage(ctx, nil)
}

func runStatsWithStorage(ctx context.Context, repo storage.Repository) error {
	// Initialize storage if not provided (for testing)
	if repo == nil {
		var err error

		repo, err = initializeStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		defer repo.Close()
	}

	// Get statistics
	stats, err := repo.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get statistics: %w", err)
	}

	// Display statistics
	fmt.Printf("Database Statistics\n")
	fmt.Printf("==================\n\n")

	fmt.Printf("Total Repositories: %d\n", stats.TotalRepositories)
	fmt.Printf("Total Content Chunks: %d\n", stats.TotalContentChunks)
	fmt.Printf("Database Size: %.2f MB\n", stats.DatabaseSizeMB)

	if !stats.LastSyncTime.IsZero() {
		fmt.Printf("Last Sync: %s\n", stats.LastSyncTime.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Sync: Never\n")
	}

	// Language breakdown
	if len(stats.LanguageBreakdown) > 0 {
		fmt.Printf("\nLanguage Breakdown:\n")

		// Sort languages by count (descending)
		type langCount struct {
			language string
			count    int
		}

		var languages []langCount
		for lang, count := range stats.LanguageBreakdown {
			languages = append(languages, langCount{lang, count})
		}

		sort.Slice(languages, func(i, j int) bool {
			return languages[i].count > languages[j].count
		})

		// Show top 10 languages
		maxShow := 10
		if len(languages) < maxShow {
			maxShow = len(languages)
		}

		for i := range maxShow {
			lang := languages[i]
			percentage := float64(lang.count) / float64(stats.TotalRepositories) * 100
			fmt.Printf("  %-15s %3d repos (%.1f%%)\n", lang.language, lang.count, percentage)
		}

		if len(languages) > maxShow {
			fmt.Printf("  ... and %d more languages\n", len(languages)-maxShow)
		}
	}

	// Topic breakdown (if available)
	if len(stats.TopicBreakdown) > 0 {
		fmt.Printf("\nTop Topics:\n")

		// Sort topics by count (descending)
		type topicCount struct {
			topic string
			count int
		}

		var topics []topicCount
		for topic, count := range stats.TopicBreakdown {
			topics = append(topics, topicCount{topic, count})
		}

		sort.Slice(topics, func(i, j int) bool {
			return topics[i].count > topics[j].count
		})

		// Show top 10 topics
		maxShow := 10
		if len(topics) < maxShow {
			maxShow = len(topics)
		}

		for i := range maxShow {
			topic := topics[i]
			percentage := float64(topic.count) / float64(stats.TotalRepositories) * 100
			fmt.Printf("  %-20s %3d repos (%.1f%%)\n", topic.topic, topic.count, percentage)
		}

		if len(topics) > maxShow {
			fmt.Printf("  ... and %d more topics\n", len(topics)-maxShow)
		}
	}

	return nil
}
