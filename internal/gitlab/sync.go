package gitlab

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// RepoSyncer handles syncing between GitLab and Ivaldi
type RepoSyncer struct {
	client    *Client
	ivaldiDir string
	workDir   string
	casStore  cas.CAS
}

// NewRepoSyncer creates a new repository syncer
func NewRepoSyncer(ivaldiDir, workDir string, owner, repo string) (*RepoSyncer, error) {
	return NewRepoSyncerWithURL(ivaldiDir, workDir, owner, repo, "")
}

// NewRepoSyncerWithURL creates a new repository syncer with a custom GitLab URL
func NewRepoSyncerWithURL(ivaldiDir, workDir string, owner, repo, baseURL string) (*RepoSyncer, error) {
	var client *Client
	var err error

	if baseURL != "" {
		client, err = NewClientWithURL(owner, repo, "", baseURL)
	} else {
		client, err = NewClient(owner, repo, "")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Initialize CAS store
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize CAS: %w", err)
	}

	return &RepoSyncer{
		client:    client,
		ivaldiDir: ivaldiDir,
		workDir:   workDir,
		casStore:  casStore,
	}, nil
}

// CloneRepository clones a GitLab repository without using Git
func (rs *RepoSyncer) CloneRepository(ctx context.Context, owner, repo string) error {
	fmt.Printf("Cloning %s/%s from GitLab...\n", owner, repo)

	// Get project info
	project, err := rs.client.GetProject(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get project info: %w", err)
	}

	fmt.Printf("Project: %s\n", project.PathWithNamespace)
	if project.Description != "" {
		fmt.Printf("Description: %s\n", project.Description)
	}
	fmt.Printf("Default branch: %s\n", project.DefaultBranch)

	// Get the default branch
	branch, err := rs.client.GetBranch(ctx, owner, repo, project.DefaultBranch)
	if err != nil {
		return fmt.Errorf("failed to get branch info: %w", err)
	}

	// Get the tree for the latest commit
	tree, err := rs.client.GetTree(ctx, owner, repo, branch.Commit.ID, true)
	if err != nil {
		return fmt.Errorf("failed to get repository tree: %w", err)
	}

	// Download files concurrently
	err = rs.downloadFiles(ctx, owner, repo, tree, branch.Commit.ID)
	if err != nil {
		return fmt.Errorf("failed to download files: %w", err)
	}

	// Create initial commit in Ivaldi
	err = rs.createIvaldiCommit(fmt.Sprintf("Import from GitLab: %s/%s", owner, repo))
	if err != nil {
		return fmt.Errorf("failed to create Ivaldi commit: %w", err)
	}

	fmt.Printf("Successfully cloned %s/%s\n", owner, repo)
	return nil
}

// downloadFiles downloads all files from a GitLab tree with optimized performance
func (rs *RepoSyncer) downloadFiles(ctx context.Context, owner, repo string, tree []TreeEntry, ref string) error {
	// Filter out files that already exist locally
	var filesToDownload []TreeEntry
	totalFiles := 0
	skippedFiles := 0

	for _, entry := range tree {
		if entry.Type == "blob" {
			totalFiles++
			// Check if file already exists locally
			localPath := filepath.Join(rs.workDir, entry.Path)
			if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
				// File exists locally, skip download
				skippedFiles++
				continue
			}
			filesToDownload = append(filesToDownload, entry)
		}
	}

	if len(filesToDownload) == 0 {
		fmt.Printf("All %d files already exist locally, nothing to download\n", totalFiles)
		return nil
	}

	fmt.Printf("Downloading %d files (%d already exist locally)...\n", len(filesToDownload), skippedFiles)

	// Dynamic worker count based on number of files
	workers := 8
	if len(filesToDownload) > 100 {
		workers = 16
	}
	if len(filesToDownload) > 500 {
		workers = 32
	}
	// Cap at 32 to avoid overwhelming the API
	if workers > 32 {
		workers = 32
	}

	jobs := make(chan TreeEntry, len(filesToDownload))
	errors := make(chan error, len(filesToDownload))
	progress := make(chan int, len(filesToDownload))

	var wg sync.WaitGroup
	var progressWg sync.WaitGroup

	// Progress reporter
	progressWg.Add(1)
	go func() {
		defer progressWg.Done()
		downloaded := 0
		for range progress {
			downloaded++
			// Update progress every 10 files or at completion
			if downloaded%10 == 0 || downloaded == len(filesToDownload) {
				percentage := (downloaded * 100) / len(filesToDownload)
				fmt.Printf("\rProgress: %d/%d files (%d%%)...", downloaded, len(filesToDownload), percentage)
			}
		}
		fmt.Println() // New line after progress
	}()

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				if err := rs.downloadFile(ctx, owner, repo, entry, ref); err != nil {
					errors <- fmt.Errorf("failed to download %s: %w", entry.Path, err)
				} else {
					progress <- 1
				}
			}
		}()
	}

	// Submit jobs
	for _, entry := range filesToDownload {
		jobs <- entry
	}
	close(jobs)

	// Wait for completion
	wg.Wait()
	close(errors)
	close(progress)
	progressWg.Wait()

	// Check for errors
	var downloadErrors []error
	for err := range errors {
		downloadErrors = append(downloadErrors, err)
	}

	if len(downloadErrors) > 0 {
		fmt.Printf("\nWarning: %d download errors occurred\n", len(downloadErrors))
		if len(downloadErrors) <= 3 {
			for _, err := range downloadErrors {
				fmt.Printf("  - %v\n", err)
			}
		} else {
			// Show first 3 errors
			for i := 0; i < 3; i++ {
				fmt.Printf("  - %v\n", downloadErrors[i])
			}
			fmt.Printf("  ... and %d more errors\n", len(downloadErrors)-3)
		}
		return fmt.Errorf("failed to download %d files", len(downloadErrors))
	}

	fmt.Printf("Successfully downloaded %d files\n", len(filesToDownload))
	return nil
}

