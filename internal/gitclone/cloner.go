package gitclone

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/logging"
	"github.com/javanhut/Ivaldi-vcs/internal/progress"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// Cloner handles cloning Git repositories and converting to Ivaldi format
type Cloner struct {
	ivaldiDir string
	workDir   string
	casStore  cas.CAS
}

// CloneOptions contains options for cloning a repository
type CloneOptions struct {
	URL         string
	Depth       int
	SkipHistory bool
	IncludeTags bool
	Branch      string
	Auth        AuthMethod
	Username    string
	Password    string
	Token       string
	SSHKey      string
}

// NewCloner creates a new Cloner instance
func NewCloner(ivaldiDir, workDir string) (*Cloner, error) {
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize CAS: %w", err)
	}

	return &Cloner{
		ivaldiDir: ivaldiDir,
		workDir:   workDir,
		casStore:  casStore,
	}, nil
}

// Clone clones a Git repository and converts it to Ivaldi format
func (c *Cloner) Clone(ctx context.Context, opts *CloneOptions) error {
	// Detect authentication
	auth, err := DetectAuth(opts.URL, opts.Username, opts.Password, opts.Token, opts.SSHKey)
	if err != nil {
		return fmt.Errorf("authentication setup failed: %w", err)
	}
	opts.Auth = auth

	// Build go-git clone options
	cloneOpts := &git.CloneOptions{
		URL:      opts.URL,
		Progress: os.Stderr,
	}

	if opts.Depth > 0 {
		cloneOpts.Depth = opts.Depth
	}

	if opts.Branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Branch)
		cloneOpts.SingleBranch = true
	}

	if opts.Auth != nil {
		cloneOpts.Auth = opts.Auth.toGoGitAuth()
	}

	// Clone to temporary directory
	tempDir, err := os.MkdirTemp("", "ivaldi-git-clone-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Println("Cloning repository using Git protocol...")
	repo, err := git.PlainCloneContext(ctx, tempDir, false, cloneOpts)
	if err != nil {
		return c.handleCloneError(err, opts.URL)
	}

	// Check if cloned repo has existing Ivaldi data
	ivaldiSrcDir := filepath.Join(tempDir, ".ivaldi")
	if info, err := os.Stat(ivaldiSrcDir); err == nil && info.IsDir() {
		fmt.Println("Found existing Ivaldi data in repository")
		return c.importExistingIvaldiData(ivaldiSrcDir, tempDir)
	}

	// Get HEAD reference
	ref, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Convert based on mode
	if opts.SkipHistory {
		fmt.Println("Extracting files without history...")
		return c.checkoutFiles(repo, ref)
	}

	fmt.Println("Importing commit history...")
	return c.importHistory(repo, ref, opts.IncludeTags)
}

// handleCloneError provides user-friendly error messages
func (c *Cloner) handleCloneError(err error, url string) error {
	errStr := err.Error()

	if err == context.DeadlineExceeded {
		return fmt.Errorf("clone timeout - try using --depth to limit history")
	}

	if contains(errStr, "authentication") || contains(errStr, "Authentication") {
		return fmt.Errorf("authentication failed - use --token, --username/--password, or --ssh-key")
	}

	if contains(errStr, "repository not found") || contains(errStr, "not found") {
		return fmt.Errorf("repository not found: %s - check URL and permissions", url)
	}

	if contains(errStr, "connection refused") || contains(errStr, "network") {
		return fmt.Errorf("cannot connect to server - check URL and network")
	}

	return fmt.Errorf("clone failed: %w", err)
}

// checkoutFiles extracts files from HEAD without importing history
func (c *Cloner) checkoutFiles(repo *git.Repository, head *plumbing.Reference) error {
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("failed to get commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("failed to get tree: %w", err)
	}

	fmt.Println("Extracting files...")
	_, err = c.extractFilesFromTree(tree, true)
	if err != nil {
		return fmt.Errorf("failed to extract files: %w", err)
	}

	fmt.Println("Files extracted successfully")
	return nil
}

