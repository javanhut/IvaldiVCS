package github

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

func setupSyncCommitTestRepo(t *testing.T) (*RepoSyncer, string) {
	t.Helper()

	workDir := t.TempDir()
	ivaldiDir := filepath.Join(workDir, ".ivaldi")
	objectsDir := filepath.Join(ivaldiDir, "objects")

	if err := os.MkdirAll(objectsDir, 0o755); err != nil {
		t.Fatalf("failed to create objects dir: %v", err)
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("failed to create refs manager: %v", err)
	}
	defer refsManager.Close()

	if err := refsManager.CreateTimeline("main", refs.LocalTimeline, [32]byte{}, [32]byte{}, "", "main"); err != nil {
		t.Fatalf("failed to create main timeline: %v", err)
	}
	if err := refsManager.SetCurrentTimeline("main"); err != nil {
		t.Fatalf("failed to set HEAD timeline: %v", err)
	}

	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		t.Fatalf("failed to create CAS: %v", err)
	}

	return &RepoSyncer{
		ivaldiDir: ivaldiDir,
		workDir:   workDir,
		casStore:  casStore,
	}, workDir
}

func TestCreateIvaldiCommitStoresGitSHA(t *testing.T) {
	rs, workDir := setupSyncCommitTestRepo(t)

	filePath := filepath.Join(workDir, "notes.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if err := rs.createIvaldiCommit("sync 1", "main", "abc123"); err != nil {
		t.Fatalf("createIvaldiCommit failed: %v", err)
	}

	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err != nil {
		t.Fatalf("failed to reopen refs manager: %v", err)
	}
	defer refsManager.Close()

	timeline, err := refsManager.GetTimeline("main", refs.LocalTimeline)
	if err != nil {
		t.Fatalf("failed to read timeline: %v", err)
	}

	if timeline.GitSHA1Hash != "abc123" {
		t.Fatalf("expected GitSHA1Hash abc123, got %q", timeline.GitSHA1Hash)
	}
	if timeline.Blake3Hash == [32]byte{} {
		t.Fatalf("expected timeline commit hash to be set")
	}
}

func TestCreateIvaldiCommitUsesExistingHeadAsParent(t *testing.T) {
	rs, workDir := setupSyncCommitTestRepo(t)

	filePath := filepath.Join(workDir, "notes.txt")
	if err := os.WriteFile(filePath, []byte("first"), 0o644); err != nil {
		t.Fatalf("failed to write first test file: %v", err)
	}

	if err := rs.createIvaldiCommit("sync 1", "main", "sha1"); err != nil {
		t.Fatalf("first createIvaldiCommit failed: %v", err)
	}

	refsManager, err := refs.NewRefsManager(rs.ivaldiDir)
	if err != nil {
		t.Fatalf("failed to open refs manager: %v", err)
	}
	defer refsManager.Close()

	firstTimeline, err := refsManager.GetTimeline("main", refs.LocalTimeline)
	if err != nil {
		t.Fatalf("failed to read timeline after first commit: %v", err)
	}
	firstHash := firstTimeline.Blake3Hash

	if err := os.WriteFile(filePath, []byte("second"), 0o644); err != nil {
		t.Fatalf("failed to write second test file: %v", err)
	}

	if err := rs.createIvaldiCommit("sync 2", "main", "sha2"); err != nil {
		t.Fatalf("second createIvaldiCommit failed: %v", err)
	}

	secondTimeline, err := refsManager.GetTimeline("main", refs.LocalTimeline)
	if err != nil {
		t.Fatalf("failed to read timeline after second commit: %v", err)
	}

	var secondHash cas.Hash
	copy(secondHash[:], secondTimeline.Blake3Hash[:])

	reader := commit.NewCommitReader(rs.casStore)
	secondCommit, err := reader.ReadCommit(secondHash)
	if err != nil {
		t.Fatalf("failed to read second commit: %v", err)
	}

	if len(secondCommit.Parents) != 1 {
		t.Fatalf("expected 1 parent, got %d", len(secondCommit.Parents))
	}

	var expectedParent cas.Hash
	copy(expectedParent[:], firstHash[:])
	if secondCommit.Parents[0] != expectedParent {
		t.Fatalf("expected parent %x, got %x", expectedParent[:8], secondCommit.Parents[0][:8])
	}
}
