package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/logging"
	"github.com/javanhut/Ivaldi-vcs/internal/progress"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

// FileChange represents a change to a file
type FileChange struct {
	Path    string
	Content []byte
	Mode    string
	Type    string // "added", "modified", "deleted"
}

// fileHashJob represents a job to compute file hash
type fileHashJob struct {
	filePath string
	tree     *commit.TreeObject
}

// fileHashResult represents the result of a hash computation
type fileHashResult struct {
	filePath string
	hash     cas.Hash
	content  []byte
	err      error
}

// computeFileDeltas compares two commits and returns changed files using parallel hash computation
func (rs *RepoSyncer) computeFileDeltas(parentHash, currentHash cas.Hash) ([]FileChange, error) {
	fmt.Printf("Computing file changes...\n")
	commitReader := commit.NewCommitReader(rs.casStore)

	// Determine worker count
	workerCount := runtime.NumCPU()
	if workerCount < 4 {
		workerCount = 4
	}

	// Read parent commit and tree
	var parentFiles sync.Map // map[string]cas.Hash
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

		// Compute parent hashes in parallel
		if len(parentFileList) > 0 {
			jobs := make(chan fileHashJob, len(parentFileList))
			results := make(chan fileHashResult, len(parentFileList))

			var wg sync.WaitGroup
			for i := 0; i < workerCount; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for job := range jobs {
						content, err := commitReader.GetFileContent(job.tree, job.filePath)
						if err != nil {
							results <- fileHashResult{filePath: job.filePath, err: err}
							continue
						}
						results <- fileHashResult{
							filePath: job.filePath,
							hash:     cas.SumB3(content),
						}
					}
				}()
			}

			// Submit jobs
			for _, filePath := range parentFileList {
				jobs <- fileHashJob{filePath: filePath, tree: parentTree}
			}
			close(jobs)

			// Wait for workers and close results
			go func() {
				wg.Wait()
				close(results)
			}()

			// Collect results
			for result := range results {
				if result.err == nil {
					parentFiles.Store(result.filePath, result.hash)
				}
			}
		}
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

	// Build map of current files with parallel content reading
	var currentFiles sync.Map // map[string][]byte

	if len(currentFileList) > 0 {
		jobs := make(chan fileHashJob, len(currentFileList))
		results := make(chan fileHashResult, len(currentFileList))

		var wg sync.WaitGroup
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					content, err := commitReader.GetFileContent(job.tree, job.filePath)
					if err != nil {
						results <- fileHashResult{filePath: job.filePath, err: err}
						continue
					}
					results <- fileHashResult{
						filePath: job.filePath,
						hash:     cas.SumB3(content),
						content:  content,
					}
				}
			}()
		}

		// Submit jobs
		for _, filePath := range currentFileList {
			jobs <- fileHashJob{filePath: filePath, tree: currentTree}
		}
		close(jobs)

		// Wait for workers and close results
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect results
		for result := range results {
			if result.err == nil {
				currentFiles.Store(result.filePath, result.content)
			}
		}
	}

	// Compute deltas
	var changes []FileChange
	var changesMu sync.Mutex

	// Check for added and modified files in parallel
	var deltaWg sync.WaitGroup
	deltaJobs := make(chan string, len(currentFileList))

	for i := 0; i < workerCount; i++ {
		deltaWg.Add(1)
		go func() {
			defer deltaWg.Done()
			for filePath := range deltaJobs {
				contentVal, ok := currentFiles.Load(filePath)
				if !ok {
					continue
				}
				content := contentVal.([]byte)
				currHash := cas.SumB3(content)

				mode := "100644" // regular file
				if len(content) > 0 && content[0] == '#' && bytes.Contains(content[:min(100, len(content))], []byte("!/")) {
					mode = "100755"
				}

				parentHashVal, existed := parentFiles.Load(filePath)
				var change *FileChange

				if !existed {
					// File added
					change = &FileChange{
						Path:    filePath,
						Content: content,
						Mode:    mode,
						Type:    "added",
					}
				} else {
					parentH := parentHashVal.(cas.Hash)
					if currHash != parentH {
						// File modified
						change = &FileChange{
							Path:    filePath,
							Content: content,
							Mode:    mode,
							Type:    "modified",
						}
					}
				}

				if change != nil {
					changesMu.Lock()
					changes = append(changes, *change)
					changesMu.Unlock()
				}
			}
		}()
	}

	// Submit delta comparison jobs
	for _, filePath := range currentFileList {
		deltaJobs <- filePath
	}
	close(deltaJobs)
	deltaWg.Wait()

	// Check for deleted files
	parentFiles.Range(func(key, _ any) bool {
		filePath := key.(string)
		if _, exists := currentFiles.Load(filePath); !exists {
			changesMu.Lock()
			changes = append(changes, FileChange{
				Path: filePath,
				Type: "deleted",
			})
			changesMu.Unlock()
		}
		return true
	})

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
	path string
	mode string
	sha  string
	err  error
}

