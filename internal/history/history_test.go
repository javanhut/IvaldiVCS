package history

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestLeafCanonicalEncoding(t *testing.T) {
	leaf := &Leaf{
		TreeRoot:   [32]byte{1, 2, 3, 4},
		TimelineID: "main",
		PrevIdx:    42,
		MergeIdxs:  []uint64{10, 20},
		Author:     "Alice",
		TimeUnix:   1640995200,
		Message:    "Test commit",
		Meta: map[string]string{
			"tag":         "v1.0",
			"autoshelved": "1",
		},
	}

	// Test canonical encoding
	canonical := leaf.CanonicalBytes()
	if len(canonical) == 0 {
		t.Error("Canonical encoding should not be empty")
	}

	// Test parsing back
	parsed, err := parseLeafCanonical(canonical)
	if err != nil {
		t.Fatalf("Failed to parse canonical bytes: %v", err)
	}

	// Compare fields
	if parsed.TreeRoot != leaf.TreeRoot {
		t.Error("TreeRoot mismatch")
	}
	if parsed.TimelineID != leaf.TimelineID {
		t.Error("TimelineID mismatch")
	}
	if parsed.PrevIdx != leaf.PrevIdx {
		t.Error("PrevIdx mismatch")
	}
	if !reflect.DeepEqual(parsed.MergeIdxs, leaf.MergeIdxs) {
		t.Error("MergeIdxs mismatch")
	}
	if parsed.Author != leaf.Author {
		t.Error("Author mismatch")
	}
	if parsed.TimeUnix != leaf.TimeUnix {
		t.Error("TimeUnix mismatch")
	}
	if parsed.Message != leaf.Message {
		t.Error("Message mismatch")
	}
	if !reflect.DeepEqual(parsed.Meta, leaf.Meta) {
		t.Errorf("Meta mismatch: want %v, got %v", leaf.Meta, parsed.Meta)
	}
}

func TestLeafHash(t *testing.T) {
	leaf1 := &Leaf{
		TreeRoot:   [32]byte{1},
		TimelineID: "main",
		Author:     "Alice",
		TimeUnix:   1000,
		Message:    "Test",
	}

	leaf2 := &Leaf{
		TreeRoot:   [32]byte{1},
		TimelineID: "main", 
		Author:     "Alice",
		TimeUnix:   1000,
		Message:    "Test",
	}

	hash1 := leaf1.Hash()
	hash2 := leaf2.Hash()

	if hash1 != hash2 {
		t.Error("Identical leaves should have same hash")
	}

	// Modify one field
	leaf2.Message = "Different"
	hash3 := leaf2.Hash()

	if hash1 == hash3 {
		t.Error("Different leaves should have different hashes")
	}
}

func TestLeafHelpers(t *testing.T) {
	// Test HasParent
	leaf1 := &Leaf{PrevIdx: NoParent}
	leaf2 := &Leaf{PrevIdx: 42}

	if leaf1.HasParent() {
		t.Error("Leaf with NoParent should not have parent")
	}
	if !leaf2.HasParent() {
		t.Error("Leaf with valid PrevIdx should have parent")
	}

	// Test IsMerge
	leaf3 := &Leaf{MergeIdxs: nil}
	leaf4 := &Leaf{MergeIdxs: []uint64{10}}

	if leaf3.IsMerge() {
		t.Error("Leaf with no merge indices should not be merge")
	}
	if !leaf4.IsMerge() {
		t.Error("Leaf with merge indices should be merge")
	}

	// Test AllParents
	leaf5 := &Leaf{
		PrevIdx:   42,
		MergeIdxs: []uint64{10, 20},
	}

	parents := leaf5.AllParents()
	expected := []uint64{42, 10, 20}
	if !reflect.DeepEqual(parents, expected) {
		t.Errorf("AllParents mismatch: want %v, got %v", expected, parents)
	}

	// Test autoshelved
	leaf6 := &Leaf{}
	if leaf6.IsAutoshelved() {
		t.Error("New leaf should not be autoshelved")
	}

	leaf6.SetAutoshelved(true)
	if !leaf6.IsAutoshelved() {
		t.Error("Leaf should be autoshelved after setting")
	}

	leaf6.SetAutoshelved(false)
	if leaf6.IsAutoshelved() {
		t.Error("Leaf should not be autoshelved after unsetting")
	}
}