// importHistory imports full commit history and converts to Ivaldi
func (c *Cloner) importHistory(repo *git.Repository, head *plumbing.Reference, includeTags bool) error {
	// Get commit history
	commits, err := c.getCommitHistory(repo, head)
	if err != nil {
		return fmt.Errorf("failed to get commit history: %w", err)
	}

	if len(commits) == 0 {
		return fmt.Errorf("no commits found in repository")
	}

	fmt.Printf("Found %d commits to import\n\n", len(commits))

	// Progress bar for commit import
	progressBar := progress.NewDownloadBar(len(commits), "Importing commits")
	defer progressBar.Finish()

	// Initialize refs manager
	refsManager, err := refs.NewRefsManager(c.ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	// Initialize MMR and commit builder
	mmr, err := history.NewPersistentMMR(c.casStore, c.ivaldiDir)
	if err != nil {
		mmr = &history.PersistentMMR{MMR: history.NewMMR()}
	}
	defer mmr.Close()

	commitBuilder := commit.NewCommitBuilder(c.casStore, mmr.MMR)

	// Process commits in chronological order (oldest first)
	for i := len(commits) - 1; i >= 0; i-- {
		gitCommit := commits[i]

		// Get commit tree
		tree, err := gitCommit.Tree()
		if err != nil {
			progressBar.Finish()
			return fmt.Errorf("failed to get tree for commit %s: %w", gitCommit.Hash, err)
		}

		// Extract files from tree
		showProgress := (i == len(commits)-1) // Only show file progress for first commit
		workspaceFiles, err := c.extractFilesFromTree(tree, showProgress)
		if err != nil {
			progressBar.Finish()
			return fmt.Errorf("failed to extract files: %w", err)
		}

		// Get parent commits
		var parents []cas.Hash
		for _, parentHash := range gitCommit.ParentHashes {
			ivaldiHash, err := refsManager.GetGitMapping(parentHash.String())
			if err == nil {
				parents = append(parents, ivaldiHash)
			}
		}

		// Create Ivaldi commit
		author := fmt.Sprintf("%s <%s>", gitCommit.Author.Name, gitCommit.Author.Email)
		committer := fmt.Sprintf("%s <%s>", gitCommit.Committer.Name, gitCommit.Committer.Email)

		commitObj, err := commitBuilder.CreateCommitWithTime(
			workspaceFiles,
			parents,
			author,
			committer,
			gitCommit.Message,
			gitCommit.Author.When,
			gitCommit.Committer.When,
		)
		if err != nil {
			progressBar.Finish()
			return fmt.Errorf("failed to create Ivaldi commit: %w", err)
		}

		// Get Ivaldi commit hash
		commitHash := commitBuilder.GetCommitHash(commitObj)

		// Store Git SHA → Ivaldi hash mapping
		err = refsManager.PutGitMapping(gitCommit.Hash.String(), commitHash)
		if err != nil {
			logging.Warn("Failed to store Git mapping", "error", err)
		}

		// Update timeline
		var hashArray [32]byte
		copy(hashArray[:], commitHash[:])

		err = refsManager.UpdateTimeline(
			"main",
			refs.LocalTimeline,
			hashArray,
			[32]byte{},
			gitCommit.Hash.String(),
		)
		if err != nil {
			progressBar.Finish()
			return fmt.Errorf("failed to update timeline: %w", err)
		}

		progressBar.Increment()
	}

	progressBar.Finish()
	fmt.Printf("Successfully imported %d commits\n\n", len(commits))

	// Import tags if requested
	if includeTags {
		return c.importTags(repo, refsManager)
	}

	return nil
}

// extractFilesFromTree extracts files from a Git tree to the workspace
func (c *Cloner) extractFilesFromTree(tree *object.Tree, showProgress bool) ([]wsindex.FileMetadata, error) {
	var files []wsindex.FileMetadata
	var fileCount int

	// Count files first if showing progress
	if showProgress {
		tree.Files().ForEach(func(f *object.File) error {
			fileCount++
			return nil
		})
	}

	var progressBar *progress.Bar
	if showProgress && fileCount > 0 {
		progressBar = progress.NewDownloadBar(fileCount, "Extracting files")
		defer progressBar.Finish()
	}

	err := tree.Files().ForEach(func(file *object.File) error {
		// Read file content
		reader, err := file.Reader()
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file.Name, err)
		}
		defer reader.Close()

		contentBytes, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("failed to read content of %s: %w", file.Name, err)
		}

		// Store in CAS
		contentHash := cas.SumB3(contentBytes)
		c.casStore.Put(contentHash, contentBytes)

		// Write to workspace
		filePath := filepath.Join(c.workDir, file.Name)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", file.Name, err)
		}

		if err := os.WriteFile(filePath, contentBytes, os.FileMode(file.Mode)); err != nil {
			return fmt.Errorf("failed to write %s: %w", file.Name, err)
		}

		// Create metadata
		files = append(files, wsindex.FileMetadata{
			Path:     file.Name,
			FileRef:  filechunk.NodeRef{Hash: contentHash},
			ModTime:  time.Now(),
			Mode:     uint32(file.Mode),
			Size:     int64(len(contentBytes)),
			Checksum: contentHash,
		})

		if progressBar != nil {
			progressBar.Increment()
		}

		return nil
	})

	if progressBar != nil {
		progressBar.Finish()
	}

	return files, err
}

