package diffmerge

import (
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