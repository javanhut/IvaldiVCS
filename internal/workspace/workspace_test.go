package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

func setupTestWorkspace(t *testing.T) (string, string, *Materializer, func()) {
	// Create temporary directories
	tempDir := t.TempDir()
	ivaldiDir := filepath.Join(tempDir, ".ivaldi")
	workDir := tempDir

	// Initialize Ivaldi directory structure
	err := os.MkdirAll(filepath.Join(ivaldiDir, "refs", "heads"), 0755)
	if err != nil {
		t.Fatalf("Failed to create ivaldi structure: %v", err)
	}

	err = os.MkdirAll(filepath.Join(ivaldiDir, "refs", "tags"), 0755)
	if err != nil {
		t.Fatalf("Failed to create tags directory: %v", err)
	}

	// Create CAS and materializer
	casStore := cas.NewMemoryCAS()
	materializer := NewMaterializer(casStore, ivaldiDir, workDir)

	// Initialize refs manager and create a default timeline
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}

	err = refsManager.CreateTimeline(
		"main",
		refs.LocalTimeline,
		[32]byte{}, // Empty blake3 hash
		[32]byte{}, // Empty sha256 hash
		"",         // No git SHA1
		"Initial timeline",
	)
	if err != nil {
		refsManager.Close()
		t.Fatalf("Failed to create main timeline: %v", err)
	}

	err = refsManager.SetCurrentTimeline("main")
	if err != nil {
		refsManager.Close()
		t.Fatalf("Failed to set current timeline: %v", err)
	}

	refsManager.Close()

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return ivaldiDir, workDir, materializer, cleanup
}

func TestGetCurrentState(t *testing.T) {
	ivaldiDir, workDir, materializer, cleanup := setupTestWorkspace(t)
	defer cleanup()

	state, err := materializer.GetCurrentState()
	if err != nil {
		t.Fatalf("GetCurrentState failed: %v", err)
	}

	if state.TimelineName != "main" {
		t.Errorf("Expected timeline 'main', got %s", state.TimelineName)
	}
	if state.IvaldiDir != ivaldiDir {
		t.Errorf("Expected ivaldi dir %s, got %s", ivaldiDir, state.IvaldiDir)
	}
	if state.WorkDir != workDir {
		t.Errorf("Expected work dir %s, got %s", workDir, state.WorkDir)
	}
}

