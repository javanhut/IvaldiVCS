package shift

import (
	"testing"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

func TestValidateRange(t *testing.T) {
	// Create in-memory CAS
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)

	// Create a chain of commits: C1 -> C2 -> C3
	files1 := []wsindex.FileMetadata{
		{
			Path: "file1.txt",
			FileRef: filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content1")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
			ModTime:  time.Now(),
			Mode:     0644,
			Size:     8,
			Checksum: cas.SumB3([]byte("content1")),
		},
	}

	commit1, err := builder.CreateCommit(files1, nil, "author", "author", "First commit")
	if err != nil {
		t.Fatalf("Failed to create commit 1: %v", err)
	}
	hash1 := builder.GetCommitHash(commit1)

	// Create second commit
	files2 := append(files1, wsindex.FileMetadata{
		Path: "file2.txt",
		FileRef: filechunk.NodeRef{
			Hash: cas.SumB3([]byte("content2")),
			Kind: filechunk.Leaf,
			Size: 8,
		},
		ModTime:  time.Now(),
		Mode:     0644,
		Size:     8,
		Checksum: cas.SumB3([]byte("content2")),
	})

	commit2, err := builder.CreateCommit(files2, []cas.Hash{hash1}, "author", "author", "Second commit")
	if err != nil {
		t.Fatalf("Failed to create commit 2: %v", err)
	}
	hash2 := builder.GetCommitHash(commit2)

	// Create third commit
	files3 := append(files2, wsindex.FileMetadata{
		Path: "file3.txt",
		FileRef: filechunk.NodeRef{
			Hash: cas.SumB3([]byte("content3")),
			Kind: filechunk.Leaf,
			Size: 8,
		},
		ModTime:  time.Now(),
		Mode:     0644,
		Size:     8,
		Checksum: cas.SumB3([]byte("content3")),
	})

	commit3, err := builder.CreateCommit(files3, []cas.Hash{hash2}, "author", "author", "Third commit")
	if err != nil {
		t.Fatalf("Failed to create commit 3: %v", err)
	}
	hash3 := builder.GetCommitHash(commit3)

	// Test valid range: hash1 to hash3
	err = squasher.ValidateRange(hash1, hash3)
	if err != nil {
		t.Errorf("Expected valid range hash1->hash3, got error: %v", err)
	}

	// Test valid range: hash2 to hash3
	err = squasher.ValidateRange(hash2, hash3)
	if err != nil {
		t.Errorf("Expected valid range hash2->hash3, got error: %v", err)
	}

	// Test invalid range: hash3 to hash1 (reversed)
	err = squasher.ValidateRange(hash3, hash1)
	if err == nil {
		t.Error("Expected error for reversed range hash3->hash1, got nil")
	}

	// Test invalid range: non-existent commit
	fakeHash := cas.SumB3([]byte("fake"))
	err = squasher.ValidateRange(hash1, fakeHash)
	if err == nil {
		t.Error("Expected error for non-existent end commit, got nil")
	}
}

func TestGetCommitRange(t *testing.T) {
	// Create in-memory CAS
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)

	// Create commits
	files := []wsindex.FileMetadata{
		{
			Path: "test.txt",
			FileRef: filechunk.NodeRef{
				Hash: cas.SumB3([]byte("test")),
				Kind: filechunk.Leaf,
				Size: 4,
			},
			ModTime:  time.Now(),
			Mode:     0644,
			Size:     4,
			Checksum: cas.SumB3([]byte("test")),
		},
	}

	// Commit 1
	commit1, _ := builder.CreateCommit(files, nil, "author", "author", "Commit 1")
	hash1 := builder.GetCommitHash(commit1)

	// Commit 2
	commit2, _ := builder.CreateCommit(files, []cas.Hash{hash1}, "author", "author", "Commit 2")
	hash2 := builder.GetCommitHash(commit2)

	// Commit 3
	commit3, _ := builder.CreateCommit(files, []cas.Hash{hash2}, "author", "author", "Commit 3")
	hash3 := builder.GetCommitHash(commit3)

	// Get range from hash1 to hash3
	commits, err := squasher.GetCommitRange(hash1, hash3)
	if err != nil {
		t.Fatalf("Failed to get commit range: %v", err)
	}

	// Verify we got all 3 commits in chronological order
	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}

	if commits[0].Message != "Commit 1" {
		t.Errorf("Expected first commit message 'Commit 1', got '%s'", commits[0].Message)
	}

	if commits[2].Message != "Commit 3" {
		t.Errorf("Expected third commit message 'Commit 3', got '%s'", commits[2].Message)
	}
}

