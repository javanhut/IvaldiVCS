package commit

import (
	"testing"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

func createTestWorkspaceFiles(casStore cas.CAS) []wsindex.FileMetadata {
	fileBuilder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())

	var files []wsindex.FileMetadata

	// Create test files
	testFiles := map[string]string{
		"README.md":        "# Test Repository\nThis is a test.",
		"src/main.go":      "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}",
		"src/util.go":      "package main\n\nfunc helper() string {\n\treturn \"help\"\n}",
		"docs/guide.md":    "# User Guide\nInstructions here.",
		"test/main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {\n\t// test\n}",
	}

	for path, content := range testFiles {
		contentBytes := []byte(content)
		fileRef, err := fileBuilder.Build(contentBytes)
		if err != nil {
			panic(err) // Test helper, panic is OK
		}

		file := wsindex.FileMetadata{
			Path:     path,
			FileRef:  fileRef,
			ModTime:  time.Unix(1640995200, 0), // 2022-01-01
			Mode:     0644,
			Size:     int64(len(contentBytes)),
			Checksum: cas.SumB3(contentBytes),
		}

		files = append(files, file)
	}

	return files
}

func TestCreateCommit(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)

	// Create test workspace
	files := createTestWorkspaceFiles(casStore)

	// Create commit
	commit, err := builder.CreateCommit(
		files,
		nil, // No parents (initial commit)
		"Test Author <test@example.com>",
		"Test Committer <test@example.com>",
		"Initial commit\n\nAdd basic project structure",
	)
	if err != nil {
		t.Fatalf("CreateCommit failed: %v", err)
	}

	// Validate commit
	if commit == nil {
		t.Fatal("Expected commit object, got nil")
	}
	if commit.TreeHash == (cas.Hash{}) {
		t.Error("Expected tree hash, got empty hash")
	}
	if len(commit.Parents) != 0 {
		t.Errorf("Expected 0 parents, got %d", len(commit.Parents))
	}
	if commit.Author != "Test Author <test@example.com>" {
		t.Errorf("Expected author 'Test Author <test@example.com>', got %s", commit.Author)
	}
	if commit.Message != "Initial commit\n\nAdd basic project structure" {
		t.Errorf("Unexpected commit message: %s", commit.Message)
	}
	// MMR position 0 is valid for the first commit
	if commit.MMRPosition > 10000 {
		t.Errorf("MMR position seems invalid: %d", commit.MMRPosition)
	}
}

func TestCreateCommitWithParents(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)

	// Create first commit
	files1 := createTestWorkspaceFiles(casStore)[:2] // Just first 2 files
	commit1, err := builder.CreateCommit(
		files1,
		nil,
		"Author 1 <author1@example.com>",
		"Author 1 <author1@example.com>",
		"First commit",
	)
	if err != nil {
		t.Fatalf("First CreateCommit failed: %v", err)
	}

	// Create second commit with first as parent
	files2 := createTestWorkspaceFiles(casStore) // All files
	commit1Hash := builder.GetCommitHash(commit1)
	
	commit2, err := builder.CreateCommit(
		files2,
		[]cas.Hash{commit1Hash}, // Parent commit
		"Author 2 <author2@example.com>",
		"Author 2 <author2@example.com>",
		"Add more files",
	)
	if err != nil {
		t.Fatalf("Second CreateCommit failed: %v", err)
	}

	// Validate second commit has parent
	if len(commit2.Parents) != 1 {
		t.Fatalf("Expected 1 parent, got %d", len(commit2.Parents))
	}
	if commit2.Parents[0] != commit1Hash {
		t.Error("Parent hash mismatch")
	}
	if commit2.MMRPosition <= commit1.MMRPosition {
		t.Error("Expected second commit to have higher MMR position")
	}
}