const (
	bootstrapGitDataReadyTimeout = 10 * time.Second
	bootstrapGitDataInitialRetry = 150 * time.Millisecond
	bootstrapGitDataMaxRetry     = 1 * time.Second
)

func changeTypeOrder(changeType string) int {
	switch changeType {
	case "added":
		return 0
	case "modified":
		return 1
	case "deleted":
		return 2
	default:
		return 3
	}
}

func sortFileChanges(changes []FileChange) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path == changes[j].Path {
			return changeTypeOrder(changes[i].Type) < changeTypeOrder(changes[j].Type)
		}
		return changes[i].Path < changes[j].Path
	})
}

func printFileChanges(changes []FileChange) {
	if len(changes) == 0 {
		return
	}

	added := 0
	modified := 0
	deleted := 0
	for _, change := range changes {
		switch change.Type {
		case "added":
			added++
		case "modified":
			modified++
		case "deleted":
			deleted++
		}
	}

	fmt.Printf("Delta detected: %d changed file(s) [%d added, %d modified, %d deleted]\n",
		len(changes), added, modified, deleted)
	fmt.Println("Files to update:")
	for _, change := range changes {
		switch change.Type {
		case "added":
			fmt.Printf("  + %s\n", change.Path)
		case "modified":
			fmt.Printf("  ~ %s\n", change.Path)
		case "deleted":
			fmt.Printf("  - %s\n", change.Path)
		default:
			fmt.Printf("  ? %s\n", change.Path)
		}
	}
}

// computeFileDeltasFromRemoteTree compares the current local commit tree with a remote Git tree SHA.
// This is used when local parent metadata is unavailable but remote parent tree is known.
func (rs *RepoSyncer) computeFileDeltasFromRemoteTree(ctx context.Context, owner, repo, remoteTreeSHA string, currentHash cas.Hash) ([]FileChange, error) {
	remoteTree, err := rs.client.GetTree(ctx, owner, repo, remoteTreeSHA, true)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote tree: %w", err)
	}

	remoteFiles := make(map[string]string)
	for _, entry := range remoteTree.Tree {
		if entry.Type == "blob" {
			remoteFiles[entry.Path] = entry.SHA
		}
	}

	commitReader := commit.NewCommitReader(rs.casStore)
	currentCommit, err := commitReader.ReadCommit(currentHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read current commit: %w", err)
	}

	currentTree, err := commitReader.ReadTree(currentCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to read current commit tree: %w", err)
	}

	currentFiles, err := commitReader.ListFiles(currentTree)
	if err != nil {
		return nil, fmt.Errorf("failed to list current files: %w", err)
	}

	seenCurrent := make(map[string]bool, len(currentFiles))
	changes := make([]FileChange, 0)

	for _, path := range currentFiles {
		seenCurrent[path] = true

		content, err := commitReader.GetFileContent(currentTree, path)
		if err != nil {
			return nil, fmt.Errorf("failed to read current file content for %s: %w", path, err)
		}

		mode := "100644"
		if len(content) > 0 && content[0] == '#' && bytes.Contains(content[:min(100, len(content))], []byte("!/")) {
			mode = "100755"
		}

		remoteSHA, exists := remoteFiles[path]
		if !exists {
			changes = append(changes, FileChange{
				Path:    path,
				Content: content,
				Mode:    mode,
				Type:    "added",
			})
			continue
		}

		localGitSHA := computeGitBlobSHA(content)
		if localGitSHA != remoteSHA {
			changes = append(changes, FileChange{
				Path:    path,
				Content: content,
				Mode:    mode,
				Type:    "modified",
			})
		}
	}

	for path := range remoteFiles {
		if !seenCurrent[path] {
			changes = append(changes, FileChange{
				Path: path,
				Type: "deleted",
			})
		}
	}

	sortFileChanges(changes)
	return changes, nil
}

