package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/kyleking/gh-star-search/internal/config"
	"github.com/kyleking/gh-star-search/internal/github"
	"github.com/kyleking/gh-star-search/internal/llm"
	"github.com/kyleking/gh-star-search/internal/processor"
	"github.com/kyleking/gh-star-search/internal/storage"
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
	TotalRepos       int
	NewRepos         int
	UpdatedRepos     int
	RemovedRepos     int
	SkippedRepos     int
	ErrorRepos       int
	ProcessedRepos   int
	StartTime        time.Time
	EndTime          time.Time
	ProcessingTime   time.Duration
	ContentChanges   int
	MetadataChanges  int
	mu               sync.Mutex // Protect concurrent access to stats
}

// ProgressTracker tracks progress during sync operations
type ProgressTracker struct {
	total     int
	processed int
	spinner   *spinner.Spinner
	mu        sync.Mutex
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(total int, message string) *ProgressTracker {
	sp := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	sp.Suffix = fmt.Sprintf(" %s (0/%d)", message, total)
	return &ProgressTracker{
		total:   total,
		spinner: sp,
	}
}

// Start begins the progress tracking
func (p *ProgressTracker) Start() {
	p.spinner.Start()
}

// Update increments the progress counter and updates the display
func (p *ProgressTracker) Update(repoName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.processed++
	p.spinner.Suffix = fmt.Sprintf(" Processing %s (%d/%d)", repoName, p.processed, p.total)
}

// Finish stops the progress tracker and shows completion
func (p *ProgressTracker) Finish(message string) {
	p.spinner.Stop()
	fmt.Printf("✓ %s (%d/%d)\n", message, p.processed, p.total)
}

// Stop stops the progress tracker without showing completion
func (p *ProgressTracker) Stop() {
	p.spinner.Stop()
}

// SafeIncrement safely increments a counter in SyncStats
func (s *SyncStats) SafeIncrement(field string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch field {
	case "new":
		s.NewRepos++
	case "updated":
		s.UpdatedRepos++
	case "removed":
		s.RemovedRepos++
	case "skipped":
		s.SkippedRepos++
	case "error":
		s.ErrorRepos++
	case "processed":
		s.ProcessedRepos++
	case "content_changes":
		s.ContentChanges++
	case "metadata_changes":
		s.MetadataChanges++
	}
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

	// Create progress tracker for fetching repositories
	fetchProgress := NewProgressTracker(1, "Fetching starred repositories")
	fetchProgress.Start()

	// Fetch all starred repositories
	starredRepos, err := s.githubClient.GetStarredRepos(ctx, "")
	if err != nil {
		fetchProgress.Stop()
		return fmt.Errorf("failed to fetch starred repositories: %w", err)
	}

	stats.TotalRepos = len(starredRepos)
	fetchProgress.Finish(fmt.Sprintf("Found %d starred repositories", stats.TotalRepos))

	// Get existing repositories from database for incremental sync
	s.logVerbose("Loading existing repositories from database...")
	existingRepos, err := s.getExistingRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing repositories: %w", err)
	}

	s.logVerbose(fmt.Sprintf("Found %d existing repositories in database", len(existingRepos)))

	// Determine sync operations with enhanced change detection
	operations := s.determineSyncOperations(starredRepos, existingRepos, force)

	fmt.Printf("\nSync Plan:\n")
	fmt.Printf("  New repositories: %d\n", len(operations.toAdd))
	fmt.Printf("  Updated repositories: %d\n", len(operations.toUpdate))
	fmt.Printf("  Removed repositories: %d\n", len(operations.toRemove))
	fmt.Printf("  Total to process: %d\n", len(operations.toAdd)+len(operations.toUpdate))

	// Remove unstarred repositories
	if len(operations.toRemove) > 0 {
		if err := s.removeRepositories(ctx, operations.toRemove, stats); err != nil {
			return fmt.Errorf("failed to remove repositories: %w", err)
		}
	}

	// Process repositories in batches with enhanced progress tracking
	allToProcess := append(operations.toAdd, operations.toUpdate...)
	if len(allToProcess) > 0 {
		if err := s.processRepositoriesInBatchesWithForce(ctx, allToProcess, batchSize, stats, operations, force); err != nil {
			return fmt.Errorf("failed to process repositories: %w", err)
		}
	} else {
		fmt.Println("\nNo repositories need processing - all up to date!")
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
	ops := &syncOperations{
		toAdd:    make([]github.Repository, 0),
		toUpdate: make([]github.Repository, 0),
		toRemove: make([]string, 0),
	}

	// Create map of starred repos for quick lookup
	starredMap := make(map[string]github.Repository)
	for _, repo := range starredRepos {
		starredMap[repo.FullName] = repo
	}

	s.logVerbose("Analyzing repository changes...")

	// Determine additions and updates
	for _, repo := range starredRepos {
		existing, exists := existingRepos[repo.FullName]

		if !exists {
			// New repository
			ops.toAdd = append(ops.toAdd, repo)
			s.logVerbose(fmt.Sprintf("  NEW: %s", repo.FullName))
		} else if force {
			// Force update
			ops.toUpdate = append(ops.toUpdate, repo)
			s.logVerbose(fmt.Sprintf("  FORCE UPDATE: %s", repo.FullName))
		} else if s.needsUpdate(repo, existing) {
			// Repository needs update
			ops.toUpdate = append(ops.toUpdate, repo)
			reason := s.getUpdateReason(repo, existing)
			s.logVerbose(fmt.Sprintf("  UPDATE: %s (%s)", repo.FullName, reason))
		} else {
			s.logVerbose(fmt.Sprintf("  SKIP: %s (up to date)", repo.FullName))
		}
	}

	// Determine removals (repositories that exist in DB but not in starred)
	for fullName := range existingRepos {
		if _, stillStarred := starredMap[fullName]; !stillStarred {
			ops.toRemove = append(ops.toRemove, fullName)
			s.logVerbose(fmt.Sprintf("  REMOVE: %s (no longer starred)", fullName))
		}
	}

	return ops
}

func (s *SyncService) needsUpdate(repo github.Repository, existing *storage.StoredRepo) bool {
	// Check if repository was updated since last sync
	return repo.UpdatedAt.After(existing.LastSynced) ||
		repo.StargazersCount != existing.StargazersCount ||
		repo.ForksCount != existing.ForksCount ||
		repo.Size != existing.SizeKB ||
		repo.Description != existing.Description ||
		repo.Language != existing.Language ||
		!s.topicsEqual(repo.Topics, existing.Topics) ||
		s.licenseChanged(repo.License, existing.LicenseName, existing.LicenseSPDXID)
}

// getUpdateReason returns a human-readable reason why a repository needs updating
func (s *SyncService) getUpdateReason(repo github.Repository, existing *storage.StoredRepo) string {
	reasons := []string{}

	if repo.UpdatedAt.After(existing.LastSynced) {
		reasons = append(reasons, "repository updated")
	}
	if repo.StargazersCount != existing.StargazersCount {
		reasons = append(reasons, fmt.Sprintf("stars: %d → %d", existing.StargazersCount, repo.StargazersCount))
	}
	if repo.ForksCount != existing.ForksCount {
		reasons = append(reasons, fmt.Sprintf("forks: %d → %d", existing.ForksCount, repo.ForksCount))
	}
	if repo.Size != existing.SizeKB {
		reasons = append(reasons, "size changed")
	}
	if repo.Description != existing.Description {
		reasons = append(reasons, "description changed")
	}
	if repo.Language != existing.Language {
		reasons = append(reasons, "language changed")
	}
	if !s.topicsEqual(repo.Topics, existing.Topics) {
		reasons = append(reasons, "topics changed")
	}
	if s.licenseChanged(repo.License, existing.LicenseName, existing.LicenseSPDXID) {
		reasons = append(reasons, "license changed")
	}

	if len(reasons) == 0 {
		return "unknown"
	}

	return strings.Join(reasons, ", ")
}

// topicsEqual compares two topic slices for equality
func (s *SyncService) topicsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps for comparison
	mapA := make(map[string]bool)
	mapB := make(map[string]bool)

	for _, topic := range a {
		mapA[topic] = true
	}
	for _, topic := range b {
		mapB[topic] = true
	}

	// Compare maps
	for topic := range mapA {
		if !mapB[topic] {
			return false
		}
	}

	return true
}