func TestReadCommit(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create commit
	files := createTestWorkspaceFiles(casStore)
	originalCommit, err := builder.CreateCommit(
		files,
		nil,
		"Test Author <test@example.com>",
		"Test Committer <test@example.com>",
		"Test commit message",
	)
	if err != nil {
		t.Fatalf("CreateCommit failed: %v", err)
	}

	// Get commit hash
	commitHash := builder.GetCommitHash(originalCommit)

	// Read commit back
	readCommit, err := reader.ReadCommit(commitHash)
	if err != nil {
		t.Fatalf("ReadCommit failed: %v", err)
	}

	// Validate
	if readCommit.TreeHash != originalCommit.TreeHash {
		t.Error("Tree hash mismatch")
	}
	if readCommit.Author != originalCommit.Author {
		t.Errorf("Author mismatch: expected %s, got %s", originalCommit.Author, readCommit.Author)
	}
	if readCommit.Committer != originalCommit.Committer {
		t.Errorf("Committer mismatch: expected %s, got %s", originalCommit.Committer, readCommit.Committer)
	}
	if readCommit.Message != originalCommit.Message {
		t.Errorf("Message mismatch: expected %s, got %s", originalCommit.Message, readCommit.Message)
	}
	if readCommit.MMRPosition != originalCommit.MMRPosition {
		t.Errorf("MMR position mismatch: expected %d, got %d", originalCommit.MMRPosition, readCommit.MMRPosition)
	}
}

func TestReadTree(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create commit
	files := createTestWorkspaceFiles(casStore)
	commit, err := builder.CreateCommit(
		files,
		nil,
		"Test Author <test@example.com>",
		"Test Committer <test@example.com>",
		"Test commit",
	)
	if err != nil {
		t.Fatalf("CreateCommit failed: %v", err)
	}

	// Read tree
	tree, err := reader.ReadTree(commit)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	// Should have root level entries: README.md, src/, docs/, test/
	if len(tree.Entries) == 0 {
		t.Fatal("Expected tree entries, got none")
	}

	// Check for expected entries
	entryNames := make(map[string]bool)
	for _, entry := range tree.Entries {
		entryNames[entry.Name] = true
	}

	expectedEntries := []string{"README.md", "src", "docs", "test"}
	for _, expected := range expectedEntries {
		if !entryNames[expected] {
			t.Errorf("Expected tree entry '%s' not found", expected)
		}
	}
}

func TestGetFileContent(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create commit
	files := createTestWorkspaceFiles(casStore)
	commit, err := builder.CreateCommit(
		files,
		nil,
		"Test Author <test@example.com>",
		"Test Committer <test@example.com>",
		"Test commit",
	)
	if err != nil {
		t.Fatalf("CreateCommit failed: %v", err)
	}

	// Read tree
	tree, err := reader.ReadTree(commit)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	// Test reading files
	testCases := map[string]string{
		"README.md":        "# Test Repository\nThis is a test.",
		"src/main.go":      "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}",
		"docs/guide.md":    "# User Guide\nInstructions here.",
		"test/main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {\n\t// test\n}",
	}

	for filePath, expectedContent := range testCases {
		content, err := reader.GetFileContent(tree, filePath)
		if err != nil {
			t.Errorf("GetFileContent(%s) failed: %v", filePath, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s:\nExpected: %q\nGot: %q", 
				filePath, expectedContent, string(content))
		}
	}
}

func TestListFiles(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create commit
	files := createTestWorkspaceFiles(casStore)
	commit, err := builder.CreateCommit(
		files,
		nil,
		"Test Author <test@example.com>",
		"Test Committer <test@example.com>",
		"Test commit",
	)
	if err != nil {
		t.Fatalf("CreateCommit failed: %v", err)
	}

	// Read tree
	tree, err := reader.ReadTree(commit)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	// List all files
	fileList, err := reader.ListFiles(tree)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Expected files
	expectedFiles := []string{
		"README.md",
		"src/main.go",
		"src/util.go",
		"docs/guide.md",
		"test/main_test.go",
	}

	if len(fileList) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d: %v", len(expectedFiles), len(fileList), fileList)
	}

	// Check all expected files are present
	fileSet := make(map[string]bool)
	for _, file := range fileList {
		fileSet[file] = true
	}

	for _, expected := range expectedFiles {
		if !fileSet[expected] {
			t.Errorf("Expected file %s not found in list", expected)
		}
	}
}

