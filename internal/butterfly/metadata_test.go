package butterfly

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
)

func setupMetadataStore(t *testing.T) (*MetadataStore, string, func()) {
	tmpDir, err := os.MkdirTemp("", "metadata-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ivaldiDir := filepath.Join(tmpDir, ".ivaldi")
	if err := os.MkdirAll(filepath.Join(ivaldiDir, "butterflies"), 0755); err != nil {
		t.Fatalf("failed to create butterflies dir: %v", err)
	}

	store, err := NewMetadataStore(ivaldiDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create metadata store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, ivaldiDir, cleanup
}

func TestNewMetadataStore(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	if store == nil {
		t.Fatal("expected metadata store to be created")
	}

	if store.db == nil {
		t.Error("expected database to be initialized")
	}
}

func TestStoreAndGetButterfly(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	bf := &Butterfly{
		Name:           "test-butterfly",
		ParentName:     "main",
		DivergenceHash: divergenceHash,
		CreatedAt:      time.Now(),
		IsOrphaned:     false,
		OriginalParent: "",
	}

	err := store.StoreButterfly(bf)
	if err != nil {
		t.Fatalf("failed to store butterfly: %v", err)
	}

	retrieved, err := store.GetButterfly("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get butterfly: %v", err)
	}

	if retrieved.Name != bf.Name {
		t.Errorf("expected name '%s', got '%s'", bf.Name, retrieved.Name)
	}

	if retrieved.ParentName != bf.ParentName {
		t.Errorf("expected parent '%s', got '%s'", bf.ParentName, retrieved.ParentName)
	}

	if retrieved.DivergenceHash != bf.DivergenceHash {
		t.Error("divergence hash mismatch")
	}

	if retrieved.IsOrphaned != bf.IsOrphaned {
		t.Errorf("expected IsOrphaned=%v, got %v", bf.IsOrphaned, retrieved.IsOrphaned)
	}
}

func TestGetButterflyNotFound(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	_, err := store.GetButterfly("nonexistent")
	if err == nil {
		t.Error("expected error when getting nonexistent butterfly")
	}
}

func TestMetadataStoreDeleteButterfly(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	bf := &Butterfly{
		Name:           "test-butterfly",
		ParentName:     "main",
		DivergenceHash: divergenceHash,
		CreatedAt:      time.Now(),
		IsOrphaned:     false,
		OriginalParent: "",
	}

	err := store.StoreButterfly(bf)
	if err != nil {
		t.Fatalf("failed to store butterfly: %v", err)
	}

	err = store.DeleteButterfly("test-butterfly")
	if err != nil {
		t.Fatalf("failed to delete butterfly: %v", err)
	}

	_, err = store.GetButterfly("test-butterfly")
	if err == nil {
		t.Error("butterfly should be deleted")
	}
}

func TestAddAndGetChildren(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	err := store.AddChild("parent", "child1")
	if err != nil {
		t.Fatalf("failed to add child1: %v", err)
	}

	err = store.AddChild("parent", "child2")
	if err != nil {
		t.Fatalf("failed to add child2: %v", err)
	}

	children, err := store.GetChildren("parent")
	if err != nil {
		t.Fatalf("failed to get children: %v", err)
	}

	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}

	found1, found2 := false, false
	for _, child := range children {
		if child == "child1" {
			found1 = true
		}
		if child == "child2" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Error("expected to find both child1 and child2")
	}
}

func TestGetChildrenEmpty(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	children, err := store.GetChildren("nonexistent")
	if err != nil {
		t.Fatalf("failed to get children: %v", err)
	}

	if len(children) != 0 {
		t.Errorf("expected 0 children, got %d", len(children))
	}
}

func TestRemoveChild(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	err := store.AddChild("parent", "child1")
	if err != nil {
		t.Fatalf("failed to add child1: %v", err)
	}

	err = store.AddChild("parent", "child2")
	if err != nil {
		t.Fatalf("failed to add child2: %v", err)
	}

	err = store.RemoveChild("parent", "child1")
	if err != nil {
		t.Fatalf("failed to remove child1: %v", err)
	}

	children, err := store.GetChildren("parent")
	if err != nil {
		t.Fatalf("failed to get children: %v", err)
	}

	if len(children) != 1 {
		t.Errorf("expected 1 child, got %d", len(children))
	}

	if children[0] != "child2" {
		t.Errorf("expected remaining child to be 'child2', got '%s'", children[0])
	}
}

func TestRemoveLastChild(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	err := store.AddChild("parent", "child1")
	if err != nil {
		t.Fatalf("failed to add child1: %v", err)
	}

	err = store.RemoveChild("parent", "child1")
	if err != nil {
		t.Fatalf("failed to remove child1: %v", err)
	}

	children, err := store.GetChildren("parent")
	if err != nil {
		t.Fatalf("failed to get children: %v", err)
	}

	if len(children) != 0 {
		t.Errorf("expected 0 children after removing last child, got %d", len(children))
	}
}