// createBlobsParallel uploads blobs in parallel
func (rs *RepoSyncer) createBlobsParallel(ctx context.Context, owner, repo string, changes []FileChange) ([]GitTreeEntry, error) {
	sortFileChanges(changes)

	// Split upload and delete operations
	var filesToUpload []FileChange
	var filesToDelete []FileChange
	for _, change := range changes {
		if change.Type == "deleted" {
			filesToDelete = append(filesToDelete, change)
		} else {
			filesToUpload = append(filesToUpload, change)
		}
	}

	if len(filesToUpload) == 0 && len(filesToDelete) == 0 {
		return nil, nil
	}

	fmt.Printf("Applying delta to GitHub: %d upload(s), %d deletion(s)\n", len(filesToUpload), len(filesToDelete))

	// Determine worker count
	workers := 8
	if len(filesToUpload) > 50 {
		workers = 16
	}
	if len(filesToUpload) > 200 {
		workers = 32
	}

	jobs := make(chan blobUploadJob, max(1, len(filesToUpload)))
	results := make(chan blobUploadResult, max(1, len(filesToUpload)))

	var wg sync.WaitGroup

	var uploadBar *progress.Bar
	if len(filesToUpload) > 0 {
		uploadBar = progress.NewUploadBar(len(filesToUpload), "Uploading files")
		defer uploadBar.Finish()
	}

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
				if uploadBar != nil {
					uploadBar.Increment()
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
			sha := result.sha
			treeEntries = append(treeEntries, GitTreeEntry{
				Path: result.path,
				Mode: result.mode,
				Type: "blob",
				SHA:  &sha,
			})
		}
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("failed to upload %d files: %w", len(errors), errors[0])
	}

	// Deletions must be explicit when using base_tree: set sha to null for each removed path.
	for _, change := range filesToDelete {
		treeEntries = append(treeEntries, GitTreeEntry{
			Path: change.Path,
			Mode: "100644",
			Type: "blob",
			SHA:  nil,
		})
	}

	return treeEntries, nil
}

// bootstrapEmptyRepo initializes an empty GitHub repository using Contents API.
// This creates a temporary commit so the Git Data API becomes usable.
// Returns the commit SHA (used only to verify repo is no longer empty).
func (rs *RepoSyncer) bootstrapEmptyRepo(ctx context.Context, owner, repo, branch string) (string, error) {
	// Minimal temporary content - will be replaced by orphan commit
	bootstrapContent := []byte("initializing")

	uploadReq := FileUploadRequest{
		Message: "Initialize repository",
		Content: base64.StdEncoding.EncodeToString(bootstrapContent),
		Branch:  branch,
	}

	uploadResp, err := rs.client.UploadFileWithResponse(ctx, owner, repo, ".ivaldi-bootstrap", uploadReq)
	if err != nil {
		return "", fmt.Errorf("failed to bootstrap: %w", err)
	}

	return uploadResp.Commit.SHA, nil
}