func TestTreeToFileMetadata(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create commit with test files
	files := createTestWorkspaceFiles(casStore)
	commit, err := builder.CreateCommit(
		files,
		nil,
		"Test Author <test@example.com>",
		"Test Committer <test@example.com>",
		"Test commit for TreeToFileMetadata",
	)
	if err != nil {
		t.Fatalf("CreateCommit failed: %v", err)
	}

	// Read tree
	tree, err := reader.ReadTree(commit)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	// Convert tree to file metadata
	metadata, err := reader.TreeToFileMetadata(tree)
	if err != nil {
		t.Fatalf("TreeToFileMetadata failed: %v", err)
	}

	// Expected files from createTestWorkspaceFiles
	expectedFiles := []string{
		"README.md",
		"src/main.go",
		"src/util.go",
		"docs/guide.md",
		"test/main_test.go",
	}

	// Verify correct number of files
	if len(metadata) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(metadata))
	}

	// Create a map for easier lookup
	metadataMap := make(map[string]wsindex.FileMetadata)
	for _, m := range metadata {
		metadataMap[m.Path] = m
	}

	// Verify each expected file exists with correct data
	for _, expectedPath := range expectedFiles {
		m, exists := metadataMap[expectedPath]
		if !exists {
			t.Errorf("Expected file %s not found in metadata", expectedPath)
			continue
		}

		// Verify FileRef is populated
		if m.FileRef.Hash == (cas.Hash{}) {
			t.Errorf("File %s has empty FileRef.Hash", expectedPath)
		}

		// Verify Size is set
		if m.Size <= 0 {
			t.Errorf("File %s has invalid size: %d", expectedPath, m.Size)
		}

		// Verify FileRef.Size matches Size
		if m.FileRef.Size != m.Size {
			t.Errorf("File %s: FileRef.Size (%d) != Size (%d)", expectedPath, m.FileRef.Size, m.Size)
		}

		// Verify Checksum is set (should equal FileRef.Hash for files)
		if m.Checksum == (cas.Hash{}) {
			t.Errorf("File %s has empty Checksum", expectedPath)
		}

		// Verify Mode is set to default
		if m.Mode != 0644 {
			t.Errorf("File %s has unexpected mode: %o (expected 0644)", expectedPath, m.Mode)
		}
	}
}

func TestTreeToFileMetadataEmpty(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create empty commit
	commit, err := builder.CreateCommit(
		nil, // No files
		nil,
		"Test Author <test@example.com>",
		"Test Committer <test@example.com>",
		"Empty commit",
	)
	if err != nil {
		t.Fatalf("CreateCommit failed: %v", err)
	}

	// Read tree
	tree, err := reader.ReadTree(commit)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	// Convert tree to file metadata
	metadata, err := reader.TreeToFileMetadata(tree)
	if err != nil {
		t.Fatalf("TreeToFileMetadata failed: %v", err)
	}

	// Should return empty slice for empty tree
	if len(metadata) != 0 {
		t.Errorf("Expected 0 files for empty tree, got %d", len(metadata))
	}
}

func TestIsAncestor_DirectParent(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create first commit (will be ancestor)
	commit1, err := builder.CreateCommit(
		nil,
		nil, // No parents
		"Author <author@example.com>",
		"Author <author@example.com>",
		"First commit",
	)
	if err != nil {
		t.Fatalf("CreateCommit 1 failed: %v", err)
	}
	commit1Hash := builder.GetCommitHash(commit1)

	// Create second commit with first as parent
	commit2, err := builder.CreateCommit(
		nil,
		[]cas.Hash{commit1Hash},
		"Author <author@example.com>",
		"Author <author@example.com>",
		"Second commit",
	)
	if err != nil {
		t.Fatalf("CreateCommit 2 failed: %v", err)
	}
	commit2Hash := builder.GetCommitHash(commit2)

	// commit1 should be ancestor of commit2
	isAncestor, err := reader.IsAncestor(commit1Hash, commit2Hash)
	if err != nil {
		t.Fatalf("IsAncestor failed: %v", err)
	}
	if !isAncestor {
		t.Error("Expected commit1 to be ancestor of commit2")
	}

	// commit2 should NOT be ancestor of commit1
	isAncestor, err = reader.IsAncestor(commit2Hash, commit1Hash)
	if err != nil {
		t.Fatalf("IsAncestor failed: %v", err)
	}
	if isAncestor {
		t.Error("Expected commit2 NOT to be ancestor of commit1")
	}
}