func TestMMR(t *testing.T) {
	mmr := NewMMR()

	if mmr.Size() != 0 {
		t.Error("New MMR should be empty")
	}

	if mmr.Root() != (Hash{}) {
		t.Error("Empty MMR should have zero root")
	}

	// Add first leaf
	leaf1 := Leaf{
		TreeRoot:   [32]byte{1},
		TimelineID: "main",
		Author:     "Alice",
		Message:    "First commit",
	}

	idx1, root1, err := mmr.AppendLeaf(leaf1)
	if err != nil {
		t.Fatalf("Failed to append leaf: %v", err)
	}

	if idx1 != 0 {
		t.Errorf("First leaf should have index 0, got %d", idx1)
	}

	if mmr.Size() != 1 {
		t.Errorf("MMR size should be 1, got %d", mmr.Size())
	}

	if root1 == (Hash{}) {
		t.Error("Root should not be zero after adding leaf")
	}

	// Add second leaf
	leaf2 := Leaf{
		TreeRoot:   [32]byte{2},
		TimelineID: "main", 
		PrevIdx:    idx1,
		Author:     "Alice",
		Message:    "Second commit",
	}

	idx2, root2, err := mmr.AppendLeaf(leaf2)
	if err != nil {
		t.Fatalf("Failed to append second leaf: %v", err)
	}

	if idx2 != 1 {
		t.Errorf("Second leaf should have index 1, got %d", idx2)
	}

	if root2 == root1 {
		t.Error("Root should change after adding second leaf")
	}

	// Test GetLeaf
	retrieved1, err := mmr.GetLeaf(idx1)
	if err != nil {
		t.Fatalf("Failed to get leaf: %v", err)
	}

	if retrieved1.Message != leaf1.Message {
		t.Error("Retrieved leaf doesn't match original")
	}

	// Test out of bounds
	_, err = mmr.GetLeaf(999)
	if err == nil {
		t.Error("Should error for out of bounds index")
	}
}

func TestMMRProofs(t *testing.T) {
	mmr := NewMMR()

	// Add several leaves
	leaves := make([]Leaf, 5)
	indices := make([]uint64, 5)
	
	for i := 0; i < 5; i++ {
		leaves[i] = Leaf{
			TreeRoot:   [32]byte{byte(i + 1)},
			TimelineID: "main",
			Author:     "Alice",
			Message:    fmt.Sprintf("Commit %d", i),
		}
		if i > 0 {
			leaves[i].PrevIdx = indices[i-1]
		} else {
			leaves[i].PrevIdx = NoParent
		}

		idx, _, err := mmr.AppendLeaf(leaves[i])
		if err != nil {
			t.Fatalf("Failed to append leaf %d: %v", i, err)
		}
		indices[i] = idx
	}

	finalRoot := mmr.Root()

	// Test proof generation and verification
	for i := 0; i < 5; i++ {
		proof, err := mmr.Proof(indices[i])
		if err != nil {
			t.Fatalf("Failed to generate proof for index %d: %v", i, err)
		}

		if proof.LeafIndex != indices[i] {
			t.Errorf("Proof has wrong leaf index: want %d, got %d", indices[i], proof.LeafIndex)
		}

		// Verify the proof
		leafHash := leaves[i].Hash()
		isValid := mmr.Verify(leafHash, proof, finalRoot)
		if !isValid {
			t.Errorf("Proof verification failed for leaf %d", i)
		}

		// Test with wrong hash
		wrongHash := [32]byte{byte(i + 100)}
		isValid = mmr.Verify(wrongHash, proof, finalRoot)
		if isValid {
			t.Errorf("Proof should not verify with wrong hash for leaf %d", i)
		}

		// Test with wrong root
		wrongRoot := [32]byte{99}
		isValid = mmr.Verify(leafHash, proof, wrongRoot)
		if isValid {
			t.Errorf("Proof should not verify with wrong root for leaf %d", i)
		}
	}
}

func TestMemoryTimelineStore(t *testing.T) {
	store := NewMemoryTimelineStore()

	// Test empty store
	if _, ok := store.GetHead("nonexistent"); ok {
		t.Error("Should not find nonexistent timeline")
	}

	if len(store.List()) != 0 {
		t.Error("Empty store should list no timelines")
	}

	// Test set/get
	err := store.SetHead("main", 42)
	if err != nil {
		t.Fatalf("Failed to set head: %v", err)
	}

	idx, ok := store.GetHead("main")
	if !ok {
		t.Error("Should find timeline after setting")
	}
	if idx != 42 {
		t.Errorf("Wrong head index: want 42, got %d", idx)
	}

	// Test list
	timelines := store.List()
	if len(timelines) != 1 || timelines[0] != "main" {
		t.Errorf("Wrong timeline list: %v", timelines)
	}

	// Test update
	err = store.SetHead("main", 100)
	if err != nil {
		t.Fatalf("Failed to update head: %v", err)
	}

	idx, ok = store.GetHead("main")
	if !ok || idx != 100 {
		t.Error("Head should be updated")
	}
}

