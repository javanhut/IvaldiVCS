package github

import (
	"context"
	"encoding/base64"
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
	fmt.Printf("Description: %s\n", repoInfo.Description)
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

	fmt.Printf("Downloading %d files...\n", len(tree.Tree))

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

// downloadFiles downloads all files from a GitHub tree
func (rs *RepoSyncer) downloadFiles(ctx context.Context, owner, repo string, tree *Tree, ref string) error {
	// Use a worker pool for concurrent downloads
	const workers = 8
	jobs := make(chan TreeEntry, len(tree.Tree))
	errors := make(chan error, len(tree.Tree))

	var wg sync.WaitGroup

	// Start workers
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

	// Submit jobs
	for _, entry := range tree.Tree {
		if entry.Type == "blob" { // Only download files, not subdirectories
			jobs <- entry
		}
	}
	close(jobs)

	// Wait for completion
	wg.Wait()
	close(errors)

	// Check for errors
	var downloadErrors []error
	for err := range errors {
		downloadErrors = append(downloadErrors, err)
	}

	if len(downloadErrors) > 0 {
		return fmt.Errorf("download errors occurred: %v", downloadErrors[0])
	}

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

	fmt.Printf("Downloaded: %s\n", entry.Path)
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

// PushCommit pushes an Ivaldi commit to GitHub
func (rs *RepoSyncer) PushCommit(ctx context.Context, owner, repo, branch string, commitHash cas.Hash) error {
	fmt.Printf("Pushing commit %s to GitHub...\n", commitHash.String()[:8])

	// Read commit from CAS
	commitReader := commit.NewCommitReader(rs.casStore)
	commitObj, err := commitReader.ReadCommit(commitHash)
	if err != nil {
		return fmt.Errorf("failed to read commit: %w", err)
	}

	// Read tree
	tree, err := commitReader.ReadTree(commitObj)
	if err != nil {
		return fmt.Errorf("failed to read tree: %w", err)
	}

	// List files
	files, err := commitReader.ListFiles(tree)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	// Upload each file
	for _, filePath := range files {
		content, err := commitReader.GetFileContent(tree, filePath)
		if err != nil {
			return fmt.Errorf("failed to get content for %s: %w", filePath, err)
		}

		// Write to local workspace temporarily
		localPath := filepath.Join(rs.workDir, filePath)
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		if err := os.WriteFile(localPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		// Upload to GitHub
		err = rs.UploadFile(ctx, owner, repo, filePath, branch, commitObj.Message)
		if err != nil {
			return fmt.Errorf("failed to upload %s: %w", filePath, err)
		}
	}

	fmt.Printf("Successfully pushed commit to GitHub\n")
	return nil
}
