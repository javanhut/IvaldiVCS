// Package diffmerge implements diff and merge utilities for Ivaldi storage components.
//
// This package provides high-level operations for:
// - Computing differences between directory trees and workspace indexes
// - Merging changes from different timelines
// - Detecting conflicts and applying resolution strategies
// - Creating patches and applying them to storage structures
//
// The utilities work with all three storage components:
// - File chunks (filechunk)
// - Directory HAMTs (hamtdir)
// - Workspace indexes (wsindex)
package diffmerge

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/hamtdir"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// ChangeType represents the type of change in a diff.
type ChangeType uint8

const (
	Added ChangeType = iota + 1
	Modified
	Removed
)

// FileChange represents a change to a single file.
type FileChange struct {
	Type     ChangeType
	Path     string
	OldFile  *wsindex.FileMetadata // nil for Added
	NewFile  *wsindex.FileMetadata // nil for Removed
}

// DirectoryChange represents a change to a directory structure.
type DirectoryChange struct {
	Type    ChangeType
	Path    string
	OldDir  *hamtdir.DirRef // nil for Added
	NewDir  *hamtdir.DirRef // nil for Removed
}

// WorkspaceDiff represents differences between two workspace states.
type WorkspaceDiff struct {
	FileChanges []FileChange
	DirChanges  []DirectoryChange
}

// Differ computes differences between storage structures.
type Differ struct {
	CAS cas.CAS
}

// NewDiffer creates a new Differ with the given CAS.
func NewDiffer(casStore cas.CAS) *Differ {
	return &Differ{CAS: casStore}
}

// DiffWorkspaces computes differences between two workspace indexes.
func (d *Differ) DiffWorkspaces(oldIndex, newIndex wsindex.IndexRef) (*WorkspaceDiff, error) {
	loader := wsindex.NewLoader(d.CAS)
	
	wsIndexDiff, err := loader.Diff(oldIndex, newIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to compute workspace diff: %w", err)
	}

	var fileChanges []FileChange

	// Convert added files
	for _, file := range wsIndexDiff.Added {
		fileChanges = append(fileChanges, FileChange{
			Type:    Added,
			Path:    file.Path,
			NewFile: &file,
		})
	}

	// Convert modified files
	for _, file := range wsIndexDiff.Modified {
		// We need to find the old version for a complete diff
		oldFile, err := loader.Lookup(oldIndex, file.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup old file %s: %w", file.Path, err)
		}
		
		fileChanges = append(fileChanges, FileChange{
			Type:    Modified,
			Path:    file.Path,
			OldFile: oldFile,
			NewFile: &file,
		})
	}

	// Convert removed files
	for _, file := range wsIndexDiff.Removed {
		fileChanges = append(fileChanges, FileChange{
			Type:    Removed,
			Path:    file.Path,
			OldFile: &file,
		})
	}

	return &WorkspaceDiff{
		FileChanges: fileChanges,
	}, nil
}

// DiffDirectories computes differences between two directory HAMTs.
func (d *Differ) DiffDirectories(oldDir, newDir hamtdir.DirRef) ([]DirectoryChange, error) {
	loader := hamtdir.NewLoader(d.CAS)

	// Get all entries from both directories
	oldEntries, err := loader.List(oldDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list old directory: %w", err)
	}

	newEntries, err := loader.List(newDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list new directory: %w", err)
	}

	// Create maps for efficient lookup
	oldMap := make(map[string]hamtdir.Entry)
	for _, entry := range oldEntries {
		oldMap[entry.Name] = entry
	}

	newMap := make(map[string]hamtdir.Entry)
	for _, entry := range newEntries {
		newMap[entry.Name] = entry
	}

	var changes []DirectoryChange

	// Find added and modified entries
	for _, newEntry := range newEntries {
		if newEntry.Type != hamtdir.DirEntry {
			continue // Only track directory changes
		}

		if oldEntry, exists := oldMap[newEntry.Name]; exists {
			if oldEntry.Type == hamtdir.DirEntry && oldEntry.Dir.Hash != newEntry.Dir.Hash {
				// Directory modified
				changes = append(changes, DirectoryChange{
					Type:   Modified,
					Path:   newEntry.Name,
					OldDir: oldEntry.Dir,
					NewDir: newEntry.Dir,
				})
			}
		} else {
			// Directory added
			changes = append(changes, DirectoryChange{
				Type:   Added,
				Path:   newEntry.Name,
				NewDir: newEntry.Dir,
			})
		}
	}

	// Find removed directories
	for _, oldEntry := range oldEntries {
		if oldEntry.Type != hamtdir.DirEntry {
			continue
		}

		if _, exists := newMap[oldEntry.Name]; !exists {
			changes = append(changes, DirectoryChange{
				Type:   Removed,
				Path:   oldEntry.Name,
				OldDir: oldEntry.Dir,
			})
		}
	}

	return changes, nil
}

