// Package workspace implements workspace materialization for Ivaldi timelines.
//
// This package provides the core functionality to:
// - Materialize workspace files when switching timelines
// - Track workspace state using the storage core components
// - Apply changes to the working directory efficiently
// - Handle file permissions, timestamps, and directory structures
//
// Integration with storage components:
// - Uses wsindex for tracking all workspace files
// - Uses hamtdir for efficient directory structure management
// - Uses filechunk for content storage and retrieval
// - Uses diffmerge for computing changes between timeline states
package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/hamtdir"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/shelf"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// WorkspaceState represents the current state of a workspace.
type WorkspaceState struct {
	TimelineName string           // Name of current timeline
	Index        wsindex.IndexRef // Workspace file index
	RootDir      *hamtdir.DirRef  // Root directory structure (optional)
	IvaldiDir    string           // Path to .ivaldi directory
	WorkDir      string           // Path to working directory
}

// Materializer handles workspace materialization operations.
type Materializer struct {
	CAS       cas.CAS
	IvaldiDir string
	WorkDir   string
}

// NewMaterializer creates a new Materializer.
func NewMaterializer(casStore cas.CAS, ivaldiDir, workDir string) *Materializer {
	return &Materializer{
		CAS:       casStore,
		IvaldiDir: ivaldiDir,
		WorkDir:   workDir,
	}
}

// GetCurrentState reads the current workspace state.
func (m *Materializer) GetCurrentState() (*WorkspaceState, error) {
	// Get current timeline
	refsManager, err := refs.NewRefsManager(m.IvaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	timelineName, err := refsManager.GetCurrentTimeline()
	if err != nil {
		// If there's no current timeline (no HEAD file), scan the current workspace
		// and create an index based on what's currently in the working directory
		wsIndex, err := m.ScanWorkspace()
		if err != nil {
			return nil, fmt.Errorf("failed to scan workspace: %w", err)
		}

		return &WorkspaceState{
			TimelineName: "", // No current timeline
			Index:        wsIndex,
			RootDir:      nil,
			IvaldiDir:    m.IvaldiDir,
			WorkDir:      m.WorkDir,
		}, nil
	}

	// Create workspace index from the actual workspace files (not from timeline state)
	wsIndex, err := m.ScanWorkspace()
	if err != nil {
		return nil, fmt.Errorf("failed to scan workspace: %w", err)
	}

	return &WorkspaceState{
		TimelineName: timelineName,
		Index:        wsIndex,
		IvaldiDir:    m.IvaldiDir,
		WorkDir:      m.WorkDir,
	}, nil
}

// ScanWorkspace scans the current working directory and creates a workspace index.
func (m *Materializer) ScanWorkspace() (wsindex.IndexRef, error) {
	var files []wsindex.FileMetadata

	err := filepath.WalkDir(m.WorkDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip .ivaldi directory
		relPath, err := filepath.Rel(m.WorkDir, path)
		if err != nil {
			return err
		}

		if strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) || relPath == ".ivaldi" {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", relPath, err)
		}

		// Create file chunks
		builder := filechunk.NewBuilder(m.CAS, filechunk.DefaultParams())
		fileRef, err := builder.Build(content)
		if err != nil {
			return fmt.Errorf("failed to create file chunks for %s: %w", relPath, err)
		}

		// Create metadata
		fileMetadata := wsindex.FileMetadata{
			Path:     relPath,
			FileRef:  fileRef,
			ModTime:  info.ModTime(),
			Mode:     uint32(info.Mode()),
			Size:     info.Size(),
			Checksum: cas.SumB3(content),
		}

		files = append(files, fileMetadata)
		return nil
	})

	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to scan workspace: %w", err)
	}

	// Build workspace index
	wsBuilder := wsindex.NewBuilder(m.CAS)
	return wsBuilder.Build(files)
}

// MaterializeTimeline materializes a timeline's state to the workspace.
func (m *Materializer) MaterializeTimeline(timelineName string) error {
	return m.MaterializeTimelineWithAutoShelf(timelineName, true)
}