func TestRemoveChildNonexistent(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	err := store.RemoveChild("nonexistent", "child")
	if err != nil {
		t.Errorf("removing child from nonexistent parent should not error: %v", err)
	}
}

func TestGetMetadataButterfly(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	bf := &Butterfly{
		Name:           "test-butterfly",
		ParentName:     "main",
		DivergenceHash: divergenceHash,
		CreatedAt:      time.Now(),
		IsOrphaned:     false,
		OriginalParent: "",
	}

	err := store.StoreButterfly(bf)
	if err != nil {
		t.Fatalf("failed to store butterfly: %v", err)
	}

	err = store.AddChild("test-butterfly", "child1")
	if err != nil {
		t.Fatalf("failed to add child: %v", err)
	}

	metadata, err := store.GetMetadata("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}

	if !metadata.IsButterfly {
		t.Error("expected metadata to indicate butterfly timeline")
	}

	if metadata.Butterfly == nil {
		t.Error("expected butterfly info in metadata")
	}

	if metadata.Timeline != "test-butterfly" {
		t.Errorf("expected timeline 'test-butterfly', got '%s'", metadata.Timeline)
	}

	if len(metadata.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(metadata.Children))
	}
}

func TestMetadataGetMetadataNonButterfly(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	metadata, err := store.GetMetadata("regular-timeline")
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}

	if metadata.IsButterfly {
		t.Error("expected metadata to indicate non-butterfly timeline")
	}

	if metadata.Butterfly != nil {
		t.Error("expected no butterfly info for regular timeline")
	}

	if metadata.Timeline != "regular-timeline" {
		t.Errorf("expected timeline 'regular-timeline', got '%s'", metadata.Timeline)
	}

	if len(metadata.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(metadata.Children))
	}
}

func TestMetadataListAllButterflies(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	bf1 := &Butterfly{
		Name:           "butterfly1",
		ParentName:     "main",
		DivergenceHash: divergenceHash,
		CreatedAt:      time.Now(),
		IsOrphaned:     false,
		OriginalParent: "",
	}

	bf2 := &Butterfly{
		Name:           "butterfly2",
		ParentName:     "main",
		DivergenceHash: divergenceHash,
		CreatedAt:      time.Now(),
		IsOrphaned:     false,
		OriginalParent: "",
	}

	err := store.StoreButterfly(bf1)
	if err != nil {
		t.Fatalf("failed to store butterfly1: %v", err)
	}

	err = store.StoreButterfly(bf2)
	if err != nil {
		t.Fatalf("failed to store butterfly2: %v", err)
	}

	butterflies, err := store.ListAllButterflies()
	if err != nil {
		t.Fatalf("failed to list butterflies: %v", err)
	}

	if len(butterflies) != 2 {
		t.Errorf("expected 2 butterflies, got %d", len(butterflies))
	}
}

func TestListAllButterfliesEmpty(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	butterflies, err := store.ListAllButterflies()
	if err != nil {
		t.Fatalf("failed to list butterflies: %v", err)
	}

	if len(butterflies) != 0 {
		t.Errorf("expected 0 butterflies, got %d", len(butterflies))
	}
}

func TestMarkOrphaned(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	bf := &Butterfly{
		Name:           "test-butterfly",
		ParentName:     "parent",
		DivergenceHash: divergenceHash,
		CreatedAt:      time.Now(),
		IsOrphaned:     false,
		OriginalParent: "",
	}

	err := store.StoreButterfly(bf)
	if err != nil {
		t.Fatalf("failed to store butterfly: %v", err)
	}

	err = store.MarkOrphaned("test-butterfly", "original-parent")
	if err != nil {
		t.Fatalf("failed to mark orphaned: %v", err)
	}

	retrieved, err := store.GetButterfly("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get butterfly: %v", err)
	}

	if !retrieved.IsOrphaned {
		t.Error("butterfly should be marked as orphaned")
	}

	if retrieved.OriginalParent != "original-parent" {
		t.Errorf("expected original parent 'original-parent', got '%s'", retrieved.OriginalParent)
	}
}

func TestUpdateButterfly(t *testing.T) {
	store, _, cleanup := setupMetadataStore(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	bf := &Butterfly{
		Name:           "test-butterfly",
		ParentName:     "main",
		DivergenceHash: divergenceHash,
		CreatedAt:      time.Now(),
		IsOrphaned:     false,
		OriginalParent: "",
	}

	err := store.StoreButterfly(bf)
	if err != nil {
		t.Fatalf("failed to store butterfly: %v", err)
	}

	var newDivergenceHash cas.Hash
	copy(newDivergenceHash[:], []byte("new-divergence-hash"))
	bf.DivergenceHash = newDivergenceHash

	err = store.StoreButterfly(bf)
	if err != nil {
		t.Fatalf("failed to update butterfly: %v", err)
	}

	retrieved, err := store.GetButterfly("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get butterfly: %v", err)
	}

	if retrieved.DivergenceHash != newDivergenceHash {
		t.Error("butterfly divergence hash should be updated")
	}
}
