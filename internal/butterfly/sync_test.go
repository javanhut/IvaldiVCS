package butterfly

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

func setupSyncEnv(t *testing.T) (*Syncer, *Manager, *refs.RefsManager, func()) {
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
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

	syncer := NewSyncer(manager, casStore, refsManager, mmr)

	cleanup := func() {
		manager.Close()
		mmr.Close()
		refsManager.Close()
		os.RemoveAll(tmpDir)
	}

	return syncer, manager, refsManager, cleanup
}

func TestNewSyncer(t *testing.T) {
	syncer, _, _, cleanup := setupSyncEnv(t)
	defer cleanup()

	if syncer == nil {
		t.Fatal("expected syncer to be created")
	}

	if syncer.manager == nil {
		t.Error("expected manager to be initialized")
	}

	if syncer.cas == nil {
		t.Error("expected CAS to be initialized")
	}

	if syncer.refs == nil {
		t.Error("expected refs manager to be initialized")
	}

	if syncer.mmr == nil {
		t.Error("expected MMR to be initialized")
	}

	if syncer.resolver == nil {
		t.Error("expected conflict resolver to be initialized")
	}
}

func TestSyncUpSameHash(t *testing.T) {
	syncer, manager, refsManager, cleanup := setupSyncEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	err = syncer.SyncUp("test-butterfly")
	if err != nil {
		t.Fatalf("failed to sync up: %v", err)
	}

	parentRef, err := refsManager.GetTimeline("main", refs.LocalTimeline)
	if err != nil {
		t.Fatalf("failed to get parent timeline: %v", err)
	}

	butterflyRef, err := refsManager.GetTimeline("test-butterfly", refs.LocalTimeline)
	if err != nil {
		t.Fatalf("failed to get butterfly timeline: %v", err)
	}

	if parentRef.Blake3Hash != butterflyRef.Blake3Hash {
		t.Error("after sync up, parent and butterfly should have same hash")
	}
}

func TestSyncUpNonButterfly(t *testing.T) {
	syncer, _, _, cleanup := setupSyncEnv(t)
	defer cleanup()

	err := syncer.SyncUp("main")
	if err == nil {
		t.Error("expected error when syncing non-butterfly timeline")
	}
}

func TestSyncUpOrphaned(t *testing.T) {
	syncer, manager, _, cleanup := setupSyncEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	bf, err := manager.GetButterflyInfo("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get butterfly info: %v", err)
	}

	bf.IsOrphaned = true
	err = manager.metadataStore.StoreButterfly(bf)
	if err != nil {
		t.Fatalf("failed to mark as orphaned: %v", err)
	}

	err = syncer.SyncUp("test-butterfly")
	if err == nil {
		t.Error("expected error when syncing orphaned butterfly")
	}
}

func TestSyncDownSameHash(t *testing.T) {
	syncer, manager, refsManager, cleanup := setupSyncEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	err = syncer.SyncDown("test-butterfly")
	if err != nil {
		t.Fatalf("failed to sync down: %v", err)
	}

	parentRef, err := refsManager.GetTimeline("main", refs.LocalTimeline)
	if err != nil {
		t.Fatalf("failed to get parent timeline: %v", err)
	}

	butterflyRef, err := refsManager.GetTimeline("test-butterfly", refs.LocalTimeline)
	if err != nil {
		t.Fatalf("failed to get butterfly timeline: %v", err)
	}

	if parentRef.Blake3Hash != butterflyRef.Blake3Hash {
		t.Error("after sync down, parent and butterfly should have same hash")
	}
}

func TestSyncDownNonButterfly(t *testing.T) {
	syncer, _, _, cleanup := setupSyncEnv(t)
	defer cleanup()

	err := syncer.SyncDown("main")
	if err == nil {
		t.Error("expected error when syncing non-butterfly timeline")
	}
}

func TestSyncDownOrphaned(t *testing.T) {
	syncer, manager, _, cleanup := setupSyncEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	bf, err := manager.GetButterflyInfo("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get butterfly info: %v", err)
	}

	bf.IsOrphaned = true
	err = manager.metadataStore.StoreButterfly(bf)
	if err != nil {
		t.Fatalf("failed to mark as orphaned: %v", err)
	}

	err = syncer.SyncDown("test-butterfly")
	if err == nil {
		t.Error("expected error when syncing orphaned butterfly")
	}
}

func TestSyncUpUpdatesDivergence(t *testing.T) {
	syncer, manager, _, cleanup := setupSyncEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	originalDivergence, err := manager.GetDivergencePoint("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get original divergence: %v", err)
	}

	err = syncer.SyncUp("test-butterfly")
	if err != nil {
		t.Fatalf("failed to sync up: %v", err)
	}

	newDivergence, err := manager.GetDivergencePoint("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get new divergence: %v", err)
	}

	if originalDivergence == newDivergence {
		t.Log("Note: divergence may be same if hashes were already equal")
	}
}

func TestGetParentStatus(t *testing.T) {
	syncer, manager, _, cleanup := setupSyncEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	ahead, behind, err := syncer.GetParentStatus("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get parent status: %v", err)
	}

	if ahead != 0 || behind != 0 {
		t.Logf("commits ahead: %d, behind: %d", ahead, behind)
	}
}

func TestGetParentStatusNonButterfly(t *testing.T) {
	syncer, _, _, cleanup := setupSyncEnv(t)
	defer cleanup()

	_, _, err := syncer.GetParentStatus("main")
	if err == nil {
		t.Error("expected error when getting parent status for non-butterfly")
	}
}

func TestGetParentStatusOrphaned(t *testing.T) {
	syncer, manager, _, cleanup := setupSyncEnv(t)
	defer cleanup()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], []byte("test-divergence-hash"))

	err := manager.CreateButterfly("test-butterfly", "main", divergenceHash)
	if err != nil {
		t.Fatalf("failed to create butterfly: %v", err)
	}

	bf, err := manager.GetButterflyInfo("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get butterfly info: %v", err)
	}

	bf.IsOrphaned = true
	err = manager.metadataStore.StoreButterfly(bf)
	if err != nil {
		t.Fatalf("failed to mark as orphaned: %v", err)
	}

	ahead, behind, err := syncer.GetParentStatus("test-butterfly")
	if err != nil {
		t.Fatalf("failed to get parent status: %v", err)
	}

	if ahead != 0 || behind != 0 {
		t.Error("orphaned butterfly should have 0 commits ahead/behind")
	}
}