// downloadFile downloads a single file from GitLab
func (rs *RepoSyncer) downloadFile(ctx context.Context, owner, repo string, entry TreeEntry, ref string) error {
	// Download file content
	content, err := rs.client.DownloadFile(ctx, owner, repo, entry.Path, ref)
	if err != nil {
		return err
	}

	// Create local file
	localPath := filepath.Join(rs.workDir, entry.Path)

	// Ensure directory exists
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(localPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Store in CAS for deduplication
	hash := cas.SumB3(content)
	if err := rs.casStore.Put(hash, content); err != nil {
		// Non-fatal, file is already written to disk
	}

	return nil
}

// createIvaldiCommit creates an Ivaldi commit from the downloaded files
func (rs *RepoSyncer) createIvaldiCommit(message string) error {
	// Scan workspace
	materializer := workspace.NewMaterializer(rs.casStore, rs.ivaldiDir, rs.workDir)
	wsIndex, err := materializer.ScanWorkspace()
	if err != nil {
		return fmt.Errorf("failed to scan workspace: %w", err)
	}

	// Get workspace files
	wsLoader := wsindex.NewLoader(rs.casStore)
	workspaceFiles, err := wsLoader.ListAll(wsIndex)
	if err != nil {
		return fmt.Errorf("failed to list workspace files: %w", err)
	}

	// Initialize MMR
	mmr, err := history.NewPersistentMMR(rs.casStore, rs.ivaldiDir)
	if err != nil {
		mmr = &history.PersistentMMR{MMR: history.NewMMR()}
	}
	defer mmr.Close()

	// Create commit
	commitBuilder := commit.NewCommitBuilder(rs.casStore, mmr.MMR)
	commitObj, err := commitBuilder.CreateCommit(
		workspaceFiles,
		nil, // No parent for initial import
		"gitlab-import",
		"gitlab-import",
		message,
	)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	// Get commit hash
	commitHash := commitBuilder.GetCommitHash(commitObj)

	// Update timeline
	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	// Get current timeline or use main
	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		currentTimeline = "main"
	}

	// Update timeline with commit
	var hashArray [32]byte
	copy(hashArray[:], commitHash[:])

	err = refsManager.UpdateTimeline(
		currentTimeline,
		refs.LocalTimeline,
		hashArray,
		[32]byte{},
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to update timeline: %w", err)
	}

	fmt.Printf("Created Ivaldi commit: %x\n", commitHash[:6])
	return nil
}

// PullChanges pulls changes from GitLab
func (rs *RepoSyncer) PullChanges(ctx context.Context, owner, repo, branch string) error {
	fmt.Printf("Pulling changes from %s/%s (branch: %s)...\n", owner, repo, branch)

	// Get the branch info
	branchInfo, err := rs.client.GetBranch(ctx, owner, repo, branch)
	if err != nil {
		return fmt.Errorf("failed to get branch info: %w", err)
	}

	// Get the tree for the latest commit
	tree, err := rs.client.GetTree(ctx, owner, repo, branchInfo.Commit.ID, true)
	if err != nil {
		return fmt.Errorf("failed to get repository tree: %w", err)
	}

	// Download updated files
	err = rs.downloadFiles(ctx, owner, repo, tree, branchInfo.Commit.ID)
	if err != nil {
		return fmt.Errorf("failed to download files: %w", err)
	}

	// Create commit in Ivaldi
	err = rs.createIvaldiCommit(fmt.Sprintf("Pull from GitLab: %s/%s@%s", owner, repo, branch))
	if err != nil {
		return fmt.Errorf("failed to create Ivaldi commit: %w", err)
	}

	fmt.Printf("Successfully pulled changes from %s/%s\n", owner, repo)
	return nil
}

// FetchTimeline fetches a specific branch from GitLab
func (rs *RepoSyncer) FetchTimeline(ctx context.Context, owner, repo, branch string) error {
	return rs.PullChanges(ctx, owner, repo, branch)
}

// SyncTimeline syncs a timeline with GitLab
func (rs *RepoSyncer) SyncTimeline(ctx context.Context, owner, repo, branch string) error {
	return rs.PullChanges(ctx, owner, repo, branch)
}
