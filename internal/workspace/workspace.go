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
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/hamtdir"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// WorkspaceState represents the current state of a workspace.
type WorkspaceState struct {
	TimelineName string              // Name of current timeline
	Index        wsindex.IndexRef    // Workspace file index
	RootDir      *hamtdir.DirRef     // Root directory structure (optional)
	IvaldiDir    string              // Path to .ivaldi directory
	WorkDir      string              // Path to working directory
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
		return nil, fmt.Errorf("failed to get current timeline: %w", err)
	}

	// Try to load workspace index from timeline metadata
	_, err = refsManager.GetTimeline(timelineName, refs.LocalTimeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline %s: %w", timelineName, err)
	}

	// For now, create empty index if timeline has no workspace state
	// In a full implementation, this would be stored in timeline metadata
	wsBuilder := wsindex.NewBuilder(m.CAS)
	emptyIndex, err := wsBuilder.Build(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create empty index: %w", err)
	}

	return &WorkspaceState{
		TimelineName: timelineName,
		Index:        emptyIndex,
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

		if strings.HasPrefix(relPath, ".ivaldi") {
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

	// For now, create target index based on timeline hash
	// In a full implementation, this would be stored with the timeline
	targetIndex, err := m.createTargetIndex(*timeline)
	if err != nil {
		return fmt.Errorf("failed to create target index: %w", err)
	}

	// Compute differences
	differ := diffmerge.NewDiffer(m.CAS)
	diff, err := differ.DiffWorkspaces(currentState.Index, targetIndex)
	if err != nil {
		return fmt.Errorf("failed to compute workspace diff: %w", err)
	}

	// Apply changes to working directory
	err = m.applyChangesToWorkspace(diff)
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

// createTargetIndex creates a target workspace index for a timeline.
// In a full implementation, this would read the actual stored workspace state.
func (m *Materializer) createTargetIndex(timeline refs.Timeline) (wsindex.IndexRef, error) {
	wsBuilder := wsindex.NewBuilder(m.CAS)
	
	// For demonstration, create some dummy files based on timeline hash
	var files []wsindex.FileMetadata
	
	if timeline.Blake3Hash != [32]byte{} {
		// Timeline has content - create a sample file
		content := fmt.Sprintf("Timeline: %s\nBlake3: %x\n", "sample", timeline.Blake3Hash)
		contentBytes := []byte(content)
		
		fileBuilder := filechunk.NewBuilder(m.CAS, filechunk.DefaultParams())
		fileRef, err := fileBuilder.Build(contentBytes)
		if err != nil {
			return wsindex.IndexRef{}, err
		}
		
		sampleFile := wsindex.FileMetadata{
			Path:     "timeline-info.txt",
			FileRef:  fileRef,
			ModTime:  time.Now(),
			Mode:     0644,
			Size:     int64(len(contentBytes)),
			Checksum: cas.SumB3(contentBytes),
		}
		
		files = append(files, sampleFile)
	}
	
	return wsBuilder.Build(files)
}

// applyChangesToWorkspace applies file changes to the working directory.
func (m *Materializer) applyChangesToWorkspace(diff *diffmerge.WorkspaceDiff) error {
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
	return m.applyChangesToWorkspace(diff)
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
	return m.applyChangesToWorkspace(diff)
}

// GetWorkspaceStatus returns detailed status of workspace files.
func (m *Materializer) GetWorkspaceStatus() (*WorkspaceStatus, error) {
	// Get current tracked state
	currentState, err := m.GetCurrentState()
	if err != nil {
		return nil, fmt.Errorf("failed to get current state: %w", err)
	}

	// Scan actual workspace
	actualIndex, err := m.ScanWorkspace()
	if err != nil {
		return nil, fmt.Errorf("failed to scan workspace: %w", err)
	}

	// Compute differences
	differ := diffmerge.NewDiffer(m.CAS)
	diff, err := differ.DiffWorkspaces(currentState.Index, actualIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to compute status diff: %w", err)
	}

	status := &WorkspaceStatus{
		TimelineName: currentState.TimelineName,
		Clean:        len(diff.FileChanges) == 0,
		Changes:      diff.FileChanges,
	}

	return status, nil
}

// WorkspaceStatus represents the current status of workspace files.
type WorkspaceStatus struct {
	TimelineName string                    // Current timeline
	Clean        bool                      // True if workspace matches tracked state
	Changes      []diffmerge.FileChange    // List of changes in workspace
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
	Name        string              // Stash name
	Description string              // Description of changes
	Index       wsindex.IndexRef    // Stashed workspace state
	Created     time.Time           // When stash was created
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
	return sm.Materializer.applyChangesToWorkspace(diff)
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