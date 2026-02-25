package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/ignore"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// DiffChangeType mirrors diffmerge.ChangeType for the engine API
type DiffChangeType uint8

const (
	DiffAdded    DiffChangeType = 1
	DiffModified DiffChangeType = 2
	DiffRemoved  DiffChangeType = 3
)

// FileDiff holds diff information for a single file
type FileDiff struct {
	Path      string
	Type      DiffChangeType
	OldSize   int64
	NewSize   int64
	Hunks     []DiffHunk // Line-level hunks (populated for text files)
	AddedLines   int
	RemovedLines int
	IsBinary  bool
}

// DiffHunk represents a chunk of line changes
type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// DiffLine represents a single line in a diff hunk
type DiffLine struct {
	Type    DiffLineType
	Content string
}

// DiffLineType indicates whether a line was added, removed, or context
type DiffLineType uint8

const (
	DiffLineContext DiffLineType = iota
	DiffLineAdd
	DiffLineRemove
)

// DiffOptions controls what to diff
type DiffOptions struct {
	Staged bool // If true, diff staged vs HEAD; otherwise working dir vs HEAD
}

// DiffResult holds the complete diff output
type DiffResult struct {
	OldName string
	NewName string
	Files   []FileDiff
	Stats   DiffStats
}

// DiffStats holds summary statistics
type DiffStats struct {
	Added    int
	Modified int
	Removed  int
}

// ComputeDiff computes the diff for the working directory
func ComputeDiff(ivaldiDir, workDir string, opts DiffOptions) (*DiffResult, error) {
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize refs: %w", err)
	}
	defer refsManager.Close()

	// Get HEAD index
	headIndex, err := getHeadIndex(casStore, ivaldiDir, refsManager)
	if err != nil {
		return nil, err
	}

	var workingIndex wsindex.IndexRef
	var oldName, newName string

	if opts.Staged {
		// Staged vs HEAD
		stagedFiles, err := GetStagedFiles(ivaldiDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get staged files: %w", err)
		}
		if len(stagedFiles) == 0 {
			return &DiffResult{
				OldName: "HEAD",
				NewName: "staged",
			}, nil
		}

		materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
		workingIndex, err = materializer.ScanSpecificFiles(stagedFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to scan staged files: %w", err)
		}
		oldName = "HEAD"
		newName = "staged"
	} else {
		// Working directory vs HEAD
		materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
		ignoreCache, _ := ignore.LoadPatternCache(workDir)
		materializer.SetIgnorePatterns(ignoreCache)
		workingIndex, err = materializer.ScanWorkspace()
		if err != nil {
			return nil, fmt.Errorf("failed to scan workspace: %w", err)
		}
		oldName = "HEAD"
		newName = "working directory"
	}

	differ := diffmerge.NewDiffer(casStore)
	wsDiff, err := differ.DiffWorkspaces(headIndex, workingIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to compute diff: %w", err)
	}

	result := &DiffResult{
		OldName: oldName,
		NewName: newName,
	}

	for _, change := range wsDiff.FileChanges {
		fileDiff := buildFileDiff(casStore, change)
		result.Files = append(result.Files, fileDiff)

		switch change.Type {
		case diffmerge.Added:
			result.Stats.Added++
		case diffmerge.Modified:
			result.Stats.Modified++
		case diffmerge.Removed:
			result.Stats.Removed++
		}
	}

	return result, nil
}

