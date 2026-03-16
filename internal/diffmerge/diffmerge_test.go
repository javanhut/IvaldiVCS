package diffmerge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/hamtdir"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

func createTestFileMetadata(path, content string) wsindex.FileMetadata {
	contentBytes := []byte(content)
	hash := cas.SumB3(contentBytes)
	
	return wsindex.FileMetadata{
		Path: path,
		FileRef: filechunk.NodeRef{
			Hash: hash,
			Kind: filechunk.Leaf,
			Size: int64(len(contentBytes)),
		},
		ModTime:  time.Unix(1640995200, 0), // 2022-01-01
		Mode:     0644,
		Size:     int64(len(contentBytes)),
		Checksum: hash,
	}
}

func TestDiffWorkspaces(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	differ := NewDiffer(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	// Create old workspace
	oldFiles := []wsindex.FileMetadata{
		createTestFileMetadata("file1.txt", "old content 1"),
		createTestFileMetadata("file2.txt", "content 2"),
		createTestFileMetadata("file3.txt", "content 3"),
	}

	oldIndex, err := wsBuilder.Build(oldFiles)
	if err != nil {
		t.Fatalf("Build old workspace failed: %v", err)
	}

	// Create new workspace with changes
	newFiles := []wsindex.FileMetadata{
		createTestFileMetadata("file1.txt", "new content 1"), // Modified
		createTestFileMetadata("file2.txt", "content 2"),      // Unchanged
		createTestFileMetadata("file4.txt", "content 4"),      // Added
		// file3.txt removed
	}

	newIndex, err := wsBuilder.Build(newFiles)
	if err != nil {
		t.Fatalf("Build new workspace failed: %v", err)
	}

	// Compute diff
	diff, err := differ.DiffWorkspaces(oldIndex, newIndex)
	if err != nil {
		t.Fatalf("DiffWorkspaces failed: %v", err)
	}

	// Check results
	var addedCount, modifiedCount, removedCount int
	for _, change := range diff.FileChanges {
		switch change.Type {
		case Added:
			addedCount++
			if change.Path != "file4.txt" {
				t.Errorf("Added file: expected file4.txt, got %s", change.Path)
			}
		case Modified:
			modifiedCount++
			if change.Path != "file1.txt" {
				t.Errorf("Modified file: expected file1.txt, got %s", change.Path)
			}
		case Removed:
			removedCount++
			if change.Path != "file3.txt" {
				t.Errorf("Removed file: expected file3.txt, got %s", change.Path)
			}
		}
	}

	if addedCount != 1 {
		t.Errorf("Expected 1 added file, got %d", addedCount)
	}
	if modifiedCount != 1 {
		t.Errorf("Expected 1 modified file, got %d", modifiedCount)
	}
	if removedCount != 1 {
		t.Errorf("Expected 1 removed file, got %d", removedCount)
	}
}

func TestDiffDirectories(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	differ := NewDiffer(casStore)
	hamtBuilder := hamtdir.NewBuilder(casStore)

	// Create subdirectory
	subEntries := []hamtdir.Entry{
		{
			Name: "subfile.txt",
			Type: hamtdir.FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("sub content")),
				Kind: filechunk.Leaf,
				Size: 11,
			},
		},
	}

	subDir, err := hamtBuilder.Build(subEntries)
	if err != nil {
		t.Fatalf("Build subdirectory failed: %v", err)
	}

	// Create old directory
	oldEntries := []hamtdir.Entry{
		{
			Name: "file1.txt",
			Type: hamtdir.FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content1")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
		{
			Name: "subdir",
			Type: hamtdir.DirEntry,
			Dir:  &subDir,
		},
	}

	oldDir, err := hamtBuilder.Build(oldEntries)
	if err != nil {
		t.Fatalf("Build old directory failed: %v", err)
	}

	// Create new directory (modify subdirectory)
	newSubEntries := []hamtdir.Entry{
		{
			Name: "subfile.txt",
			Type: hamtdir.FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("new sub content")),
				Kind: filechunk.Leaf,
				Size: 15,
			},
		},
	}

	newSubDir, err := hamtBuilder.Build(newSubEntries)
	if err != nil {
		t.Fatalf("Build new subdirectory failed: %v", err)
	}

	newEntries := []hamtdir.Entry{
		{
			Name: "file1.txt",
			Type: hamtdir.FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content1")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
		{
			Name: "subdir",
			Type: hamtdir.DirEntry,
			Dir:  &newSubDir,
		},
		{
			Name: "newdir",
			Type: hamtdir.DirEntry,
			Dir:  &subDir, // Reuse old subdir
		},
	}

	newDir, err := hamtBuilder.Build(newEntries)
	if err != nil {
		t.Fatalf("Build new directory failed: %v", err)
	}

	// Compute diff
	changes, err := differ.DiffDirectories(oldDir, newDir)
	if err != nil {
		t.Fatalf("DiffDirectories failed: %v", err)
	}

	// Should have one modified directory (subdir) and one added directory (newdir)
	var modifiedCount, addedCount int
	for _, change := range changes {
		switch change.Type {
		case Modified:
			modifiedCount++
			if change.Path != "subdir" {
				t.Errorf("Modified directory: expected subdir, got %s", change.Path)
			}
		case Added:
			addedCount++
			if change.Path != "newdir" {
				t.Errorf("Added directory: expected newdir, got %s", change.Path)
			}
		}
	}

	if modifiedCount != 1 {
		t.Errorf("Expected 1 modified directory, got %d", modifiedCount)
	}
	if addedCount != 1 {
		t.Errorf("Expected 1 added directory, got %d", addedCount)
	}
}