// MaterializeTimelineWithAutoShelf materializes a timeline with optional auto-shelving.
func (m *Materializer) MaterializeTimelineWithAutoShelf(timelineName string, enableAutoShelf bool) error {
	// Get timeline information
	refsManager, err := refs.NewRefsManager(m.IvaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	timeline, err := refsManager.GetTimeline(timelineName, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get timeline %s: %w", timelineName, err)
	}

	// Get current workspace state
	currentState, err := m.GetCurrentState()
	if err != nil {
		return fmt.Errorf("failed to get current workspace state: %w", err)
	}

	// Get current timeline name for auto-shelving
	currentTimelineName := currentState.TimelineName
	if currentTimelineName == "" {
		// If no current timeline, try to get it from refs
		if currentTL, err := refsManager.GetCurrentTimeline(); err == nil {
			currentTimelineName = currentTL
		} else {
			currentTimelineName = "main" // Default fallback
		}
	}

	// Auto-shelf current changes before switching (if enabled and switching between different timelines)
	if enableAutoShelf && currentTimelineName != "" && currentTimelineName != timelineName {
		shelfManager := shelf.NewShelfManager(m.CAS, m.IvaldiDir)

		// Always remove any existing auto-shelf for the current timeline first
		if err := shelfManager.RemoveAutoShelf(currentTimelineName); err != nil {
			// Log but don't fail - maybe there was no auto-shelf
		}

		// Get the CURRENT timeline's committed state to use as base
		currentTimelineBase, err := m.getTimelineBaseIndex(currentTimelineName, refsManager)
		if err != nil {
			// If we can't get the base, use an empty index
			wsBuilder := wsindex.NewBuilder(m.CAS)
			currentTimelineBase, _ = wsBuilder.Build(nil)
		}

		// ALWAYS create new auto-shelf with the CURRENT workspace state
		// This preserves the exact workspace state including all untracked files
		autoShelf, err := shelfManager.CreateAutoShelf(currentTimelineName, currentState.Index, currentTimelineBase)
		if err != nil {
			return fmt.Errorf("failed to create auto-shelf: %w", err)
		}

		// Count and report changes if any
		differ := diffmerge.NewDiffer(m.CAS)
		diff, err := differ.DiffWorkspaces(currentTimelineBase, currentState.Index)
		if err == nil && len(diff.FileChanges) > 0 {
			fmt.Printf("Auto-shelved %d changes from timeline '%s' (shelf: %s)\n",
				len(diff.FileChanges), currentTimelineName, autoShelf.ID)
		} else {
			// Even with no changes, we still shelved the workspace state
			fmt.Printf("Auto-shelved workspace state for timeline '%s' (shelf: %s)\n",
				currentTimelineName, autoShelf.ID)
		}
	}

	// Check if there's an auto-shelf for the target timeline to restore first
	// This takes priority over the committed timeline state
	var targetIndex wsindex.IndexRef
	var hasAutoShelf bool

	if enableAutoShelf {
		shelfManager := shelf.NewShelfManager(m.CAS, m.IvaldiDir)
		autoShelf, err := shelfManager.GetAutoShelf(timelineName)
		if err == nil && autoShelf != nil {
			// Use the shelved workspace state instead of the clean timeline state
			targetIndex = autoShelf.WorkspaceIndex
			hasAutoShelf = true
			fmt.Printf("Restoring auto-shelved changes for timeline '%s' (shelf: %s)\n",
				timelineName, autoShelf.ID)

			// Restore staged files if any
			if err := shelfManager.RestoreStagedFiles(autoShelf); err != nil {
				fmt.Printf("Warning: failed to restore staged files: %v\n", err)
			}

			// Remove the auto-shelf since we're applying it
			if err := shelfManager.RemoveAutoShelf(timelineName); err != nil {
				fmt.Printf("Warning: failed to remove applied auto-shelf: %v\n", err)
			}
		}
	}

	// Only create target index from timeline commit if we don't have an autoshelf
	if !hasAutoShelf {
		var err error
		targetIndex, err = m.CreateTargetIndex(*timeline)
		if err != nil {
			return fmt.Errorf("failed to create target index: %w", err)
		}
	}

	// Compute differences between current state and target
	differ := diffmerge.NewDiffer(m.CAS)
	diff, err := differ.DiffWorkspaces(currentState.Index, targetIndex)
	if err != nil {
		return fmt.Errorf("failed to compute workspace diff: %w", err)
	}

	// Apply changes to working directory
	err = m.ApplyChangesToWorkspace(diff)
	if err != nil {
		return fmt.Errorf("failed to apply changes to workspace: %w", err)
	}

	// Update current timeline
	err = refsManager.SetCurrentTimeline(timelineName)
	if err != nil {
		return fmt.Errorf("failed to update current timeline: %w", err)
	}

	return nil
}

// CreateTargetIndex creates a target workspace index for a timeline.
// This reads the actual commit object and extracts the workspace files.
func (m *Materializer) CreateTargetIndex(timeline refs.Timeline) (wsindex.IndexRef, error) {
	wsBuilder := wsindex.NewBuilder(m.CAS)

	// If timeline has no content (empty hash), return empty index
	// This represents a truly empty repository with no initial commit
	if timeline.Blake3Hash == [32]byte{} {
		return wsBuilder.Build(nil)
	}

	// Read the commit object using the timeline's Blake3 hash
	var commitHash cas.Hash
	copy(commitHash[:], timeline.Blake3Hash[:])

	// Use commit reader to get the commit and its tree
	commitReader := commit.NewCommitReader(m.CAS)
	commitObj, err := commitReader.ReadCommit(commitHash)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to read commit object: %w", err)
	}

	// Read the tree structure
	tree, err := commitReader.ReadTree(commitObj)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to read tree structure: %w", err)
	}

	// List all files in the tree
	filePaths, err := commitReader.ListFiles(tree)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to list files in tree: %w", err)
	}

	// Create file metadata for each file
	var files []wsindex.FileMetadata
	for _, filePath := range filePaths {
		// Get file content to determine size and checksum
		content, err := commitReader.GetFileContent(tree, filePath)
		if err != nil {
			return wsindex.IndexRef{}, fmt.Errorf("failed to get content for file %s: %w", filePath, err)
		}

		// Get the file's NodeRef from the tree by navigating to it
		fileRef, err := m.getFileRefFromTree(tree, filePath)
		if err != nil {
			return wsindex.IndexRef{}, fmt.Errorf("failed to get file ref for %s: %w", filePath, err)
		}

		// Create file metadata
		fileMetadata := wsindex.FileMetadata{
			Path:     filePath,
			FileRef:  fileRef,
			ModTime:  commitObj.CommitTime, // Use commit time as file mod time
			Mode:     0644,                 // Default file mode
			Size:     int64(len(content)),
			Checksum: cas.SumB3(content),
		}

		files = append(files, fileMetadata)
	}

	// Build the workspace index
	return wsBuilder.Build(files)
}