func TestIsAncestor_Grandparent(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create chain: commit1 <- commit2 <- commit3
	commit1, _ := builder.CreateCommit(nil, nil, "A", "A", "First")
	commit1Hash := builder.GetCommitHash(commit1)

	commit2, _ := builder.CreateCommit(nil, []cas.Hash{commit1Hash}, "A", "A", "Second")
	commit2Hash := builder.GetCommitHash(commit2)

	commit3, _ := builder.CreateCommit(nil, []cas.Hash{commit2Hash}, "A", "A", "Third")
	commit3Hash := builder.GetCommitHash(commit3)

	// commit1 (grandparent) should be ancestor of commit3
	isAncestor, err := reader.IsAncestor(commit1Hash, commit3Hash)
	if err != nil {
		t.Fatalf("IsAncestor failed: %v", err)
	}
	if !isAncestor {
		t.Error("Expected commit1 (grandparent) to be ancestor of commit3")
	}

	// commit2 (parent) should also be ancestor of commit3
	isAncestor, err = reader.IsAncestor(commit2Hash, commit3Hash)
	if err != nil {
		t.Fatalf("IsAncestor failed: %v", err)
	}
	if !isAncestor {
		t.Error("Expected commit2 (parent) to be ancestor of commit3")
	}
}

func TestIsAncestor_SameCommit(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	commit1, _ := builder.CreateCommit(nil, nil, "A", "A", "First")
	commit1Hash := builder.GetCommitHash(commit1)

	// A commit should be its own ancestor
	isAncestor, err := reader.IsAncestor(commit1Hash, commit1Hash)
	if err != nil {
		t.Fatalf("IsAncestor failed: %v", err)
	}
	if !isAncestor {
		t.Error("Expected commit to be its own ancestor")
	}
}

func TestIsAncestor_UnrelatedCommits(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create two unrelated commits (both have no parents)
	commit1, _ := builder.CreateCommit(nil, nil, "A", "A", "First branch")
	commit1Hash := builder.GetCommitHash(commit1)

	commit2, _ := builder.CreateCommit(nil, nil, "A", "A", "Second branch")
	commit2Hash := builder.GetCommitHash(commit2)

	// Neither should be ancestor of the other
	isAncestor, err := reader.IsAncestor(commit1Hash, commit2Hash)
	if err != nil {
		t.Fatalf("IsAncestor failed: %v", err)
	}
	if isAncestor {
		t.Error("Expected unrelated commits NOT to have ancestry")
	}

	isAncestor, err = reader.IsAncestor(commit2Hash, commit1Hash)
	if err != nil {
		t.Fatalf("IsAncestor failed: %v", err)
	}
	if isAncestor {
		t.Error("Expected unrelated commits NOT to have ancestry")
	}
}

func TestIsAncestor_MergeCommit(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Create base commit
	base, _ := builder.CreateCommit(nil, nil, "A", "A", "Base")
	baseHash := builder.GetCommitHash(base)

	// Create two branches from base
	branch1, _ := builder.CreateCommit(nil, []cas.Hash{baseHash}, "A", "A", "Branch 1")
	branch1Hash := builder.GetCommitHash(branch1)

	branch2, _ := builder.CreateCommit(nil, []cas.Hash{baseHash}, "A", "A", "Branch 2")
	branch2Hash := builder.GetCommitHash(branch2)

	// Create merge commit with both branches as parents
	merge, _ := builder.CreateCommit(nil, []cas.Hash{branch1Hash, branch2Hash}, "A", "A", "Merge")
	mergeHash := builder.GetCommitHash(merge)

	// Both branches should be ancestors of merge
	isAncestor, _ := reader.IsAncestor(branch1Hash, mergeHash)
	if !isAncestor {
		t.Error("Expected branch1 to be ancestor of merge")
	}

	isAncestor, _ = reader.IsAncestor(branch2Hash, mergeHash)
	if !isAncestor {
		t.Error("Expected branch2 to be ancestor of merge")
	}

	// Base should also be ancestor of merge (through both branches)
	isAncestor, _ = reader.IsAncestor(baseHash, mergeHash)
	if !isAncestor {
		t.Error("Expected base to be ancestor of merge")
	}
}

