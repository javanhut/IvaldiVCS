package github

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
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

// RepoSyncer handles syncing between GitHub and Ivaldi
type RepoSyncer struct {
	client    *Client
	ivaldiDir string
	workDir   string
	casStore  cas.CAS
}

// NewRepoSyncer creates a new repository syncer
func NewRepoSyncer(ivaldiDir, workDir string) (*RepoSyncer, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
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

// CloneRepository clones a GitHub repository without using Git
func (rs *RepoSyncer) CloneRepository(ctx context.Context, owner, repo string) error {
	fmt.Printf("Cloning %s/%s from GitHub...\n", owner, repo)

	// Check rate limits
	rs.client.WaitForRateLimit()

	// Get repository info
	repoInfo, err := rs.client.GetRepository(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	fmt.Printf("Repository: %s\n", repoInfo.FullName)
	if repoInfo.Description != "" {
		fmt.Printf("Description: %s\n", repoInfo.Description)
	}
	fmt.Printf("Default branch: %s\n", repoInfo.DefaultBranch)

	// Get the default branch
	branch, err := rs.client.GetBranch(ctx, owner, repo, repoInfo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("failed to get branch info: %w", err)
	}

	// Get the tree for the latest commit
	tree, err := rs.client.GetTree(ctx, owner, repo, branch.Commit.SHA, true)
	if err != nil {
		return fmt.Errorf("failed to get repository tree: %w", err)
	}

	// Download files concurrently
	err = rs.downloadFiles(ctx, owner, repo, tree, branch.Commit.SHA)
	if err != nil {
		return fmt.Errorf("failed to download files: %w", err)
	}

	// Create initial commit in Ivaldi
	err = rs.createIvaldiCommit(fmt.Sprintf("Import from GitHub: %s/%s", owner, repo))
	if err != nil {
		return fmt.Errorf("failed to create Ivaldi commit: %w", err)
	}

	fmt.Printf("Successfully cloned %s/%s\n", owner, repo)
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
		"github-import",
		"github-import",
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

	return nil
}

// PullChanges pulls latest changes from GitHub
func (rs *RepoSyncer) PullChanges(ctx context.Context, owner, repo, branch string) error {
	fmt.Printf("Pulling changes from %s/%s...\n", owner, repo)

	// Get latest commit SHA
	branchInfo, err := rs.client.GetBranch(ctx, owner, repo, branch)
	if err != nil {
		return fmt.Errorf("failed to get branch info: %w", err)
	}

	// TODO: Compare with local state and download only changed files
	// For now, we'll download the entire tree
	tree, err := rs.client.GetTree(ctx, owner, repo, branchInfo.Commit.SHA, true)
	if err != nil {
		return fmt.Errorf("failed to get tree: %w", err)
	}

	// Download changed files
	err = rs.downloadFiles(ctx, owner, repo, tree, branchInfo.Commit.SHA)
	if err != nil {
		return fmt.Errorf("failed to download files: %w", err)
	}

	// Create new commit
	err = rs.createIvaldiCommit(fmt.Sprintf("Pull from GitHub: %s", branchInfo.Commit.SHA[:7]))
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	fmt.Println("Successfully pulled changes")
	return nil
}

// FileChange represents a change to a file
type FileChange struct {
	Path    string
	Content []byte
	Mode    string
	Type    string // "added", "modified", "deleted"
}

// computeFileDeltas compares two commits and returns changed files
func (rs *RepoSyncer) computeFileDeltas(parentHash, currentHash cas.Hash) ([]FileChange, error) {
	commitReader := commit.NewCommitReader(rs.casStore)

	// Read parent commit and tree
	var parentFiles map[string]cas.Hash
	if parentHash != (cas.Hash{}) {
		parentCommit, err := commitReader.ReadCommit(parentHash)
		if err != nil {
			return nil, fmt.Errorf("failed to read parent commit: %w", err)
		}

		parentTree, err := commitReader.ReadTree(parentCommit)
		if err != nil {
			return nil, fmt.Errorf("failed to read parent tree: %w", err)
		}

		parentFileList, err := commitReader.ListFiles(parentTree)
		if err != nil {
			return nil, fmt.Errorf("failed to list parent files: %w", err)
		}

		parentFiles = make(map[string]cas.Hash)
		for _, filePath := range parentFileList {
			content, err := commitReader.GetFileContent(parentTree, filePath)
			if err != nil {
				continue
			}
			parentFiles[filePath] = cas.SumB3(content)
		}
	} else {
		parentFiles = make(map[string]cas.Hash)
	}

	// Read current commit and tree
	currentCommit, err := commitReader.ReadCommit(currentHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read current commit: %w", err)
	}

	currentTree, err := commitReader.ReadTree(currentCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to read current tree: %w", err)
	}

	currentFileList, err := commitReader.ListFiles(currentTree)
	if err != nil {
		return nil, fmt.Errorf("failed to list current files: %w", err)
	}

	// Build map of current files
	currentFiles := make(map[string][]byte)
	for _, filePath := range currentFileList {
		content, err := commitReader.GetFileContent(currentTree, filePath)
		if err != nil {
			continue
		}
		currentFiles[filePath] = content
	}

	// Compute deltas
	var changes []FileChange

	// Check for added and modified files
	for filePath, content := range currentFiles {
		currentHash := cas.SumB3(content)
		parentHash, existed := parentFiles[filePath]

		mode := "100644" // regular file
		if len(content) > 0 && content[0] == '#' && bytes.Contains(content[:min(100, len(content))], []byte("!/")) {
			mode = "100755"
		}

		if !existed {
			// File added
			changes = append(changes, FileChange{
				Path:    filePath,
				Content: content,
				Mode:    mode,
				Type:    "added",
			})
		} else if currentHash != parentHash {
			// File modified
			changes = append(changes, FileChange{
				Path:    filePath,
				Content: content,
				Mode:    mode,
				Type:    "modified",
			})
		}
		// If hashes match, file unchanged - skip
	}

	// Check for deleted files
	for filePath := range parentFiles {
		if _, exists := currentFiles[filePath]; !exists {
			changes = append(changes, FileChange{
				Path: filePath,
				Type: "deleted",
			})
		}
	}

	return changes, nil
}

// blobUploadJob represents a blob upload job
type blobUploadJob struct {
	path    string
	content []byte
	mode    string
}

// blobUploadResult represents the result of a blob upload
type blobUploadResult struct {
	path  string
	mode  string
	sha   string
	err   error
}

// createBlobsParallel uploads blobs in parallel
func (rs *RepoSyncer) createBlobsParallel(ctx context.Context, owner, repo string, changes []FileChange) ([]GitTreeEntry, error) {
	// Filter out deletions
	var filesToUpload []FileChange
	for _, change := range changes {
		if change.Type != "deleted" {
			filesToUpload = append(filesToUpload, change)
		}
	}

	if len(filesToUpload) == 0 {
		return nil, nil
	}

	// Determine worker count
	workers := 8
	if len(filesToUpload) > 50 {
		workers = 16
	}
	if len(filesToUpload) > 200 {
		workers = 32
	}

	jobs := make(chan blobUploadJob, len(filesToUpload))
	results := make(chan blobUploadResult, len(filesToUpload))

	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				blob, err := rs.client.CreateBlob(ctx, owner, repo, job.content)
				if err != nil {
					results <- blobUploadResult{
						path: job.path,
						err:  err,
					}
				} else {
					results <- blobUploadResult{
						path: job.path,
						mode: job.mode,
						sha:  blob.SHA,
						err:  nil,
					}
				}
			}
		}()
	}

	// Submit jobs
	for _, change := range filesToUpload {
		jobs <- blobUploadJob{
			path:    change.Path,
			content: change.Content,
			mode:    change.Mode,
		}
	}
	close(jobs)

	// Wait for completion
	wg.Wait()
	close(results)

	// Collect results
	var treeEntries []GitTreeEntry
	var errors []error

	for result := range results {
		if result.err != nil {
			errors = append(errors, fmt.Errorf("failed to upload %s: %w", result.path, result.err))
		} else {
			treeEntries = append(treeEntries, GitTreeEntry{
				Path: result.path,
				Mode: result.mode,
				Type: "blob",
				SHA:  result.sha,
			})
			fmt.Printf("Uploaded: %s\n", result.path)
		}
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("failed to upload %d files: %v", len(errors), errors[0])
	}

	// Add deletions as tree entries with nil SHA
	for _, change := range changes {
		if change.Type == "deleted" {
			treeEntries = append(treeEntries, GitTreeEntry{
				Path: change.Path,
				Mode: "100644",
				Type: "blob",
				SHA:  "", // Empty SHA means delete
			})
		}
	}

	return treeEntries, nil
}