func TestMergeWorkspaces(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	merger := NewMerger(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	// Create base workspace
	baseFiles := []wsindex.FileMetadata{
		createTestFileMetadata("file1.txt", "base content 1"),
		createTestFileMetadata("file2.txt", "base content 2"),
		createTestFileMetadata("file3.txt", "base content 3"),
	}

	baseIndex, err := wsBuilder.Build(baseFiles)
	if err != nil {
		t.Fatalf("Build base workspace failed: %v", err)
	}

	// Create left workspace (modify file1, add file4)
	leftFiles := []wsindex.FileMetadata{
		createTestFileMetadata("file1.txt", "left content 1"), // Modified
		createTestFileMetadata("file2.txt", "base content 2"),  // Unchanged
		createTestFileMetadata("file3.txt", "base content 3"),  // Unchanged
		createTestFileMetadata("file4.txt", "left content 4"),  // Added
	}

	leftIndex, err := wsBuilder.Build(leftFiles)
	if err != nil {
		t.Fatalf("Build left workspace failed: %v", err)
	}

	// Create right workspace (modify file2, add file5)
	rightFiles := []wsindex.FileMetadata{
		createTestFileMetadata("file1.txt", "base content 1"),  // Unchanged
		createTestFileMetadata("file2.txt", "right content 2"), // Modified
		createTestFileMetadata("file3.txt", "base content 3"),  // Unchanged
		createTestFileMetadata("file5.txt", "right content 5"), // Added
	}

	rightIndex, err := wsBuilder.Build(rightFiles)
	if err != nil {
		t.Fatalf("Build right workspace failed: %v", err)
	}

	// Perform merge
	result, err := merger.MergeWorkspaces(baseIndex, leftIndex, rightIndex)
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}

	// Should succeed without conflicts
	if !result.Success {
		t.Fatalf("Expected successful merge, got conflicts: %v", result.Conflicts)
	}

	if result.MergedIndex == nil {
		t.Fatal("Expected merged index, got nil")
	}

	// Verify merged content
	loader := wsindex.NewLoader(casStore)
	mergedFiles, err := loader.ListAll(*result.MergedIndex)
	if err != nil {
		t.Fatalf("List merged files failed: %v", err)
	}

	// Should have 5 files: file1(left), file2(right), file3(base), file4(left), file5(right)
	if len(mergedFiles) != 5 {
		t.Fatalf("Expected 5 merged files, got %d", len(mergedFiles))
	}

	fileMap := make(map[string]wsindex.FileMetadata)
	for _, file := range mergedFiles {
		fileMap[file.Path] = file
	}

	// Check specific content
	if file1, exists := fileMap["file1.txt"]; !exists {
		t.Error("file1.txt missing from merge")
	} else if file1.FileRef.Hash != cas.SumB3([]byte("left content 1")) {
		t.Error("file1.txt should have left content")
	}

	if file2, exists := fileMap["file2.txt"]; !exists {
		t.Error("file2.txt missing from merge")
	} else if file2.FileRef.Hash != cas.SumB3([]byte("right content 2")) {
		t.Error("file2.txt should have right content")
	}
}

func TestMergeConflicts(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	merger := NewMerger(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	// Create base workspace
	baseFiles := []wsindex.FileMetadata{
		createTestFileMetadata("conflict.txt", "base content"),
	}

	baseIndex, err := wsBuilder.Build(baseFiles)
	if err != nil {
		t.Fatalf("Build base workspace failed: %v", err)
	}

	// Create left workspace (modify file)
	leftFiles := []wsindex.FileMetadata{
		createTestFileMetadata("conflict.txt", "left content"),
	}

	leftIndex, err := wsBuilder.Build(leftFiles)
	if err != nil {
		t.Fatalf("Build left workspace failed: %v", err)
	}

	// Create right workspace (modify file differently)
	rightFiles := []wsindex.FileMetadata{
		createTestFileMetadata("conflict.txt", "right content"),
	}

	rightIndex, err := wsBuilder.Build(rightFiles)
	if err != nil {
		t.Fatalf("Build right workspace failed: %v", err)
	}

	// Perform merge
	result, err := merger.MergeWorkspaces(baseIndex, leftIndex, rightIndex)
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}

	// Should have conflicts
	if result.Success {
		t.Fatal("Expected merge conflicts, got success")
	}

	if len(result.Conflicts) != 1 {
		t.Fatalf("Expected 1 conflict, got %d", len(result.Conflicts))
	}

	conflict := result.Conflicts[0]
	if conflict.Type != FileFileConflict {
		t.Errorf("Expected FileFileConflict, got %v", conflict.Type)
	}
	if conflict.Path != "conflict.txt" {
		t.Errorf("Expected conflict path 'conflict.txt', got %s", conflict.Path)
	}
}