func TestEmptyCommit(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)

	// Create commit with no files
	commit, err := builder.CreateCommit(
		nil, // No files
		nil, // No parents
		"Test Author <test@example.com>",
		"Test Committer <test@example.com>",
		"Empty initial commit",
	)
	if err != nil {
		t.Fatalf("CreateCommit with no files failed: %v", err)
	}

	// Should have a tree hash (empty tree)
	if commit.TreeHash == (cas.Hash{}) {
		t.Error("Expected tree hash for empty commit, got empty hash")
	}

	// Read the tree
	reader := NewCommitReader(casStore)
	tree, err := reader.ReadTree(commit)
	if err != nil {
		t.Fatalf("ReadTree for empty commit failed: %v", err)
	}

	// Should have no entries
	if len(tree.Entries) != 0 {
		t.Errorf("Expected 0 entries in empty tree, got %d", len(tree.Entries))
	}
}

func TestCommitEncoding(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)

	// Create a commit object manually
	treeHash := cas.SumB3([]byte("test tree"))
	parentHash := cas.SumB3([]byte("test parent"))
	
	commit := &CommitObject{
		TreeHash:    treeHash,
		Parents:     []cas.Hash{parentHash},
		Author:      "Test Author <test@example.com>",
		Committer:   "Test Committer <commit@example.com>",
		AuthorTime:  time.Unix(1640995200, 0),
		CommitTime:  time.Unix(1640995260, 0),
		Message:     "Test commit message\n\nWith multiple lines",
		MMRPosition: 42,
	}

	// Encode and decode
	encoded := builder.encodeCommit(commit)
	reader := NewCommitReader(casStore)
	decoded, err := reader.parseCommit(encoded)
	if err != nil {
		t.Fatalf("parseCommit failed: %v", err)
	}

	// Validate
	if decoded.TreeHash != commit.TreeHash {
		t.Error("Tree hash mismatch after encoding/decoding")
	}
	if len(decoded.Parents) != 1 || decoded.Parents[0] != parentHash {
		t.Error("Parent hash mismatch after encoding/decoding")
	}
	if decoded.Author != commit.Author {
		t.Errorf("Author mismatch: expected %s, got %s", commit.Author, decoded.Author)
	}
	if decoded.Committer != commit.Committer {
		t.Errorf("Committer mismatch: expected %s, got %s", commit.Committer, decoded.Committer)
	}
	if !decoded.AuthorTime.Equal(commit.AuthorTime) {
		t.Errorf("Author time mismatch: expected %v, got %v", commit.AuthorTime, decoded.AuthorTime)
	}
	if !decoded.CommitTime.Equal(commit.CommitTime) {
		t.Errorf("Commit time mismatch: expected %v, got %v", commit.CommitTime, decoded.CommitTime)
	}
	if decoded.Message != commit.Message {
		t.Errorf("Message mismatch: expected %q, got %q", commit.Message, decoded.Message)
	}
	if decoded.MMRPosition != commit.MMRPosition {
		t.Errorf("MMR position mismatch: expected %d, got %d", commit.MMRPosition, decoded.MMRPosition)
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"/", []string{}},
		{"file.txt", []string{"file.txt"}},
		{"dir/file.txt", []string{"dir", "file.txt"}},
		{"/dir/file.txt", []string{"dir", "file.txt"}},
		{"dir/file.txt/", []string{"dir", "file.txt"}},
		{"/dir1/dir2/file.txt/", []string{"dir1", "dir2", "file.txt"}},
		{"a/b/c/d/e.txt", []string{"a", "b", "c", "d", "e.txt"}},
	}

	for _, test := range tests {
		result := splitPath(test.input)
		if len(result) != len(test.expected) {
			t.Errorf("splitPath(%q): expected length %d, got %d", 
				test.input, len(test.expected), len(result))
			continue
		}

		for i, part := range result {
			if string(part) != test.expected[i] {
				t.Errorf("splitPath(%q)[%d]: expected %q, got %q", 
					test.input, i, test.expected[i], string(part))
			}
		}
	}
}

// createNamedTestFiles creates workspace files from a path→content map.
func createNamedTestFiles(casStore cas.CAS, files map[string]string) []wsindex.FileMetadata {
	fileBuilder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())
	var result []wsindex.FileMetadata

	for path, content := range files {
		contentBytes := []byte(content)
		fileRef, err := fileBuilder.Build(contentBytes)
		if err != nil {
			panic(err)
		}
		result = append(result, wsindex.FileMetadata{
			Path:     path,
			FileRef:  fileRef,
			ModTime:  time.Unix(1640995200, 0),
			Mode:     0644,
			Size:     int64(len(contentBytes)),
			Checksum: cas.SumB3(contentBytes),
		})
	}
	return result
}

