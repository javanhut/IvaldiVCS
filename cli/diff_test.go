package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

func TestDiffWorkingOrStaged_NoContentChanges(t *testing.T) {
	workDir := t.TempDir()
	ivaldiDir := filepath.Join(workDir, ".ivaldi")

	if err := os.MkdirAll(ivaldiDir, 0o755); err != nil {
		t.Fatalf("failed to create .ivaldi directory: %v", err)
	}

	mainFile := filepath.Join(workDir, "main.go")
	content := []byte("package main\n\nfunc main() {}\n")
	if err := os.WriteFile(mainFile, content, 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("failed to create refs manager: %v", err)
	}
	defer refsManager.Close()

	var zeroHash [32]byte
	if err := refsManager.CreateTimeline("main", refs.LocalTimeline, zeroHash, zeroHash, "", "test timeline"); err != nil {
		t.Fatalf("failed to create timeline: %v", err)
	}
	if err := refsManager.SetCurrentTimeline("main"); err != nil {
		t.Fatalf("failed to set current timeline: %v", err)
	}

	commitHash, err := createInitialCommit(ivaldiDir, workDir)
	if err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}
	if commitHash == nil {
		t.Fatal("expected non-nil commit hash from initial commit")
	}

	if err := refsManager.UpdateTimeline("main", refs.LocalTimeline, *commitHash, [32]byte{}, ""); err != nil {
		t.Fatalf("failed to update timeline: %v", err)
	}

	// Ensure mtime differs from commit metadata baseline while content remains identical.
	now := time.Now().Add(2 * time.Minute)
	if err := os.Chtimes(mainFile, now, now); err != nil {
		t.Fatalf("failed to update mtime: %v", err)
	}

	casStore, err := cas.NewFileCAS(filepath.Join(ivaldiDir, "objects"))
	if err != nil {
		t.Fatalf("failed to create cas store: %v", err)
	}

	oldDiffStaged := diffStaged
	oldDiffStat := diffStat
	diffStaged = false
	diffStat = false
	defer func() {
		diffStaged = oldDiffStaged
		diffStat = oldDiffStat
	}()

	output := captureStdout(t, func() {
		if err := diffWorkingOrStaged(casStore, ivaldiDir, workDir); err != nil {
			t.Fatalf("diffWorkingOrStaged failed: %v", err)
		}
	})

	if !strings.Contains(output, "No differences.") {
		t.Fatalf("expected 'No differences.' in output, got: %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	return <-done
}