func TestGetCombinedMessage(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)

	// Test empty commits
	commits := []CommitInfo{}
	msg := squasher.GetCombinedMessage(commits)
	if msg != "Empty squash" {
		t.Errorf("Expected 'Empty squash' for empty commits, got '%s'", msg)
	}

	// Test single commit
	commits = []CommitInfo{
		{Message: "Single commit message"},
	}
	msg = squasher.GetCombinedMessage(commits)
	if msg != "Single commit message" {
		t.Errorf("Expected single message to be preserved, got '%s'", msg)
	}

	// Test multiple commits
	commits = []CommitInfo{
		{Message: "First commit"},
		{Message: "Second commit"},
		{Message: "Third commit"},
	}
	msg = squasher.GetCombinedMessage(commits)
	expected := "Squashed 3 commits:\n\nFirst commit\nSecond commit\nThird commit"
	if msg != expected {
		t.Errorf("Expected combined message:\n%s\n\nGot:\n%s", expected, msg)
	}

	// Test multi-line commits (should only use first line)
	commits = []CommitInfo{
		{Message: "First line\nSecond line\nThird line"},
		{Message: "Another commit\nWith details"},
	}
	msg = squasher.GetCombinedMessage(commits)
	expected = "Squashed 2 commits:\n\nFirst line\nAnother commit"
	if msg != expected {
		t.Errorf("Expected first lines only:\n%s\n\nGot:\n%s", expected, msg)
	}
}

func TestGetParentOfStart(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)

	files := []wsindex.FileMetadata{
		{
			Path: "test.txt",
			FileRef: filechunk.NodeRef{
				Hash: cas.SumB3([]byte("test")),
				Kind: filechunk.Leaf,
				Size: 4,
			},
			ModTime:  time.Now(),
			Mode:     0644,
			Size:     4,
			Checksum: cas.SumB3([]byte("test")),
		},
	}

	// Create root commit (no parent)
	commit1, _ := builder.CreateCommit(files, nil, "author", "author", "Root commit")
	hash1 := builder.GetCommitHash(commit1)

	// Create child commit
	commit2, _ := builder.CreateCommit(files, []cas.Hash{hash1}, "author", "author", "Child commit")
	hash2 := builder.GetCommitHash(commit2)

	// Test root commit (should return zero hash)
	parent, err := squasher.GetParentOfStart(hash1)
	if err != nil {
		t.Fatalf("Failed to get parent of root: %v", err)
	}
	if parent != (cas.Hash{}) {
		t.Error("Expected zero hash for root commit parent")
	}

	// Test child commit (should return hash1)
	parent, err = squasher.GetParentOfStart(hash2)
	if err != nil {
		t.Fatalf("Failed to get parent of child: %v", err)
	}
	if parent != hash1 {
		t.Error("Expected parent to be hash1")
	}
}