// mergeParentAndStaged simulates the merge logic from cli/seal.go:
// parent files are included unless overridden by a staged file at the same path.
func mergeParentAndStaged(parentFiles, stagedFiles []wsindex.FileMetadata) []wsindex.FileMetadata {
	stagedPathSet := make(map[string]bool, len(stagedFiles))
	for _, f := range stagedFiles {
		stagedPathSet[f.Path] = true
	}
	merged := append([]wsindex.FileMetadata{}, stagedFiles...)
	for _, pf := range parentFiles {
		if !stagedPathSet[pf.Path] {
			merged = append(merged, pf)
		}
	}
	return merged
}

// TestSnapshotMerge_NewFilePreservesParentFiles verifies that when a new file
// is staged on top of a parent commit, the resulting commit tree contains both
// the parent's files and the new file. This is the core regression test for the
// bug where upload deleted files from previous commits.
func TestSnapshotMerge_NewFilePreservesParentFiles(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// First commit: 3 files
	initialFiles := createNamedTestFiles(casStore, map[string]string{
		"README.md":   "# Project",
		"src/main.go": "package main",
		"src/util.go": "package main\nfunc helper() {}",
	})
	commit1, err := builder.CreateCommit(initialFiles, nil, "A", "A", "Initial commit")
	if err != nil {
		t.Fatalf("CreateCommit 1 failed: %v", err)
	}
	commit1Hash := builder.GetCommitHash(commit1)

	// Read parent tree to get file metadata (simulates what seal.go does)
	parentTree, err := reader.ReadTree(commit1)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}
	parentFiles, err := reader.TreeToFileMetadata(parentTree)
	if err != nil {
		t.Fatalf("TreeToFileMetadata failed: %v", err)
	}

	// Second commit: only 1 new staged file
	stagedFiles := createNamedTestFiles(casStore, map[string]string{
		"docs/guide.md": "# Guide",
	})

	// Merge parent + staged (the fix logic)
	allFiles := mergeParentAndStaged(parentFiles, stagedFiles)

	commit2, err := builder.CreateCommit(allFiles, []cas.Hash{commit1Hash}, "A", "A", "Add guide")
	if err != nil {
		t.Fatalf("CreateCommit 2 failed: %v", err)
	}

	// Verify commit2's tree has ALL 4 files
	tree2, err := reader.ReadTree(commit2)
	if err != nil {
		t.Fatalf("ReadTree commit2 failed: %v", err)
	}
	fileList, err := reader.ListFiles(tree2)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	expectedFiles := map[string]bool{
		"README.md":     true,
		"src/main.go":   true,
		"src/util.go":   true,
		"docs/guide.md": true,
	}

	if len(fileList) != len(expectedFiles) {
		t.Fatalf("Expected %d files, got %d: %v", len(expectedFiles), len(fileList), fileList)
	}
	for _, f := range fileList {
		if !expectedFiles[f] {
			t.Errorf("Unexpected file in commit tree: %s", f)
		}
	}
}

// TestSnapshotMerge_UpdatedFileTakesPrecedence verifies that when a staged file
// has the same path as a parent file, the staged version wins.
func TestSnapshotMerge_UpdatedFileTakesPrecedence(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// First commit
	initialFiles := createNamedTestFiles(casStore, map[string]string{
		"README.md":   "# Project v1",
		"src/main.go": "package main\nfunc main() {}",
	})
	commit1, err := builder.CreateCommit(initialFiles, nil, "A", "A", "Initial")
	if err != nil {
		t.Fatalf("CreateCommit 1 failed: %v", err)
	}
	commit1Hash := builder.GetCommitHash(commit1)

	parentTree, err := reader.ReadTree(commit1)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}
	parentFiles, err := reader.TreeToFileMetadata(parentTree)
	if err != nil {
		t.Fatalf("TreeToFileMetadata failed: %v", err)
	}

	// Second commit: update README.md with new content
	stagedFiles := createNamedTestFiles(casStore, map[string]string{
		"README.md": "# Project v2 - Updated",
	})

	allFiles := mergeParentAndStaged(parentFiles, stagedFiles)

	commit2, err := builder.CreateCommit(allFiles, []cas.Hash{commit1Hash}, "A", "A", "Update README")
	if err != nil {
		t.Fatalf("CreateCommit 2 failed: %v", err)
	}

	// Verify commit2 has both files
	tree2, err := reader.ReadTree(commit2)
	if err != nil {
		t.Fatalf("ReadTree commit2 failed: %v", err)
	}
	fileList, err := reader.ListFiles(tree2)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if len(fileList) != 2 {
		t.Fatalf("Expected 2 files, got %d: %v", len(fileList), fileList)
	}

	// Verify README.md has the updated content, not the original
	content, err := reader.GetFileContent(tree2, "README.md")
	if err != nil {
		t.Fatalf("GetFileContent failed: %v", err)
	}
	if string(content) != "# Project v2 - Updated" {
		t.Errorf("Expected updated README content, got: %q", string(content))
	}
}