// ConflictType represents the type of merge conflict.
type ConflictType uint8

const (
	FileFileConflict ConflictType = iota + 1 // Both sides modified same file
	FileDirectoryConflict                    // One side has file, other has directory
	DirectoryFileConflict                    // One side has directory, other has file
)

// Conflict represents a merge conflict.
type Conflict struct {
	Type     ConflictType
	Path     string
	BaseFile *wsindex.FileMetadata // Common ancestor file (if any)
	LeftFile *wsindex.FileMetadata // Left side file (if any)
	RightFile *wsindex.FileMetadata // Right side file (if any)
	BaseDir  *hamtdir.DirRef       // Common ancestor directory (if any)
	LeftDir  *hamtdir.DirRef       // Left side directory (if any)
	RightDir *hamtdir.DirRef       // Right side directory (if any)
}

// MergeResult represents the result of a merge operation.
type MergeResult struct {
	Success    bool
	MergedIndex *wsindex.IndexRef // Result of merge (if successful)
	Conflicts  []Conflict         // Conflicts that need resolution
}

// Merger performs three-way merges of storage structures.
type Merger struct {
	CAS cas.CAS
}

// NewMerger creates a new Merger with the given CAS.
func NewMerger(casStore cas.CAS) *Merger {
	return &Merger{CAS: casStore}
}

// MergeWorkspaces performs a three-way merge of workspace indexes.
func (m *Merger) MergeWorkspaces(base, left, right wsindex.IndexRef) (*MergeResult, error) {
	loader := wsindex.NewLoader(m.CAS)

	// Get all files from each version
	baseFiles, err := m.getFilesMap(loader, base)
	if err != nil {
		return nil, fmt.Errorf("failed to get base files: %w", err)
	}

	leftFiles, err := m.getFilesMap(loader, left)
	if err != nil {
		return nil, fmt.Errorf("failed to get left files: %w", err)
	}

	rightFiles, err := m.getFilesMap(loader, right)
	if err != nil {
		return nil, fmt.Errorf("failed to get right files: %w", err)
	}

	// Collect all paths that exist in any version
	allPaths := make(map[string]bool)
	for path := range baseFiles {
		allPaths[path] = true
	}
	for path := range leftFiles {
		allPaths[path] = true
	}
	for path := range rightFiles {
		allPaths[path] = true
	}

	var mergedFiles []wsindex.FileMetadata
	var conflicts []Conflict

	// Process each path
	for path := range allPaths {
		baseFile := baseFiles[path]
		leftFile := leftFiles[path]
		rightFile := rightFiles[path]

		conflict, mergedFile := m.mergeFile(path, baseFile, leftFile, rightFile)
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
		} else if mergedFile != nil {
			mergedFiles = append(mergedFiles, *mergedFile)
		}
		// If both conflict and mergedFile are nil, the file was deleted on both sides
	}

	if len(conflicts) > 0 {
		return &MergeResult{
			Success:   false,
			Conflicts: conflicts,
		}, nil
	}

	// Build the merged index
	builder := wsindex.NewBuilder(m.CAS)
	mergedIndex, err := builder.Build(mergedFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to build merged index: %w", err)
	}

	return &MergeResult{
		Success:     true,
		MergedIndex: &mergedIndex,
	}, nil
}