// waitForGitDataReady waits until Git Data API endpoints become usable after bootstrapping
// an empty repository through the Contents API.
func (rs *RepoSyncer) waitForGitDataReady(ctx context.Context, owner, repo, bootstrapCommitSHA string) error {
	if bootstrapCommitSHA == "" {
		return fmt.Errorf("bootstrap commit SHA is empty")
	}

	deadline := time.Now().Add(bootstrapGitDataReadyTimeout)
	delay := bootstrapGitDataInitialRetry
	var lastErr error

	for {
		_, err := rs.client.GetCommit(ctx, owner, repo, bootstrapCommitSHA)
		if err == nil {
			return nil
		}
		lastErr = err

		if time.Now().After(deadline) {
			break
		}

		sleepFor := delay
		remaining := time.Until(deadline)
		if sleepFor > remaining {
			sleepFor = remaining
		}
		if sleepFor <= 0 {
			break
		}

		timer := time.NewTimer(sleepFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("bootstrap readiness wait cancelled: %w", ctx.Err())
		case <-timer.C:
		}

		delay *= 2
		if delay > bootstrapGitDataMaxRetry {
			delay = bootstrapGitDataMaxRetry
		}
	}

	return fmt.Errorf("git data API was not ready after bootstrap commit %s: %w", bootstrapCommitSHA, lastErr)
}

