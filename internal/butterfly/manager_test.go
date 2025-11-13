package butterfly

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

func setupTestEnv(t *testing.T) (string, *Manager, func()) {
	tmpDir, err := os.MkdirTemp("", "butterfly-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ivaldiDir := filepath.Join(tmpDir, ".ivaldi")
	if err := os.MkdirAll(ivaldiDir, 0755); err != nil {
		t.Fatalf("failed to create .ivaldi dir: %v", err)
	}

	objectsDir := filepath.Join(ivaldiDir, "objects")
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		t.Fatalf("failed to create objects dir: %v", err)
	}

	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		t.Fatalf("failed to create CAS: %v", err)
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("failed to create refs manager: %v", err)
	}

	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		refsManager.Close()
		t.Fatalf("failed to create MMR: %v", err)
	}

	var testHash [32]byte
	copy(testHash[:], []byte("test-hash-value-for-timeline"))
	var sha256Hash [32]byte
	copy(sha256Hash[:], []byte("sha256-hash-value-timeline"))
	gitHash := "test-git-sha1-hash"

	err = refsManager.CreateTimeline("main", refs.LocalTimeline, testHash, sha256Hash, gitHash, "Initial timeline")
	if err != nil {
		mmr.Close()
		refsManager.Close()
		t.Fatalf("failed to create main timeline: %v", err)
	}

	manager, err := NewManager(ivaldiDir, casStore, refsManager, mmr)
	if err != nil {
		mmr.Close()
		refsManager.Close()
		t.Fatalf("failed to create manager: %v", err)
	}

	cleanup := func() {
		manager.Close()
		mmr.Close()
		refsManager.Close()
		os.RemoveAll(tmpDir)
	}

	return ivaldiDir, manager, cleanup
}

func TestNewManager(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	if manager.metadataStore == nil {
		t.Error("expected metadata store to be initialized")
	}
}

func TestCreateButterfly(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	isButterfly := manager.IsButterfly("test-butterfly")
	if !isButterfly {
		t.Error("expected 'test-butterfly' to be a butterfly")
	}

	bf, err := manager.GetButterflyInfo("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get butterfly info: %v", err)
	}

	if bf.Name != "test-butterfly" {
		t.Errorf("expected name 'test-butterfly', got '%s'", bf.Name)
	}

	if bf.ParentName != "main" {
		t.Errorf("expected parent 'main', got '%s'", bf.ParentName)
	}

	if bf.DivergenceHash != divergenceHash {
		t.Error("divergence hash mismatch")
	}

	if bf.IsOrphaned {
		t.Error("new butterfly should not be orphaned")
	}
}

func TestCreateButterflyDuplicate(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	err = manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err == nil {
		t.Error("expected error when creating duplicate butterfly")
	}
}

func TestCreateButterflyInvalidParent(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "nonexistent", divergenceHash)
	if err == nil {
		t.Error("expected error when creating butterfly with nonexistent parent")
	}
}

func TestGetParent(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	parent, err := manager.GetParent("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get parent: %v", err)
	}

	if parent != "main" {
		t.Errorf("expected parent 'main', got '%s'", parent)
	}
}

func TestGetChildren(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("child1", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create child1: %v", err)
	}

	err = manager.CreateButterfly("child2", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create child2: %v", err)
	}

	children, err := manager.GetChildren("main")
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

func TestDeleteButterfly(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	err = manager.DeleteButterfly("test-butterfly", false)
	if err != nil {
		t.Fatalf("failed to delete butterfly: %v", err)
	}

	isButterfly := manager.IsButterfly("test-butterfly")
	if isButterfly {
		t.Error("butterfly should be deleted")
	}
}

func TestDeleteButterflyWithChildren(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("parent-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create parent butterfly: %v", err)
	}

	err = manager.CreateButterfly("child-butterfly", "parent-butterfly", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create child butterfly: %v", err)
	}

	err = manager.DeleteButterfly("parent-butterfly", false)
	if err != nil {
		t.Fatalf("failed to delete parent butterfly: %v", err)
	}

	bf, err := manager.GetButterflyInfo("child-butterfly")
	if err != nil {
		t.Fatalf("child should still exist: %v", err)
	}

	if !bf.IsOrphaned {
		t.Error("child should be marked as orphaned")
	}
}

func TestDeleteButterflyCascade(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("parent-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create parent butterfly: %v", err)
	}

	err = manager.CreateButterfly("child-butterfly", "parent-butterfly", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create child butterfly: %v", err)
	}

	err = manager.DeleteButterfly("parent-butterfly", true)
	if err != nil {
		t.Fatalf("failed to cascade delete: %v", err)
	}

	if manager.IsButterfly("parent-butterfly") {
		t.Error("parent should be deleted")
	}

	if manager.IsButterfly("child-butterfly") {
		t.Error("child should be deleted in cascade")
	}
}

func TestGetMetadata(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	metadata, err := manager.GetMetadata("test-butterfly")
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
		t.Errorf("expected timeline name 'test-butterfly', got '%s'", metadata.Timeline)
	}
}

func TestGetMetadataNonButterfly(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	metadata, err := manager.GetMetadata("main")
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}

	if metadata.IsButterfly {
		t.Error("expected metadata to indicate non-butterfly timeline")
	}

	if metadata.Butterfly != nil {
		t.Error("expected no butterfly info for regular timeline")
	}
}

func TestListAllButterflies(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("bf1", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create bf1: %v", err)
	}

	err = manager.CreateButterfly("bf2", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create bf2: %v", err)
	}

	butterflies, err := manager.ListAllButterflies()
	if err != nil {
		t.Fatalf("failed to list butterflies: %v", err)
	}

	if len(butterflies) != 2 {
		t.Errorf("expected 2 butterflies, got %d", len(butterflies))
	}
}

func TestGetDivergencePoint(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	hash, err := manager.GetDivergencePoint("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get divergence point: %v", err)
	}

	if hash != divergenceHash {
		t.Error("divergence hash mismatch")
	}
}

func TestUpdateDivergence(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	var newDivergenceHash cas.Hash
	copy(newDivergenceHash[:], []byte("new-divergence-hash"))

	err = manager.UpdateDivergence("test-butterfly", newDivergenceHash)
	if err != nil {
		t.Fatalf("failed to update divergence: %v", err)
	}

	hash, err := manager.GetDivergencePoint("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get divergence point: %v", err)
	}

	if hash != newDivergenceHash {
		t.Error("divergence hash should be updated")
	}
}

func TestButterflyCreatedAt(t *testing.T) {
	_, manager, cleanup := setupTestEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	before := time.Now()
	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}
	after := time.Now()

	bf, err := manager.GetButterflyInfo("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get butterfly info: %v", err)
	}

	if bf.CreatedAt.Before(before) || bf.CreatedAt.After(after) {
		t.Error("butterfly created timestamp should be within expected range")
	}
}