// UploadFile uploads a file to GitHub
func (rs *RepoSyncer) UploadFile(ctx context.Context, owner, repo, path, branch, message string) error {
	// Read file content
	localPath := filepath.Join(rs.workDir, path)
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Create upload request
	uploadReq := FileUploadRequest{
		Message: message,
		Content: base64.StdEncoding.EncodeToString(content),
		Branch:  branch,
	}

	// Check if file exists to get SHA for update
	existing, err := rs.client.GetFileContent(ctx, owner, repo, path, branch)
	if err == nil && existing != nil {
		uploadReq.SHA = existing.SHA
	}

	// Upload file
	err = rs.client.UploadFile(ctx, owner, repo, path, uploadReq)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	fmt.Printf("Uploaded: %s\n", path)
	return nil
}

// PushCommit pushes an Ivaldi commit to GitHub as a single commit with delta optimization
func (rs *RepoSyncer) PushCommit(ctx context.Context, owner, repo, branch string, commitHash cas.Hash) error {
	fmt.Printf("Pushing commit %s to GitHub...\n", commitHash.String()[:8])

	// Check if branch exists on GitHub
	branchInfo, err := rs.client.GetBranch(ctx, owner, repo, branch)
	var parentSHA string
	var parentTreeSHA string
	var isNewBranch bool

	if err != nil {
		// Branch doesn't exist
		fmt.Printf("Branch '%s' doesn't exist on GitHub, creating it...\n", branch)

		// Try to get repository info to find default branch
		repoInfo, err := rs.client.GetRepository(ctx, owner, repo)
		if err != nil {
			return fmt.Errorf("failed to get repository info: %w", err)
		}

		// Try to get default branch info to get its SHA
		// This may fail if the repository is completely empty
		defaultBranch, err := rs.client.GetBranch(ctx, owner, repo, repoInfo.DefaultBranch)
		if err != nil {
			// Repository is empty (no branches yet), we'll create the first commit without a parent
			fmt.Printf("Repository is empty, creating initial branch '%s'\n", branch)
			parentSHA = ""
			isNewBranch = true
		} else {
			// Repository has commits, create new branch from default branch
			err = rs.client.CreateBranch(ctx, owner, repo, branch, defaultBranch.Commit.SHA)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			fmt.Printf("Created branch '%s' from '%s'\n", branch, repoInfo.DefaultBranch)
			parentSHA = defaultBranch.Commit.SHA
			isNewBranch = true
		}
	} else {
		parentSHA = branchInfo.Commit.SHA
		isNewBranch = false
	}

	// Get parent tree SHA from GitHub for delta optimization
	if parentSHA != "" && !isNewBranch {
		// Fetch the parent commit to get its tree SHA
		commit, err := rs.client.GetCommit(ctx, owner, repo, parentSHA)
		if err == nil && commit != nil {
			parentTreeSHA = commit.TreeSHA
		}
	}

	// Read current commit
	commitReader := commit.NewCommitReader(rs.casStore)
	commitObj, err := commitReader.ReadCommit(commitHash)
	if err != nil {
		return fmt.Errorf("failed to read commit: %w", err)
	}

	// Determine if we should use delta upload
	var treeEntries []GitTreeEntry
	var useDeltaUpload bool

	// Try to get parent commit hash from Ivaldi
	var parentCommitHash cas.Hash
	if len(commitObj.Parents) > 0 {
		parentCommitHash = commitObj.Parents[0]
	}

	// Use delta upload if we have both a parent commit and parent tree on GitHub
	useDeltaUpload = parentTreeSHA != "" && parentCommitHash != (cas.Hash{})

	if useDeltaUpload {
		// Compute file deltas
		changes, err := rs.computeFileDeltas(parentCommitHash, commitHash)
		if err != nil {
			fmt.Printf("Warning: failed to compute deltas, falling back to full upload: %v\n", err)
			useDeltaUpload = false
		} else if len(changes) == 0 {
			fmt.Printf("No file changes detected\n")
			return nil
		} else {
			fmt.Printf("Delta upload: %d file(s) changed\n", len(changes))

			// Upload blobs in parallel for changed files only
			treeEntries, err = rs.createBlobsParallel(ctx, owner, repo, changes)
			if err != nil {
				return fmt.Errorf("failed to create blobs: %w", err)
			}
		}
	}

	// Fallback to full upload if delta upload is not available
	if !useDeltaUpload {
		// Read tree
		tree, err := commitReader.ReadTree(commitObj)
		if err != nil {
			return fmt.Errorf("failed to read tree: %w", err)
		}

		// List all files
		files, err := commitReader.ListFiles(tree)
		if err != nil {
			return fmt.Errorf("failed to list files: %w", err)
		}

		// Special case: empty repository requires using Contents API for first commit
		if parentSHA == "" {
			fmt.Printf("Initial upload to empty repository: uploading %d files using Contents API\n", len(files))

			// Upload files using Contents API (creates commits automatically)
			for _, filePath := range files {
				content, err := commitReader.GetFileContent(tree, filePath)
				if err != nil {
					return fmt.Errorf("failed to get content for %s: %w", filePath, err)
				}

				// Create upload request directly with content (don't use UploadFile helper)
				uploadReq := FileUploadRequest{
					Message: commitObj.Message,
					Content: base64.StdEncoding.EncodeToString(content),
					Branch:  branch,
				}

				// Upload file using Contents API
				err = rs.client.UploadFile(ctx, owner, repo, filePath, uploadReq)
				if err != nil {
					return fmt.Errorf("failed to upload %s: %w", filePath, err)
				}

				fmt.Printf("Uploaded: %s\n", filePath)
			}

			fmt.Printf("Successfully uploaded %d files to empty repository\n", len(files))

			// Get the branch to find the commit SHA created by Contents API
			branchInfo, err := rs.client.GetBranch(ctx, owner, repo, branch)
			if err != nil {
				fmt.Printf("Warning: could not get branch info after upload: %v\n", err)
				return nil
			}

			// Store GitHub commit SHA in timeline
			err = rs.updateTimelineWithGitHubSHA(branch, commitHash, branchInfo.Commit.SHA)
			if err != nil {
				fmt.Printf("Warning: failed to update timeline with GitHub SHA: %v\n", err)
			}

			return nil
		}

		// Regular full upload using Git Data API
		fmt.Printf("Full upload: uploading all files\n")

		// Build change list for all files
		var allChanges []FileChange
		for _, filePath := range files {
			content, err := commitReader.GetFileContent(tree, filePath)
			if err != nil {
				return fmt.Errorf("failed to get content for %s: %w", filePath, err)
			}

			mode := "100644" // regular file
			if len(content) > 0 && content[0] == '#' && bytes.Contains(content[:min(100, len(content))], []byte("!/")) {
				mode = "100755"
			}

			allChanges = append(allChanges, FileChange{
				Path:    filePath,
				Content: content,
				Mode:    mode,
				Type:    "added",
			})
		}

		// Upload all files in parallel
		treeEntries, err = rs.createBlobsParallel(ctx, owner, repo, allChanges)
		if err != nil {
			return fmt.Errorf("failed to create blobs: %w", err)
		}
	}

	// Create tree on GitHub
	treeReq := CreateTreeRequest{
		Tree: treeEntries,
	}

	// Use base_tree for delta uploads
	if useDeltaUpload && parentTreeSHA != "" {
		treeReq.BaseTree = parentTreeSHA
		fmt.Printf("Using base tree %s for delta upload\n", parentTreeSHA[:7])
	}

	treeResp, err := rs.client.CreateTree(ctx, owner, repo, treeReq)
	if err != nil {
		return fmt.Errorf("failed to create tree: %w", err)
	}

	// Create commit on GitHub
	var parents []string
	if parentSHA != "" {
		parents = []string{parentSHA}
	}

	commitReq := CreateCommitRequest{
		Message: commitObj.Message,
		Tree:    treeResp.SHA,
		Parents: parents,
	}
	commitResp, err := rs.client.CreateGitCommit(ctx, owner, repo, commitReq)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	// Create or update branch reference to point to new commit
	if parentSHA == "" {
		// Empty repository - create the branch reference
		err = rs.client.CreateBranch(ctx, owner, repo, branch, commitResp.SHA)
		if err != nil {
			return fmt.Errorf("failed to create branch reference: %w", err)
		}
		fmt.Printf("Created branch '%s' with initial commit\n", branch)
	} else {
		// Update existing branch reference
		updateReq := UpdateRefRequest{
			SHA: commitResp.SHA,
		}
		err = rs.client.UpdateRef(ctx, owner, repo, fmt.Sprintf("heads/%s", branch), updateReq)
		if err != nil {
			return fmt.Errorf("failed to update branch: %w", err)
		}
	}

	fmt.Printf("Successfully pushed commit %s to GitHub\n", commitResp.SHA[:7])

	// Store GitHub commit SHA in timeline for future delta uploads
	err = rs.updateTimelineWithGitHubSHA(branch, commitHash, commitResp.SHA)
	if err != nil {
		// Non-fatal: log but don't fail the push
		fmt.Printf("Warning: failed to update timeline with GitHub SHA: %v\n", err)
	}

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// updateTimelineWithGitHubSHA updates the timeline with the GitHub commit SHA
func (rs *RepoSyncer) updateTimelineWithGitHubSHA(branch string, ivaldiCommitHash cas.Hash, githubCommitSHA string) error {
	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	// Get the timeline
	timeline, err := refsManager.GetTimeline(branch, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get timeline: %w", err)
	}

	// Verify the timeline's commit hash matches what we just pushed
	var timelineHash cas.Hash
	copy(timelineHash[:], timeline.Blake3Hash[:])
	if timelineHash != ivaldiCommitHash {
		return fmt.Errorf("timeline commit mismatch: expected %s, got %s",
			ivaldiCommitHash.String()[:8], timelineHash.String()[:8])
	}

	// Update timeline with GitHub SHA
	var blake3Hash [32]byte
	copy(blake3Hash[:], ivaldiCommitHash[:])

	err = refsManager.UpdateTimeline(
		branch,
		refs.LocalTimeline,
		blake3Hash,
		timeline.SHA256Hash,
		githubCommitSHA,
	)
	if err != nil {
		return fmt.Errorf("failed to update timeline: %w", err)
	}

	return nil
}

// GetRemoteTimelines fetches all branches from GitHub and creates remote timeline references
func (rs *RepoSyncer) GetRemoteTimelines(ctx context.Context, owner, repo string) ([]*Branch, error) {
	branches, err := rs.client.ListBranches(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	// Update refs with remote timeline information
	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	for _, branch := range branches {
		// Create or update remote timeline reference
		description := fmt.Sprintf("Remote branch from %s/%s (SHA: %s)", owner, repo, branch.Commit.SHA[:7])
		err = refsManager.CreateRemoteTimeline(branch.Name, branch.Commit.SHA, description)
		if err != nil {
			// Timeline might already exist, that's okay
			continue
		}
	}

	return branches, nil
}

// TimelineDelta represents changes between local and remote timelines
type TimelineDelta struct {
	AddedFiles    []string
	ModifiedFiles []string
	DeletedFiles  []string
	NoChanges     bool
}

// SyncTimeline performs an incremental sync of a timeline with remote changes
func (rs *RepoSyncer) SyncTimeline(ctx context.Context, owner, repo, branch string, localCommitHash [32]byte) (*TimelineDelta, error) {
	fmt.Printf("Fetching remote state for branch '%s'...\n", branch)

	// Get remote branch information
	branchInfo, err := rs.client.GetBranch(ctx, owner, repo, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote branch info: %w", err)
	}

	// Check if we already have this remote commit SHA stored
	// If the GitHub SHA matches what we have locally, there are no changes
	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err == nil {
		defer refsManager.Close()
		timeline, err := refsManager.GetTimeline(branch, refs.LocalTimeline)
		if err == nil && timeline.GitSHA1Hash == branchInfo.Commit.SHA {
			// Remote hasn't changed since last sync
			return &TimelineDelta{NoChanges: true}, nil
		}
	}

	// Get the remote tree
	remoteTree, err := rs.client.GetTree(ctx, owner, repo, branchInfo.Commit.SHA, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote tree: %w", err)
	}

	// Build map of remote files
	remoteFiles := make(map[string]string) // path -> SHA
	for _, entry := range remoteTree.Tree {
		if entry.Type == "blob" {
			remoteFiles[entry.Path] = entry.SHA
		}
	}

	// Get local files from commit
	var localFiles map[string][]byte
	if localCommitHash != [32]byte{} {
		// Read local commit to get file list
		commitReader := commit.NewCommitReader(rs.casStore)
		commitObj, err := commitReader.ReadCommit(cas.Hash(localCommitHash))
		if err != nil {
			// If we can't read local commit, treat as empty
			localFiles = make(map[string][]byte)
		} else {
			tree, err := commitReader.ReadTree(commitObj)
			if err != nil {
				localFiles = make(map[string][]byte)
			} else {
				filePaths, err := commitReader.ListFiles(tree)
				if err != nil {
					localFiles = make(map[string][]byte)
				} else {
					localFiles = make(map[string][]byte)
					for _, filePath := range filePaths {
						content, err := commitReader.GetFileContent(tree, filePath)
						if err == nil {
							localFiles[filePath] = content
						}
					}
				}
			}
		}
	} else {
		// No local commit, all remote files are new
		localFiles = make(map[string][]byte)
	}

	// Compute delta
	delta := &TimelineDelta{
		AddedFiles:    []string{},
		ModifiedFiles: []string{},
		DeletedFiles:  []string{},
	}

	// Check for added and modified files
	for remotePath, remoteSHA := range remoteFiles {
		localContent, existsLocally := localFiles[remotePath]
		if !existsLocally {
			// File is new on remote
			delta.AddedFiles = append(delta.AddedFiles, remotePath)
		} else {
			// File exists both locally and remotely - check if content changed
			// Compute Git blob SHA for local content to compare with GitHub SHA
			localGitSHA := computeGitBlobSHA(localContent)

			if localGitSHA != remoteSHA {
				// Content has changed
				delta.ModifiedFiles = append(delta.ModifiedFiles, remotePath)
			}
			// If SHAs match, file is unchanged - don't add to any list
		}
	}

	// Check for deleted files (exist locally but not on remote)
	for localPath := range localFiles {
		if _, existsRemotely := remoteFiles[localPath]; !existsRemotely {
			delta.DeletedFiles = append(delta.DeletedFiles, localPath)
		}
	}

	// If no changes, return early
	if len(delta.AddedFiles) == 0 && len(delta.ModifiedFiles) == 0 && len(delta.DeletedFiles) == 0 {
		delta.NoChanges = true
		return delta, nil
	}

	// Download changed files
	fmt.Printf("Downloading %d changed file(s)...\n",
		len(delta.AddedFiles)+len(delta.ModifiedFiles))

	var filesToDownload []TreeEntry
	for _, path := range delta.AddedFiles {
		if sha, ok := remoteFiles[path]; ok {
			filesToDownload = append(filesToDownload, TreeEntry{
				Path: path,
				SHA:  sha,
				Type: "blob",
			})
		}
	}
	for _, path := range delta.ModifiedFiles {
		if sha, ok := remoteFiles[path]; ok {
			filesToDownload = append(filesToDownload, TreeEntry{
				Path: path,
				SHA:  sha,
				Type: "blob",
			})
		}
	}

	// Use existing download infrastructure
	for _, entry := range filesToDownload {
		if err := rs.downloadFile(ctx, owner, repo, entry, branchInfo.Commit.SHA); err != nil {
			return nil, fmt.Errorf("failed to download %s: %w", entry.Path, err)
		}
	}

	// Handle deletions
	for _, path := range delta.DeletedFiles {
		localPath := filepath.Join(rs.workDir, path)
		if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to delete %s: %v\n", path, err)
		}
	}

	// Create new commit for synced state
	err = rs.createIvaldiCommit(fmt.Sprintf("Sync with remote %s/%s@%s",
		owner, repo, branchInfo.Commit.SHA[:7]))
	if err != nil {
		return nil, fmt.Errorf("failed to create commit after sync: %w", err)
	}

	return delta, nil
}

// FetchTimeline downloads a specific timeline (branch) from GitHub
func (rs *RepoSyncer) FetchTimeline(ctx context.Context, owner, repo, timelineName string) error {
	fmt.Printf("Fetching timeline '%s' from %s/%s...\n", timelineName, owner, repo)

	// Get branch information
	branchInfo, err := rs.client.GetBranch(ctx, owner, repo, timelineName)
	if err != nil {
		return fmt.Errorf("failed to get branch info: %w", err)
	}

	// Get the tree for this branch
	tree, err := rs.client.GetTree(ctx, owner, repo, branchInfo.Commit.SHA, true)
	if err != nil {
		return fmt.Errorf("failed to get tree: %w", err)
	}

	fmt.Printf("Branch SHA: %s, Total files: %d\n", branchInfo.Commit.SHA[:7], len(tree.Tree))

	// TEMPORARY SOLUTION: Create a temporary workspace for this timeline
	// In the future, we should implement proper timeline isolation
	tempDir := filepath.Join(rs.ivaldiDir, "harvest_temp", timelineName)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Save current working directory
	originalWorkDir := rs.workDir

	// Temporarily change workspace to temp directory
	rs.workDir = tempDir

	// Download all files for this timeline to temp directory
	err = rs.downloadFiles(ctx, owner, repo, tree, branchInfo.Commit.SHA)
	if err != nil {
		rs.workDir = originalWorkDir // Restore original workspace
		return fmt.Errorf("failed to download files: %w", err)
	}

	// Create workspace index from temp directory
	materializer := workspace.NewMaterializer(rs.casStore, rs.ivaldiDir, rs.workDir)
	wsIndex, err := materializer.ScanWorkspace()
	if err != nil {
		rs.workDir = originalWorkDir
		return fmt.Errorf("failed to scan workspace: %w", err)
	}

	wsLoader := wsindex.NewLoader(rs.casStore)
	workspaceFiles, err := wsLoader.ListAll(wsIndex)
	if err != nil {
		rs.workDir = originalWorkDir
		return fmt.Errorf("failed to list workspace files: %w", err)
	}

	// Restore original workspace
	rs.workDir = originalWorkDir

	// Create persistent MMR
	mmr, err := history.NewPersistentMMR(rs.casStore, rs.ivaldiDir)
	if err != nil {
		mmr = &history.PersistentMMR{MMR: history.NewMMR()}
	}
	defer mmr.Close()

	// Check if timeline already exists to get parent commit
	var parents []cas.Hash
	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err == nil {
		if existingTimeline, err := refsManager.GetTimeline(timelineName, refs.LocalTimeline); err == nil {
			// Timeline exists, use its current commit as parent
			if existingTimeline.Blake3Hash != [32]byte{} {
				var parentHash cas.Hash
				copy(parentHash[:], existingTimeline.Blake3Hash[:])
				parents = append(parents, parentHash)
			}
		}
	}

	// Create commit for this timeline
	commitBuilder := commit.NewCommitBuilder(rs.casStore, mmr.MMR)
	commitObj, err := commitBuilder.CreateCommit(
		workspaceFiles,
		parents,
		"timeline-harvest",
		"timeline-harvest",
		fmt.Sprintf("Harvested timeline '%s' from GitHub (SHA: %s)", timelineName, branchInfo.Commit.SHA[:7]),
	)
	if err != nil {
		if refsManager != nil {
			refsManager.Close()
		}
		return fmt.Errorf("failed to create commit: %w", err)
	}

	// Get commit hash
	commitHash := commitBuilder.GetCommitHash(commitObj)

	// Reopen refs manager if it was nil
	if refsManager == nil {
		refsManager, err = refs.NewRefsManager(rs.ivaldiDir)
		if err != nil {
			return fmt.Errorf("failed to create refs manager: %w", err)
		}
	}
	defer refsManager.Close()

	// Convert to hash array
	var hashArray [32]byte
	copy(hashArray[:], commitHash[:])

	// Create local timeline
	err = refsManager.CreateTimeline(
		timelineName,
		refs.LocalTimeline,
		hashArray,
		[32]byte{},
		branchInfo.Commit.SHA,
		fmt.Sprintf("Harvested from GitHub: %s/%s", owner, repo),
	)
	if err != nil {
		// Timeline might already exist, update it instead
		err = refsManager.UpdateTimeline(
			timelineName,
			refs.LocalTimeline,
			hashArray,
			[32]byte{},
			branchInfo.Commit.SHA,
		)
		if err != nil {
			return fmt.Errorf("failed to update timeline: %w", err)
		}
	}

	// Also update the remote timeline reference with the harvested content
	err = refsManager.UpdateRemoteTimeline(timelineName, hashArray, [32]byte{}, branchInfo.Commit.SHA)
	if err != nil {
		// Remote timeline might not exist, that's okay
	}

	fmt.Printf("Successfully harvested timeline '%s' (workspace preserved)\n", timelineName)
	return nil
}

// computeGitBlobSHA computes the Git blob SHA-1 hash for content
// Git blob format: "blob <size>\0<content>"
func computeGitBlobSHA(content []byte) string {
	header := fmt.Sprintf("blob %d\x00", len(content))
	fullContent := append([]byte(header), content...)
	hash := sha1.Sum(fullContent)
	return hex.EncodeToString(hash[:])
}