func TestExtractFinalState(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)

	// Build actual files in CAS using filechunk.NewBuilder
	fileBuilder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())

	content1 := []byte("hello world")
	fileRef1, err := fileBuilder.Build(content1)
	if err != nil {
		t.Fatalf("Failed to build file1: %v", err)
	}

	content2 := []byte("goodbye world")
	fileRef2, err := fileBuilder.Build(content2)
	if err != nil {
		t.Fatalf("Failed to build file2: %v", err)
	}

	files := []wsindex.FileMetadata{
		{
			Path:     "file1.txt",
			FileRef:  fileRef1,
			ModTime:  time.Now(),
			Mode:     0644,
			Size:     int64(len(content1)),
			Checksum: cas.SumB3(content1),
		},
		{
			Path:     "file2.txt",
			FileRef:  fileRef2,
			ModTime:  time.Now(),
			Mode:     0644,
			Size:     int64(len(content2)),
			Checksum: cas.SumB3(content2),
		},
	}

	commitObj, err := builder.CreateCommit(files, nil, "author", "author", "Test commit")
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}
	commitHash := builder.GetCommitHash(commitObj)

	// Extract final state
	state, err := squasher.ExtractFinalState(commitHash)
	if err != nil {
		t.Fatalf("Failed to extract final state: %v", err)
	}

	if len(state) != 2 {
		t.Fatalf("Expected 2 files in final state, got %d", len(state))
	}

	// Build a map for easier lookup
	stateMap := make(map[string]wsindex.FileMetadata)
	for _, f := range state {
		stateMap[f.Path] = f
	}

	// Verify file1.txt
	f1, ok := stateMap["file1.txt"]
	if !ok {
		t.Fatal("file1.txt not found in final state")
	}
	if f1.Size != int64(len(content1)) {
		t.Errorf("file1.txt: expected size %d, got %d", len(content1), f1.Size)
	}
	if f1.Checksum != cas.SumB3(content1) {
		t.Error("file1.txt: checksum mismatch")
	}

	// Verify file2.txt
	f2, ok := stateMap["file2.txt"]
	if !ok {
		t.Fatal("file2.txt not found in final state")
	}
	if f2.Size != int64(len(content2)) {
		t.Errorf("file2.txt: expected size %d, got %d", len(content2), f2.Size)
	}
	if f2.Checksum != cas.SumB3(content2) {
		t.Error("file2.txt: checksum mismatch")
	}
}