func TestApplyPatch(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	patcher := NewPatcher(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	// Create base workspace
	baseFiles := []wsindex.FileMetadata{
		createTestFileMetadata("file1.txt", "content 1"),
		createTestFileMetadata("file2.txt", "content 2"),
	}

	baseIndex, err := wsBuilder.Build(baseFiles)
	if err != nil {
		t.Fatalf("Build base workspace failed: %v", err)
	}

	// Create patch
	newFile := createTestFileMetadata("file3.txt", "new content")
	modifiedFile := createTestFileMetadata("file1.txt", "modified content")
	
	patch := &Patch{
		Description: "Test patch",
		Changes: []FileChange{
			{
				Type:    Added,
				Path:    "file3.txt",
				NewFile: &newFile,
			},
			{
				Type:    Modified,
				Path:    "file1.txt",
				NewFile: &modifiedFile,
			},
			{
				Type: Removed,
				Path: "file2.txt",
			},
		},
	}

	// Apply patch
	patchedIndex, err := patcher.ApplyPatch(baseIndex, patch)
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}

	// Verify results
	loader := wsindex.NewLoader(casStore)
	patchedFiles, err := loader.ListAll(patchedIndex)
	if err != nil {
		t.Fatalf("List patched files failed: %v", err)
	}

	if len(patchedFiles) != 2 {
		t.Fatalf("Expected 2 files after patch, got %d", len(patchedFiles))
	}

	fileMap := make(map[string]wsindex.FileMetadata)
	for _, file := range patchedFiles {
		fileMap[file.Path] = file
	}

	// Check file1 was modified
	if file1, exists := fileMap["file1.txt"]; !exists {
		t.Error("file1.txt missing after patch")
	} else if file1.FileRef.Hash != cas.SumB3([]byte("modified content")) {
		t.Error("file1.txt should have modified content")
	}

	// Check file3 was added
	if file3, exists := fileMap["file3.txt"]; !exists {
		t.Error("file3.txt missing after patch")
	} else if file3.FileRef.Hash != cas.SumB3([]byte("new content")) {
		t.Error("file3.txt should have new content")
	}

	// Check file2 was removed
	if _, exists := fileMap["file2.txt"]; exists {
		t.Error("file2.txt should have been removed")
	}
}

func TestAnalyzeChanges(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	analyzer := NewAnalyzer(casStore)

	// Create test diff
	diff := &WorkspaceDiff{
		FileChanges: []FileChange{
			{Type: Added, Path: "src/main.go"},
			{Type: Added, Path: "src/util.go"},
			{Type: Modified, Path: "README.md"},
			{Type: Modified, Path: "docs/guide.md"},
			{Type: Removed, Path: "old/legacy.txt"},
		},
	}

	analysis := analyzer.AnalyzeChanges(diff)

	// Check file change counts
	fileChanges := analysis["file_changes"].(map[string]int)
	if fileChanges["added"] != 2 {
		t.Errorf("Expected 2 added files, got %d", fileChanges["added"])
	}
	if fileChanges["modified"] != 2 {
		t.Errorf("Expected 2 modified files, got %d", fileChanges["modified"])
	}
	if fileChanges["removed"] != 1 {
		t.Errorf("Expected 1 removed file, got %d", fileChanges["removed"])
	}
	if fileChanges["total"] != 5 {
		t.Errorf("Expected 5 total changes, got %d", fileChanges["total"])
	}

	// Check extension analysis
	byExtension := analysis["by_extension"].(map[string]int)
	if byExtension[".go"] != 2 {
		t.Errorf("Expected 2 .go files, got %d", byExtension[".go"])
	}
	if byExtension[".md"] != 2 {
		t.Errorf("Expected 2 .md files, got %d", byExtension[".md"])
	}

	// Check directory analysis
	byDirectory := analysis["by_directory"].(map[string]int)
	if byDirectory["src"] != 2 {
		t.Errorf("Expected 2 files in src/, got %d", byDirectory["src"])
	}
}

func TestDetectRenames(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	analyzer := NewAnalyzer(casStore)

	// Create diff with potential rename
	oldFile := createTestFileMetadata("old/file.txt", "same content")
	newFile := createTestFileMetadata("new/file.txt", "same content")

	diff := &WorkspaceDiff{
		FileChanges: []FileChange{
			{
				Type:    Removed,
				Path:    "old/file.txt",
				OldFile: &oldFile,
			},
			{
				Type:    Added,
				Path:    "new/file.txt",
				NewFile: &newFile,
			},
		},
	}

	renames := analyzer.DetectRenames(diff, 0.8) // 80% threshold

	if len(renames) != 1 {
		t.Fatalf("Expected 1 rename detection, got %d", len(renames))
	}

	rename := renames[0]
	if rename.OldPath != "old/file.txt" {
		t.Errorf("Expected old path 'old/file.txt', got %s", rename.OldPath)
	}
	if rename.NewPath != "new/file.txt" {
		t.Errorf("Expected new path 'new/file.txt', got %s", rename.NewPath)
	}
	if rename.Similarity != 1.0 {
		t.Errorf("Expected similarity 1.0, got %f", rename.Similarity)
	}
}

// --- Additional three-way merge scenario tests ---

func TestMerge_AddedOnBothSides_SameContent(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	merger := NewMerger(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	baseIndex, _ := wsBuilder.Build(nil)

	sameFile := createTestFileMetadata("newfile.txt", "same content")
	leftIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{sameFile})
	rightIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{sameFile})

	result, err := merger.MergeWorkspaces(baseIndex, leftIndex, rightIndex)
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Expected success when both sides add identical file")
	}

	loader := wsindex.NewLoader(casStore)
	files, _ := loader.ListAll(*result.MergedIndex)
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}
	if files[0].Path != "newfile.txt" {
		t.Errorf("Expected newfile.txt, got %s", files[0].Path)
	}
}

func TestMerge_AddedOnBothSides_DifferentContent(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	merger := NewMerger(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	baseIndex, _ := wsBuilder.Build(nil)

	leftFile := createTestFileMetadata("conflict.txt", "left version")
	rightFile := createTestFileMetadata("conflict.txt", "right version")
	leftIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{leftFile})
	rightIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{rightFile})

	result, err := merger.MergeWorkspaces(baseIndex, leftIndex, rightIndex)
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}
	if result.Success {
		t.Fatal("Expected conflict when both sides add different content")
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("Expected 1 conflict, got %d", len(result.Conflicts))
	}
	if result.Conflicts[0].Path != "conflict.txt" {
		t.Errorf("Expected conflict on conflict.txt, got %s", result.Conflicts[0].Path)
	}
}

