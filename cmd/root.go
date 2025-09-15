package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/kyleking/gh-star-search/internal/config"
	gherrors "github.com/kyleking/gh-star-search/internal/errors"
	"github.com/kyleking/gh-star-search/internal/logging"
)

// contextKey is a type for context keys to avoid string collisions
type contextKey string

const (
	configContextKey contextKey = "config"
)

var (
	// Global configuration flags
	configFile string
	logLevel   string
	verbose    bool
	debug      bool
	dbPath     string
	cacheDir   string
)

var rootCmd = &cobra.Command{
	Use:   "gh-star-search",
	Short: "Search your starred GitHub repositories using natural language",
	Long: `gh-star-search is a GitHub CLI extension that ingests and indexes all repositories
starred by the currently logged-in user. It enables natural language search queries
against a local DuckDB database containing both structured metadata and unstructured
content from your starred repositories.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		return initializeGlobalConfig(cmd)
	},
}

func Execute() error {
	ctx := context.Background()

	// Set up fallback logger in case initialization fails
	logging.SetupFallbackLogger()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		// Handle structured errors with user-friendly messages
		var structErr *gherrors.Error
		if errors.As(err, &structErr) {
			printStructuredError(structErr)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

		return err
	}

	return nil
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path (default: ~/.config/gh-star-search/config.json)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug mode")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db-path", "", "database file path")
	rootCmd.PersistentFlags().StringVar(&cacheDir, "cache-dir", "", "cache directory path")

	// Add subcommands
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(clearCmd)
}

// initializeGlobalConfig initializes the global configuration and logging
func initializeGlobalConfig(cmd *cobra.Command) error {
	// Prepare flag overrides
	flagOverrides := make(map[string]interface{})

	if logLevel != "" {
		flagOverrides["log-level"] = logLevel
	}

	if verbose {
		flagOverrides["verbose"] = verbose
	}

	if debug {
		flagOverrides["debug"] = debug
	}

	if dbPath != "" {
		flagOverrides["db-path"] = dbPath
	}

	if cacheDir != "" {
		flagOverrides["cache-dir"] = cacheDir
	}

	// Set custom config file path if provided
	if configFile != "" {
		os.Setenv("GH_STAR_SEARCH_CONFIG", configFile)
	}

	// Load configuration with overrides
	cfg, err := config.LoadConfigWithOverrides(flagOverrides)
	if err != nil {
		return gherrors.Wrap(err, gherrors.ErrTypeConfig, "failed to load configuration")
	}

	// Expand paths and ensure directories exist
	cfg.ExpandAllPaths()

	if err := cfg.EnsureDirectories(); err != nil {
		return gherrors.Wrap(err, gherrors.ErrTypeFileSystem, "failed to create required directories")
	}

	// Initialize logging with slog
	if err := logging.SetupLogger(cfg.Logging); err != nil {
		return gherrors.Wrap(err, gherrors.ErrTypeConfig, "failed to initialize logging")
	}

	// Log startup information using slog
	slog.Info("gh-star-search starting",
		slog.String("version", getVersion()),
		slog.String("config", cfg.Database.Path))

	if cfg.Debug.Enabled {
		slog.Debug("Debug mode enabled")
		slog.Debug("Configuration loaded", slog.Any("config", cfg))
	}

	// Store config in context for subcommands
	ctx := context.WithValue(cmd.Context(), configContextKey, cfg)
	cmd.SetContext(ctx)

	return nil
}

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

	if err.Cause != nil && debug {
		fmt.Fprintf(os.Stderr, "\nUnderlying error: %v\n", err.Cause)
	}

	if err.Stack != "" && debug {
		fmt.Fprintf(os.Stderr, "\nStack trace:\n%s\n", err.Stack)
	}
}

// getVersion returns the application version
func getVersion() string {
	// This would typically be set during build time
	return "dev"
}

// GetConfigFromContext retrieves the configuration from the command context
func GetConfigFromContext(cmd *cobra.Command) (*config.Config, error) {
	cfg, ok := cmd.Context().Value(configContextKey).(*config.Config)
	if !ok {
		return nil, gherrors.New(gherrors.ErrTypeInternal, "configuration not found in context")
	}

	return cfg, nil
}
