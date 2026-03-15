package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/logging"
	"github.com/javanhut/Ivaldi-vcs/internal/progress"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

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

	// OPTIMIZATION 1: Pre-fetch all unique trees in parallel with caching
	fmt.Printf("Fetching tree data for %d commits...\n", totalCommits)
	treeCache := make(map[string]*Tree)
	var treeMutex sync.Mutex
	var treeWg sync.WaitGroup
	treeSemaphore := make(chan struct{}, 20)

	// Collect unique tree SHAs
	uniqueTrees := make(map[string]bool)
	for _, commit := range commits {
		uniqueTrees[commit.TreeSHA] = true
	}

	treeProgress := progress.NewDownloadBar(len(uniqueTrees), "Fetching trees")

	for treeSHA := range uniqueTrees {
		treeWg.Add(1)
		go func(sha string) {
			defer treeWg.Done()
			treeSemaphore <- struct{}{}
			defer func() { <-treeSemaphore }()

			tree, err := rs.client.GetTree(ctx, owner, repo, sha, true)
			if err == nil {
				treeMutex.Lock()
				treeCache[sha] = tree
				treeMutex.Unlock()
			}
			treeProgress.Increment()
		}(treeSHA)
	}
	treeWg.Wait()
	treeProgress.Finish()

	fmt.Printf("Fetched %d unique trees\n", len(treeCache))

	// OPTIMIZATION 2: Download all files for all commits in parallel upfront
	fmt.Printf("Downloading files...\n")
	allFiles := make(map[string]bool) // Track unique files
	for _, tree := range treeCache {
		for _, entry := range tree.Tree {
			if entry.Type == "blob" {
				allFiles[entry.Path] = true
			}
		}
	}

	// Download all unique files in parallel
	var filesToDownload []TreeEntry
	for _, tree := range treeCache {
		for _, entry := range tree.Tree {
			if entry.Type == "blob" {
				localPath := filepath.Join(rs.workDir, entry.Path)
				if _, err := os.Stat(localPath); os.IsNotExist(err) {
					filesToDownload = append(filesToDownload, entry)
				}
			}
		}
	}

	if len(filesToDownload) > 0 {
		fileProgress := progress.NewDownloadBar(len(filesToDownload), "Downloading files")
		var fileWg sync.WaitGroup
		fileSemaphore := make(chan struct{}, 20) // Increased from 3 to 20

		for _, entry := range filesToDownload {
			fileWg.Add(1)
			go func(e TreeEntry) {
				defer fileWg.Done()
				fileSemaphore <- struct{}{}
				defer func() { <-fileSemaphore }()

				// Use first commit's SHA as ref (doesn't matter which)
				for _, commit := range commits {
					rs.downloadFile(ctx, owner, repo, e, commit.SHA)
					break
				}
				fileProgress.Increment()
			}(entry)
		}
		fileWg.Wait()
		fileProgress.Finish()
	}

	// Create progress bar for commit processing
	progressBar := progress.NewDownloadBar(totalCommits, "Creating commits")
	defer progressBar.Finish()

	// OPTIMIZATION 3: Process commits in chronological order without batching
	// Reverse to oldest first
	for i := len(commits) - 1; i >= 0; i-- {
		gitCommit := commits[i]

		// Update progress bar
		progressBar.Increment()

		// Get tree from cache
		tree, exists := treeCache[gitCommit.TreeSHA]
		if !exists {
			progressBar.Finish()
			return fmt.Errorf("tree %s not found in cache", gitCommit.TreeSHA)
		}

		// OPTIMIZATION 4: Build file list from tree without filesystem scanning
		workspaceFiles := make([]wsindex.FileMetadata, 0, len(tree.Tree))
		for _, entry := range tree.Tree {
			if entry.Type == "blob" {
				filePath := filepath.Join(rs.workDir, entry.Path)
				content, err := os.ReadFile(filePath)
				if err != nil {
					// File might not exist yet, skip it
					continue
				}

				// Store content in CAS
				contentHash := cas.SumB3(content)
				rs.casStore.Put(contentHash, content)

				// Get file info
				fileInfo, err := os.Stat(filePath)
				if err != nil {
					continue
				}

				workspaceFiles = append(workspaceFiles, wsindex.FileMetadata{
					Path:     entry.Path,
					FileRef:  filechunk.NodeRef{Hash: contentHash},
					ModTime:  fileInfo.ModTime(),
					Mode:     uint32(fileInfo.Mode()),
					Size:     int64(len(content)),
					Checksum: contentHash,
				})
			}
		}

		// Determine parent commits
		var parents []cas.Hash
		for _, parentInfo := range gitCommit.Parents {
			ivaldiParentHash, err := refsManager.GetGitMapping(parentInfo.SHA)
			if err == nil {
				parents = append(parents, ivaldiParentHash)
			}
		}

		// Create author/committer strings
		author := fmt.Sprintf("%s <%s>", gitCommit.Author.Name, gitCommit.Author.Email)
		committer := fmt.Sprintf("%s <%s>", gitCommit.Committer.Name, gitCommit.Committer.Email)

		// Create Ivaldi commit with preserved metadata
		commitObj, err := commitBuilder.CreateCommitWithTime(
			workspaceFiles,
			parents,
			author,
			committer,
			gitCommit.Message,
			gitCommit.Author.Date,
			gitCommit.Committer.Date,
		)
		if err != nil {
			progressBar.Finish()
			return fmt.Errorf("failed to create Ivaldi commit: %w", err)
		}

		// Get commit hash
		commitHash := commitBuilder.GetCommitHash(commitObj)

		// Store Git SHA1 → Ivaldi BLAKE3 mapping
		err = refsManager.PutGitMapping(gitCommit.SHA, commitHash)
		if err != nil {
			logging.Warn("Failed to store Git mapping", "commit", gitCommit.SHA, "error", err)
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
			gitCommit.SHA,
		)
		if err != nil {
			progressBar.Finish()
			return fmt.Errorf("failed to update timeline: %w", err)
		}
	}

	progressBar.Finish()
	fmt.Printf("Successfully imported %d commits\n\n", totalCommits)
	return nil
}