func TestCreateSquashedCommit(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)
	fileBuilder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())

	// Build files in CAS
	contentA := []byte("alpha content")
	refA, err := fileBuilder.Build(contentA)
	if err != nil {
		t.Fatalf("Failed to build fileA: %v", err)
	}

	contentB := []byte("beta content")
	refB, err := fileBuilder.Build(contentB)
	if err != nil {
		t.Fatalf("Failed to build fileB: %v", err)
	}

	contentC := []byte("gamma content")
	refC, err := fileBuilder.Build(contentC)
	if err != nil {
		t.Fatalf("Failed to build fileC: %v", err)
	}

	// Create chain of 3 commits
	files1 := []wsindex.FileMetadata{
		{Path: "a.txt", FileRef: refA, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentA)), Checksum: cas.SumB3(contentA)},
	}
	commit1, err := builder.CreateCommit(files1, nil, "author", "author", "Add a.txt")
	if err != nil {
		t.Fatalf("Failed to create commit1: %v", err)
	}
	hash1 := builder.GetCommitHash(commit1)

	files2 := []wsindex.FileMetadata{
		{Path: "a.txt", FileRef: refA, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentA)), Checksum: cas.SumB3(contentA)},
		{Path: "b.txt", FileRef: refB, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentB)), Checksum: cas.SumB3(contentB)},
	}
	commit2, err := builder.CreateCommit(files2, []cas.Hash{hash1}, "author", "author", "Add b.txt")
	if err != nil {
		t.Fatalf("Failed to create commit2: %v", err)
	}
	hash2 := builder.GetCommitHash(commit2)

	files3 := []wsindex.FileMetadata{
		{Path: "a.txt", FileRef: refA, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentA)), Checksum: cas.SumB3(contentA)},
		{Path: "b.txt", FileRef: refB, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentB)), Checksum: cas.SumB3(contentB)},
		{Path: "c.txt", FileRef: refC, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentC)), Checksum: cas.SumB3(contentC)},
	}
	commit3, err := builder.CreateCommit(files3, []cas.Hash{hash2}, "author", "author", "Add c.txt")
	if err != nil {
		t.Fatalf("Failed to create commit3: %v", err)
	}
	hash3 := builder.GetCommitHash(commit3)

	// Extract final state from commit3
	finalState, err := squasher.ExtractFinalState(hash3)
	if err != nil {
		t.Fatalf("Failed to extract final state: %v", err)
	}

	// Get parent of start (commit1 has no parent, so zero hash)
	parentHash, err := squasher.GetParentOfStart(hash1)
	if err != nil {
		t.Fatalf("Failed to get parent of start: %v", err)
	}

	// Create squashed commit
	squashedCommit, squashedHash, err := squasher.CreateSquashedCommit(
		finalState, parentHash, "author", "Squashed: add a, b, c",
	)
	if err != nil {
		t.Fatalf("Failed to create squashed commit: %v", err)
	}

	// Verify squashed commit has no parents (since parent is zero hash)
	if len(squashedCommit.Parents) != 0 {
		t.Errorf("Expected 0 parents for squashed commit, got %d", len(squashedCommit.Parents))
	}

	// Verify message
	if squashedCommit.Message != "Squashed: add a, b, c" {
		t.Errorf("Expected message 'Squashed: add a, b, c', got '%s'", squashedCommit.Message)
	}

	// Verify the squashed commit can be read back and its tree contains all files
	reader := commit.NewCommitReader(casStore)
	readBack, err := reader.ReadCommit(squashedHash)
	if err != nil {
		t.Fatalf("Failed to read squashed commit: %v", err)
	}

	tree, err := reader.ReadTree(readBack)
	if err != nil {
		t.Fatalf("Failed to read squashed commit tree: %v", err)
	}

	listedFiles, err := reader.ListFiles(tree)
	if err != nil {
		t.Fatalf("Failed to list files from squashed tree: %v", err)
	}

	if len(listedFiles) != 3 {
		t.Errorf("Expected 3 files in squashed tree, got %d", len(listedFiles))
	}

	_ = hash3 // used above in ExtractFinalState
}

func TestCreateSquashedCommit_NoParent(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)
	fileBuilder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())

	content := []byte("root file content")
	ref, err := fileBuilder.Build(content)
	if err != nil {
		t.Fatalf("Failed to build file: %v", err)
	}

	files := []wsindex.FileMetadata{
		{Path: "root.txt", FileRef: ref, ModTime: time.Now(), Mode: 0644, Size: int64(len(content)), Checksum: cas.SumB3(content)},
	}

	// Create squashed commit with zero hash parent (root commit)
	squashedCommit, squashedHash, err := squasher.CreateSquashedCommit(
		files, cas.Hash{}, "author", "Root squashed commit",
	)
	if err != nil {
		t.Fatalf("Failed to create squashed commit with no parent: %v", err)
	}

	// Verify no parents
	if len(squashedCommit.Parents) != 0 {
		t.Errorf("Expected 0 parents, got %d", len(squashedCommit.Parents))
	}

	if squashedCommit.Message != "Root squashed commit" {
		t.Errorf("Expected message 'Root squashed commit', got '%s'", squashedCommit.Message)
	}

	// Verify commit is readable and tree has the file
	reader := commit.NewCommitReader(casStore)
	readBack, err := reader.ReadCommit(squashedHash)
	if err != nil {
		t.Fatalf("Failed to read back squashed commit: %v", err)
	}

	tree, err := reader.ReadTree(readBack)
	if err != nil {
		t.Fatalf("Failed to read tree: %v", err)
	}

	listedFiles, err := reader.ListFiles(tree)
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(listedFiles) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(listedFiles))
	}
	if listedFiles[0] != "root.txt" {
		t.Errorf("Expected file 'root.txt', got '%s'", listedFiles[0])
	}
}