func TestHistoryManager(t *testing.T) {
	mmr := NewMMR()
	timelineStore := NewMemoryTimelineStore()
	manager := NewHistoryManager(mmr, timelineStore)

	// Test first commit
	leaf1 := Leaf{
		TreeRoot: [32]byte{1},
		Author:   "Alice",
		TimeUnix: time.Now().Unix(),
		Message:  "Initial commit",
	}

	idx1, root1, err := manager.Commit("main", leaf1)
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	if idx1 != 0 {
		t.Error("First commit should have index 0")
	}

	// Verify timeline head was set
	head, ok := manager.GetTimelineHead("main")
	if !ok || head != idx1 {
		t.Error("Timeline head should be set to first commit")
	}

	// Verify leaf was filled correctly
	retrievedLeaf, err := manager.Accumulator().GetLeaf(idx1)
	if err != nil {
		t.Fatalf("Failed to get leaf: %v", err)
	}

	if retrievedLeaf.TimelineID != "main" {
		t.Error("TimelineID should be set to 'main'")
	}
	if retrievedLeaf.PrevIdx != NoParent {
		t.Error("First commit should have no parent")
	}

	// Test second commit
	leaf2 := Leaf{
		TreeRoot: [32]byte{2},
		Author:   "Alice",
		TimeUnix: time.Now().Unix(),
		Message:  "Second commit",
	}

	idx2, root2, err := manager.Commit("main", leaf2)
	if err != nil {
		t.Fatalf("Failed to commit second: %v", err)
	}

	// Verify parent was set automatically
	retrievedLeaf2, err := manager.Accumulator().GetLeaf(idx2)
	if err != nil {
		t.Fatalf("Failed to get second leaf: %v", err)
	}

	if retrievedLeaf2.PrevIdx != idx1 {
		t.Error("Second commit should have first as parent")
	}

	// Verify timeline head was updated
	head, ok = manager.GetTimelineHead("main")
	if !ok || head != idx2 {
		t.Error("Timeline head should be updated to second commit")
	}

	if root2 == root1 {
		t.Error("Root should change after second commit")
	}
}

func TestBranchingAndLCA(t *testing.T) {
	mmr := NewMMR()
	timelineStore := NewMemoryTimelineStore()
	manager := NewHistoryManager(mmr, timelineStore)

	// Create initial commits on main
	baseLeaf := Leaf{
		TreeRoot: [32]byte{1},
		Author:   "Alice",
		Message:  "Base commit",
	}
	baseIdx, _, err := manager.Commit("main", baseLeaf)
	if err != nil {
		t.Fatalf("Failed to create base commit: %v", err)
	}

	commit1Leaf := Leaf{
		TreeRoot: [32]byte{2},
		Author:   "Alice", 
		Message:  "Main commit 1",
	}
	main1Idx, _, err := manager.Commit("main", commit1Leaf)
	if err != nil {
		t.Fatalf("Failed to create main commit 1: %v", err)
	}

	// Create branch from base
	err = manager.SetTimelineHead("feature", baseIdx)
	if err != nil {
		t.Fatalf("Failed to create branch: %v", err)
	}

	// Commit to feature branch
	featureLeaf := Leaf{
		TreeRoot: [32]byte{3},
		Author:   "Bob",
		Message:  "Feature commit",
	}
	featureIdx, _, err := manager.Commit("feature", featureLeaf)
	if err != nil {
		t.Fatalf("Failed to commit to feature: %v", err)
	}

	// Test LCA of same timeline
	lca, err := manager.LCA(main1Idx, baseIdx)
	if err != nil {
		t.Fatalf("Failed to compute LCA: %v", err)
	}
	if lca != baseIdx {
		t.Errorf("LCA should be base commit, got %d", lca)
	}

	// Test LCA across timelines
	lca, err = manager.LCA(main1Idx, featureIdx)
	if err != nil {
		t.Fatalf("Failed to compute cross-timeline LCA: %v", err)
	}
	if lca != baseIdx {
		t.Errorf("Cross-timeline LCA should be base commit, got %d", lca)
	}

	// Test LCA of identical commits
	lca, err = manager.LCA(baseIdx, baseIdx)
	if err != nil {
		t.Fatalf("Failed to compute LCA of same commit: %v", err)
	}
	if lca != baseIdx {
		t.Errorf("LCA of same commit should be itself, got %d", lca)
	}
}

