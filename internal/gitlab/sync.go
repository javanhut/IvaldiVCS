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
func (rs *RepoSyncer) CloneRepository(ctx context.Context, owner, repo string, depth int, skipHistory bool, includeTags bool) error {
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

	// Check if we should skip history migration (backward compatibility)
	if skipHistory {
		fmt.Println("Skipping history migration, downloading latest snapshot only...")
		return rs.cloneSnapshot(ctx, owner, repo, branch.Commit.ID, project.DefaultBranch)
	}

	// Fetch commit history
	fmt.Printf("Fetching commit history (depth: ")
	if depth == 0 {
		fmt.Printf("full history")
	} else {
		fmt.Printf("%d commits", depth)
	}
	fmt.Println(")...")

	commits, err := rs.client.ListCommits(ctx, owner, repo, project.DefaultBranch, depth)
	if err != nil {
		return fmt.Errorf("failed to fetch commit history: %w", err)
	}

	if len(commits) == 0 {
		return fmt.Errorf("no commits found in repository")
	}

	fmt.Printf("Found %d commits to import\n", len(commits))

	// Import commits in chronological order (reverse the list)
	err = rs.importCommitHistory(ctx, owner, repo, commits)
	if err != nil {
		return fmt.Errorf("failed to import commit history: %w", err)
	}

	// Import tags if requested
	if includeTags {
		fmt.Println("Importing tags and releases...")
		err = rs.importTags(ctx, owner, repo)
		if err != nil {
			fmt.Printf("Warning: failed to import tags: %v\n", err)
		}
	}

	fmt.Printf("Successfully cloned %s/%s with %d commits\n", owner, repo, len(commits))
	return nil
}

// cloneSnapshot downloads only the latest snapshot without history (backward compatibility)
func (rs *RepoSyncer) cloneSnapshot(ctx context.Context, owner, repo, commitID, branchName string) error {
	// Get the tree for the latest commit
	tree, err := rs.client.GetTree(ctx, owner, repo, commitID, true)
	if err != nil {
		return fmt.Errorf("failed to get repository tree: %w", err)
	}

	// Download files concurrently
	err = rs.downloadFiles(ctx, owner, repo, tree, commitID)
	if err != nil {
		return fmt.Errorf("failed to download files: %w", err)
	}

	// Create single initial commit in Ivaldi
	err = rs.createIvaldiCommit(fmt.Sprintf("Import from GitLab: %s/%s", owner, repo))
	if err != nil {
		return fmt.Errorf("failed to create Ivaldi commit: %w", err)
	}

	fmt.Printf("Successfully cloned snapshot from %s/%s\n", owner, repo)
	return nil
}

// gitlabCommitDownloadResult holds the downloaded state for a commit
type gitlabCommitDownloadResult struct {
	commit         *Commit
	tree           []TreeEntry
	workspaceFiles []wsindex.FileMetadata
	err            error
}