func TestScanWorkspace(t *testing.T) {
	_, workDir, materializer, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create some test files
	err := os.WriteFile(filepath.Join(workDir, "test1.txt"), []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.MkdirAll(filepath.Join(workDir, "subdir"), 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	err = os.WriteFile(filepath.Join(workDir, "subdir", "test2.txt"), []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file in subdir: %v", err)
	}

	// Scan workspace
	index, err := materializer.ScanWorkspace()
	if err != nil {
		t.Fatalf("ScanWorkspace failed: %v", err)
	}

	if index.Count != 2 {
		t.Errorf("Expected 2 files in index, got %d", index.Count)
	}

	// Verify files are in index
	loader := materializer.CAS.(*cas.MemoryCAS)
	if loader.Len() < 2 { // Should have at least the file chunks
		t.Error("Expected file chunks to be stored in CAS")
	}
}

func TestMaterializeTimeline(t *testing.T) {
	ivaldiDir, workDir, materializer, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create initial files
	err := os.WriteFile(filepath.Join(workDir, "initial.txt"), []byte("initial content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Create a second timeline
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}

	err = refsManager.CreateTimeline(
		"feature",
		refs.LocalTimeline,
		[32]byte{1, 2, 3}, // Different hash to simulate different content
		[32]byte{},        // Empty sha256 hash
		"",                // No git SHA1
		"Feature timeline",
	)
	if err != nil {
		refsManager.Close()
		t.Fatalf("Failed to create feature timeline: %v", err)
	}
	refsManager.Close()

	// Materialize the feature timeline
	err = materializer.MaterializeTimeline("feature")
	if err != nil {
		t.Fatalf("MaterializeTimeline failed: %v", err)
	}

	// Verify current timeline was updated
	state, err := materializer.GetCurrentState()
	if err != nil {
		t.Fatalf("GetCurrentState failed: %v", err)
	}

	if state.TimelineName != "feature" {
		t.Errorf("Expected current timeline 'feature', got %s", state.TimelineName)
	}

	// Check that timeline-info.txt was created (from createTargetIndex)
	infoPath := filepath.Join(workDir, "timeline-info.txt")
	if _, err := os.Stat(infoPath); os.IsNotExist(err) {
		t.Error("Expected timeline-info.txt to be created")
	}
}

func TestGetWorkspaceStatus(t *testing.T) {
	_, workDir, materializer, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Initially, workspace should be clean
	status, err := materializer.GetWorkspaceStatus()
	if err != nil {
		t.Fatalf("GetWorkspaceStatus failed: %v", err)
	}

	if !status.Clean {
		t.Error("Expected clean workspace initially")
	}
	if status.TimelineName != "main" {
		t.Errorf("Expected timeline 'main', got %s", status.TimelineName)
	}

	// Create a file to make workspace dirty
	err = os.WriteFile(filepath.Join(workDir, "new_file.txt"), []byte("new content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Now workspace should be dirty
	status, err = materializer.GetWorkspaceStatus()
	if err != nil {
		t.Fatalf("GetWorkspaceStatus failed: %v", err)
	}

	if status.Clean {
		t.Error("Expected dirty workspace after adding file")
	}
	if len(status.Changes) == 0 {
		t.Error("Expected changes to be detected")
	}

	// Check summary
	summary := status.Summary()
	if summary == "" {
		t.Error("Expected non-empty summary")
	}
	t.Logf("Workspace status summary: %s", summary)

	// Check change list
	changes := status.ListChanges()
	if len(changes) == 0 {
		t.Error("Expected non-empty change list")
	}
	t.Logf("Changes: %v", changes)
}

func TestBackupAndRestore(t *testing.T) {
	_, workDir, materializer, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create some files for backup
	err := os.WriteFile(filepath.Join(workDir, "backup_test.txt"), []byte("backup content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create backup test file: %v", err)
	}

	// Create backup
	err = materializer.BackupWorkspace("test-backup")
	if err != nil {
		t.Fatalf("BackupWorkspace failed: %v", err)
	}

	// Modify workspace
	err = os.WriteFile(filepath.Join(workDir, "backup_test.txt"), []byte("modified content"), 0644)
	if err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Restore from backup
	err = materializer.RestoreWorkspace("test-backup")
	if err != nil {
		t.Fatalf("RestoreWorkspace failed: %v", err)
	}

	// Verify content was restored
	content, err := os.ReadFile(filepath.Join(workDir, "backup_test.txt"))
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}

	if string(content) != "backup content" {
		t.Errorf("Expected 'backup content', got %s", string(content))
	}
}

func TestCleanWorkspace(t *testing.T) {
	_, workDir, materializer, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create some files
	err := os.WriteFile(filepath.Join(workDir, "file1.txt"), []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	err = os.MkdirAll(filepath.Join(workDir, "subdir"), 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	err = os.WriteFile(filepath.Join(workDir, "subdir", "file2.txt"), []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Clean workspace
	err = materializer.CleanWorkspace()
	if err != nil {
		t.Fatalf("CleanWorkspace failed: %v", err)
	}

	// Verify files are removed
	_, err = os.Stat(filepath.Join(workDir, "file1.txt"))
	if !os.IsNotExist(err) {
		t.Error("Expected file1.txt to be removed")
	}

	_, err = os.Stat(filepath.Join(workDir, "subdir", "file2.txt"))
	if !os.IsNotExist(err) {
		t.Error("Expected file2.txt to be removed")
	}

	// Subdirectory should also be removed (empty)
	_, err = os.Stat(filepath.Join(workDir, "subdir"))
	if !os.IsNotExist(err) {
		t.Error("Expected empty subdirectory to be removed")
	}
}

func TestStashManager(t *testing.T) {
	_, workDir, materializer, cleanup := setupTestWorkspace(t)
	defer cleanup()

	stashManager := NewStashManager(materializer)

	// Create some files to stash
	err := os.WriteFile(filepath.Join(workDir, "stash_test.txt"), []byte("stash content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create stash test file: %v", err)
	}

	// Create stash
	err = stashManager.CreateStash("test-stash", "Test stash description")
	if err != nil {
		t.Fatalf("CreateStash failed: %v", err)
	}

	// List stashes
	stashes, err := stashManager.ListStashes()
	if err != nil {
		t.Fatalf("ListStashes failed: %v", err)
	}

	if len(stashes) != 1 {
		t.Fatalf("Expected 1 stash, got %d", len(stashes))
	}
	if stashes[0] != "test-stash" {
		t.Errorf("Expected stash name 'test-stash', got %s", stashes[0])
	}

	// Modify workspace
	err = os.WriteFile(filepath.Join(workDir, "stash_test.txt"), []byte("modified content"), 0644)
	if err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Apply stash
	err = stashManager.ApplyStash("test-stash")
	if err != nil {
		t.Fatalf("ApplyStash failed: %v", err)
	}

	// Verify content was restored
	content, err := os.ReadFile(filepath.Join(workDir, "stash_test.txt"))
	if err != nil {
		t.Fatalf("Failed to read stashed file: %v", err)
	}

	if string(content) != "stash content" {
		t.Errorf("Expected 'stash content', got %s", string(content))
	}

	// Drop stash
	err = stashManager.DropStash("test-stash")
	if err != nil {
		t.Fatalf("DropStash failed: %v", err)
	}

	// Verify stash is gone
	stashes, err = stashManager.ListStashes()
	if err != nil {
		t.Fatalf("ListStashes failed: %v", err)
	}

	if len(stashes) != 0 {
		t.Errorf("Expected 0 stashes after drop, got %d", len(stashes))
	}
}

func TestRemoveEmptyDirectories(t *testing.T) {
	_, workDir, materializer, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create nested directories
	deepPath := filepath.Join(workDir, "level1", "level2", "level3")
	err := os.MkdirAll(deepPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	// Create a file in the deepest directory
	filePath := filepath.Join(deepPath, "test.txt")
	err = os.WriteFile(filePath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Remove the file
	err = os.Remove(filePath)
	if err != nil {
		t.Fatalf("Failed to remove test file: %v", err)
	}

	// Call removeEmptyDirectories
	materializer.removeEmptyDirectories(deepPath)

	// Check that empty directories were removed
	_, err = os.Stat(deepPath)
	if !os.IsNotExist(err) {
		t.Error("Expected empty level3 directory to be removed")
	}

	_, err = os.Stat(filepath.Join(workDir, "level1", "level2"))
	if !os.IsNotExist(err) {
		t.Error("Expected empty level2 directory to be removed")
	}

	_, err = os.Stat(filepath.Join(workDir, "level1"))
	if !os.IsNotExist(err) {
		t.Error("Expected empty level1 directory to be removed")
	}

	// Working directory should still exist
	_, err = os.Stat(workDir)
	if err != nil {
		t.Error("Working directory should not be removed")
	}
}

func BenchmarkScanWorkspace(b *testing.B) {
	tempDir := b.TempDir()
	ivaldiDir := filepath.Join(tempDir, ".ivaldi")
	workDir := tempDir

	// Setup
	err := os.MkdirAll(filepath.Join(ivaldiDir, "refs", "heads"), 0755)
	if err != nil {
		b.Fatalf("Setup failed: %v", err)
	}

	casStore := cas.NewMemoryCAS()
	materializer := NewMaterializer(casStore, ivaldiDir, workDir)

	// Create test files
	for i := 0; i < 100; i++ {
		filename := filepath.Join(workDir, "file"+string(rune('0'+i%10))+".txt")
		content := "test content " + string(rune('0'+i%10))
		err := os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			b.Fatalf("Failed to create test file: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := materializer.ScanWorkspace()
		if err != nil {
			b.Fatalf("ScanWorkspace failed: %v", err)
		}
	}
}

func BenchmarkGetWorkspaceStatus(b *testing.B) {
	tempDir := b.TempDir()
	ivaldiDir := filepath.Join(tempDir, ".ivaldi")
	workDir := tempDir

	// Setup
	err := os.MkdirAll(filepath.Join(ivaldiDir, "refs", "heads"), 0755)
	if err != nil {
		b.Fatalf("Setup failed: %v", err)
	}

	casStore := cas.NewMemoryCAS()
	materializer := NewMaterializer(casStore, ivaldiDir, workDir)

	// Setup refs
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		b.Fatalf("Setup failed: %v", err)
	}

	err = refsManager.CreateTimeline("main", refs.LocalTimeline, [32]byte{}, [32]byte{}, "", "Main")
	if err != nil {
		refsManager.Close()
		b.Fatalf("Setup failed: %v", err)
	}

	err = refsManager.SetCurrentTimeline("main")
	if err != nil {
		refsManager.Close()
		b.Fatalf("Setup failed: %v", err)
	}
	refsManager.Close()

	// Create test files
	for i := 0; i < 50; i++ {
		filename := filepath.Join(workDir, "file"+string(rune('0'+i%10))+".txt")
		content := "test content " + string(rune('0'+i%10))
		err := os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			b.Fatalf("Failed to create test file: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := materializer.GetWorkspaceStatus()
		if err != nil {
			b.Fatalf("GetWorkspaceStatus failed: %v", err)
		}
	}
}