func TestSquashWorkflow_EndToEnd(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)
	fileBuilder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())

	// Build 3 different files in CAS
	contentX := []byte("x-ray data")
	refX, err := fileBuilder.Build(contentX)
	if err != nil {
		t.Fatalf("Failed to build fileX: %v", err)
	}

	contentY := []byte("yankee data")
	refY, err := fileBuilder.Build(contentY)
	if err != nil {
		t.Fatalf("Failed to build fileY: %v", err)
	}

	contentZ := []byte("zulu data")
	refZ, err := fileBuilder.Build(contentZ)
	if err != nil {
		t.Fatalf("Failed to build fileZ: %v", err)
	}

	// Create 3 commits, each adding a new file
	files1 := []wsindex.FileMetadata{
		{Path: "x.txt", FileRef: refX, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentX)), Checksum: cas.SumB3(contentX)},
	}
	c1, err := builder.CreateCommit(files1, nil, "dev", "dev", "Add x.txt")
	if err != nil {
		t.Fatalf("Failed to create c1: %v", err)
	}
	h1 := builder.GetCommitHash(c1)

	files2 := []wsindex.FileMetadata{
		{Path: "x.txt", FileRef: refX, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentX)), Checksum: cas.SumB3(contentX)},
		{Path: "y.txt", FileRef: refY, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentY)), Checksum: cas.SumB3(contentY)},
	}
	c2, err := builder.CreateCommit(files2, []cas.Hash{h1}, "dev", "dev", "Add y.txt")
	if err != nil {
		t.Fatalf("Failed to create c2: %v", err)
	}
	h2 := builder.GetCommitHash(c2)

	files3 := []wsindex.FileMetadata{
		{Path: "x.txt", FileRef: refX, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentX)), Checksum: cas.SumB3(contentX)},
		{Path: "y.txt", FileRef: refY, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentY)), Checksum: cas.SumB3(contentY)},
		{Path: "z.txt", FileRef: refZ, ModTime: time.Now(), Mode: 0644, Size: int64(len(contentZ)), Checksum: cas.SumB3(contentZ)},
	}
	c3, err := builder.CreateCommit(files3, []cas.Hash{h2}, "dev", "dev", "Add z.txt")
	if err != nil {
		t.Fatalf("Failed to create c3: %v", err)
	}
	h3 := builder.GetCommitHash(c3)

	// Step 1: Get commit range
	commits, err := squasher.GetCommitRange(h1, h3)
	if err != nil {
		t.Fatalf("GetCommitRange failed: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("Expected 3 commits in range, got %d", len(commits))
	}

	// Step 2: Get combined message
	msg := squasher.GetCombinedMessage(commits)
	expectedMsg := "Squashed 3 commits:\n\nAdd x.txt\nAdd y.txt\nAdd z.txt"
	if msg != expectedMsg {
		t.Errorf("Combined message mismatch.\nExpected:\n%s\nGot:\n%s", expectedMsg, msg)
	}

	// Step 3: Extract final state from end commit
	finalState, err := squasher.ExtractFinalState(h3)
	if err != nil {
		t.Fatalf("ExtractFinalState failed: %v", err)
	}
	if len(finalState) != 3 {
		t.Fatalf("Expected 3 files in final state, got %d", len(finalState))
	}

	// Step 4: Get parent of start
	parentHash, err := squasher.GetParentOfStart(h1)
	if err != nil {
		t.Fatalf("GetParentOfStart failed: %v", err)
	}
	if parentHash != (cas.Hash{}) {
		t.Error("Expected zero hash parent for root commit")
	}

	// Step 5: Create squashed commit
	squashedCommit, squashedHash, err := squasher.CreateSquashedCommit(
		finalState, parentHash, "dev", msg,
	)
	if err != nil {
		t.Fatalf("CreateSquashedCommit failed: %v", err)
	}

	// Verify squashed commit properties
	if len(squashedCommit.Parents) != 0 {
		t.Errorf("Expected 0 parents, got %d", len(squashedCommit.Parents))
	}
	if squashedCommit.Message != expectedMsg {
		t.Errorf("Squashed commit message mismatch")
	}

	// Step 6: Verify the squashed commit tree has all 3 files
	reader := commit.NewCommitReader(casStore)
	readBack, err := reader.ReadCommit(squashedHash)
	if err != nil {
		t.Fatalf("Failed to read squashed commit: %v", err)
	}

	tree, err := reader.ReadTree(readBack)
	if err != nil {
		t.Fatalf("Failed to read squashed tree: %v", err)
	}

	listedFiles, err := reader.ListFiles(tree)
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(listedFiles) != 3 {
		t.Fatalf("Expected 3 files in squashed tree, got %d", len(listedFiles))
	}

	// Verify each file's content is readable from the squashed tree
	fileSet := make(map[string]bool)
	for _, f := range listedFiles {
		fileSet[f] = true
	}
	for _, expected := range []string{"x.txt", "y.txt", "z.txt"} {
		if !fileSet[expected] {
			t.Errorf("File %s not found in squashed tree", expected)
		}
	}

	// Verify file contents are intact
	contentMap := map[string][]byte{
		"x.txt": contentX,
		"y.txt": contentY,
		"z.txt": contentZ,
	}
	for path, expectedContent := range contentMap {
		actual, err := reader.GetFileContent(tree, path)
		if err != nil {
			t.Errorf("Failed to read %s from squashed tree: %v", path, err)
			continue
		}
		if string(actual) != string(expectedContent) {
			t.Errorf("Content mismatch for %s: expected %q, got %q", path, expectedContent, actual)
		}
	}
}