// createGitCommitOnBranch creates a commit with the provided tree and updates the target branch.
func (rs *RepoSyncer) createGitCommitOnBranch(ctx context.Context, owner, repo, branch, message, treeSHA, parentSHA string, force bool) (*CommitResponse, error) {
	var parents []string
	if parentSHA != "" {
		parents = []string{parentSHA}
	}

	commitReq := CreateCommitRequest{
		Message: message,
		Tree:    treeSHA,
		Parents: parents,
	}
	commitResp, err := rs.client.CreateGitCommit(ctx, owner, repo, commitReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit: %w", err)
	}

	if parentSHA == "" {
		err = rs.client.CreateBranch(ctx, owner, repo, branch, commitResp.SHA)
		if err != nil {
			return nil, fmt.Errorf("failed to create branch reference: %w", err)
		}
		fmt.Printf("Created branch '%s' with initial commit\n", branch)
	} else {
		updateReq := UpdateRefRequest{
			SHA:   commitResp.SHA,
			Force: force,
		}
		err = rs.client.UpdateRef(ctx, owner, repo, fmt.Sprintf("heads/%s", branch), updateReq)
		if err != nil {
			return nil, fmt.Errorf("failed to update branch: %w", err)
		}
	}

	return commitResp, nil
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

// PushCommit pushes an Ivaldi commit to GitHub as a single commit with delta optimization.
// localTimelineName is the local timeline whose commit is being pushed.
func (rs *RepoSyncer) PushCommit(ctx context.Context, owner, repo, branch string, commitHash cas.Hash, force bool, localTimelineName string) error {
	if localTimelineName == "" {
		localTimelineName = branch
	}

	if force {
		fmt.Printf("Force pushing commit %s to GitHub...\n", commitHash.String()[:8])
	} else {
		fmt.Printf("Pushing commit %s to GitHub...\n", commitHash.String()[:8])
	}

	// Check if branch exists on GitHub
	branchInfo, err := rs.client.GetBranch(ctx, owner, repo, branch)
	var parentSHA string
	var parentTreeSHA string
	var isNewBranch bool
	bootstrapBranch := branch

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
			if repoInfo.DefaultBranch != "" {
				bootstrapBranch = repoInfo.DefaultBranch
			}
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

	// Check if this exact commit was already pushed to this branch
	if parentSHA != "" && !isNewBranch && !force {
		refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
		if err == nil {
			timeline, err := refsManager.GetTimeline(localTimelineName, refs.LocalTimeline)
			refsManager.Close()
			if err == nil && timeline.GitSHA1Hash == parentSHA {
				fmt.Printf("Already up to date - no new commits to push\n")
				return nil
			}
		}
	}

	// Get parent tree SHA from GitHub for delta optimization
	if parentSHA != "" {
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
	var sameTreeCommit bool

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
			logging.Warn("Failed to compute deltas, falling back to full upload", "error", err)
			useDeltaUpload = false
		} else if len(changes) == 0 {
			fmt.Printf("No file deltas detected against local parent\n")
			sameTreeCommit = true
			useDeltaUpload = false
		} else {
			sortFileChanges(changes)
			printFileChanges(changes)

			// Upload blobs in parallel for changed files only
			treeEntries, err = rs.createBlobsParallel(ctx, owner, repo, changes)
			if err != nil {
				return fmt.Errorf("failed to create blobs: %w", err)
			}
		}
	}

	// Fallback delta strategy: compare current local commit with remote parent tree directly.
	if !useDeltaUpload && parentTreeSHA != "" && parentCommitHash == (cas.Hash{}) {
		changes, err := rs.computeFileDeltasFromRemoteTree(ctx, owner, repo, parentTreeSHA, commitHash)
		if err != nil {
			logging.Warn("Failed remote-tree delta comparison, falling back to full upload", "error", err)
		} else if len(changes) == 0 {
			fmt.Printf("No file deltas detected against remote tree\n")
			sameTreeCommit = true
		} else {
			fmt.Printf("Using remote-tree delta fallback\n")
			printFileChanges(changes)
			treeEntries, err = rs.createBlobsParallel(ctx, owner, repo, changes)
			if err != nil {
				return fmt.Errorf("failed to create blobs: %w", err)
			}
			useDeltaUpload = true
		}
	}

	if sameTreeCommit {
		if parentSHA == "" || parentTreeSHA == "" {
			return fmt.Errorf("cannot create same-tree commit without remote parent context")
		}

		fmt.Printf("No file deltas; creating commit with unchanged tree to preserve seal history\n")
		commitResp, err := rs.createGitCommitOnBranch(ctx, owner, repo, branch, commitObj.Message, parentTreeSHA, parentSHA, force)
		if err != nil {
			return err
		}

		fmt.Printf("Successfully pushed commit %s to GitHub\n", commitResp.SHA[:7])
		err = rs.updateTimelineWithGitHubSHA(localTimelineName, branch, commitHash, commitResp.SHA)
		if err != nil {
			logging.Warn("Failed to update timeline with GitHub SHA", "error", err)
		}
		return nil
	}

	// Fallback to full upload if delta upload is not available
	if !useDeltaUpload {
		if parentSHA != "" && parentTreeSHA == "" {
			fmt.Printf("Delta upload unavailable: could not read parent tree from remote, falling back to full upload\n")
		}
		if parentTreeSHA != "" && parentCommitHash == (cas.Hash{}) {
			fmt.Printf("Delta upload unavailable: local commit has no parent, falling back to full upload\n")
		}

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

		// Empty repository case: bootstrap then use Git Data API
		if parentSHA == "" {
			fmt.Printf("Initial upload to empty repository: %d files\n", len(files))

			// Phase 1: Bootstrap - create temp commit so Git Data API works
			fmt.Printf("Initializing repository (bootstrap branch: '%s')...\n", bootstrapBranch)
			bootstrapCommitSHA, err := rs.bootstrapEmptyRepo(ctx, owner, repo, bootstrapBranch)
			if err != nil {
				return fmt.Errorf("failed to initialize empty repo: %w", err)
			}
			err = rs.waitForGitDataReady(ctx, owner, repo, bootstrapCommitSHA)
			if err != nil {
				return fmt.Errorf("failed waiting for git data API readiness: %w", err)
			}

			// Phase 2: Upload all blobs via Git Data API (now works)
			fmt.Printf("Uploading %d files...\n", len(files))

			var initialChanges []FileChange
			for _, filePath := range files {
				content, err := commitReader.GetFileContent(tree, filePath)
				if err != nil {
					return fmt.Errorf("failed to get content for %s: %w", filePath, err)
				}

				mode := "100644"
				if len(content) > 0 && content[0] == '#' && bytes.Contains(content[:min(100, len(content))], []byte("!/")) {
					mode = "100755"
				}

				initialChanges = append(initialChanges, FileChange{
					Path:    filePath,
					Content: content,
					Mode:    mode,
					Type:    "added",
				})
			}

			initialTreeEntries, err := rs.createBlobsParallel(ctx, owner, repo, initialChanges)
			if err != nil {
				return fmt.Errorf("failed to create blobs: %w", err)
			}

			// Phase 3: Create tree with only user files (no base_tree)
			initialTreeReq := CreateTreeRequest{
				Tree: initialTreeEntries,
			}
			initialTreeResp, err := rs.client.CreateTree(ctx, owner, repo, initialTreeReq)
			if err != nil {
				return fmt.Errorf("failed to create tree: %w", err)
			}

			// Phase 4: Create ORPHAN commit (no parents = initial commit)
			initialCommitReq := CreateCommitRequest{
				Message: commitObj.Message,
				Tree:    initialTreeResp.SHA,
				Parents: []string{}, // Empty = orphan commit
			}
			initialCommitResp, err := rs.client.CreateGitCommit(ctx, owner, repo, initialCommitReq)
			if err != nil {
				return fmt.Errorf("failed to create commit: %w", err)
			}

			// Phase 5: Force update bootstrap branch to point to orphan commit.
			// This replaces the bootstrap commit entirely.
			updateReq := UpdateRefRequest{
				SHA:   initialCommitResp.SHA,
				Force: true, // Force required to replace bootstrap commit
			}
			err = rs.client.UpdateRef(ctx, owner, repo, fmt.Sprintf("heads/%s", bootstrapBranch), updateReq)
			if err != nil {
				return fmt.Errorf("failed to update bootstrap branch: %w", err)
			}

			// If the requested branch is not the bootstrap branch, create it now.
			if branch != bootstrapBranch {
				err = rs.client.CreateBranch(ctx, owner, repo, branch, initialCommitResp.SHA)
				if err != nil {
					return fmt.Errorf("failed to create branch '%s' from bootstrap branch '%s': %w", branch, bootstrapBranch, err)
				}
			}

			fmt.Printf("Successfully uploaded %d files\n", len(files))
			fmt.Printf("Created commit %s on branch '%s'\n", initialCommitResp.SHA[:7], branch)

			err = rs.updateTimelineWithGitHubSHA(localTimelineName, branch, commitHash, initialCommitResp.SHA)
			if err != nil {
				logging.Warn("Failed to update timeline with GitHub SHA", "error", err)
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

	// Skip if the resulting tree is identical to what's already on GitHub
	if parentTreeSHA != "" && treeResp.SHA == parentTreeSHA {
		if parentSHA != "" {
			fmt.Printf("Created tree matches remote parent; creating commit to preserve seal history\n")
		} else {
			fmt.Printf("Already up to date - no file changes to push\n")
			return nil
		}
	}

	commitResp, err := rs.createGitCommitOnBranch(ctx, owner, repo, branch, commitObj.Message, treeResp.SHA, parentSHA, force)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully pushed commit %s to GitHub\n", commitResp.SHA[:7])

	// Store GitHub commit SHA in timeline for future delta uploads
	err = rs.updateTimelineWithGitHubSHA(localTimelineName, branch, commitHash, commitResp.SHA)
	if err != nil {
		// Non-fatal: log but don't fail the push
		logging.Warn("Failed to update timeline with GitHub SHA", "error", err)
	}

	return nil
}

// updateTimelineWithGitHubSHA updates local and remote timeline metadata with the latest GitHub SHA.
func (rs *RepoSyncer) updateTimelineWithGitHubSHA(localTimelineName, remoteBranch string, ivaldiCommitHash cas.Hash, githubCommitSHA string) error {
	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	// Get the timeline
	timeline, err := refsManager.GetTimeline(localTimelineName, refs.LocalTimeline)
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
		localTimelineName,
		refs.LocalTimeline,
		blake3Hash,
		timeline.SHA256Hash,
		githubCommitSHA,
	)
	if err != nil {
		return fmt.Errorf("failed to update timeline: %w", err)
	}

	// Keep the corresponding remote timeline reference in sync after successful pushes.
	remoteTimeline, err := refsManager.GetTimeline(remoteBranch, refs.RemoteTimeline)
	if err == nil {
		_ = refsManager.UpdateRemoteTimeline(remoteBranch, blake3Hash, remoteTimeline.SHA256Hash, githubCommitSHA)
	} else {
		_ = refsManager.CreateTimeline(
			remoteBranch,
			refs.RemoteTimeline,
			blake3Hash,
			[32]byte{},
			githubCommitSHA,
			fmt.Sprintf("Remote branch from upload (%s)", remoteBranch),
		)
	}

	return nil
}