// importCommitHistory imports Git commits as Ivaldi commits in chronological order
func (rs *RepoSyncer) importCommitHistory(ctx context.Context, owner, repo string, commits []*Commit) error {
	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	// Initialize MMR
	mmr, err := history.NewPersistentMMR(rs.casStore, rs.ivaldiDir)
	if err != nil {
		mmr = &history.PersistentMMR{MMR: history.NewMMR()}
	}
	defer mmr.Close()

	commitBuilder := commit.NewCommitBuilder(rs.casStore, mmr.MMR)

	totalCommits := len(commits)
	fmt.Printf("\nImporting %d commits with full history...\n", totalCommits)

	// Process commits in batches for parallel downloading
	batchSize := 3
	processedCount := 0

	// Reverse commits to process in chronological order (oldest first)
	for batchStart := len(commits) - 1; batchStart >= 0; batchStart -= batchSize {
		batchEnd := batchStart - batchSize + 1
		if batchEnd < 0 {
			batchEnd = 0
		}

		// Phase 1: Download commits in parallel
		batchCommits := commits[batchEnd : batchStart+1]
		downloadResults := make([]gitlabCommitDownloadResult, len(batchCommits))

		fmt.Printf("\rDownloading commits: %d/%d", processedCount, totalCommits)

		var downloadWg sync.WaitGroup
		semaphore := make(chan struct{}, 3) // Limit concurrent downloads

		for idx, gitCommit := range batchCommits {
			downloadWg.Add(1)
			go func(idx int, gc *Commit) {
				defer downloadWg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				result := gitlabCommitDownloadResult{commit: gc}

				// Download tree
				tree, err := rs.client.GetTree(ctx, owner, repo, gc.ID, true)
				if err != nil {
					result.err = fmt.Errorf("failed to get tree: %w", err)
					downloadResults[idx] = result
					return
				}
				result.tree = tree

				// Download files to main workspace (synchronized per file, not per commit)
				err = rs.downloadFilesQuiet(ctx, owner, repo, tree, gc.ID)
				if err != nil {
					result.err = fmt.Errorf("failed to download files: %w", err)
					downloadResults[idx] = result
					return
				}

				result.workspaceFiles = nil // Will scan workspace sequentially
				downloadResults[idx] = result
			}(idx, gitCommit)
		}

		downloadWg.Wait()

		// Phase 2: Create commits sequentially (preserves parent relationships)
		for _, result := range downloadResults {
			if result.err != nil {
				return fmt.Errorf("failed to process commit %s: %w", result.commit.ID, result.err)
			}

			gitCommit := result.commit
			processedCount++

			// Show progress
			fmt.Printf("\rCreating commits: %d/%d", processedCount, totalCommits)

			// Scan workspace sequentially to avoid race conditions
			materializer := workspace.NewMaterializer(rs.casStore, rs.ivaldiDir, rs.workDir)
			wsIndex, err := materializer.ScanWorkspace()
			if err != nil {
				return fmt.Errorf("failed to scan workspace: %w", err)
			}

			wsLoader := wsindex.NewLoader(rs.casStore)
			workspaceFiles, err := wsLoader.ListAll(wsIndex)
			if err != nil {
				return fmt.Errorf("failed to list workspace files: %w", err)
			}

			// Determine parent commits
			var parents []cas.Hash
			for _, parentID := range gitCommit.ParentIDs {
				ivaldiParentHash, err := refsManager.GetGitMapping(parentID)
				if err == nil {
					parents = append(parents, ivaldiParentHash)
				}
			}

			// Create author/committer strings
			author := fmt.Sprintf("%s <%s>", gitCommit.AuthorName, gitCommit.AuthorEmail)
			committer := fmt.Sprintf("%s <%s>", gitCommit.CommitterName, gitCommit.CommitterEmail)

			// Create Ivaldi commit with preserved metadata
			commitObj, err := commitBuilder.CreateCommitWithTime(
				workspaceFiles,
				parents,
				author,
				committer,
				gitCommit.Message,
				gitCommit.AuthoredDate,
				gitCommit.CommittedDate,
			)
			if err != nil {
				return fmt.Errorf("failed to create Ivaldi commit: %w", err)
			}

			// Get commit hash
			commitHash := commitBuilder.GetCommitHash(commitObj)

			// Store Git SHA1 → Ivaldi BLAKE3 mapping
			err = refsManager.PutGitMapping(gitCommit.ID, commitHash)
			if err != nil {
				fmt.Printf("\nWarning: failed to store Git mapping for %s: %v\n", gitCommit.ID, err)
			}

			// Update timeline with this commit
			var hashArray [32]byte
			copy(hashArray[:], commitHash[:])

			currentTimeline, err := refsManager.GetCurrentTimeline()
			if err != nil {
				currentTimeline = "main"
			}

			err = refsManager.UpdateTimeline(
				currentTimeline,
				refs.LocalTimeline,
				hashArray,
				[32]byte{},
				gitCommit.ID,
			)
			if err != nil {
				return fmt.Errorf("failed to update timeline: %w", err)
			}
		}
	}

	fmt.Printf("\rSuccessfully imported %d commits\n\n", totalCommits)
	return nil
}

// downloadFilesQuiet downloads files without progress output
func (rs *RepoSyncer) downloadFilesQuiet(ctx context.Context, owner, repo string, tree []TreeEntry, ref string) error {
	var filesToDownload []TreeEntry
	for _, entry := range tree {
		if entry.Type == "blob" {
			localPath := filepath.Join(rs.workDir, entry.Path)
			if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
				continue
			}
			filesToDownload = append(filesToDownload, entry)
		}
	}

	if len(filesToDownload) == 0 {
		return nil
	}

	workers := 8
	if len(filesToDownload) > 100 {
		workers = 16
	}
	if len(filesToDownload) > 500 {
		workers = 32
	}

	jobs := make(chan TreeEntry, len(filesToDownload))
	errors := make(chan error, len(filesToDownload))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				if err := rs.downloadFile(ctx, owner, repo, entry, ref); err != nil {
					errors <- fmt.Errorf("failed to download %s: %w", entry.Path, err)
				}
			}
		}()
	}

	for _, entry := range filesToDownload {
		jobs <- entry
	}
	close(jobs)

	wg.Wait()
	close(errors)

	var downloadErrors []error
	for err := range errors {
		downloadErrors = append(downloadErrors, err)
	}

	if len(downloadErrors) > 0 {
		return fmt.Errorf("failed to download %d files", len(downloadErrors))
	}

	return nil
}

// importTags imports tags and releases from GitLab as Ivaldi references
func (rs *RepoSyncer) importTags(ctx context.Context, owner, repo string) error {
	tags, err := rs.client.ListTags(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}

	if len(tags) == 0 {
		fmt.Println("No tags found")
		return nil
	}

	fmt.Printf("Found %d tags\n", len(tags))

	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	importedCount := 0
	for _, tag := range tags {
		// Get the Ivaldi commit hash for this Git commit
		ivaldiHash, err := refsManager.GetGitMapping(tag.Commit.ID)
		if err != nil {
			fmt.Printf("Warning: tag '%s' points to commit %s which was not imported, skipping\n", tag.Name, tag.Commit.ID[:7])
			continue
		}

		// Create tag reference in Ivaldi
		var hashArray [32]byte
		copy(hashArray[:], ivaldiHash[:])

		err = refsManager.CreateTimeline(
			"tags/"+tag.Name,
			refs.LocalTimeline,
			hashArray,
			[32]byte{},
			tag.Commit.ID,
			fmt.Sprintf("Tag: %s", tag.Name),
		)
		if err != nil {
			fmt.Printf("Warning: failed to create tag '%s': %v\n", tag.Name, err)
			continue
		}

		importedCount++
		fmt.Printf("Imported tag: %s\n", tag.Name)
	}

	fmt.Printf("Successfully imported %d/%d tags\n", importedCount, len(tags))
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