func TestGetCommitRange_SingleCommit(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)
	fileBuilder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())

	content := []byte("single commit file")
	ref, err := fileBuilder.Build(content)
	if err != nil {
		t.Fatalf("Failed to build file: %v", err)
	}

	files := []wsindex.FileMetadata{
		{Path: "only.txt", FileRef: ref, ModTime: time.Now(), Mode: 0644, Size: int64(len(content)), Checksum: cas.SumB3(content)},
	}

	c, err := builder.CreateCommit(files, nil, "author", "author", "Only commit")
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}
	h := builder.GetCommitHash(c)

	// Get range where start == end
	commits, err := squasher.GetCommitRange(h, h)
	if err != nil {
		t.Fatalf("GetCommitRange with single commit failed: %v", err)
	}

	if len(commits) != 1 {
		t.Fatalf("Expected 1 commit, got %d", len(commits))
	}

	if commits[0].Message != "Only commit" {
		t.Errorf("Expected message 'Only commit', got '%s'", commits[0].Message)
	}

	if commits[0].Hash != h {
		t.Error("Expected commit hash to match")
	}
}

func TestValidateRange_SameCommit(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)
	fileBuilder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())

	content := []byte("same commit content")
	ref, err := fileBuilder.Build(content)
	if err != nil {
		t.Fatalf("Failed to build file: %v", err)
	}

	files := []wsindex.FileMetadata{
		{Path: "same.txt", FileRef: ref, ModTime: time.Now(), Mode: 0644, Size: int64(len(content)), Checksum: cas.SumB3(content)},
	}

	c, err := builder.CreateCommit(files, nil, "author", "author", "Same commit")
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}
	h := builder.GetCommitHash(c)

	// Validate range where start == end (should be valid)
	err = squasher.ValidateRange(h, h)
	if err != nil {
		t.Errorf("Expected ValidateRange with same commit to succeed, got error: %v", err)
	}
}