// getTimelineBaseIndex gets the base workspace index for a timeline (the committed state).
func (m *Materializer) getTimelineBaseIndex(timelineName string, refsManager *refs.RefsManager) (wsindex.IndexRef, error) {
	timeline, err := refsManager.GetTimeline(timelineName, refs.LocalTimeline)
	if err != nil {
		// If timeline doesn't exist, return empty index
		wsBuilder := wsindex.NewBuilder(m.CAS)
		return wsBuilder.Build(nil)
	}

	return m.CreateTargetIndex(*timeline)
}

// getFileRefFromTree extracts the NodeRef for a specific file from the tree.
func (m *Materializer) getFileRefFromTree(tree *commit.TreeObject, filePath string) (filechunk.NodeRef, error) {
	// Split the path into parts
	parts := strings.Split(filePath, string(filepath.Separator))
	if len(parts) == 0 {
		return filechunk.NodeRef{}, fmt.Errorf("invalid file path: %s", filePath)
	}

	// Navigate through the HAMT structure to find the file
	hamtLoader := hamtdir.NewLoader(m.CAS)
	currentDirRef := tree.DirRef

	for i, part := range parts {
		entries, err := hamtLoader.List(currentDirRef)
		if err != nil {
			return filechunk.NodeRef{}, fmt.Errorf("failed to read directory entries: %w", err)
		}

		if i == len(parts)-1 {
			// This is the final file
			for _, entry := range entries {
				if entry.Name == part && entry.Type == hamtdir.FileEntry {
					return *entry.File, nil
				}
			}
			return filechunk.NodeRef{}, fmt.Errorf("file not found: %s", part)
		} else {
			// Navigate to subdirectory
			found := false
			for _, entry := range entries {
				if entry.Name == part && entry.Type == hamtdir.DirEntry {
					currentDirRef = *entry.Dir
					found = true
					break
				}
			}
			if !found {
				return filechunk.NodeRef{}, fmt.Errorf("directory not found: %s", part)
			}
		}
	}

	return filechunk.NodeRef{}, fmt.Errorf("unexpected error in getFileRefFromTree")
}