// getCommitHistory retrieves commit history from repository
func (c *Cloner) getCommitHistory(repo *git.Repository, head *plumbing.Reference) ([]*object.Commit, error) {
	commitIter, err := repo.Log(&git.LogOptions{
		From: head.Hash(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	var commits []*object.Commit

	err = commitIter.ForEach(func(commit *object.Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// importTags imports tags from the Git repository
func (c *Cloner) importTags(repo *git.Repository, refsManager *refs.RefsManager) error {
	tags, err := repo.Tags()
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	tagCount := 0
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		tagName := ref.Name().Short()

		// Get the commit this tag points to
		var commitHash plumbing.Hash

		// Handle both lightweight and annotated tags
		obj, err := repo.Object(plumbing.AnyObject, ref.Hash())
		if err != nil {
			return nil // Skip tags we can't resolve
		}

		switch o := obj.(type) {
		case *object.Commit:
			commitHash = o.Hash
		case *object.Tag:
			commitHash = o.Target
		default:
			return nil // Skip non-commit/non-tag references
		}

		// Get Ivaldi commit hash from Git SHA
		ivaldiHash, err := refsManager.GetGitMapping(commitHash.String())
		if err != nil {
			// Tag points to commit we don't have, skip it
			return nil
		}

		// Store tag reference
		var hashArray [32]byte
		copy(hashArray[:], ivaldiHash[:])

		err = refsManager.CreateTimeline(
			"tag/"+tagName,
			refs.LocalTimeline,
			hashArray,
			[32]byte{},
			"",
			fmt.Sprintf("Tag: %s", tagName),
		)
		if err == nil {
			tagCount++
		}

		return nil
	})

	if err != nil {
		logging.Warn("Failed to import some tags", "error", err)
	}

	if tagCount > 0 {
		fmt.Printf("Imported %d tags\n", tagCount)
	}

	return nil
}

// contains is a helper to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// copyFile copies a single file from src to dst, preserving file mode
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		return copyFile(path, dstPath)
	})
}

// importExistingIvaldiData imports an existing .ivaldi directory from a cloned repo
// instead of converting Git commits to Ivaldi format. This preserves the original
// Ivaldi hashes, mappings, and history.
func (c *Cloner) importExistingIvaldiData(srcIvaldiDir, srcWorkDir string) error {
	fmt.Println("Importing existing Ivaldi data...")

	// 1. Copy objects directory (CAS store)
	srcObjects := filepath.Join(srcIvaldiDir, "objects")
	if info, err := os.Stat(srcObjects); err == nil && info.IsDir() {
		dstObjects := filepath.Join(c.ivaldiDir, "objects")
		if err := copyDir(srcObjects, dstObjects); err != nil {
			return fmt.Errorf("failed to copy objects directory: %w", err)
		}
		fmt.Println("  Copied CAS objects")
	}

	// 2. Copy refs directory (timelines, tags, seals)
	srcRefs := filepath.Join(srcIvaldiDir, "refs")
	if info, err := os.Stat(srcRefs); err == nil && info.IsDir() {
		dstRefs := filepath.Join(c.ivaldiDir, "refs")
		if err := copyDir(srcRefs, dstRefs); err != nil {
			return fmt.Errorf("failed to copy refs directory: %w", err)
		}
		fmt.Println("  Copied refs")
	}

	// 3. Copy objects.db (BoltDB with hash mappings and config)
	srcDB := filepath.Join(srcIvaldiDir, "objects.db")
	if _, err := os.Stat(srcDB); err == nil {
		dstDB := filepath.Join(c.ivaldiDir, "objects.db")
		if err := copyFile(srcDB, dstDB); err != nil {
			return fmt.Errorf("failed to copy objects.db: %w", err)
		}
		fmt.Println("  Copied database")
	}

	// 4. Copy HEAD file
	srcHEAD := filepath.Join(srcIvaldiDir, "HEAD")
	if _, err := os.Stat(srcHEAD); err == nil {
		dstHEAD := filepath.Join(c.ivaldiDir, "HEAD")
		if err := copyFile(srcHEAD, dstHEAD); err != nil {
			return fmt.Errorf("failed to copy HEAD: %w", err)
		}
		fmt.Println("  Copied HEAD")
	}

	// 5. Copy config if present
	srcConfig := filepath.Join(srcIvaldiDir, "config")
	if _, err := os.Stat(srcConfig); err == nil {
		dstConfig := filepath.Join(c.ivaldiDir, "config")
		if err := copyFile(srcConfig, dstConfig); err != nil {
			return fmt.Errorf("failed to copy config: %w", err)
		}
		fmt.Println("  Copied config")
	}

	// 6. Copy MMR data if present
	srcMMR := filepath.Join(srcIvaldiDir, "mmr.db")
	if _, err := os.Stat(srcMMR); err == nil {
		dstMMR := filepath.Join(c.ivaldiDir, "mmr.db")
		if err := copyFile(srcMMR, dstMMR); err != nil {
			return fmt.Errorf("failed to copy mmr.db: %w", err)
		}
		fmt.Println("  Copied MMR data")
	}

	// 7. Copy workspace files (non-.git, non-.ivaldi files)
	err := filepath.Walk(srcWorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcWorkDir, path)
		if err != nil {
			return err
		}

		// Skip .git and .ivaldi directories
		if relPath == ".git" || relPath == ".ivaldi" ||
			strings.HasPrefix(relPath, ".git"+string(filepath.Separator)) ||
			strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(c.workDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		return copyFile(path, dstPath)
	})
	if err != nil {
		return fmt.Errorf("failed to copy workspace files: %w", err)
	}
	fmt.Println("  Copied workspace files")

	// 8. Validate imported data
	if err := c.validateImportedData(); err != nil {
		return fmt.Errorf("imported Ivaldi data validation failed: %w", err)
	}

	fmt.Println("Successfully imported existing Ivaldi repository")
	return nil
}

// validateImportedData verifies that the imported Ivaldi data is valid
func (c *Cloner) validateImportedData() error {
	// Reinitialize CAS to use the imported objects
	objectsDir := filepath.Join(c.ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to open CAS: %w", err)
	}
	c.casStore = casStore

	// Verify HEAD points to valid timeline
	refsManager, err := refs.NewRefsManager(c.ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to open refs manager: %w", err)
	}
	defer refsManager.Close()

	timeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return fmt.Errorf("no current timeline: %w", err)
	}

	// Verify latest commit exists in CAS
	timelineInfo, err := refsManager.GetTimeline(timeline, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get timeline %s: %w", timeline, err)
	}

	// If timeline has a commit, verify it exists
	var zeroHash [32]byte
	if timelineInfo.Blake3Hash != zeroHash {
		var commitHash cas.Hash
		copy(commitHash[:], timelineInfo.Blake3Hash[:])
		if _, err := c.casStore.Get(commitHash); err != nil {
			return fmt.Errorf("commit %x not found in CAS: %w", timelineInfo.Blake3Hash[:8], err)
		}
	}

	return nil
}