func TestMerge_DeletedOnBothSides(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	merger := NewMerger(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	baseFile := createTestFileMetadata("todelete.txt", "content")
	baseIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{baseFile})
	emptyIndex, _ := wsBuilder.Build(nil)

	result, err := merger.MergeWorkspaces(baseIndex, emptyIndex, emptyIndex)
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Expected success when both sides delete")
	}

	loader := wsindex.NewLoader(casStore)
	files, _ := loader.ListAll(*result.MergedIndex)
	if len(files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(files))
	}
}

func TestMerge_ModifiedLeftDeletedRight(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	merger := NewMerger(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	baseFile := createTestFileMetadata("file.txt", "base")
	modifiedFile := createTestFileMetadata("file.txt", "modified on left")
	baseIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{baseFile})
	leftIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{modifiedFile})
	rightIndex, _ := wsBuilder.Build(nil) // deleted on right

	result, err := merger.MergeWorkspaces(baseIndex, leftIndex, rightIndex)
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}
	if result.Success {
		t.Fatal("Expected conflict: modified on left, deleted on right")
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("Expected 1 conflict, got %d", len(result.Conflicts))
	}
}

func TestMerge_DeletedLeftModifiedRight(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	merger := NewMerger(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	baseFile := createTestFileMetadata("file.txt", "base")
	modifiedFile := createTestFileMetadata("file.txt", "modified on right")
	baseIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{baseFile})
	leftIndex, _ := wsBuilder.Build(nil) // deleted on left
	rightIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{modifiedFile})

	result, err := merger.MergeWorkspaces(baseIndex, leftIndex, rightIndex)
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}
	if result.Success {
		t.Fatal("Expected conflict: deleted on left, modified on right")
	}
}

func TestMerge_UnchangedLeftModifiedRight(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	merger := NewMerger(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	baseFile := createTestFileMetadata("file.txt", "base content")
	modifiedFile := createTestFileMetadata("file.txt", "right changed this")
	baseIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{baseFile})
	leftIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{baseFile})      // unchanged
	rightIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{modifiedFile}) // modified

	result, err := merger.MergeWorkspaces(baseIndex, leftIndex, rightIndex)
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Expected success: only right modified")
	}

	loader := wsindex.NewLoader(casStore)
	files, _ := loader.ListAll(*result.MergedIndex)
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}
	if files[0].FileRef.Hash != modifiedFile.FileRef.Hash {
		t.Error("Expected right's version to be taken")
	}
}

func TestMerge_ModifiedLeftUnchangedRight(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	merger := NewMerger(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	baseFile := createTestFileMetadata("file.txt", "base content")
	modifiedFile := createTestFileMetadata("file.txt", "left changed this")
	baseIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{baseFile})
	leftIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{modifiedFile}) // modified
	rightIndex, _ := wsBuilder.Build([]wsindex.FileMetadata{baseFile})   // unchanged

	result, err := merger.MergeWorkspaces(baseIndex, leftIndex, rightIndex)
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Expected success: only left modified")
	}

	loader := wsindex.NewLoader(casStore)
	files, _ := loader.ListAll(*result.MergedIndex)
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}
	if files[0].FileRef.Hash != modifiedFile.FileRef.Hash {
		t.Error("Expected left's version to be taken")
	}
}

func TestDiffWorkspaces_EmptyToFull(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	differ := NewDiffer(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	emptyIndex, _ := wsBuilder.Build(nil)
	files := []wsindex.FileMetadata{
		createTestFileMetadata("a.txt", "aaa"),
		createTestFileMetadata("b.txt", "bbb"),
	}
	fullIndex, _ := wsBuilder.Build(files)

	diff, err := differ.DiffWorkspaces(emptyIndex, fullIndex)
	if err != nil {
		t.Fatalf("DiffWorkspaces failed: %v", err)
	}

	if len(diff.FileChanges) != 2 {
		t.Fatalf("Expected 2 changes, got %d", len(diff.FileChanges))
	}
	for _, change := range diff.FileChanges {
		if change.Type != Added {
			t.Errorf("Expected Added, got %v for %s", change.Type, change.Path)
		}
	}
}

func TestDiffWorkspaces_FullToEmpty(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	differ := NewDiffer(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	files := []wsindex.FileMetadata{
		createTestFileMetadata("a.txt", "aaa"),
		createTestFileMetadata("b.txt", "bbb"),
	}
	fullIndex, _ := wsBuilder.Build(files)
	emptyIndex, _ := wsBuilder.Build(nil)

	diff, err := differ.DiffWorkspaces(fullIndex, emptyIndex)
	if err != nil {
		t.Fatalf("DiffWorkspaces failed: %v", err)
	}

	if len(diff.FileChanges) != 2 {
		t.Fatalf("Expected 2 changes, got %d", len(diff.FileChanges))
	}
	for _, change := range diff.FileChanges {
		if change.Type != Removed {
			t.Errorf("Expected Removed, got %v for %s", change.Type, change.Path)
		}
	}
}

func TestDiffWorkspaces_NoChanges(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	differ := NewDiffer(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	files := []wsindex.FileMetadata{
		createTestFileMetadata("same.txt", "unchanged"),
	}
	index1, _ := wsBuilder.Build(files)
	index2, _ := wsBuilder.Build(files)

	diff, err := differ.DiffWorkspaces(index1, index2)
	if err != nil {
		t.Fatalf("DiffWorkspaces failed: %v", err)
	}
	if len(diff.FileChanges) != 0 {
		t.Errorf("Expected 0 changes for identical workspaces, got %d", len(diff.FileChanges))
	}
}

func TestDetectRenames_NoRenames(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	analyzer := NewAnalyzer(casStore)

	oldFile := createTestFileMetadata("old.txt", "old content")
	newFile := createTestFileMetadata("new.txt", "completely different content")

	diff := &WorkspaceDiff{
		FileChanges: []FileChange{
			{Type: Removed, Path: "old.txt", OldFile: &oldFile},
			{Type: Added, Path: "new.txt", NewFile: &newFile},
		},
	}

	renames := analyzer.DetectRenames(diff, 0.8)
	if len(renames) != 0 {
		t.Errorf("Expected 0 renames, got %d", len(renames))
	}
}

func TestAnalyzer_GetConflictSummary(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	analyzer := NewAnalyzer(casStore)

	conflicts := []Conflict{
		{Type: FileFileConflict, Path: "src/main.go"},
		{Type: FileFileConflict, Path: "README.md"},
		{Type: FileDirectoryConflict, Path: "lib"},
	}

	summary := analyzer.GetConflictSummary(conflicts)

	byType := summary["by_type"].(map[string]int)
	if byType["total"] != 3 {
		t.Errorf("Expected 3 total conflicts, got %d", byType["total"])
	}
	if byType["file_file"] != 2 {
		t.Errorf("Expected 2 file-file conflicts, got %d", byType["file_file"])
	}
	if byType["file_directory"] != 1 {
		t.Errorf("Expected 1 file-directory conflict, got %d", byType["file_directory"])
	}

	paths := summary["paths"].([]string)
	if len(paths) != 3 {
		t.Errorf("Expected 3 conflict paths, got %d", len(paths))
	}
}

// --- Resolution storage tests ---

func TestResolutionStorage_SaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "diffmerge-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewResolutionStorage(tmpDir)

	resolution := CreateResolution("feature", "main", cas.Hash{}, cas.Hash{}, StrategyAuto)
	resolution.Status = "in_progress"

	err = storage.Save(resolution)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := storage.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Expected loaded resolution, got nil")
	}
	if loaded.SourceTimeline != "feature" {
		t.Errorf("Expected source 'feature', got %q", loaded.SourceTimeline)
	}
	if loaded.TargetTimeline != "main" {
		t.Errorf("Expected target 'main', got %q", loaded.TargetTimeline)
	}
	if loaded.Status != "in_progress" {
		t.Errorf("Expected status 'in_progress', got %q", loaded.Status)
	}
}

func TestResolutionStorage_LoadNoFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "diffmerge-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewResolutionStorage(tmpDir)
	loaded, err := storage.Load()
	if err != nil {
		t.Fatalf("Load should not fail for missing file: %v", err)
	}
	if loaded != nil {
		t.Error("Expected nil when no resolution exists")
	}
}

