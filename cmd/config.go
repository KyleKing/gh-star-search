package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kyleking/gh-star-search/internal/config"
	"github.com/kyleking/gh-star-search/internal/errors"
	"github.com/urfave/cli/v3"
)

// Ensure config package is used
var _ *config.Config

func ConfigCommand() *cli.Command {
	return &cli.Command{
		Name:        "config",
		Usage:       "Display the active configuration",
		Description: `Show the current active configuration including all settings from file, environment variables, and command-line flags.`,
		Action:      runConfig,
	}
}

func runConfig(ctx context.Context, _ *cli.Command) error {
	// Try to get config from context first, fallback to loading directly
	cfg := getConfigFromContext(ctx)

	// Ensure we have a valid config
	if cfg == nil {
		return errors.NewConfigError("failed to load configuration", "")
	}

	// Display configuration in a readable format
	fmt.Println("====================")
	fmt.Println("Active Configuration:")

	// Database configuration
	fmt.Println("\nDatabase:")
	fmt.Printf("  Path: %s\n", cfg.Database.Path)
	fmt.Printf("  Max Connections: %d\n", cfg.Database.MaxConnections)
	fmt.Printf("  Query Timeout: %s\n", cfg.Database.QueryTimeout)

	// Cache configuration
	fmt.Println("\nCache:")
	fmt.Printf("  Directory: %s\n", cfg.Cache.Directory)
	fmt.Printf("  Max Size: %d MB\n", cfg.Cache.MaxSizeMB)
	fmt.Printf("  TTL: %d hours\n", cfg.Cache.TTLHours)
	fmt.Printf("  Cleanup Frequency: %s\n", cfg.Cache.CleanupFreq)
	fmt.Printf("  Metadata Stale: %d days\n", cfg.Cache.MetadataStaleDays)
	fmt.Printf("  Stats Stale: %d days\n", cfg.Cache.StatsStaleDays)

	// Logging configuration
	fmt.Println("\nLogging:")
	fmt.Printf("  Level: %s\n", cfg.Logging.Level)
	fmt.Printf("  Format: %s\n", cfg.Logging.Format)
	fmt.Printf("  Output: %s\n", cfg.Logging.Output)

	if cfg.Logging.Output == "file" {
		fmt.Printf("  File: %s\n", cfg.Logging.File)
		fmt.Printf("  Max Size: %d MB\n", cfg.Logging.MaxSizeMB)
		fmt.Printf("  Max Backups: %d\n", cfg.Logging.MaxBackups)
		fmt.Printf("  Max Age: %d days\n", cfg.Logging.MaxAgeDays)
	}

	fmt.Printf("  Add Source: %t\n", cfg.Logging.AddSource)

	// Debug configuration
	fmt.Println("\nDebug:")
	fmt.Printf("  Enabled: %t\n", cfg.Debug.Enabled)

	if cfg.Debug.Enabled {
		fmt.Printf("  Profile Port: %d\n", cfg.Debug.ProfilePort)
		fmt.Printf("  Metrics Port: %d\n", cfg.Debug.MetricsPort)
	}

	fmt.Printf("  Verbose: %t\n", cfg.Debug.Verbose)
	fmt.Printf("  Trace API: %t\n", cfg.Debug.TraceAPI)

	// Show raw JSON if debug is enabled
	if cfg.Debug.Enabled {
		fmt.Println("\nRaw Configuration (JSON):")
		fmt.Println("==========================")

		jsonData, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config to JSON: %w", err)
		}

		fmt.Println(string(jsonData))
	}

	return nil
}
