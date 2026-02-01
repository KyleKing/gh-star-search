package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/KyleKing/gh-star-search/cmd"
	"github.com/KyleKing/gh-star-search/internal/config"
	gherrors "github.com/KyleKing/gh-star-search/internal/errors"
	"github.com/KyleKing/gh-star-search/internal/logging"
)

var globalLogCloser io.Closer

func main() {
	defer func() {
		if globalLogCloser != nil {
			globalLogCloser.Close()
		}
	}()
	app := &cli.Command{
		Name:  "gh-star-search",
		Usage: "Search your starred GitHub repositories using natural language",
		Description: `gh-star-search is a GitHub CLI extension that ingests and indexes all repositories
starred by the currently logged-in user. It enables natural language search queries
against a local DuckDB database containing both structured metadata and unstructured
content from your starred repositories.`,
		Version: getVersion(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "config file path (default: ~/.config/gh-star-search/config.json)",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Usage:   "log level (debug, info, warn, error)",
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "enable verbose output",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "enable debug mode",
			},
			&cli.StringFlag{
				Name:  "db-path",
				Usage: "database file path",
			},
			&cli.StringFlag{
				Name:  "cache-dir",
				Usage: "cache directory path",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) error {
			_, err := initializeGlobalConfig(ctx, cmd)
			return err
		},
		Commands: []*cli.Command{
			cmd.SyncCommand(),
			cmd.ListCommand(),
			cmd.InfoCommand(),
			cmd.StatsCommand(),
			cmd.ClearCommand(),
			cmd.QueryCommand(),
			cmd.RelatedCommand(),
			cmd.ConfigCommand(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		// Handle structured errors with user-friendly messages
		var structErr *gherrors.Error
		if errors.As(err, &structErr) {
			printStructuredError(structErr)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

		os.Exit(1)
	}
}

// getVersion returns the application version
func getVersion() string {
	// This would typically be set during build time
	return "dev"
}

// initializeGlobalConfig initializes the global configuration and logging
func initializeGlobalConfig(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	// Prepare flag overrides
	flagOverrides := make(map[string]interface{})

	if logLevel := cmd.String("log-level"); logLevel != "" {
		flagOverrides["log-level"] = logLevel
	}

	if verbose := cmd.Bool("verbose"); verbose {
		flagOverrides["verbose"] = verbose
	}

	if debug := cmd.Bool("debug"); debug {
		flagOverrides["debug"] = debug
	}

	if dbPath := cmd.String("db-path"); dbPath != "" {
		flagOverrides["db-path"] = dbPath
	}

	if cacheDir := cmd.String("cache-dir"); cacheDir != "" {
		flagOverrides["cache-dir"] = cacheDir
	}

	// Set custom config file path if provided
	if configFile := cmd.String("config"); configFile != "" {
		os.Setenv("GH_STAR_SEARCH_CONFIG", configFile)
	}

	// Load configuration with overrides
	cfg, err := config.LoadConfigWithOverrides(flagOverrides)
	if err != nil {
		return ctx, gherrors.Wrap(err, gherrors.ErrTypeConfig, "failed to load configuration")
	}

	// Expand paths and ensure directories exist
	cfg.ExpandAllPaths()

	if err := cfg.EnsureDirectories(); err != nil {
		return ctx, gherrors.Wrap(
			err,
			gherrors.ErrTypeFileSystem,
			"failed to create required directories",
		)
	}

	// Initialize logging with slog
	logCloser, err := logging.SetupLogger(cfg.Logging)
	if err != nil {
		return ctx, gherrors.Wrap(err, gherrors.ErrTypeConfig, "failed to initialize logging")
	}

	globalLogCloser = logCloser

	// Log startup information using slog
	slog.Info("gh-star-search starting",
		slog.String("version", getVersion()),
		slog.String("config", cfg.Database.Path))

	debugMode = cfg.Debug.Enabled
	if debugMode {
		slog.Debug("Debug mode enabled")
		slog.Debug("Configuration loaded", slog.Any("config", cfg))
	}

	// Store config in context
	ctx = context.WithValue(ctx, configContextKey, cfg)

	return ctx, nil
}

// contextKey is a type for context keys to avoid string collisions
type contextKey string

const (
	configContextKey contextKey = "config"
)

var debugMode bool

// printStructuredError prints a user-friendly error message
func printStructuredError(err *gherrors.Error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Message)

	if err.Code != "" {
		fmt.Fprintf(os.Stderr, "Code: %s\n", err.Code)
	}

	if len(err.Context) > 0 {
		fmt.Fprintf(os.Stderr, "Context:\n")

		for k, v := range err.Context {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", k, v)
		}
	}

	if len(err.Suggestions) > 0 {
		fmt.Fprintf(os.Stderr, "\nSuggestions:\n")

		for _, suggestion := range err.Suggestions {
			fmt.Fprintf(os.Stderr, "  • %s\n", suggestion)
		}
	}

	if err.Cause != nil && debugMode {
		fmt.Fprintf(os.Stderr, "\nUnderlying error: %v\n", err.Cause)
	}

	if err.Stack != "" && debugMode {
		fmt.Fprintf(os.Stderr, "\nStack trace:\n%s\n", err.Stack)
	}
}