// licenseChanged checks if license information has changed
func (s *SyncService) licenseChanged(newLicense *github.License, existingName, existingSPDX string) bool {
	if newLicense == nil {
		return existingName != "" || existingSPDX != ""
	}

	return newLicense.Name != existingName || newLicense.SPDXID != existingSPDX
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
	if len(toRemove) == 0 {
		return nil
	}

	fmt.Printf("\nRemoving %d unstarred repositories...\n", len(toRemove))

	progress := NewProgressTracker(len(toRemove), "Removing repositories")
	progress.Start()

	for _, fullName := range toRemove {
		select {
		case <-ctx.Done():
			progress.Stop()
			return ctx.Err()
		default:
		}

		progress.Update(fullName)

		if err := s.storage.DeleteRepository(ctx, fullName); err != nil {
			s.logVerbose(fmt.Sprintf("Failed to remove %s: %v", fullName, err))
			stats.SafeIncrement("error")
		} else {
			s.logVerbose(fmt.Sprintf("Removed: %s", fullName))
			stats.SafeIncrement("removed")
		}
	}

	progress.Finish("Removed unstarred repositories")
	return nil
}

func (s *SyncService) processRepositoriesInBatches(ctx context.Context, repos []github.Repository, batchSize int, stats *SyncStats, operations *syncOperations) error {
	return s.processRepositoriesInBatchesWithForce(ctx, repos, batchSize, stats, operations, false)
}