// MergeWorkspacesWithStrategy performs intelligent chunk-level merge with a strategy.
func (m *Merger) MergeWorkspacesWithStrategy(base, left, right wsindex.IndexRef, strategy StrategyType) (*MergeResult, error) {
	loader := wsindex.NewLoader(m.CAS)

	// Get all files from each version
	baseFiles, err := m.getFilesMap(loader, base)
	if err != nil {
		return nil, fmt.Errorf("failed to get base files: %w", err)
	}

	leftFiles, err := m.getFilesMap(loader, left)
	if err != nil {
		return nil, fmt.Errorf("failed to get left files: %w", err)
	}

	rightFiles, err := m.getFilesMap(loader, right)
	if err != nil {
		return nil, fmt.Errorf("failed to get right files: %w", err)
	}

	// Collect all paths
	allPaths := make(map[string]bool)
	for path := range baseFiles {
		allPaths[path] = true
	}
	for path := range leftFiles {
		allPaths[path] = true
	}
	for path := range rightFiles {
		allPaths[path] = true
	}

	// Create strategy resolver
	resolver := NewStrategyResolver(m.CAS)

	var mergedFiles []wsindex.FileMetadata
	var conflicts []Conflict

	// Process each file with the strategy
	for path := range allPaths {
		baseFile := baseFiles[path]
		leftFile := leftFiles[path]
		rightFile := rightFiles[path]

		// Use strategy resolver
		result, err := resolver.Resolve(strategy, path, baseFile, leftFile, rightFile)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", path, err)
		}

		if result.Success {
			// Successfully merged - build file metadata
			if len(result.MergedChunks) > 0 {
				// Rebuild file from merged chunks
				fileRef, err := BuildMergedFile(m.CAS, result.MergedChunks, result.MergedSize)
				if err != nil {
					return nil, fmt.Errorf("failed to build merged file %s: %w", path, err)
				}

				// Use metadata from left or right (prefer left)
				var metadata wsindex.FileMetadata
				if leftFile != nil {
					metadata = *leftFile
				} else if rightFile != nil {
					metadata = *rightFile
				} else {
					continue // Shouldn't happen, but skip if both nil
				}

				metadata.FileRef = fileRef
				metadata.Checksum = fileRef.Hash
				mergedFiles = append(mergedFiles, metadata)
			}
			// If no merged chunks, file was deleted (intentionally left out)
		} else {
			// Conflicts remain - convert to legacy Conflict format
			// Only create one conflict per file
			conflict := Conflict{
				Type: FileFileConflict,
				Path: path,
			}

			if baseFile != nil {
				conflict.BaseFile = baseFile
			}
			if leftFile != nil {
				conflict.LeftFile = leftFile
			}
			if rightFile != nil {
				conflict.RightFile = rightFile
			}

			conflicts = append(conflicts, conflict)
		}
	}

	if len(conflicts) > 0 {
		return &MergeResult{
			Success:   false,
			Conflicts: conflicts,
		}, nil
	}

	// Build merged index
	builder := wsindex.NewBuilder(m.CAS)
	mergedIndex, err := builder.Build(mergedFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to build merged index: %w", err)
	}

	return &MergeResult{
		Success:     true,
		MergedIndex: &mergedIndex,
	}, nil
}

// getFilesMap converts a workspace index to a map for easier processing.
func (m *Merger) getFilesMap(loader *wsindex.Loader, index wsindex.IndexRef) (map[string]*wsindex.FileMetadata, error) {
	if index.Count == 0 {
		return make(map[string]*wsindex.FileMetadata), nil
	}

	files, err := loader.ListAll(index)
	if err != nil {
		return nil, err
	}

	fileMap := make(map[string]*wsindex.FileMetadata)
	for i := range files {
		fileMap[files[i].Path] = &files[i]
	}

	return fileMap, nil
}