// buildFileDiff creates a FileDiff from a FileChange, including line-level diffs
func buildFileDiff(casStore cas.CAS, change diffmerge.FileChange) FileDiff {
	fd := FileDiff{
		Path: change.Path,
	}

	switch change.Type {
	case diffmerge.Added:
		fd.Type = DiffAdded
		if change.NewFile != nil {
			fd.NewSize = change.NewFile.FileRef.Size
			content, err := readContent(casStore, change.NewFile)
			if err == nil && !isBinaryContent(content) {
				lines := strings.Split(string(content), "\n")
				fd.AddedLines = len(lines)
				fd.Hunks = []DiffHunk{buildAddHunk(lines)}
			} else {
				fd.IsBinary = true
			}
		}

	case diffmerge.Removed:
		fd.Type = DiffRemoved
		if change.OldFile != nil {
			fd.OldSize = change.OldFile.FileRef.Size
			content, err := readContent(casStore, change.OldFile)
			if err == nil && !isBinaryContent(content) {
				lines := strings.Split(string(content), "\n")
				fd.RemovedLines = len(lines)
				fd.Hunks = []DiffHunk{buildRemoveHunk(lines)}
			} else {
				fd.IsBinary = true
			}
		}

	case diffmerge.Modified:
		fd.Type = DiffModified
		if change.OldFile != nil {
			fd.OldSize = change.OldFile.FileRef.Size
		}
		if change.NewFile != nil {
			fd.NewSize = change.NewFile.FileRef.Size
		}

		if change.OldFile != nil && change.NewFile != nil {
			oldContent, errOld := readContent(casStore, change.OldFile)
			newContent, errNew := readContent(casStore, change.NewFile)

			if errOld == nil && errNew == nil && !isBinaryContent(oldContent) && !isBinaryContent(newContent) {
				oldLines := strings.Split(string(oldContent), "\n")
				newLines := strings.Split(string(newContent), "\n")
				hunks := computeUnifiedDiff(oldLines, newLines, 3)
				fd.Hunks = hunks

				for _, h := range hunks {
					for _, l := range h.Lines {
						switch l.Type {
						case DiffLineAdd:
							fd.AddedLines++
						case DiffLineRemove:
							fd.RemovedLines++
						}
					}
				}
			} else {
				fd.IsBinary = true
			}
		}
	}

	return fd
}

// readContent reads file content from CAS
func readContent(casStore cas.CAS, file *wsindex.FileMetadata) ([]byte, error) {
	loader := filechunk.NewLoader(casStore)
	return loader.ReadAll(file.FileRef)
}

// isBinaryContent checks if content appears to be binary
func isBinaryContent(data []byte) bool {
	// Check first 8KB for null bytes
	checkLen := len(data)
	if checkLen > 8192 {
		checkLen = 8192
	}
	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// buildAddHunk creates a hunk for an entirely new file
func buildAddHunk(lines []string) DiffHunk {
	h := DiffHunk{
		OldStart: 0,
		OldCount: 0,
		NewStart: 1,
		NewCount: len(lines),
	}
	for _, line := range lines {
		h.Lines = append(h.Lines, DiffLine{Type: DiffLineAdd, Content: line})
	}
	return h
}

// buildRemoveHunk creates a hunk for an entirely removed file
func buildRemoveHunk(lines []string) DiffHunk {
	h := DiffHunk{
		OldStart: 1,
		OldCount: len(lines),
		NewStart: 0,
		NewCount: 0,
	}
	for _, line := range lines {
		h.Lines = append(h.Lines, DiffLine{Type: DiffLineRemove, Content: line})
	}
	return h
}

// computeUnifiedDiff computes unified diff hunks with context lines
func computeUnifiedDiff(oldLines, newLines []string, contextLines int) []DiffHunk {
	// Compute LCS-based edit script
	edits := myersDiff(oldLines, newLines)

	if len(edits) == 0 {
		return nil
	}

	// Group edits into hunks with context
	return groupIntoHunks(edits, oldLines, newLines, contextLines)
}

// editOp represents a single edit operation
type editOp struct {
	oldIdx int
	newIdx int
	kind   DiffLineType // DiffLineContext, DiffLineAdd, DiffLineRemove
}

// myersDiff computes the edit script between two slices of strings
// using a simplified O(ND) approach
func myersDiff(oldLines, newLines []string) []editOp {
	m := len(oldLines)
	n := len(newLines)

	// Build a simple LCS-based diff
	// Use dynamic programming for the LCS
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = lcs[i+1][j]
				if lcs[i][j+1] > lcs[i][j] {
					lcs[i][j] = lcs[i][j+1]
				}
			}
		}
	}

	// Trace back to get edit operations
	var edits []editOp
	i, j := 0, 0
	for i < m || j < n {
		if i < m && j < n && oldLines[i] == newLines[j] {
			edits = append(edits, editOp{oldIdx: i, newIdx: j, kind: DiffLineContext})
			i++
			j++
		} else if j < n && (i >= m || lcs[i][j+1] >= lcs[i+1][j]) {
			edits = append(edits, editOp{oldIdx: -1, newIdx: j, kind: DiffLineAdd})
			j++
		} else if i < m {
			edits = append(edits, editOp{oldIdx: i, newIdx: -1, kind: DiffLineRemove})
			i++
		}
	}

	return edits
}