func (s *SyncService) processRepositoriesInBatchesWithForce(ctx context.Context, repos []github.Repository, batchSize int, stats *SyncStats, operations *syncOperations, forceUpdate bool) error {
	if len(repos) == 0 {
		return nil
	}

	totalBatches := (len(repos) + batchSize - 1) / batchSize

	fmt.Printf("\nProcessing %d repositories in %d batches (batch size: %d)...\n", len(repos), totalBatches, batchSize)

	// Create a map to track which repos are new vs updates for better progress reporting
	isNewRepo := make(map[string]bool)
	for _, repo := range operations.toAdd {
		isNewRepo[repo.FullName] = true
	}

	for i := 0; i < len(repos); i += batchSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		end := i + batchSize
		if end > len(repos) {
			end = len(repos)
		}

		batch := repos[i:end]
		batchNum := (i / batchSize) + 1

		fmt.Printf("\n--- Batch %d/%d ---\n", batchNum, totalBatches)

		progress := NewProgressTracker(len(batch), fmt.Sprintf("Processing batch %d/%d", batchNum, totalBatches))
		progress.Start()

		if err := s.processBatch(ctx, batch, stats, progress, isNewRepo, forceUpdate); err != nil {
			progress.Stop()
			return fmt.Errorf("failed to process batch %d: %w", batchNum, err)
		}

		progress.Finish(fmt.Sprintf("Completed batch %d/%d", batchNum, totalBatches))

		// Small delay between batches to be respectful to APIs
		if batchNum < totalBatches {
			s.logVerbose("Waiting between batches...")
			time.Sleep(2 * time.Second)
		}
	}

	return nil
}