func TestResolutionStorage_DeleteAndExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "diffmerge-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewResolutionStorage(tmpDir)

	resolution := CreateResolution("a", "b", cas.Hash{}, cas.Hash{}, StrategyOurs)
	storage.Save(resolution)

	if !storage.Exists() {
		t.Error("Expected resolution to exist after save")
	}

	err = storage.Delete()
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if storage.Exists() {
		t.Error("Expected resolution to not exist after delete")
	}

	// Delete again should not error
	err = storage.Delete()
	if err != nil {
		t.Fatalf("Double delete should not fail: %v", err)
	}
}

func TestMergeResolution_StatusTransitions(t *testing.T) {
	res := CreateResolution("src", "dst", cas.Hash{}, cas.Hash{}, StrategyAuto)

	if res.Status != "in_progress" {
		t.Errorf("Expected initial status 'in_progress', got %q", res.Status)
	}
	if res.CompletedAt != nil {
		t.Error("Expected nil CompletedAt initially")
	}

	res.MarkCompleted()
	if res.Status != "resolved" {
		t.Errorf("Expected status 'resolved', got %q", res.Status)
	}
	if res.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}

	res2 := CreateResolution("src", "dst", cas.Hash{}, cas.Hash{}, StrategyAuto)
	res2.MarkAborted()
	if res2.Status != "aborted" {
		t.Errorf("Expected status 'aborted', got %q", res2.Status)
	}
}

func TestMergeResolution_IsFullyResolved(t *testing.T) {
	res := CreateResolution("src", "dst", cas.Hash{}, cas.Hash{}, StrategyAuto)

	// No files = fully resolved
	if !res.IsFullyResolved() {
		t.Error("Expected empty resolution to be fully resolved")
	}

	// Add resolved file
	res.Files["a.txt"] = &FileResolution{Path: "a.txt", Resolved: true}
	if !res.IsFullyResolved() {
		t.Error("Expected fully resolved with one resolved file")
	}

	// Add unresolved file
	res.Files["b.txt"] = &FileResolution{Path: "b.txt", Resolved: false}
	if res.IsFullyResolved() {
		t.Error("Expected NOT fully resolved with one unresolved file")
	}
}

func TestMergeResolution_GetUnresolvedFiles(t *testing.T) {
	res := CreateResolution("src", "dst", cas.Hash{}, cas.Hash{}, StrategyAuto)
	res.Files["resolved.txt"] = &FileResolution{Path: "resolved.txt", Resolved: true}
	res.Files["conflict1.txt"] = &FileResolution{Path: "conflict1.txt", Resolved: false}
	res.Files["conflict2.txt"] = &FileResolution{Path: "conflict2.txt", Resolved: false}

	unresolved := res.GetUnresolvedFiles()
	if len(unresolved) != 2 {
		t.Fatalf("Expected 2 unresolved files, got %d", len(unresolved))
	}
}

func TestMergeResolution_GetConflictCount(t *testing.T) {
	res := CreateResolution("src", "dst", cas.Hash{}, cas.Hash{}, StrategyAuto)
	res.Files["ok.txt"] = &FileResolution{Path: "ok.txt", Resolved: true}
	res.Files["bad.txt"] = &FileResolution{
		Path:     "bad.txt",
		Resolved: false,
		Chunks: []ChunkResolution{
			{ChunkIndex: 0, Choice: ChoiceCustom},
			{ChunkIndex: 1, Choice: ChoiceCustom},
		},
	}
	res.Files["bad2.txt"] = &FileResolution{Path: "bad2.txt", Resolved: false}

	count := res.GetConflictCount()
	// bad.txt has 2 chunk conflicts, bad2.txt has 0 chunks so counts as 1
	if count != 3 {
		t.Errorf("Expected 3 conflict count, got %d", count)
	}
}

