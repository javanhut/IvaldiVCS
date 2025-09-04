package fsmerkle

import (
	"bytes"
	"reflect"
	"testing"

	"lukechampine.com/blake3"
)

func TestBlobNode(t *testing.T) {
	content := []byte("Hello, World!")
	blob := &BlobNode{Size: len(content)}

	// Test canonical bytes
	canonical := blob.CanonicalBytes()
	expected := []byte("blob 13\x00")
	if !bytes.Equal(canonical, expected) {
		t.Errorf("Canonical bytes mismatch:\nwant: %q\ngot:  %q", expected, canonical)
	}

	// Test hashing
	hash1 := blob.Hash(content)
	hash2 := blob.Hash(content)
	if hash1 != hash2 {
		t.Error("Same blob should produce same hash")
	}

	// Test with different content should produce different hash
	otherContent := []byte("Different content")
	otherBlob := &BlobNode{Size: len(otherContent)}
	otherHash := otherBlob.Hash(otherContent)
	if hash1 == otherHash {
		t.Error("Different blobs should produce different hashes")
	}
}

func TestBlobNodePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for content size mismatch")
		}
	}()
	
	blob := &BlobNode{Size: 5}
	blob.Hash([]byte("too long content"))
}

func TestTreeNode(t *testing.T) {
	entries := []Entry{
		{Name: "file.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
		{Name: "subdir", Mode: 040000, Kind: KindTree, Hash: [32]byte{2}},
	}

	tree := &TreeNode{Entries: entries}

	// Test canonical bytes
	canonical, err := tree.CanonicalBytes()
	if err != nil {
		t.Fatalf("Failed to get canonical bytes: %v", err)
	}

	// Verify we can parse it back
	parsed, err := parseTreeCanonical(canonical)
	if err != nil {
		t.Fatalf("Failed to parse canonical bytes: %v", err)
	}

	if !reflect.DeepEqual(tree.Entries, parsed.Entries) {
		t.Error("Parsed tree doesn't match original")
	}

	// Test hashing
	hash1, err := tree.Hash()
	if err != nil {
		t.Fatalf("Failed to hash tree: %v", err)
	}

	hash2, err := tree.Hash()
	if err != nil {
		t.Fatalf("Failed to hash tree: %v", err)
	}

	if hash1 != hash2 {
		t.Error("Same tree should produce same hash")
	}
}

func TestTreeNodeSorting(t *testing.T) {
	// Create unsorted entries
	entries := []Entry{
		{Name: "zebra.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
		{Name: "alpha.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{2}},
		{Name: "beta", Mode: 040000, Kind: KindTree, Hash: [32]byte{3}},
	}

	tree := &TreeNode{Entries: entries}
	tree.SortEntries()

	expectedOrder := []string{"alpha.txt", "beta", "zebra.txt"}
	for i, entry := range tree.Entries {
		if entry.Name != expectedOrder[i] {
			t.Errorf("Entry %d: expected %s, got %s", i, expectedOrder[i], entry.Name)
		}
	}
}

func TestTreeNodeValidation(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		wantErr bool
	}{
		{
			name: "valid tree",
			entries: []Entry{
				{Name: "file.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			entries: []Entry{
				{Name: "", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
			},
			wantErr: true,
		},
		{
			name: "invalid name '.'",
			entries: []Entry{
				{Name: ".", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
			},
			wantErr: true,
		},
		{
			name: "invalid name '..'",
			entries: []Entry{
				{Name: "..", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
			},
			wantErr: true,
		},
		{
			name: "name with slash",
			entries: []Entry{
				{Name: "path/file.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
			},
			wantErr: true,
		},
		{
			name: "duplicate names",
			entries: []Entry{
				{Name: "file.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
				{Name: "file.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{2}},
			},
			wantErr: true,
		},
		{
			name: "invalid mode for blob",
			entries: []Entry{
				{Name: "file.txt", Mode: 0755, Kind: KindBlob, Hash: [32]byte{1}},
			},
			wantErr: true,
		},
		{
			name: "invalid mode for tree",
			entries: []Entry{
				{Name: "subdir", Mode: 0100644, Kind: KindTree, Hash: [32]byte{1}},
			},
			wantErr: true,
		},
		{
			name: "unsorted entries",
			entries: []Entry{
				{Name: "zebra.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
				{Name: "alpha.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{2}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := &TreeNode{Entries: tt.entries}
			err := tree.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryCAS(t *testing.T) {
	cas := NewMemoryCAS()

	// Test Put/Get/Has
	content := []byte("test content")
	hash := blake3.Sum256(content)

	// Initially should not exist
	exists, err := cas.Has(hash)
	if err != nil {
		t.Fatalf("Has() failed: %v", err)
	}
	if exists {
		t.Error("Hash should not exist initially")
	}

	// Put content
	err = cas.Put(hash, content)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Should now exist
	exists, err = cas.Has(hash)
	if err != nil {
		t.Fatalf("Has() failed: %v", err)
	}
	if !exists {
		t.Error("Hash should exist after Put()")
	}

	// Get content back
	retrieved, err := cas.Get(hash)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if !bytes.Equal(content, retrieved) {
		t.Errorf("Retrieved content mismatch:\nwant: %q\ngot:  %q", content, retrieved)
	}
}

func TestMemoryCASHashMismatch(t *testing.T) {
	cas := NewMemoryCAS()

	content := []byte("test content")
	wrongHash := [32]byte{1, 2, 3} // Incorrect hash

	err := cas.Put(wrongHash, content)
	if err == nil {
		t.Error("Put() should fail with hash mismatch")
	}
}

func TestStore(t *testing.T) {
	cas := NewMemoryCAS()
	store := NewStore(cas)

	// Test blob storage
	content := []byte("Hello, World!")
	hash, size, err := store.PutBlob(content)
	if err != nil {
		t.Fatalf("PutBlob() failed: %v", err)
	}

	if size != len(content) {
		t.Errorf("Size mismatch: want %d, got %d", len(content), size)
	}

	// Load blob back
	blob, loadedContent, err := store.LoadBlob(hash)
	if err != nil {
		t.Fatalf("LoadBlob() failed: %v", err)
	}

	if blob.Size != len(content) {
		t.Errorf("Blob size mismatch: want %d, got %d", len(content), blob.Size)
	}

	if !bytes.Equal(content, loadedContent) {
		t.Errorf("Content mismatch:\nwant: %q\ngot:  %q", content, loadedContent)
	}

	// Test tree storage
	entries := []Entry{
		{Name: "file.txt", Mode: 0100644, Kind: KindBlob, Hash: hash},
	}

	treeHash, err := store.PutTree(entries)
	if err != nil {
		t.Fatalf("PutTree() failed: %v", err)
	}

	// Load tree back
	loadedTree, err := store.LoadTree(treeHash)
	if err != nil {
		t.Fatalf("LoadTree() failed: %v", err)
	}

	if len(loadedTree.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(loadedTree.Entries))
	}

	if loadedTree.Entries[0].Name != "file.txt" {
		t.Errorf("Entry name mismatch: want 'file.txt', got %q", loadedTree.Entries[0].Name)
	}
}

func TestBuildTreeFromMap(t *testing.T) {
	files := map[string][]byte{
		"README.md":    []byte("# Project"),
		"src/main.go":  []byte("package main"),
		"src/lib.go":   []byte("package lib"),
		"docs/api.md":  []byte("# API"),
	}

	rootHash, nodeCount, err := BuildTreeFromMap(files)
	if err != nil {
		t.Fatalf("BuildTreeFromMap() failed: %v", err)
	}

	if nodeCount == 0 {
		t.Error("Expected non-zero node count")
	}

	// Note: BuildTreeFromMap uses its own internal store, so we can't directly
	// load from our store. This is a limitation of the current API design.
	// In a real implementation, you'd want to pass the store to BuildTreeFromMap.
	_ = rootHash // Suppress unused variable warning
}

func TestBuildTreeFromMapEmpty(t *testing.T) {
	files := map[string][]byte{}

	rootHash, nodeCount, err := BuildTreeFromMap(files)
	if err != nil {
		t.Fatalf("BuildTreeFromMap() with empty map failed: %v", err)
	}

	if nodeCount != 1 {
		t.Errorf("Expected node count 1 for empty tree, got %d", nodeCount)
	}

	// Empty tree should have a deterministic hash
	rootHash2, _, err := BuildTreeFromMap(files)
	if err != nil {
		t.Fatalf("BuildTreeFromMap() second call failed: %v", err)
	}

	if rootHash != rootHash2 {
		t.Error("Empty trees should have same hash")
	}
}

func TestHashStability(t *testing.T) {
	// Test that same inputs always produce same hashes
	content := []byte("stable content")
	blob := &BlobNode{Size: len(content)}

	hash1 := blob.Hash(content)
	hash2 := blob.Hash(content)

	if hash1 != hash2 {
		t.Error("Hash should be stable for same input")
	}

	// Test tree hash stability
	entries := []Entry{
		{Name: "file.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
	}

	tree := &TreeNode{Entries: entries}
	treeHash1, err := tree.Hash()
	if err != nil {
		t.Fatalf("Tree hash failed: %v", err)
	}

	treeHash2, err := tree.Hash()
	if err != nil {
		t.Fatalf("Tree hash failed: %v", err)
	}

	if treeHash1 != treeHash2 {
		t.Error("Tree hash should be stable for same input")
	}
}

func TestStructuralSharing(t *testing.T) {
	// Create two trees that share a subtree
	files1 := map[string][]byte{
		"shared/file.txt": []byte("shared content"),
		"unique1.txt":     []byte("unique to tree 1"),
	}

	files2 := map[string][]byte{
		"shared/file.txt": []byte("shared content"),
		"unique2.txt":     []byte("unique to tree 2"),
	}

	root1, _, err := BuildTreeFromMap(files1)
	if err != nil {
		t.Fatalf("BuildTreeFromMap() failed: %v", err)
	}

	root2, _, err := BuildTreeFromMap(files2)
	if err != nil {
		t.Fatalf("BuildTreeFromMap() failed: %v", err)
	}

	// The roots should be different
	if root1 == root2 {
		t.Error("Different trees should have different root hashes")
	}

	// This test demonstrates structural sharing concept but we can't easily verify
	// it without exposing more internals. The shared subtree should have the same hash.
}

func TestDiffTrees(t *testing.T) {
	cas := NewMemoryCAS()
	store := NewStore(cas)

	// Create first tree
	content1 := []byte("content 1")
	hash1, _, err := store.PutBlob(content1)
	if err != nil {
		t.Fatalf("PutBlob failed: %v", err)
	}

	entries1 := []Entry{
		{Name: "file1.txt", Mode: 0100644, Kind: KindBlob, Hash: hash1},
	}
	tree1Hash, err := store.PutTree(entries1)
	if err != nil {
		t.Fatalf("PutTree failed: %v", err)
	}

	// Create second tree with modifications
	content2 := []byte("content 2")
	hash2, _, err := store.PutBlob(content2)
	if err != nil {
		t.Fatalf("PutBlob failed: %v", err)
	}

	content3 := []byte("content 3")
	hash3, _, err := store.PutBlob(content3)
	if err != nil {
		t.Fatalf("PutBlob failed: %v", err)
	}

	entries2 := []Entry{
		{Name: "file1.txt", Mode: 0100644, Kind: KindBlob, Hash: hash2}, // Modified
		{Name: "file2.txt", Mode: 0100644, Kind: KindBlob, Hash: hash3}, // Added
	}
	tree2Hash, err := store.PutTree(entries2)
	if err != nil {
		t.Fatalf("PutTree failed: %v", err)
	}

	// Diff the trees
	changes, err := DiffTrees(tree1Hash, tree2Hash, store)
	if err != nil {
		t.Fatalf("DiffTrees failed: %v", err)
	}

	if len(changes) != 2 {
		t.Fatalf("Expected 2 changes, got %d", len(changes))
	}

	// Check for modified file
	found := false
	for _, change := range changes {
		if change.Path == "file1.txt" && change.Kind == Modified {
			found = true
			if change.OldHash != hash1 || change.NewHash != hash2 {
				t.Error("Modified change has incorrect hashes")
			}
		}
	}
	if !found {
		t.Error("Expected modified change for file1.txt")
	}

	// Check for added file
	found = false
	for _, change := range changes {
		if change.Path == "file2.txt" && change.Kind == Added {
			found = true
			if change.NewHash != hash3 {
				t.Error("Added change has incorrect hash")
			}
		}
	}
	if !found {
		t.Error("Expected added change for file2.txt")
	}
}

func TestDiffTreesIdentical(t *testing.T) {
	cas := NewMemoryCAS()
	store := NewStore(cas)

	// Create a tree
	content := []byte("content")
	hash, _, err := store.PutBlob(content)
	if err != nil {
		t.Fatalf("PutBlob failed: %v", err)
	}

	entries := []Entry{
		{Name: "file.txt", Mode: 0100644, Kind: KindBlob, Hash: hash},
	}
	treeHash, err := store.PutTree(entries)
	if err != nil {
		t.Fatalf("PutTree failed: %v", err)
	}

	// Diff identical trees
	changes, err := DiffTrees(treeHash, treeHash, store)
	if err != nil {
		t.Fatalf("DiffTrees failed: %v", err)
	}

	if len(changes) != 0 {
		t.Errorf("Expected no changes for identical trees, got %d", len(changes))
	}
}

func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindBlob, "blob"},
		{KindTree, "tree"},
		{Kind(99), "unknown(99)"},
	}

	for _, tt := range tests {
		got := tt.kind.String()
		if got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestChangeKindString(t *testing.T) {
	tests := []struct {
		kind ChangeKind
		want string
	}{
		{Added, "added"},
		{Deleted, "deleted"},
		{Modified, "modified"},
		{TypeChange, "typechange"},
		{ChangeKind(99), "unknown(99)"},
	}

	for _, tt := range tests {
		got := tt.kind.String()
		if got != tt.want {
			t.Errorf("ChangeKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// Test utility functions
func TestSplitPath(t *testing.T) {
	tests := []struct {
		input   string
		wantDir string
		wantName string
	}{
		{"file.txt", "", "file.txt"},
		{"dir/file.txt", "dir", "file.txt"},
		{"deep/nested/file.txt", "deep/nested", "file.txt"},
		{".", "", ""},
		{"/absolute/path", "/absolute", "path"},
	}

	for _, tt := range tests {
		gotDir, gotName := splitPath(tt.input)
		if gotDir != tt.wantDir || gotName != tt.wantName {
			t.Errorf("splitPath(%q) = (%q, %q), want (%q, %q)",
				tt.input, gotDir, gotName, tt.wantDir, tt.wantName)
		}
	}
}