// importTags imports tags and releases from GitHub as Ivaldi references
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
		ivaldiHash, err := refsManager.GetGitMapping(tag.CommitSHA)
		if err != nil {
			logging.Warn("Tag points to unimported commit, skipping", "tag", tag.Name, "commit", tag.CommitSHA[:7])
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
			tag.CommitSHA,
			fmt.Sprintf("Tag: %s", tag.Name),
		)
		if err != nil {
			logging.Warn("Failed to create tag", "tag", tag.Name, "error", err)
			continue
		}

		importedCount++
		fmt.Printf("Imported tag: %s\n", tag.Name)
	}

	fmt.Printf("Successfully imported %d/%d tags\n", importedCount, len(tags))
	return nil
}

// downloadFiles downloads all files from a GitHub tree with optimized performance
func (rs *RepoSyncer) downloadFiles(ctx context.Context, owner, repo string, tree *Tree, ref string) error {
	// Filter out files that already exist in CAS
	var filesToDownload []TreeEntry
	totalFiles := 0
	skippedFiles := 0

	for _, entry := range tree.Tree {
		if entry.Type == "blob" {
			totalFiles++
			// Check if we already have this content (by SHA)
			if entry.SHA != "" {
				// For delta downloads, check if file already exists
				// This is a simple optimization - could be enhanced with SHA comparison
				localPath := filepath.Join(rs.workDir, entry.Path)
				if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
					// File exists locally, skip download (could compare SHA for better accuracy)
					skippedFiles++
					continue
				}
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
	progressChan := make(chan int, len(filesToDownload))

	var wg sync.WaitGroup
	var progressWg sync.WaitGroup

	// Create progress bar for file downloads
	downloadBar := progress.NewDownloadBar(len(filesToDownload), "Downloading files")

	// Progress reporter
	progressWg.Add(1)
	go func() {
		defer progressWg.Done()
		for range progressChan {
			downloadBar.Increment()
		}
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
					progressChan <- 1
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
	close(progressChan)
	progressWg.Wait()
	downloadBar.Finish()

	// Check for errors
	var downloadErrors []error
	for err := range errors {
		downloadErrors = append(downloadErrors, err)
	}

	if len(downloadErrors) > 0 {
		logging.Warn("Download errors occurred", "count", len(downloadErrors))
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

// downloadFile downloads a single file from GitHub
func (rs *RepoSyncer) downloadFile(ctx context.Context, owner, repo string, entry TreeEntry, ref string) error {
	// Check rate limits
	if rs.client.IsRateLimited() {
		rs.client.WaitForRateLimit()
	}

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

	// No verbose output per file
	return nil
}

// createIvaldiCommit creates an Ivaldi commit from downloaded files and updates timeline metadata.
func (rs *RepoSyncer) createIvaldiCommit(message, timelineName, gitSHA string) error {
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

	// Initialize refs manager and resolve target timeline
	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	if timelineName == "" {
		timelineName, err = refsManager.GetCurrentTimeline()
		if err != nil {
			timelineName = "main"
		}
	}

	// Use current timeline head as parent so sync/pull operations keep local history connected.
	var parents []cas.Hash
	existingTimeline, err := refsManager.GetTimeline(timelineName, refs.LocalTimeline)
	if err == nil && existingTimeline.Blake3Hash != [32]byte{} {
		var parentHash cas.Hash
		copy(parentHash[:], existingTimeline.Blake3Hash[:])
		parents = append(parents, parentHash)
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
		parents,
		"github-import",
		"github-import",
		message,
	)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	// Get commit hash
	commitHash := commitBuilder.GetCommitHash(commitObj)

	// Update timeline with commit
	var hashArray [32]byte
	copy(hashArray[:], commitHash[:])

	sha256Hash := [32]byte{}
	if existingTimeline != nil {
		sha256Hash = existingTimeline.SHA256Hash
	}

	err = refsManager.UpdateTimeline(
		timelineName,
		refs.LocalTimeline,
		hashArray,
		sha256Hash,
		gitSHA,
	)
	if err != nil {
		return fmt.Errorf("failed to update timeline: %w", err)
	}

	// Keep remote tracking ref aligned when we know the upstream Git commit SHA.
	if gitSHA != "" {
		remoteTimeline, remoteErr := refsManager.GetTimeline(timelineName, refs.RemoteTimeline)
		if remoteErr == nil {
			_ = refsManager.UpdateRemoteTimeline(timelineName, hashArray, remoteTimeline.SHA256Hash, gitSHA)
		} else {
			_ = refsManager.CreateTimeline(
				timelineName,
				refs.RemoteTimeline,
				hashArray,
				[32]byte{},
				gitSHA,
				fmt.Sprintf("Remote branch from sync (%s)", timelineName),
			)
		}
	}

	return nil
}