func TestMergeCommit(t *testing.T) {
	mmr := NewMMR()
	timelineStore := NewMemoryTimelineStore()
	manager := NewHistoryManager(mmr, timelineStore)

	// Set up two branches
	baseLeaf := Leaf{TreeRoot: [32]byte{1}, Author: "Alice", Message: "Base"}
	baseIdx, _, _ := manager.Commit("main", baseLeaf)

	manager.SetTimelineHead("feature", baseIdx)

	mainLeaf := Leaf{TreeRoot: [32]byte{2}, Author: "Alice", Message: "Main work"}
	mainIdx, _, _ := manager.Commit("main", mainLeaf)

	featureLeaf := Leaf{TreeRoot: [32]byte{3}, Author: "Bob", Message: "Feature work"}
	featureIdx, _, _ := manager.Commit("feature", featureLeaf)

	// Create merge commit
	mergeLeaf := Leaf{
		TreeRoot:  [32]byte{4}, // Merged tree
		Author:    "Alice",
		Message:   "Merge feature into main",
		MergeIdxs: []uint64{featureIdx}, // Merge in feature
	}

	mergeIdx, _, err := manager.Commit("main", mergeLeaf)
	if err != nil {
		t.Fatalf("Failed to create merge commit: %v", err)
	}

	// Verify merge commit properties
	retrievedMerge, err := manager.Accumulator().GetLeaf(mergeIdx)
	if err != nil {
		t.Fatalf("Failed to get merge commit: %v", err)
	}

	if !retrievedMerge.IsMerge() {
		t.Error("Merge commit should be detected as merge")
	}

	if retrievedMerge.PrevIdx != mainIdx {
		t.Error("Merge commit should have main as primary parent")
	}

	if len(retrievedMerge.MergeIdxs) != 1 || retrievedMerge.MergeIdxs[0] != featureIdx {
		t.Error("Merge commit should have feature as merge parent")
	}

	allParents := retrievedMerge.AllParents()
	expectedParents := []uint64{mainIdx, featureIdx}
	if !reflect.DeepEqual(allParents, expectedParents) {
		t.Errorf("All parents mismatch: want %v, got %v", expectedParents, allParents)
	}
}

func TestSkipTable(t *testing.T) {
	mmr := NewMMR()
	skipTable := NewSkipTable()

	// Create a linear chain
	var indices []uint64
	for i := 0; i < 10; i++ {
		leaf := Leaf{
			TreeRoot:   [32]byte{byte(i + 1)},
			TimelineID: "main",
			Author:     "Alice",
			Message:    fmt.Sprintf("Commit %d", i),
		}
		
		if i > 0 {
			leaf.PrevIdx = indices[i-1]
		} else {
			leaf.PrevIdx = NoParent
		}

		idx, _, err := mmr.AppendLeaf(leaf)
		if err != nil {
			t.Fatalf("Failed to append leaf %d: %v", i, err)
		}
		
		indices = append(indices, idx)
		skipTable.AddLeaf(idx, mmr)
	}

	// Test LCA within the chain
	lca, err := skipTable.LCA(indices[8], indices[3], mmr)
	if err != nil {
		t.Fatalf("Failed to compute LCA: %v", err)
	}
	if lca != indices[3] {
		t.Errorf("LCA should be indices[3], got %d", lca)
	}

	// Test LCA of same node
	lca, err = skipTable.LCA(indices[5], indices[5], mmr)
	if err != nil {
		t.Fatalf("Failed to compute LCA of same node: %v", err)
	}
	if lca != indices[5] {
		t.Errorf("LCA of same node should be itself")
	}
}

func TestAutoshelving(t *testing.T) {
	mmr := NewMMR()
	timelineStore := NewMemoryTimelineStore()
	manager := NewHistoryManager(mmr, timelineStore)

	// Create normal commit
	normalLeaf := Leaf{
		TreeRoot: [32]byte{1},
		Author:   "Alice",
		Message:  "Normal commit",
	}
	normalIdx, _, _ := manager.Commit("main", normalLeaf)

	// Create autoshelved commit
	autoshelvedLeaf := Leaf{
		TreeRoot: [32]byte{2},
		Author:   "Alice",
		Message:  "Auto-shelved commit",
	}
	autoshelvedLeaf.SetAutoshelved(true)
	
	autoshelvedIdx, _, _ := manager.Commit("temp", autoshelvedLeaf)

	// Verify autoshelving status
	retrievedNormal, _ := manager.Accumulator().GetLeaf(normalIdx)
	retrievedAutoshelved, _ := manager.Accumulator().GetLeaf(autoshelvedIdx)

	if retrievedNormal.IsAutoshelved() {
		t.Error("Normal commit should not be autoshelved")
	}

	if !retrievedAutoshelved.IsAutoshelved() {
		t.Error("Autoshelved commit should be detected as autoshelved")
	}
}

