package github

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/logging"
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

// NewRepoSyncer creates a new repository syncer (requires authentication for push operations)
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

// NewRepoSyncerOptionalAuth creates a repository syncer that works with or without auth.
// Suitable for read-only operations on public repos (scout, harvest, download).
func NewRepoSyncerOptionalAuth(ivaldiDir, workDir string) (*RepoSyncer, error) {
	// Use optional auth - allows downloading public repos without login
	client := NewClientOptionalAuth()

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

// IsAuthenticated returns whether the syncer has auth configured
func (rs *RepoSyncer) IsAuthenticated() bool {
	return rs.client.IsAuthenticated()
}

// PullChanges pulls latest changes from GitHub
func (rs *RepoSyncer) PullChanges(ctx context.Context, owner, repo, branch string) error {
	fmt.Printf("Pulling changes from %s/%s...\n", owner, repo)

	// Get latest commit SHA
	branchInfo, err := rs.client.GetBranch(ctx, owner, repo, branch)
	if err != nil {
		return fmt.Errorf("failed to get branch info: %w", err)
	}

	// Use archive download (no rate limits) instead of individual file downloads
	fileCount, err := rs.downloadAndExtractArchive(ctx, owner, repo, branchInfo.Commit.SHA)
	if err != nil {
		// Fallback to individual file downloads if archive fails
		fmt.Printf("Archive download failed (%v), falling back to API...\n", err)

		tree, err := rs.client.GetTree(ctx, owner, repo, branchInfo.Commit.SHA, true)
		if err != nil {
			return fmt.Errorf("failed to get tree: %w", err)
		}

		err = rs.downloadFiles(ctx, owner, repo, tree, branchInfo.Commit.SHA)
		if err != nil {
			return fmt.Errorf("failed to download files: %w", err)
		}
	} else {
		fmt.Printf("Extracted %d files from archive\n", fileCount)
	}

	// Create new commit
	err = rs.createIvaldiCommit(fmt.Sprintf("Pull from GitHub: %s", branchInfo.Commit.SHA[:7]))
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	fmt.Println("Successfully pulled changes")
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
			logging.Warn("Failed to delete file", "path", path, "error", err)
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

	fmt.Printf("Branch SHA: %s\n", branchInfo.Commit.SHA[:7])

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

	// Use archive download (no rate limits) instead of individual file downloads
	fileCount, err := rs.downloadAndExtractArchive(ctx, owner, repo, branchInfo.Commit.SHA)
	if err != nil {
		// Fallback to individual file downloads if archive fails
		fmt.Printf("Archive download failed (%v), falling back to API...\n", err)

		// Get the tree for this branch
		tree, err := rs.client.GetTree(ctx, owner, repo, branchInfo.Commit.SHA, true)
		if err != nil {
			rs.workDir = originalWorkDir
			return fmt.Errorf("failed to get tree: %w", err)
		}

		err = rs.downloadFiles(ctx, owner, repo, tree, branchInfo.Commit.SHA)
		if err != nil {
			rs.workDir = originalWorkDir // Restore original workspace
			return fmt.Errorf("failed to download files: %w", err)
		}
	} else {
		fmt.Printf("Extracted %d files from archive\n", fileCount)
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