func TestMergeResolution_Summary(t *testing.T) {
	res := CreateResolution("src", "dst", cas.Hash{}, cas.Hash{}, StrategyAuto)
	res.Files["a.txt"] = &FileResolution{Path: "a.txt", Resolved: true}
	res.Files["b.txt"] = &FileResolution{Path: "b.txt", Resolved: true}

	summary := res.Summary()
	if summary != "All 2 files resolved using auto strategy" {
		t.Errorf("Unexpected summary: %q", summary)
	}

	res.Files["c.txt"] = &FileResolution{Path: "c.txt", Resolved: false}
	summary = res.Summary()
	if summary == "" {
		t.Error("Expected non-empty summary")
	}
}

func TestResolutionStorage_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewResolutionStorage(tmpDir)

	if storage.Exists() {
		t.Error("Expected Exists() to return false when no resolution file")
	}

	resolution := CreateResolution("feature", "main", cas.Hash{}, cas.Hash{}, StrategyAuto)
	if err := storage.Save(resolution); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !storage.Exists() {
		t.Error("Expected Exists() to return true after save")
	}
}

func TestResolutionStorage_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewResolutionStorage(tmpDir)

	resolution := CreateResolution("feature", "main", cas.Hash{}, cas.Hash{}, StrategyAuto)
	if err := storage.Save(resolution); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	resPath := filepath.Join(tmpDir, "MERGE_RESOLUTION")
	if _, err := os.Stat(resPath); os.IsNotExist(err) {
		t.Fatal("Expected MERGE_RESOLUTION file to exist after save")
	}

	if err := storage.Delete(); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := os.Stat(resPath); !os.IsNotExist(err) {
		t.Error("Expected MERGE_RESOLUTION file to be deleted")
	}

	// Deleting again should not error
	if err := storage.Delete(); err != nil {
		t.Fatalf("Delete of non-existent file should not error: %v", err)
	}
}

func TestMergeResolution_MarkCompleted(t *testing.T) {
	mr := CreateResolution("src", "dst", cas.Hash{}, cas.Hash{}, StrategyAuto)

	mr.MarkCompleted()

	if mr.Status != "resolved" {
		t.Errorf("Expected status 'resolved', got %s", mr.Status)
	}
	if mr.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}
}

func TestMergeResolution_MarkAborted(t *testing.T) {
	mr := CreateResolution("src", "dst", cas.Hash{}, cas.Hash{}, StrategyAuto)

	mr.MarkAborted()

	if mr.Status != "aborted" {
		t.Errorf("Expected status 'aborted', got %s", mr.Status)
	}
	if mr.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}
}

func TestCreateResolution(t *testing.T) {
	sourceHash := cas.SumB3([]byte("source"))
	targetHash := cas.SumB3([]byte("target"))

	mr := CreateResolution("feature", "main", sourceHash, targetHash, StrategyTheirs)

	if mr.SourceTimeline != "feature" {
		t.Errorf("Expected SourceTimeline 'feature', got %s", mr.SourceTimeline)
	}
	if mr.TargetTimeline != "main" {
		t.Errorf("Expected TargetTimeline 'main', got %s", mr.TargetTimeline)
	}
	if mr.SourceHash != sourceHash.String() {
		t.Errorf("Expected SourceHash %s, got %s", sourceHash.String(), mr.SourceHash)
	}
	if mr.TargetHash != targetHash.String() {
		t.Errorf("Expected TargetHash %s, got %s", targetHash.String(), mr.TargetHash)
	}
	if mr.Strategy != StrategyTheirs {
		t.Errorf("Expected strategy 'theirs', got %s", mr.Strategy)
	}
	if mr.Status != "in_progress" {
		t.Errorf("Expected status 'in_progress', got %s", mr.Status)
	}
	if mr.Files == nil {
		t.Error("Expected Files map to be initialized")
	}
	if len(mr.Files) != 0 {
		t.Errorf("Expected 0 files initially, got %d", len(mr.Files))
	}
	if mr.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestCreatePatch_And_CreateFromDiff(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	differ := NewDiffer(casStore)
	patcher := NewPatcher(casStore)
	wsBuilder := wsindex.NewBuilder(casStore)

	oldFiles := []wsindex.FileMetadata{
		createTestFileMetadata("keep.txt", "keep"),
		createTestFileMetadata("modify.txt", "old"),
		createTestFileMetadata("remove.txt", "gone"),
	}
	oldIndex, err := wsBuilder.Build(oldFiles)
	if err != nil {
		t.Fatalf("Build old workspace failed: %v", err)
	}

	newFiles := []wsindex.FileMetadata{
		createTestFileMetadata("keep.txt", "keep"),
		createTestFileMetadata("modify.txt", "new"),
		createTestFileMetadata("added.txt", "fresh"),
	}
	newIndex, err := wsBuilder.Build(newFiles)
	if err != nil {
		t.Fatalf("Build new workspace failed: %v", err)
	}

	diff, err := differ.DiffWorkspaces(oldIndex, newIndex)
	if err != nil {
		t.Fatalf("DiffWorkspaces failed: %v", err)
	}

	patch := patcher.CreatePatch("test patch", diff)
	if patch.Description != "test patch" {
		t.Errorf("Expected description 'test patch', got %s", patch.Description)
	}
	if len(patch.Changes) != 3 {
		t.Fatalf("Expected 3 changes in patch, got %d", len(patch.Changes))
	}

	patchedIndex, err := patcher.ApplyPatch(oldIndex, patch)
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}

	loader := wsindex.NewLoader(casStore)
	patchedFiles, err := loader.ListAll(patchedIndex)
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}

	if len(patchedFiles) != 3 {
		t.Fatalf("Expected 3 files after patch, got %d", len(patchedFiles))
	}

	fileMap := make(map[string]wsindex.FileMetadata)
	for _, f := range patchedFiles {
		fileMap[f.Path] = f
	}

	if _, ok := fileMap["keep.txt"]; !ok {
		t.Error("keep.txt missing after patch")
	}
	if f, ok := fileMap["modify.txt"]; !ok {
		t.Error("modify.txt missing after patch")
	} else if f.FileRef.Hash != cas.SumB3([]byte("new")) {
		t.Error("modify.txt should have new content")
	}
	if _, ok := fileMap["added.txt"]; !ok {
		t.Error("added.txt missing after patch")
	}
	if _, ok := fileMap["remove.txt"]; ok {
		t.Error("remove.txt should have been removed")
	}
}

