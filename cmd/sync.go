package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/username/gh-star-search/internal/config"
	"github.com/username/gh-star-search/internal/github"
	"github.com/username/gh-star-search/internal/llm"
	"github.com/username/gh-star-search/internal/processor"
	"github.com/username/gh-star-search/internal/storage"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync starred repositories to local database",
	Long: `Incrementally fetch and process each repository that the authenticated GitHub user 
has starred. Collects both structured metadata and unstructured content to enable 
intelligent search capabilities.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		return runSync(ctx, cmd, args)
	},
}

func init() {
	syncCmd.Flags().StringP("repo", "r", "", "Sync a specific repository for fine-tuning")
	syncCmd.Flags().BoolP("verbose", "v", false, "Show detailed processing steps")
	syncCmd.Flags().BoolP("compare-models", "c", false, "Compare different LLM backends")
	syncCmd.Flags().IntP("batch-size", "b", 10, "Number of repositories to process in each batch")
	syncCmd.Flags().BoolP("force", "f", false, "Force re-processing of all repositories")
}

// SyncService handles the synchronization of starred repositories
type SyncService struct {
	githubClient github.Client
	processor    processor.Service
	storage      storage.Repository
	config       *config.Config
	verbose      bool
}

// SyncStats tracks synchronization statistics
type SyncStats struct {
	TotalRepos     int
	NewRepos       int
	UpdatedRepos   int
	RemovedRepos   int
	SkippedRepos   int
	ErrorRepos     int
	StartTime      time.Time
	EndTime        time.Time
	ProcessingTime time.Duration
}

func runSync(ctx context.Context, cmd *cobra.Command, args []string) error {
	// Parse flags
	specificRepo, _ := cmd.Flags().GetString("repo")
	verbose, _ := cmd.Flags().GetBool("verbose")
	compareModels, _ := cmd.Flags().GetBool("compare-models")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	force, _ := cmd.Flags().GetBool("force")

	// Load configuration
	cfg := config.DefaultConfig()
	
	// Initialize services
	syncService, err := initializeSyncService(cfg, verbose)
	if err != nil {
		return fmt.Errorf("failed to initialize sync service: %w", err)
	}
	defer syncService.storage.Close()

	// Initialize database
	if err := syncService.storage.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Handle specific repository sync
	if specificRepo != "" {
		return syncService.syncSpecificRepository(ctx, specificRepo, compareModels)
	}

	// Perform full sync
	return syncService.performFullSync(ctx, batchSize, force)
}

func initializeSyncService(cfg *config.Config, verbose bool) (*SyncService, error) {
	// Initialize GitHub client
	githubClient, err := github.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Initialize storage
	dbPath := expandPath(cfg.Database.Path)
	repo, err := storage.NewDuckDBRepository(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage repository: %w", err)
	}

	// Initialize LLM service (optional)
	var llmService processor.LLMService
	if cfg.LLM.DefaultProvider != "" {
		llmManager := llm.NewManager(llm.DefaultManagerConfig())
		
		// Register available providers with default configs
		defaultConfig := llm.Config{Provider: llm.ProviderOpenAI, Model: llm.ModelGPT35Turbo}
		if err := llmManager.RegisterProvider(llm.ProviderOpenAI, llm.NewClient(defaultConfig)); err != nil {
			fmt.Printf("Warning: Failed to register OpenAI provider: %v\n", err)
		}
		
		defaultConfig.Provider = llm.ProviderAnthropic
		defaultConfig.Model = llm.ModelClaude3
		if err := llmManager.RegisterProvider(llm.ProviderAnthropic, llm.NewClient(defaultConfig)); err != nil {
			fmt.Printf("Warning: Failed to register Anthropic provider: %v\n", err)
		}
		
		defaultConfig.Provider = llm.ProviderLocal
		defaultConfig.Model = llm.ModelLlama2
		if err := llmManager.RegisterProvider(llm.ProviderLocal, llm.NewClient(defaultConfig)); err != nil {
			fmt.Printf("Warning: Failed to register Local provider: %v\n", err)
		}
		
		// Configure the default provider
		if llmConfig, exists := cfg.LLM.Providers[cfg.LLM.DefaultProvider]; exists {
			if err := llmManager.Configure(llmConfig); err != nil {
				fmt.Printf("Warning: Failed to configure LLM service: %v\n", err)
				fmt.Println("Continuing with basic content processing...")
			} else {
				llmService = &llmServiceAdapter{manager: llmManager}
			}
		}
	}

	// Initialize processor
	processorService := processor.NewService(githubClient, llmService)

	return &SyncService{
		githubClient: githubClient,
		processor:    processorService,
		storage:      repo,
		config:       cfg,
		verbose:      verbose,
	}, nil
}

func (s *SyncService) performFullSync(ctx context.Context, batchSize int, force bool) error {
	stats := &SyncStats{
		StartTime: time.Now(),
	}

	s.logVerbose("Starting full sync of starred repositories...")

	// Create spinner for progress indication
	sp := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	sp.Suffix = " Fetching starred repositories..."
	sp.Start()

	// Fetch all starred repositories
	starredRepos, err := s.githubClient.GetStarredRepos(ctx, "")
	if err != nil {
		sp.Stop()
		return fmt.Errorf("failed to fetch starred repositories: %w", err)
	}

	stats.TotalRepos = len(starredRepos)
	sp.Stop()

	fmt.Printf("Found %d starred repositories\n", stats.TotalRepos)

	// Get existing repositories from database for incremental sync
	existingRepos, err := s.getExistingRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing repositories: %w", err)
	}

	// Determine sync operations
	operations := s.determineSyncOperations(starredRepos, existingRepos, force)
	
	fmt.Printf("Sync plan: %d new, %d updates, %d removals\n", 
		len(operations.toAdd), len(operations.toUpdate), len(operations.toRemove))

	// Remove unstarred repositories
	if len(operations.toRemove) > 0 {
		if err := s.removeRepositories(ctx, operations.toRemove, stats); err != nil {
			return fmt.Errorf("failed to remove repositories: %w", err)
		}
	}

	// Process repositories in batches
	allToProcess := append(operations.toAdd, operations.toUpdate...)
	if len(allToProcess) > 0 {
		if err := s.processRepositoriesInBatches(ctx, allToProcess, batchSize, stats); err != nil {
			return fmt.Errorf("failed to process repositories: %w", err)
		}
	}

	stats.EndTime = time.Now()
	stats.ProcessingTime = stats.EndTime.Sub(stats.StartTime)

	s.printSyncSummary(stats)
	return nil
}

func (s *SyncService) syncSpecificRepository(ctx context.Context, repoName string, compareModels bool) error {
	s.logVerbose(fmt.Sprintf("Syncing specific repository: %s", repoName))

	// Fetch the specific repository
	starredRepos, err := s.githubClient.GetStarredRepos(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to fetch starred repositories: %w", err)
	}

	var targetRepo *github.Repository
	for _, repo := range starredRepos {
		if repo.FullName == repoName {
			targetRepo = &repo
			break
		}
	}

	if targetRepo == nil {
		return fmt.Errorf("repository %s not found in starred repositories", repoName)
	}

	if compareModels {
		return s.compareModelsForRepository(ctx, *targetRepo)
	}

	return s.processRepository(ctx, *targetRepo, true)
}

func (s *SyncService) compareModelsForRepository(ctx context.Context, repo github.Repository) error {
	fmt.Printf("Comparing LLM models for repository: %s\n", repo.FullName)

	// Extract content first
	content, err := s.processor.ExtractContent(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to extract content: %w", err)
	}

	// Test different LLM providers
	providers := []string{llm.ProviderOpenAI, llm.ProviderAnthropic, llm.ProviderLocal}
	
	for _, provider := range providers {
		fmt.Printf("\n--- Testing %s ---\n", provider)
		
		// Create processor with specific provider
		llmManager := llm.NewManager(llm.DefaultManagerConfig())
		
		// Register and configure the provider
		defaultConfig := llm.Config{Provider: provider, Model: llm.ModelGPT35Turbo}
		if err := llmManager.RegisterProvider(provider, llm.NewClient(defaultConfig)); err != nil {
			fmt.Printf("Skipping %s: failed to register: %v\n", provider, err)
			continue
		}
		
		if llmConfig, exists := s.config.LLM.Providers[provider]; exists {
			if err := llmManager.Configure(llmConfig); err != nil {
				fmt.Printf("Skipping %s: failed to configure: %v\n", provider, err)
				continue
			}
		}

		processorService := processor.NewService(s.githubClient, &llmServiceAdapter{manager: llmManager})
		
		// Process repository
		processed, err := processorService.ProcessRepository(ctx, repo, content)
		if err != nil {
			fmt.Printf("Error with %s: %v\n", provider, err)
			continue
		}

		// Display results
		fmt.Printf("Purpose: %s\n", processed.Summary.Purpose)
		fmt.Printf("Technologies: %v\n", processed.Summary.Technologies)
		fmt.Printf("Features: %v\n", processed.Summary.Features)
	}

	return nil
}

type syncOperations struct {
	toAdd    []github.Repository
	toUpdate []github.Repository
	toRemove []string
}

func (s *SyncService) determineSyncOperations(starredRepos []github.Repository, existingRepos map[string]*storage.StoredRepo, force bool) *syncOperations {
	ops := &syncOperations{}

	// Create map of starred repos for quick lookup
	starredMap := make(map[string]github.Repository)
	for _, repo := range starredRepos {
		starredMap[repo.FullName] = repo
	}

	// Determine additions and updates
	for _, repo := range starredRepos {
		existing, exists := existingRepos[repo.FullName]
		
		if !exists {
			// New repository
			ops.toAdd = append(ops.toAdd, repo)
		} else if force || s.needsUpdate(repo, existing) {
			// Repository needs update
			ops.toUpdate = append(ops.toUpdate, repo)
		}
	}

	// Determine removals (repositories that exist in DB but not in starred)
	for fullName := range existingRepos {
		if _, stillStarred := starredMap[fullName]; !stillStarred {
			ops.toRemove = append(ops.toRemove, fullName)
		}
	}

	return ops
}

func (s *SyncService) needsUpdate(repo github.Repository, existing *storage.StoredRepo) bool {
	// Check if repository was updated since last sync
	return repo.UpdatedAt.After(existing.LastSynced) ||
		repo.StargazersCount != existing.StargazersCount ||
		repo.ForksCount != existing.ForksCount ||
		repo.Size != existing.SizeKB
}

func (s *SyncService) getExistingRepositories(ctx context.Context) (map[string]*storage.StoredRepo, error) {
	existingMap := make(map[string]*storage.StoredRepo)
	
	// Get all repositories from database (paginated)
	limit := 100
	offset := 0
	
	for {
		repos, err := s.storage.ListRepositories(ctx, limit, offset)
		if err != nil {
			return nil, err
		}
		
		if len(repos) == 0 {
			break
		}
		
		for _, repo := range repos {
			repoCopy := repo // Create copy to avoid pointer issues
			existingMap[repo.FullName] = &repoCopy
		}
		
		if len(repos) < limit {
			break
		}
		
		offset += limit
	}
	
	return existingMap, nil
}

func (s *SyncService) removeRepositories(ctx context.Context, toRemove []string, stats *SyncStats) error {
	s.logVerbose(fmt.Sprintf("Removing %d unstarred repositories...", len(toRemove)))

	for _, fullName := range toRemove {
		if err := s.storage.DeleteRepository(ctx, fullName); err != nil {
			s.logVerbose(fmt.Sprintf("Failed to remove %s: %v", fullName, err))
			stats.ErrorRepos++
		} else {
			s.logVerbose(fmt.Sprintf("Removed: %s", fullName))
			stats.RemovedRepos++
		}
	}

	return nil
}

func (s *SyncService) processRepositoriesInBatches(ctx context.Context, repos []github.Repository, batchSize int, stats *SyncStats) error {
	totalBatches := (len(repos) + batchSize - 1) / batchSize
	
	for i := 0; i < len(repos); i += batchSize {
		end := i + batchSize
		if end > len(repos) {
			end = len(repos)
		}
		
		batch := repos[i:end]
		batchNum := (i / batchSize) + 1
		
		fmt.Printf("Processing batch %d/%d (%d repositories)...\n", batchNum, totalBatches, len(batch))
		
		if err := s.processBatch(ctx, batch, stats); err != nil {
			return fmt.Errorf("failed to process batch %d: %w", batchNum, err)
		}
		
		// Small delay between batches to be respectful to APIs
		if batchNum < totalBatches {
			time.Sleep(1 * time.Second)
		}
	}
	
	return nil
}

func (s *SyncService) processBatch(ctx context.Context, batch []github.Repository, stats *SyncStats) error {
	for _, repo := range batch {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err := s.processRepository(ctx, repo, false); err != nil {
			s.logVerbose(fmt.Sprintf("Failed to process %s: %v", repo.FullName, err))
			stats.ErrorRepos++
		} else {
			// Check if this was an update or new repository
			existing, err := s.storage.GetRepository(ctx, repo.FullName)
			if err != nil || existing == nil {
				stats.NewRepos++
			} else {
				stats.UpdatedRepos++
			}
		}
	}
	
	return nil
}

func (s *SyncService) processRepository(ctx context.Context, repo github.Repository, showDetails bool) error {
	if showDetails {
		fmt.Printf("Processing repository: %s\n", repo.FullName)
	} else {
		s.logVerbose(fmt.Sprintf("Processing: %s", repo.FullName))
	}

	// Extract content
	content, err := s.processor.ExtractContent(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to extract content: %w", err)
	}

	if showDetails {
		fmt.Printf("  Extracted %d content files\n", len(content))
	}

	// Process repository
	processed, err := s.processor.ProcessRepository(ctx, repo, content)
	if err != nil {
		return fmt.Errorf("failed to process repository: %w", err)
	}

	if showDetails {
		fmt.Printf("  Generated %d content chunks\n", len(processed.Chunks))
		fmt.Printf("  Content hash: %s\n", processed.ContentHash)
		if processed.Summary.Purpose != "" {
			fmt.Printf("  Purpose: %s\n", processed.Summary.Purpose)
		}
		if len(processed.Summary.Technologies) > 0 {
			fmt.Printf("  Technologies: %v\n", processed.Summary.Technologies)
		}
	}

	// Check if repository already exists
	existing, err := s.storage.GetRepository(ctx, repo.FullName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to check existing repository: %w", err)
	}

	// Store or update repository
	if existing == nil {
		if err := s.storage.StoreRepository(ctx, *processed); err != nil {
			return fmt.Errorf("failed to store repository: %w", err)
		}
		if showDetails {
			fmt.Printf("  Stored new repository\n")
		}
	} else {
		// Update if content has changed OR if repository metadata has changed
		contentChanged := existing.ContentHash != processed.ContentHash
		metadataChanged := existing.StargazersCount != processed.Repository.StargazersCount ||
			existing.ForksCount != processed.Repository.ForksCount ||
			existing.SizeKB != processed.Repository.Size ||
			existing.Description != processed.Repository.Description
		
		if contentChanged || metadataChanged {
			if err := s.storage.UpdateRepository(ctx, *processed); err != nil {
				return fmt.Errorf("failed to update repository: %w", err)
			}
			if showDetails {
				if contentChanged && metadataChanged {
					fmt.Printf("  Updated repository (content and metadata changed)\n")
				} else if contentChanged {
					fmt.Printf("  Updated repository (content changed)\n")
				} else {
					fmt.Printf("  Updated repository (metadata changed)\n")
				}
			}
		} else {
			if showDetails {
				fmt.Printf("  Skipped (no changes)\n")
			}
		}
	}

	return nil
}

func (s *SyncService) printSyncSummary(stats *SyncStats) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("SYNC SUMMARY")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Total repositories: %d\n", stats.TotalRepos)
	fmt.Printf("New repositories: %d\n", stats.NewRepos)
	fmt.Printf("Updated repositories: %d\n", stats.UpdatedRepos)
	fmt.Printf("Removed repositories: %d\n", stats.RemovedRepos)
	fmt.Printf("Skipped repositories: %d\n", stats.SkippedRepos)
	fmt.Printf("Failed repositories: %d\n", stats.ErrorRepos)
	fmt.Printf("Processing time: %v\n", stats.ProcessingTime)
	fmt.Println(strings.Repeat("=", 50))
}

func (s *SyncService) logVerbose(message string) {
	if s.verbose {
		fmt.Printf("[VERBOSE] %s\n", message)
	}
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// llmServiceAdapter adapts the llm.Manager to implement processor.LLMService
type llmServiceAdapter struct {
	manager *llm.Manager
}

func (a *llmServiceAdapter) Summarize(ctx context.Context, prompt string, content string) (*processor.SummaryResponse, error) {
	response, err := a.manager.Summarize(ctx, prompt, content)
	if err != nil {
		return nil, err
	}
	
	// Convert llm.SummaryResponse to processor.SummaryResponse
	return &processor.SummaryResponse{
		Purpose:      response.Purpose,
		Technologies: response.Technologies,
		UseCases:     response.UseCases,
		Features:     response.Features,
		Installation: response.Installation,
		Usage:        response.Usage,
		Confidence:   response.Confidence,
	}, nil
}