func (s *SyncService) processBatch(ctx context.Context, batch []github.Repository, stats *SyncStats, progress *ProgressTracker, isNewRepo map[string]bool, forceUpdate bool) error {
	for _, repo := range batch {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		progress.Update(repo.FullName)

		// Get existing repository to track changes
		existing, _ := s.storage.GetRepository(ctx, repo.FullName)

		result, err := s.processRepositoryWithChangeTrackingAndForce(ctx, repo, existing, false, forceUpdate)
		if err != nil {
			s.logVerbose(fmt.Sprintf("Failed to process %s: %v", repo.FullName, err))
			stats.SafeIncrement("error")
		} else {
			stats.SafeIncrement("processed")

			// Track the type of operation
			if isNewRepo[repo.FullName] {
				stats.SafeIncrement("new")
			} else {
				stats.SafeIncrement("updated")

				// Track what type of changes occurred
				if result.ContentChanged {
					stats.SafeIncrement("content_changes")
				}
				if result.MetadataChanged {
					stats.SafeIncrement("metadata_changes")
				}
			}
		}

		// Small delay between repositories to be respectful to APIs
		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

// ProcessResult tracks what changes were made during processing
type ProcessResult struct {
	ContentChanged  bool
	MetadataChanged bool
	Skipped         bool
}

func (s *SyncService) processRepository(ctx context.Context, repo github.Repository, showDetails bool) error {
	result, err := s.processRepositoryWithChangeTracking(ctx, repo, nil, showDetails)
	if err != nil {
		return err
	}

	_ = result // Ignore result for backward compatibility
	return nil
}

func (s *SyncService) processRepositoryWithChangeTracking(ctx context.Context, repo github.Repository, existing *storage.StoredRepo, showDetails bool) (*ProcessResult, error) {
	return s.processRepositoryWithChangeTrackingAndForce(ctx, repo, existing, showDetails, false)
}

func (s *SyncService) processRepositoryWithChangeTrackingAndForce(ctx context.Context, repo github.Repository, existing *storage.StoredRepo, showDetails bool, forceUpdate bool) (*ProcessResult, error) {
	if showDetails {
		fmt.Printf("Processing repository: %s\n", repo.FullName)
	} else {
		s.logVerbose(fmt.Sprintf("Processing: %s", repo.FullName))
	}

	result := &ProcessResult{}

	// Extract content
	content, err := s.processor.ExtractContent(ctx, repo)
	if err != nil {
		return result, fmt.Errorf("failed to extract content: %w", err)
	}

	if showDetails {
		fmt.Printf("  Extracted %d content files\n", len(content))
	}

	// Process repository
	processed, err := s.processor.ProcessRepository(ctx, repo, content)
	if err != nil {
		return result, fmt.Errorf("failed to process repository: %w", err)
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

	// Get existing repository if not provided
	if existing == nil {
		existing, err = s.storage.GetRepository(ctx, repo.FullName)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return result, fmt.Errorf("failed to check existing repository: %w", err)
		}
	}

	// Store or update repository with detailed change tracking
	if existing == nil {
		if err := s.storage.StoreRepository(ctx, *processed); err != nil {
			return result, fmt.Errorf("failed to store repository: %w", err)
		}
		if showDetails {
			fmt.Printf("  Stored new repository\n")
		}
	} else {
		// Enhanced change detection with content hash comparison
		contentChanged := existing.ContentHash != processed.ContentHash
		metadataChanged := s.hasMetadataChanged(existing, processed)

		if contentChanged || metadataChanged || forceUpdate {
			if err := s.storage.UpdateRepository(ctx, *processed); err != nil {
				return result, fmt.Errorf("failed to update repository: %w", err)
			}

			result.ContentChanged = contentChanged
			result.MetadataChanged = metadataChanged

			if showDetails {
				if forceUpdate && !contentChanged && !metadataChanged {
					fmt.Printf("  Force updated repository (LastSynced timestamp updated)\n")
				} else {
					changes := []string{}
					if contentChanged {
						changes = append(changes, "content")
					}
					if metadataChanged {
						changes = append(changes, "metadata")
					}
					fmt.Printf("  Updated repository (%s changed)\n", strings.Join(changes, " and "))

					if contentChanged {
						fmt.Printf("    Content hash: %s → %s\n", existing.ContentHash[:8], processed.ContentHash[:8])
					}
					if metadataChanged {
						s.logMetadataChanges(existing, processed)
					}
				}
			}
		} else {
			result.Skipped = true
			if showDetails {
				fmt.Printf("  Skipped (no changes detected)\n")
			}
		}
	}

	return result, nil
}

// hasMetadataChanged checks if any metadata fields have changed
func (s *SyncService) hasMetadataChanged(existing *storage.StoredRepo, processed *processor.ProcessedRepo) bool {
	return existing.StargazersCount != processed.Repository.StargazersCount ||
		existing.ForksCount != processed.Repository.ForksCount ||
		existing.SizeKB != processed.Repository.Size ||
		existing.Description != processed.Repository.Description ||
		existing.Language != processed.Repository.Language ||
		!s.topicsEqual(existing.Topics, processed.Repository.Topics) ||
		s.licenseChanged(processed.Repository.License, existing.LicenseName, existing.LicenseSPDXID)
}

// logMetadataChanges logs detailed metadata changes for verbose output
func (s *SyncService) logMetadataChanges(existing *storage.StoredRepo, processed *processor.ProcessedRepo) {
	if existing.StargazersCount != processed.Repository.StargazersCount {
		fmt.Printf("    Stars: %d → %d\n", existing.StargazersCount, processed.Repository.StargazersCount)
	}
	if existing.ForksCount != processed.Repository.ForksCount {
		fmt.Printf("    Forks: %d → %d\n", existing.ForksCount, processed.Repository.ForksCount)
	}
	if existing.SizeKB != processed.Repository.Size {
		fmt.Printf("    Size: %d KB → %d KB\n", existing.SizeKB, processed.Repository.Size)
	}
	if existing.Description != processed.Repository.Description {
		fmt.Printf("    Description changed\n")
	}
	if existing.Language != processed.Repository.Language {
		fmt.Printf("    Language: %s → %s\n", existing.Language, processed.Repository.Language)
	}
}

func (s *SyncService) printSyncSummary(stats *SyncStats) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("SYNC SUMMARY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total starred repositories: %d\n", stats.TotalRepos)
	fmt.Printf("Repositories processed: %d\n", stats.ProcessedRepos)
	fmt.Printf("New repositories added: %d\n", stats.NewRepos)
	fmt.Printf("Repositories updated: %d\n", stats.UpdatedRepos)
	fmt.Printf("Repositories removed: %d\n", stats.RemovedRepos)
	fmt.Printf("Repositories skipped: %d\n", stats.SkippedRepos)
	fmt.Printf("Failed repositories: %d\n", stats.ErrorRepos)

	if stats.UpdatedRepos > 0 {
		fmt.Printf("\nChange Details:\n")
		fmt.Printf("  Content changes: %d\n", stats.ContentChanges)
		fmt.Printf("  Metadata changes: %d\n", stats.MetadataChanges)
	}

	fmt.Printf("\nTiming:\n")
	fmt.Printf("  Total processing time: %v\n", stats.ProcessingTime)
	if stats.ProcessedRepos > 0 {
		avgTime := stats.ProcessingTime / time.Duration(stats.ProcessedRepos)
		fmt.Printf("  Average time per repository: %v\n", avgTime)
	}

	// Success rate
	successRate := float64(stats.ProcessedRepos) / float64(stats.ProcessedRepos+stats.ErrorRepos) * 100
	if stats.ProcessedRepos+stats.ErrorRepos > 0 {
		fmt.Printf("  Success rate: %.1f%%\n", successRate)
	}

	fmt.Println(strings.Repeat("=", 60))

	if stats.ErrorRepos > 0 {
		fmt.Printf("⚠️  %d repositories failed to process. Check logs for details.\n", stats.ErrorRepos)
	} else if stats.ProcessedRepos > 0 {
		fmt.Printf("✅ All repositories processed successfully!\n")
	} else {
		fmt.Printf("ℹ️  No repositories needed processing.\n")
	}
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