// TestSnapshotMerge_WithoutMerge_LosesParentFiles demonstrates that without
// the merge step, a commit with only staged files would lose parent files.
// This documents the bug behavior to prevent regressions.
func TestSnapshotMerge_WithoutMerge_LosesParentFiles(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// First commit: 3 files
	initialFiles := createNamedTestFiles(casStore, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
		"file3.txt": "content3",
	})
	commit1, err := builder.CreateCommit(initialFiles, nil, "A", "A", "Initial")
	if err != nil {
		t.Fatalf("CreateCommit 1 failed: %v", err)
	}
	commit1Hash := builder.GetCommitHash(commit1)

	// Create commit2 with ONLY the new file (no merge — the buggy behavior)
	onlyNewFile := createNamedTestFiles(casStore, map[string]string{
		"file4.txt": "content4",
	})
	buggyCommit, err := builder.CreateCommit(onlyNewFile, []cas.Hash{commit1Hash}, "A", "A", "Add file4 (buggy)")
	if err != nil {
		t.Fatalf("CreateCommit buggy failed: %v", err)
	}

	buggyTree, err := reader.ReadTree(buggyCommit)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}
	buggyFiles, err := reader.ListFiles(buggyTree)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Without merge, commit2 only has 1 file — the bug
	if len(buggyFiles) != 1 {
		t.Fatalf("Expected buggy commit to have 1 file (demonstrating the bug), got %d", len(buggyFiles))
	}

	// Now do it correctly with merge
	parentTree, _ := reader.ReadTree(commit1)
	parentFiles, _ := reader.TreeToFileMetadata(parentTree)
	allFiles := mergeParentAndStaged(parentFiles, onlyNewFile)

	fixedCommit, err := builder.CreateCommit(allFiles, []cas.Hash{commit1Hash}, "A", "A", "Add file4 (fixed)")
	if err != nil {
		t.Fatalf("CreateCommit fixed failed: %v", err)
	}

	fixedTree, err := reader.ReadTree(fixedCommit)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}
	fixedFiles, err := reader.ListFiles(fixedTree)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// With merge, commit2 has all 4 files
	if len(fixedFiles) != 4 {
		t.Fatalf("Expected fixed commit to have 4 files, got %d: %v", len(fixedFiles), fixedFiles)
	}
}