// groupIntoHunks groups edit operations into hunks with context
func groupIntoHunks(edits []editOp, oldLines, newLines []string, contextLines int) []DiffHunk {
	// Find ranges of non-context edits
	type changeRange struct {
		start, end int // indices into edits
	}
	var ranges []changeRange

	inChange := false
	var currentRange changeRange
	for i, edit := range edits {
		if edit.kind != DiffLineContext {
			if !inChange {
				currentRange = changeRange{start: i, end: i}
				inChange = true
			} else {
				currentRange.end = i
			}
		} else if inChange {
			// Check if context gap is large enough to split
			gapStart := currentRange.end + 1
			gapEnd := i
			if gapEnd-gapStart >= contextLines*2 {
				ranges = append(ranges, currentRange)
				inChange = false
			} else {
				currentRange.end = i // extend through context
			}
		}
	}
	if inChange {
		ranges = append(ranges, currentRange)
	}

	// Build hunks from ranges
	var hunks []DiffHunk
	for _, r := range ranges {
		// Expand range to include context
		start := r.start - contextLines
		if start < 0 {
			start = 0
		}
		end := r.end + contextLines + 1
		if end > len(edits) {
			end = len(edits)
		}

		var hunk DiffHunk
		hunk.OldStart = -1
		hunk.NewStart = -1

		for i := start; i < end; i++ {
			edit := edits[i]

			switch edit.kind {
			case DiffLineContext:
				if hunk.OldStart < 0 {
					hunk.OldStart = edit.oldIdx + 1
				}
				if hunk.NewStart < 0 {
					hunk.NewStart = edit.newIdx + 1
				}
				hunk.OldCount++
				hunk.NewCount++
				hunk.Lines = append(hunk.Lines, DiffLine{
					Type:    DiffLineContext,
					Content: oldLines[edit.oldIdx],
				})

			case DiffLineAdd:
				if hunk.NewStart < 0 {
					hunk.NewStart = edit.newIdx + 1
				}
				if hunk.OldStart < 0 && edit.newIdx > 0 {
					hunk.OldStart = 1
				} else if hunk.OldStart < 0 {
					hunk.OldStart = 0
				}
				hunk.NewCount++
				hunk.Lines = append(hunk.Lines, DiffLine{
					Type:    DiffLineAdd,
					Content: newLines[edit.newIdx],
				})

			case DiffLineRemove:
				if hunk.OldStart < 0 {
					hunk.OldStart = edit.oldIdx + 1
				}
				if hunk.NewStart < 0 && edit.oldIdx > 0 {
					hunk.NewStart = 1
				} else if hunk.NewStart < 0 {
					hunk.NewStart = 0
				}
				hunk.OldCount++
				hunk.Lines = append(hunk.Lines, DiffLine{
					Type:    DiffLineRemove,
					Content: oldLines[edit.oldIdx],
				})
			}
		}

		if len(hunk.Lines) > 0 {
			hunks = append(hunks, hunk)
		}
	}

	return hunks
}

// getHeadIndex returns the workspace index for the HEAD commit
func getHeadIndex(casStore cas.CAS, ivaldiDir string, refsManager *refs.RefsManager) (wsindex.IndexRef, error) {
	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to get current timeline: %w", err)
	}

	timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to get timeline: %w", err)
	}

	if timeline.Blake3Hash == [32]byte{} {
		wsBuilder := wsindex.NewBuilder(casStore)
		return wsBuilder.Build(nil)
	}

	return getCommitIndex(casStore, timeline.Blake3Hash)
}

// getCommitIndex returns the workspace index for a commit
func getCommitIndex(casStore cas.CAS, commitHash [32]byte) (wsindex.IndexRef, error) {
	var hash cas.Hash
	copy(hash[:], commitHash[:])

	commitReader := commit.NewCommitReader(casStore)
	commitObj, err := commitReader.ReadCommit(hash)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to read commit: %w", err)
	}

	tree, err := commitReader.ReadTree(commitObj)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to read tree: %w", err)
	}

	files, err := commitReader.TreeToFileMetadata(tree)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to convert tree to metadata: %w", err)
	}

	wsBuilder := wsindex.NewBuilder(casStore)
	return wsBuilder.Build(files)
}