func TestDetectRenames_BelowThreshold(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	analyzer := NewAnalyzer(casStore)

	// Different content means different hashes; the implementation only detects
	// exact hash matches (similarity 1.0), so no renames are found regardless of threshold.
	oldFile := createTestFileMetadata("old.txt", "content A")
	newFile := createTestFileMetadata("new.txt", "content B")

	diff := &WorkspaceDiff{
		FileChanges: []FileChange{
			{Type: Removed, Path: "old.txt", OldFile: &oldFile},
			{Type: Added, Path: "new.txt", NewFile: &newFile},
		},
	}

	renames := analyzer.DetectRenames(diff, 0.5)
	if len(renames) != 0 {
		t.Errorf("Expected 0 renames for different content even with low threshold, got %d", len(renames))
	}
}

func TestMergeResolution_Summary_Detailed(t *testing.T) {
	mr := CreateResolution("src", "dst", cas.Hash{}, cas.Hash{}, StrategyAuto)
	mr.Files["a.txt"] = &FileResolution{Path: "a.txt", Resolved: true}
	mr.Files["b.txt"] = &FileResolution{Path: "b.txt", Resolved: true}

	summary := mr.Summary()
	if !strings.Contains(summary, "All 2 files resolved") {
		t.Errorf("Expected fully resolved summary, got: %s", summary)
	}
	if !strings.Contains(summary, string(StrategyAuto)) {
		t.Errorf("Expected strategy in summary, got: %s", summary)
	}

	mr.Files["c.txt"] = &FileResolution{
		Path:     "c.txt",
		Resolved: false,
		Chunks: []ChunkResolution{
			{ChunkIndex: 0, Choice: ChoiceCustom},
		},
	}

	summary = mr.Summary()
	if !strings.Contains(summary, "2/3 files resolved") {
		t.Errorf("Expected partial summary, got: %s", summary)
	}
	if !strings.Contains(summary, "1 conflicts remaining") {
		t.Errorf("Expected conflict count in summary, got: %s", summary)
	}
}

func TestChunkMerger_MergeFile_AllCases(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	cm := NewChunkMerger(casStore)

	// Store actual chunk data in CAS for each test file
	storeContent := func(content string) *wsindex.FileMetadata {
		data := []byte(content)
		hash := cas.SumB3(data)
		casStore.Put(hash, data)
		fm := createTestFileMetadata("test", content)
		return &fm
	}

	base := storeContent("base content")
	left := storeContent("left content")
	right := storeContent("right content")
	sameAsBase := storeContent("base content")

	tests := []struct {
		name    string
		base    *wsindex.FileMetadata
		left    *wsindex.FileMetadata
		right   *wsindex.FileMetadata
		success bool
	}{
		{"none-none-none", nil, nil, nil, true},
		{"none-left-none", nil, left, nil, true},
		{"none-none-right", nil, nil, right, true},
		{"none-left-right-same", nil, left, left, true},          // same content added
		{"none-left-right-diff", nil, left, right, false},        // different content added = conflict
		{"base-none-none", base, nil, nil, true},                 // deleted both sides
		{"base-left-none-modified", base, left, nil, false},      // modified left, deleted right = conflict
		{"base-left-none-unchanged", base, sameAsBase, nil, true}, // unchanged left, deleted right = accept deletion
		{"base-none-right-modified", base, nil, right, false},    // deleted left, modified right = conflict
		{"base-none-right-unchanged", base, nil, sameAsBase, true}, // deleted left, unchanged right = accept deletion
		{"base-left-right-same", base, left, left, true},         // both same change
		{"base-leftUnchanged-right", base, sameAsBase, right, true}, // only right changed
		{"base-left-rightUnchanged", base, left, sameAsBase, true},  // only left changed
		{"base-left-right-diff", base, left, right, false},       // both changed differently = conflict
		{"base-left-right-allSame", base, sameAsBase, sameAsBase, true}, // no changes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cm.MergeFile("test.txt", tt.base, tt.left, tt.right)
			if err != nil {
				t.Fatalf("MergeFile error: %v", err)
			}
			if result.Success != tt.success {
				t.Errorf("Expected success=%v, got %v (conflicts: %d)", tt.success, result.Success, len(result.Conflicts))
			}
		})
	}
}