// mergeFile performs three-way merge for a single file.
func (m *Merger) mergeFile(path string, base, left, right *wsindex.FileMetadata) (*Conflict, *wsindex.FileMetadata) {
	// Case analysis for three-way merge
	baseExists := base != nil
	leftExists := left != nil
	rightExists := right != nil

	switch {
	case !baseExists && !leftExists && !rightExists:
		// File doesn't exist anywhere - shouldn't happen
		return nil, nil

	case !baseExists && leftExists && !rightExists:
		// Added on left only
		return nil, left

	case !baseExists && !leftExists && rightExists:
		// Added on right only
		return nil, right

	case !baseExists && leftExists && rightExists:
		// Added on both sides
		if m.filesEqual(left, right) {
			return nil, left // Same content, take either
		}
		// Different content - conflict
		conflict := &Conflict{
			Type:      FileFileConflict,
			Path:      path,
			LeftFile:  left,
			RightFile: right,
		}
		return conflict, nil

	case baseExists && !leftExists && !rightExists:
		// Deleted on both sides
		return nil, nil

	case baseExists && leftExists && !rightExists:
		// Modified on left, deleted on right
		if m.filesEqual(base, left) {
			// No change on left, deleted on right
			return nil, nil
		}
		// Modified on left, deleted on right - conflict
		conflict := &Conflict{
			Type:     FileFileConflict,
			Path:     path,
			BaseFile: base,
			LeftFile: left,
		}
		return conflict, nil

	case baseExists && !leftExists && rightExists:
		// Deleted on left, modified on right
		if m.filesEqual(base, right) {
			// Deleted on left, no change on right
			return nil, nil
		}
		// Deleted on left, modified on right - conflict
		conflict := &Conflict{
			Type:      FileFileConflict,
			Path:      path,
			BaseFile:  base,
			RightFile: right,
		}
		return conflict, nil

	case baseExists && leftExists && rightExists:
		// Exists in all three versions
		if m.filesEqual(left, right) {
			// Both sides made same change (or no change)
			return nil, left
		}
		
		if m.filesEqual(base, left) {
			// No change on left, take right
			return nil, right
		}
		
		if m.filesEqual(base, right) {
			// No change on right, take left
			return nil, left
		}
		
		// Both sides changed - conflict
		conflict := &Conflict{
			Type:      FileFileConflict,
			Path:      path,
			BaseFile:  base,
			LeftFile:  left,
			RightFile: right,
		}
		return conflict, nil
	}

	return nil, nil
}

// filesEqual checks if two file metadata entries are equal.
func (m *Merger) filesEqual(a, b *wsindex.FileMetadata) bool {
	if a == nil || b == nil {
		return a == b
	}
	
	return a.Path == b.Path &&
		a.FileRef.Hash == b.FileRef.Hash &&
		a.FileRef.Kind == b.FileRef.Kind &&
		a.FileRef.Size == b.FileRef.Size &&
		a.Checksum == b.Checksum
		// Note: We don't compare ModTime and Mode for merge equality
}

// Patch represents a set of changes to apply to a workspace.
type Patch struct {
	Description string
	Changes     []FileChange
}

// Patcher applies patches to workspace indexes.
type Patcher struct {
	CAS cas.CAS
}

// NewPatcher creates a new Patcher with the given CAS.
func NewPatcher(casStore cas.CAS) *Patcher {
	return &Patcher{CAS: casStore}
}

// CreatePatch creates a patch from a workspace diff.
func (p *Patcher) CreatePatch(description string, diff *WorkspaceDiff) *Patch {
	return &Patch{
		Description: description,
		Changes:     diff.FileChanges,
	}
}

// ApplyPatch applies a patch to a workspace index.
func (p *Patcher) ApplyPatch(index wsindex.IndexRef, patch *Patch) (wsindex.IndexRef, error) {
	loader := wsindex.NewLoader(p.CAS)
	builder := wsindex.NewBuilder(p.CAS)

	// Get current files
	currentFiles, err := loader.ListAll(index)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to list current files: %w", err)
	}

	// Create a map for efficient updates
	fileMap := make(map[string]*wsindex.FileMetadata)
	for i := range currentFiles {
		fileMap[currentFiles[i].Path] = &currentFiles[i]
	}

	// Apply changes
	for _, change := range patch.Changes {
		switch change.Type {
		case Added:
			if change.NewFile != nil {
				fileMap[change.Path] = change.NewFile
			}
		case Modified:
			if change.NewFile != nil {
				fileMap[change.Path] = change.NewFile
			}
		case Removed:
			delete(fileMap, change.Path)
		}
	}

	// Convert back to slice
	var updatedFiles []wsindex.FileMetadata
	for _, file := range fileMap {
		updatedFiles = append(updatedFiles, *file)
	}

	// Build new index
	return builder.Build(updatedFiles)
}