// ApplyChangesToWorkspace applies file changes to the working directory.
func (m *Materializer) ApplyChangesToWorkspace(diff *diffmerge.WorkspaceDiff) error {
	loader := filechunk.NewLoader(m.CAS)

	for _, change := range diff.FileChanges {
		fullPath := filepath.Join(m.WorkDir, change.Path)

		switch change.Type {
		case diffmerge.Added, diffmerge.Modified:
			if change.NewFile == nil {
				continue
			}

			// Ensure parent directory exists
			parentDir := filepath.Dir(fullPath)
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
			}

			// Read file content from chunks
			content, err := loader.ReadAll(change.NewFile.FileRef)
			if err != nil {
				return fmt.Errorf("failed to read file content for %s: %w", change.Path, err)
			}

			// Write file
			err = os.WriteFile(fullPath, content, os.FileMode(change.NewFile.Mode))
			if err != nil {
				return fmt.Errorf("failed to write file %s: %w", change.Path, err)
			}

			// Set modification time
			err = os.Chtimes(fullPath, change.NewFile.ModTime, change.NewFile.ModTime)
			if err != nil {
				// Don't fail on timestamp errors, just log
				fmt.Printf("Warning: failed to set timestamp for %s: %v\n", change.Path, err)
			}

		case diffmerge.Removed:
			// Remove file
			err := os.Remove(fullPath)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove file %s: %w", change.Path, err)
			}

			// Try to remove empty parent directories
			parentDir := filepath.Dir(fullPath)
			m.removeEmptyDirectories(parentDir)
		}
	}

	return nil
}

// removeEmptyDirectories removes empty directories up the tree.
func (m *Materializer) removeEmptyDirectories(dir string) {
	// Don't remove the working directory itself
	if dir == m.WorkDir || dir == "." {
		return
	}

	// Check if directory is empty
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}

	// Remove empty directory
	err = os.Remove(dir)
	if err != nil {
		return
	}

	// Recursively check parent
	parent := filepath.Dir(dir)
	if parent != dir { // Prevent infinite loop
		m.removeEmptyDirectories(parent)
	}
}