// TestSnapshotMerge_ThreeCommitsChained verifies the merge works across a chain
// of 3 commits, each adding one new file. The final commit should have all files.
func TestSnapshotMerge_ThreeCommitsChained(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Commit 1: 2 files
	files1 := createNamedTestFiles(casStore, map[string]string{
		"a.txt": "aaa",
		"b.txt": "bbb",
	})
	c1, err := builder.CreateCommit(files1, nil, "A", "A", "Commit 1")
	if err != nil {
		t.Fatalf("Commit 1 failed: %v", err)
	}
	c1Hash := builder.GetCommitHash(c1)

	// Commit 2: add c.txt, merge with parent
	parentTree1, _ := reader.ReadTree(c1)
	parentFiles1, _ := reader.TreeToFileMetadata(parentTree1)
	staged2 := createNamedTestFiles(casStore, map[string]string{"c.txt": "ccc"})
	all2 := mergeParentAndStaged(parentFiles1, staged2)

	c2, err := builder.CreateCommit(all2, []cas.Hash{c1Hash}, "A", "A", "Commit 2")
	if err != nil {
		t.Fatalf("Commit 2 failed: %v", err)
	}
	c2Hash := builder.GetCommitHash(c2)

	// Commit 3: add d.txt, merge with parent
	parentTree2, _ := reader.ReadTree(c2)
	parentFiles2, _ := reader.TreeToFileMetadata(parentTree2)
	staged3 := createNamedTestFiles(casStore, map[string]string{"d.txt": "ddd"})
	all3 := mergeParentAndStaged(parentFiles2, staged3)

	c3, err := builder.CreateCommit(all3, []cas.Hash{c2Hash}, "A", "A", "Commit 3")
	if err != nil {
		t.Fatalf("Commit 3 failed: %v", err)
	}

	// Verify final commit has all 4 files
	tree3, err := reader.ReadTree(c3)
	if err != nil {
		t.Fatalf("ReadTree commit3 failed: %v", err)
	}
	fileList, err := reader.ListFiles(tree3)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	expected := map[string]bool{"a.txt": true, "b.txt": true, "c.txt": true, "d.txt": true}
	if len(fileList) != len(expected) {
		t.Fatalf("Expected %d files, got %d: %v", len(expected), len(fileList), fileList)
	}
	for _, f := range fileList {
		if !expected[f] {
			t.Errorf("Unexpected file: %s", f)
		}
	}
}

// TestSnapshotMerge_SubdirectoryFiles verifies merge works correctly when
// parent and staged files are in nested directories.
func TestSnapshotMerge_SubdirectoryFiles(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Commit 1: files in subdirectories
	files1 := createNamedTestFiles(casStore, map[string]string{
		"src/main.go":      "package main",
		"src/lib/helper.go": "package lib",
		"docs/readme.md":   "# Docs",
	})
	c1, err := builder.CreateCommit(files1, nil, "A", "A", "Initial")
	if err != nil {
		t.Fatalf("Commit 1 failed: %v", err)
	}
	c1Hash := builder.GetCommitHash(c1)

	// Commit 2: add file in existing subdir and a new subdir
	parentTree, _ := reader.ReadTree(c1)
	parentFiles, _ := reader.TreeToFileMetadata(parentTree)
	staged := createNamedTestFiles(casStore, map[string]string{
		"src/lib/utils.go": "package lib\nfunc util() {}",
		"test/main_test.go": "package test",
	})
	allFiles := mergeParentAndStaged(parentFiles, staged)

	c2, err := builder.CreateCommit(allFiles, []cas.Hash{c1Hash}, "A", "A", "Add more")
	if err != nil {
		t.Fatalf("Commit 2 failed: %v", err)
	}

	tree2, err := reader.ReadTree(c2)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}
	fileList, err := reader.ListFiles(tree2)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	expected := map[string]bool{
		"src/main.go":       true,
		"src/lib/helper.go": true,
		"src/lib/utils.go":  true,
		"docs/readme.md":    true,
		"test/main_test.go": true,
	}
	if len(fileList) != len(expected) {
		t.Fatalf("Expected %d files, got %d: %v", len(expected), len(fileList), fileList)
	}
	for _, f := range fileList {
		if !expected[f] {
			t.Errorf("Unexpected file: %s", f)
		}
	}
}

func BenchmarkCreateCommit(b *testing.B) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	files := createTestWorkspaceFiles(casStore)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.CreateCommit(
			files,
			nil,
			"Benchmark Author <bench@example.com>",
			"Benchmark Committer <bench@example.com>",
			"Benchmark commit",
		)
		if err != nil {
			b.Fatalf("CreateCommit failed: %v", err)
		}
	}
}

func BenchmarkReadCommit(b *testing.B) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := NewCommitBuilder(casStore, mmr)
	reader := NewCommitReader(casStore)

	// Setup
	files := createTestWorkspaceFiles(casStore)
	commit, err := builder.CreateCommit(
		files,
		nil,
		"Benchmark Author <bench@example.com>",
		"Benchmark Committer <bench@example.com>",
		"Benchmark commit",
	)
	if err != nil {
		b.Fatalf("Setup CreateCommit failed: %v", err)
	}

	commitHash := builder.GetCommitHash(commit)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reader.ReadCommit(commitHash)
		if err != nil {
			b.Fatalf("ReadCommit failed: %v", err)
		}
	}
}