// Analyzer provides higher-level analysis of diffs and merges.
type Analyzer struct {
	CAS cas.CAS
}

// NewAnalyzer creates a new Analyzer with the given CAS.
func NewAnalyzer(casStore cas.CAS) *Analyzer {
	return &Analyzer{CAS: casStore}
}

// AnalyzeChanges provides detailed analysis of workspace changes.
func (a *Analyzer) AnalyzeChanges(diff *WorkspaceDiff) map[string]interface{} {
	analysis := make(map[string]interface{})

	// Count changes by type
	var addedCount, modifiedCount, removedCount int
	for _, change := range diff.FileChanges {
		switch change.Type {
		case Added:
			addedCount++
		case Modified:
			modifiedCount++
		case Removed:
			removedCount++
		}
	}

	analysis["file_changes"] = map[string]int{
		"added":    addedCount,
		"modified": modifiedCount,
		"removed":  removedCount,
		"total":    len(diff.FileChanges),
	}

	// Analyze by file extension
	extensionStats := make(map[string]int)
	for _, change := range diff.FileChanges {
		ext := filepath.Ext(change.Path)
		if ext == "" {
			ext = "(no extension)"
		}
		extensionStats[ext]++
	}
	analysis["by_extension"] = extensionStats

	// Analyze by directory
	dirStats := make(map[string]int)
	for _, change := range diff.FileChanges {
		dir := filepath.Dir(change.Path)
		if dir == "." {
			dir = "(root)"
		}
		dirStats[dir]++
	}
	analysis["by_directory"] = dirStats

	return analysis
}

// GetConflictSummary provides a summary of merge conflicts.
func (a *Analyzer) GetConflictSummary(conflicts []Conflict) map[string]interface{} {
	summary := make(map[string]interface{})

	// Count by type
	var fileFileCount, fileDirCount, dirFileCount int
	for _, conflict := range conflicts {
		switch conflict.Type {
		case FileFileConflict:
			fileFileCount++
		case FileDirectoryConflict:
			fileDirCount++
		case DirectoryFileConflict:
			dirFileCount++
		}
	}

	summary["by_type"] = map[string]int{
		"file_file":      fileFileCount,
		"file_directory": fileDirCount,
		"directory_file": dirFileCount,
		"total":          len(conflicts),
	}

	// List conflict paths
	var conflictPaths []string
	for _, conflict := range conflicts {
		conflictPaths = append(conflictPaths, conflict.Path)
	}
	sort.Strings(conflictPaths)
	summary["paths"] = conflictPaths

	return summary
}

// DetectRenames detects if files were renamed between two workspace states.
func (a *Analyzer) DetectRenames(diff *WorkspaceDiff, threshold float64) []RenameDetection {
	var renames []RenameDetection

	// Group changes by type
	var added, removed []FileChange
	for _, change := range diff.FileChanges {
		switch change.Type {
		case Added:
			added = append(added, change)
		case Removed:
			removed = append(removed, change)
		}
	}

	// Compare each removed file with each added file
	for _, removedFile := range removed {
		if removedFile.OldFile == nil {
			continue
		}

		for _, addedFile := range added {
			if addedFile.NewFile == nil {
				continue
			}

			// Check if content is similar (same hash indicates exact match)
			if removedFile.OldFile.FileRef.Hash == addedFile.NewFile.FileRef.Hash {
				renames = append(renames, RenameDetection{
					OldPath:    removedFile.Path,
					NewPath:    addedFile.Path,
					Similarity: 1.0, // Exact match
				})
			}
		}
	}

	// Filter by threshold
	var filtered []RenameDetection
	for _, rename := range renames {
		if rename.Similarity >= threshold {
			filtered = append(filtered, rename)
		}
	}

	return filtered
}

// RenameDetection represents a detected file rename.
type RenameDetection struct {
	OldPath    string
	NewPath    string
	Similarity float64 // 0.0 to 1.0, where 1.0 is exact match
}