// BackupWorkspace creates a backup of the current workspace state.
func (m *Materializer) BackupWorkspace(backupName string) error {
	// Scan current workspace
	currentIndex, err := m.ScanWorkspace()
	if err != nil {
		return fmt.Errorf("failed to scan workspace for backup: %w", err)
	}

	// Store backup in refs (as a tag)
	refsManager, err := refs.NewRefsManager(m.IvaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	// Use index hash as backup identifier
	indexHash := currentIndex.Hash
	var blake3Hash [32]byte
	copy(blake3Hash[:], indexHash[:])

	err = refsManager.CreateTimeline(
		backupName,
		refs.TagTimeline,
		blake3Hash,
		[32]byte{}, // No SHA256
		"",         // No Git SHA1
		fmt.Sprintf("Workspace backup: %s", backupName),
	)

	if err != nil {
		return fmt.Errorf("failed to create backup tag: %w", err)
	}

	return nil
}

// RestoreWorkspace restores workspace from a backup.
func (m *Materializer) RestoreWorkspace(backupName string) error {
	refsManager, err := refs.NewRefsManager(m.IvaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	// Get backup tag
	backup, err := refsManager.GetTimeline(backupName, refs.TagTimeline)
	if err != nil {
		return fmt.Errorf("backup %s not found: %w", backupName, err)
	}

	// Create index from backup hash
	var indexHash cas.Hash
	copy(indexHash[:], backup.Blake3Hash[:])

	backupIndex := wsindex.IndexRef{
		Hash:  indexHash,
		Count: 0, // Count will be determined when loading
	}

	// Get current state
	currentState, err := m.GetCurrentState()
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	// Compute diff
	differ := diffmerge.NewDiffer(m.CAS)
	diff, err := differ.DiffWorkspaces(currentState.Index, backupIndex)
	if err != nil {
		return fmt.Errorf("failed to compute restore diff: %w", err)
	}

	// Apply changes
	return m.ApplyChangesToWorkspace(diff)
}

// CleanWorkspace removes all files from the workspace.
func (m *Materializer) CleanWorkspace() error {
	// Get current state
	currentIndex, err := m.ScanWorkspace()
	if err != nil {
		return fmt.Errorf("failed to scan workspace: %w", err)
	}

	// Create empty target
	wsBuilder := wsindex.NewBuilder(m.CAS)
	emptyIndex, err := wsBuilder.Build(nil)
	if err != nil {
		return fmt.Errorf("failed to create empty index: %w", err)
	}

	// Compute diff (everything should be removed)
	differ := diffmerge.NewDiffer(m.CAS)
	diff, err := differ.DiffWorkspaces(currentIndex, emptyIndex)
	if err != nil {
		return fmt.Errorf("failed to compute clean diff: %w", err)
	}

	// Apply changes
	return m.ApplyChangesToWorkspace(diff)
}

// GetWorkspaceStatus returns detailed status of workspace files.
func (m *Materializer) GetWorkspaceStatus() (*WorkspaceStatus, error) {
	// Get current timeline name
	refsManager, err := refs.NewRefsManager(m.IvaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	timelineName, err := refsManager.GetCurrentTimeline()
	if err != nil {
		// No current timeline, return empty status
		return &WorkspaceStatus{
			TimelineName: "",
			Clean:        true,
			Changes:      nil,
		}, nil
	}

	// Get the committed state for this timeline
	committedIndex, err := m.getTimelineBaseIndex(timelineName, refsManager)
	if err != nil {
		return nil, fmt.Errorf("failed to get committed timeline state: %w", err)
	}

	// Scan actual workspace
	actualIndex, err := m.ScanWorkspace()
	if err != nil {
		return nil, fmt.Errorf("failed to scan workspace: %w", err)
	}

	// Compute differences between committed state and actual workspace
	differ := diffmerge.NewDiffer(m.CAS)
	diff, err := differ.DiffWorkspaces(committedIndex, actualIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to compute status diff: %w", err)
	}

	status := &WorkspaceStatus{
		TimelineName: timelineName,
		Clean:        len(diff.FileChanges) == 0,
		Changes:      diff.FileChanges,
	}

	return status, nil
}

// WorkspaceStatus represents the current status of workspace files.
type WorkspaceStatus struct {
	TimelineName string                 // Current timeline
	Clean        bool                   // True if workspace matches tracked state
	Changes      []diffmerge.FileChange // List of changes in workspace
}

// Summary returns a human-readable summary of the workspace status.
func (s *WorkspaceStatus) Summary() string {
	if s.Clean {
		return fmt.Sprintf("Workspace clean (timeline: %s)", s.TimelineName)
	}

	var added, modified, removed int
	for _, change := range s.Changes {
		switch change.Type {
		case diffmerge.Added:
			added++
		case diffmerge.Modified:
			modified++
		case diffmerge.Removed:
			removed++
		}
	}

	return fmt.Sprintf("Workspace dirty (timeline: %s, +%d ~%d -%d)",
		s.TimelineName, added, modified, removed)
}

// ListChanges returns a list of change descriptions.
func (s *WorkspaceStatus) ListChanges() []string {
	var changes []string
	for _, change := range s.Changes {
		switch change.Type {
		case diffmerge.Added:
			changes = append(changes, fmt.Sprintf("A  %s", change.Path))
		case diffmerge.Modified:
			changes = append(changes, fmt.Sprintf("M  %s", change.Path))
		case diffmerge.Removed:
			changes = append(changes, fmt.Sprintf("D  %s", change.Path))
		}
	}
	return changes
}

// Stash represents a temporary storage of workspace changes.
type Stash struct {
	Name        string           // Stash name
	Description string           // Description of changes
	Index       wsindex.IndexRef // Stashed workspace state
	Created     time.Time        // When stash was created
}

// StashManager handles workspace stashing operations.
type StashManager struct {
	Materializer *Materializer
}

// NewStashManager creates a new StashManager.
func NewStashManager(materializer *Materializer) *StashManager {
	return &StashManager{Materializer: materializer}
}

// CreateStash creates a new stash with the current workspace changes.
func (sm *StashManager) CreateStash(name, description string) error {
	// Scan current workspace
	currentIndex, err := sm.Materializer.ScanWorkspace()
	if err != nil {
		return fmt.Errorf("failed to scan workspace for stash: %w", err)
	}

	// Store stash as a tag
	refsManager, err := refs.NewRefsManager(sm.Materializer.IvaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	// Use index hash as stash identifier
	var blake3Hash [32]byte
	copy(blake3Hash[:], currentIndex.Hash[:])

	stashTagName := fmt.Sprintf("stash/%s", name)
	err = refsManager.CreateTimeline(
		stashTagName,
		refs.TagTimeline,
		blake3Hash,
		[32]byte{}, // No SHA256
		"",         // No Git SHA1
		fmt.Sprintf("Stash: %s - %s", name, description),
	)

	return err
}

// ApplyStash applies a stash to the current workspace.
func (sm *StashManager) ApplyStash(name string) error {
	refsManager, err := refs.NewRefsManager(sm.Materializer.IvaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	// Get stash tag
	stashTagName := fmt.Sprintf("stash/%s", name)
	stash, err := refsManager.GetTimeline(stashTagName, refs.TagTimeline)
	if err != nil {
		return fmt.Errorf("stash %s not found: %w", name, err)
	}

	// Create index from stash hash
	var indexHash cas.Hash
	copy(indexHash[:], stash.Blake3Hash[:])

	stashIndex := wsindex.IndexRef{
		Hash:  indexHash,
		Count: 0, // Count will be determined when loading
	}

	// Get current state
	currentState, err := sm.Materializer.GetCurrentState()
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	// Compute diff
	differ := diffmerge.NewDiffer(sm.Materializer.CAS)
	diff, err := differ.DiffWorkspaces(currentState.Index, stashIndex)
	if err != nil {
		return fmt.Errorf("failed to compute stash diff: %w", err)
	}

	// Apply changes
	return sm.Materializer.ApplyChangesToWorkspace(diff)
}

// ListStashes returns a list of available stashes.
func (sm *StashManager) ListStashes() ([]string, error) {
	refsManager, err := refs.NewRefsManager(sm.Materializer.IvaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	tags, err := refsManager.ListTimelines(refs.TagTimeline)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	var stashes []string
	for _, tag := range tags {
		if strings.HasPrefix(tag.Name, "stash/") {
			stashName := strings.TrimPrefix(tag.Name, "stash/")
			stashes = append(stashes, stashName)
		}
	}

	return stashes, nil
}

// DropStash removes a stash.
func (sm *StashManager) DropStash(name string) error {
	refsManager, err := refs.NewRefsManager(sm.Materializer.IvaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	stashTagName := fmt.Sprintf("stash/%s", name)

	// Remove the tag file
	tagPath := filepath.Join(sm.Materializer.IvaldiDir, "refs", "tags", stashTagName)
	err = os.Remove(tagPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stash tag: %w", err)
	}

	return nil
}