func TestChunkMerger_ExtractChunks_InternalNode(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	cm := NewChunkMerger(casStore)

	// Build a multi-chunk file using filechunk.Builder with small leaf size
	builder := filechunk.NewBuilder(casStore, filechunk.Params{LeafSize: 5})
	content := []byte("hello world test data chunks")
	ref, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// For an internal node, extractChunks should return individual leaf hashes
	chunks, size, extractErr := cm.extractChunks(ref)
	if extractErr != nil {
		t.Fatalf("extractChunks failed: %v", extractErr)
	}

	if size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), size)
	}

	// For a file that needs multiple chunks, we should get more than 1 hash
	if ref.Kind == filechunk.Node && len(chunks) <= 1 {
		t.Errorf("Expected multiple chunks for internal node, got %d", len(chunks))
	}

	// All chunk hashes should exist in CAS
	for i, h := range chunks {
		_, err := casStore.Get(h)
		if err != nil {
			t.Errorf("Chunk %d hash not found in CAS: %v", i, err)
		}
	}
}

func TestChunkMerger_MergeChunks_ConflictData(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	cm := NewChunkMerger(casStore)

	// Create files with actual CAS-backed data
	storeAndCreate := func(content string) *wsindex.FileMetadata {
		data := []byte(content)
		// Build through filechunk to get proper leaf encoding
		builder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())
		ref, err := builder.Build(data)
		if err != nil {
			t.Fatalf("Build failed: %v", err)
		}
		return &wsindex.FileMetadata{
			Path:    "conflict.txt",
			FileRef: ref,
			Size:    int64(len(data)),
		}
	}

	base := storeAndCreate("base content")
	left := storeAndCreate("left modified content")
	right := storeAndCreate("right modified content")

	result, err := cm.MergeFile("conflict.txt", base, left, right)
	if err != nil {
		t.Fatalf("MergeFile error: %v", err)
	}

	if result.Success {
		t.Fatal("Expected conflict, got success")
	}
	if len(result.Conflicts) == 0 {
		t.Fatal("Expected at least one conflict")
	}

	conflict := result.Conflicts[0]
	if conflict.BaseChunk == nil {
		t.Error("Expected BaseChunk to be set")
	}
	if conflict.LeftChunk == nil {
		t.Error("Expected LeftChunk to be set")
	}
	if conflict.RightChunk == nil {
		t.Error("Expected RightChunk to be set")
	}
	// BaseData/LeftData/RightData should be populated
	if len(conflict.BaseData) == 0 {
		t.Error("Expected BaseData to be populated")
	}
	if len(conflict.LeftData) == 0 {
		t.Error("Expected LeftData to be populated")
	}
	if len(conflict.RightData) == 0 {
		t.Error("Expected RightData to be populated")
	}
}

func TestChunkMerger_CorruptedCAS(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	cm := NewChunkMerger(casStore)

	// Create file metadata pointing to non-existent CAS data
	fakeHash := cas.SumB3([]byte("does not exist in CAS"))
	fakeMeta := &wsindex.FileMetadata{
		Path: "fake.txt",
		FileRef: filechunk.NodeRef{
			Hash: fakeHash,
			Kind: filechunk.Leaf,
			Size: 100,
		},
		Size: 100,
	}

	base := fakeMeta
	left := fakeMeta
	right := fakeMeta

	// This should not panic. It may return an error or a result with no conflict data.
	result, err := cm.MergeFile("fake.txt", base, left, right)
	// Both outcomes are acceptable as long as no panic
	_ = result
	_ = err
}

func TestStrategyResolver_UnknownStrategy(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	resolver := NewStrategyResolver(casStore)

	_, err := resolver.Resolve(StrategyType("bogus"), "file.txt", nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for unknown strategy, got nil")
	}
}

func TestStrategyResolver_OursAlwaysWins(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	resolver := NewStrategyResolver(casStore)

	base := createTestFileMetadata("f.txt", "base")
	left := createTestFileMetadata("f.txt", "left wins")
	right := createTestFileMetadata("f.txt", "right loses")

	// Store in CAS so extractChunks works
	for _, content := range []string{"base", "left wins", "right loses"} {
		data := []byte(content)
		casStore.Put(cas.SumB3(data), data)
	}

	result, err := resolver.Resolve(StrategyOurs, "f.txt", &base, &left, &right)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Expected success with ours strategy")
	}
	// Ours = left; should have left's chunks
	if len(result.MergedChunks) == 0 {
		t.Fatal("Expected merged chunks")
	}
	if result.MergedChunks[0] != left.FileRef.Hash {
		t.Error("Expected ours (left) hash to be used")
	}
}

func TestStrategyResolver_TheirsAlwaysWins(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	resolver := NewStrategyResolver(casStore)

	base := createTestFileMetadata("f.txt", "base")
	left := createTestFileMetadata("f.txt", "left loses")
	right := createTestFileMetadata("f.txt", "right wins")

	for _, content := range []string{"base", "left loses", "right wins"} {
		data := []byte(content)
		casStore.Put(cas.SumB3(data), data)
	}

	result, err := resolver.Resolve(StrategyTheirs, "f.txt", &base, &left, &right)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Expected success with theirs strategy")
	}
	if len(result.MergedChunks) == 0 {
		t.Fatal("Expected merged chunks")
	}
	if result.MergedChunks[0] != right.FileRef.Hash {
		t.Error("Expected theirs (right) hash to be used")
	}
}

func TestStrategyResolver_ResolveWithFallback(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	resolver := NewStrategyResolver(casStore)

	base := createTestFileMetadata("f.txt", "base")
	left := createTestFileMetadata("f.txt", "left changed")
	right := createTestFileMetadata("f.txt", "right changed")

	for _, content := range []string{"base", "left changed", "right changed"} {
		data := []byte(content)
		casStore.Put(cas.SumB3(data), data)
	}

	// Auto strategy will produce a conflict; fallback to ours should resolve it
	result, err := resolver.ResolveWithFallback(StrategyAuto, StrategyOurs, "f.txt", &base, &left, &right)
	if err != nil {
		t.Fatalf("ResolveWithFallback failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Expected success with fallback to ours")
	}
}