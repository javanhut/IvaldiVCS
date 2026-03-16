package refs

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// setupTestRefsDir creates a temporary directory structure for testing
func setupTestRefsDir(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "ivaldi-refs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create .ivaldi structure
	ivaldiDir := filepath.Join(tmpDir, ".ivaldi")
	refsDir := filepath.Join(ivaldiDir, "refs")
	sealsDir := filepath.Join(refsDir, "seals")

	for _, dir := range []string{
		filepath.Join(refsDir, "heads"),
		filepath.Join(refsDir, "remotes"),
		filepath.Join(refsDir, "tags"),
		sealsDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			os.RemoveAll(tmpDir)
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create shared.db file (empty BoltDB will be created by NewRefsManager)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return ivaldiDir, cleanup
}

// createTestSeal creates a seal file for testing
func createTestSeal(t *testing.T, ivaldiDir, sealName, hashHex, message string) {
	t.Helper()

	sealsDir := filepath.Join(ivaldiDir, "refs", "seals")
	sealPath := filepath.Join(sealsDir, sealName)

	// Format: hash_hex timestamp message
	content := hashHex + " 1640995200 " + message + "\n"

	if err := os.WriteFile(sealPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test seal %s: %v", sealName, err)
	}
}

func TestResolveHashPrefix_UniqueMatch(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Create test seal with known hash (64 hex chars = 32 bytes)
	testHash := "abc123def456789012345678901234567890123456789012345678901234abcd"
	createTestSeal(t, ivaldiDir, "test-seal-1", testHash, "Test commit 1")

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Test with unique prefix
	hash, err := rm.ResolveHashPrefix("abc123")
	if err != nil {
		t.Fatalf("ResolveHashPrefix failed: %v", err)
	}

	// Verify the hash matches
	resultHex := hex.EncodeToString(hash[:])
	if resultHex != testHash {
		t.Errorf("Hash mismatch: expected %s, got %s", testHash, resultHex)
	}
}

func TestResolveHashPrefix_FullHash(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Create test seal with known hash (64 hex chars = 32 bytes)
	testHash := "abc123def456789012345678901234567890123456789012345678901234abcd"
	createTestSeal(t, ivaldiDir, "test-seal-full", testHash, "Test commit full")

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Test with full 64-character hash
	hash, err := rm.ResolveHashPrefix(testHash)
	if err != nil {
		t.Fatalf("ResolveHashPrefix with full hash failed: %v", err)
	}

	resultHex := hex.EncodeToString(hash[:])
	if resultHex != testHash {
		t.Errorf("Hash mismatch: expected %s, got %s", testHash, resultHex)
	}
}

func TestResolveHashPrefix_NotFound(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Create test seal with known hash (64 hex chars = 32 bytes)
	testHash := "abc123def456789012345678901234567890123456789012345678901234abcd"
	createTestSeal(t, ivaldiDir, "test-seal-1", testHash, "Test commit 1")

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Test with non-existent prefix
	_, err = rm.ResolveHashPrefix("xyz999")
	if err == nil {
		t.Fatal("Expected error for non-existent prefix, got nil")
	}

	if !strings.Contains(err.Error(), "no commits found") {
		t.Errorf("Expected 'no commits found' error, got: %v", err)
	}
}

func TestResolveHashPrefix_Ambiguous(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Create multiple seals with similar prefixes (64 hex chars = 32 bytes)
	hash1 := "abc123def456789012345678901234567890123456789012345678901234abcd"
	hash2 := "abc123fff456789012345678901234567890123456789012345678901234abcd"
	hash3 := "abc123999456789012345678901234567890123456789012345678901234abcd"

	createTestSeal(t, ivaldiDir, "test-seal-1", hash1, "Test commit 1")
	createTestSeal(t, ivaldiDir, "test-seal-2", hash2, "Test commit 2")
	createTestSeal(t, ivaldiDir, "test-seal-3", hash3, "Test commit 3")

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Test with ambiguous prefix (matches all 3)
	_, err = rm.ResolveHashPrefix("abc123")
	if err == nil {
		t.Fatal("Expected error for ambiguous prefix, got nil")
	}

	if !strings.Contains(err.Error(), "ambiguous prefix") {
		t.Errorf("Expected 'ambiguous prefix' error, got: %v", err)
	}

	// Should contain suggestions
	if !strings.Contains(err.Error(), "could be:") {
		t.Errorf("Expected error to contain suggestions, got: %v", err)
	}
}

func TestResolveHashPrefix_TooShort(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Test with prefix too short (< 4 chars)
	shortPrefixes := []string{"a", "ab", "abc"}
	for _, prefix := range shortPrefixes {
		_, err = rm.ResolveHashPrefix(prefix)
		if err == nil {
			t.Errorf("Expected error for short prefix %q, got nil", prefix)
			continue
		}

		if !strings.Contains(err.Error(), "too short") {
			t.Errorf("Expected 'too short' error for prefix %q, got: %v", prefix, err)
		}
	}
}

func TestResolveHashPrefix_CaseInsensitive(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Create test seal with lowercase hash (64 hex chars = 32 bytes)
	testHash := "abc123def456789012345678901234567890123456789012345678901234abcd"
	createTestSeal(t, ivaldiDir, "test-seal-case", testHash, "Test commit case")

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Test with uppercase prefix
	hash, err := rm.ResolveHashPrefix("ABC123")
	if err != nil {
		t.Fatalf("ResolveHashPrefix with uppercase prefix failed: %v", err)
	}

	resultHex := hex.EncodeToString(hash[:])
	if resultHex != testHash {
		t.Errorf("Hash mismatch: expected %s, got %s", testHash, resultHex)
	}

	// Test with mixed case prefix
	hash, err = rm.ResolveHashPrefix("AbC123")
	if err != nil {
		t.Fatalf("ResolveHashPrefix with mixed case prefix failed: %v", err)
	}

	resultHex = hex.EncodeToString(hash[:])
	if resultHex != testHash {
		t.Errorf("Hash mismatch: expected %s, got %s", testHash, resultHex)
	}
}

// Helper to create deterministic test hashes
func makeTestHash(seed byte) [32]byte {
	var h [32]byte
	for i := range h {
		h[i] = seed + byte(i)
	}
	return h
}

func TestCreateTimeline_And_GetTimeline(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	blake3Hash := makeTestHash(0x10)
	sha256Hash := makeTestHash(0x20)
	gitSHA1 := "abcdef1234567890abcdef1234567890abcdef12"
	description := "initial commit"

	err = rm.CreateTimeline("main", LocalTimeline, blake3Hash, sha256Hash, gitSHA1, description)
	if err != nil {
		t.Fatalf("CreateTimeline failed: %v", err)
	}

	tl, err := rm.GetTimeline("main", LocalTimeline)
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}

	if tl.Name != "main" {
		t.Errorf("Expected name 'main', got %q", tl.Name)
	}
	if tl.Type != LocalTimeline {
		t.Errorf("Expected type LocalTimeline, got %q", tl.Type)
	}
	if tl.Blake3Hash != blake3Hash {
		t.Errorf("Blake3Hash mismatch")
	}
	if tl.SHA256Hash != sha256Hash {
		t.Errorf("SHA256Hash mismatch")
	}
	if tl.GitSHA1Hash != gitSHA1 {
		t.Errorf("GitSHA1Hash mismatch: expected %s, got %s", gitSHA1, tl.GitSHA1Hash)
	}
	if tl.Description != description {
		t.Errorf("Description mismatch: expected %q, got %q", description, tl.Description)
	}
}

func TestListTimelines_Empty(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	timelines, err := rm.ListTimelines(LocalTimeline)
	if err != nil {
		t.Fatalf("ListTimelines failed: %v", err)
	}
	if len(timelines) != 0 {
		t.Errorf("Expected 0 timelines, got %d", len(timelines))
	}
}

func TestListTimelines_Multiple(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	names := []string{"main", "develop", "feature-x"}
	for i, name := range names {
		err = rm.CreateTimeline(name, LocalTimeline, makeTestHash(byte(i)), makeTestHash(byte(i+100)), "abc123", name+" branch")
		if err != nil {
			t.Fatalf("CreateTimeline(%s) failed: %v", name, err)
		}
	}

	timelines, err := rm.ListTimelines(LocalTimeline)
	if err != nil {
		t.Fatalf("ListTimelines failed: %v", err)
	}
	if len(timelines) != 3 {
		t.Fatalf("Expected 3 timelines, got %d", len(timelines))
	}

	// Collect names and sort for deterministic comparison
	got := make([]string, len(timelines))
	for i, tl := range timelines {
		got[i] = tl.Name
	}
	sort.Strings(got)
	sort.Strings(names)
	for i := range names {
		if got[i] != names[i] {
			t.Errorf("Timeline name mismatch at %d: expected %q, got %q", i, names[i], got[i])
		}
	}
}

func TestGetCurrentTimeline(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Write a HEAD file
	headPath := filepath.Join(ivaldiDir, "HEAD")
	err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write HEAD: %v", err)
	}

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	name, err := rm.GetCurrentTimeline()
	if err != nil {
		t.Fatalf("GetCurrentTimeline failed: %v", err)
	}
	if name != "main" {
		t.Errorf("Expected 'main', got %q", name)
	}
}

func TestSetCurrentTimeline(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	err = rm.SetCurrentTimeline("develop")
	if err != nil {
		t.Fatalf("SetCurrentTimeline failed: %v", err)
	}

	name, err := rm.GetCurrentTimeline()
	if err != nil {
		t.Fatalf("GetCurrentTimeline failed: %v", err)
	}
	if name != "develop" {
		t.Errorf("Expected 'develop', got %q", name)
	}

	// Verify file content directly
	headPath := filepath.Join(ivaldiDir, "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("Failed to read HEAD file: %v", err)
	}
	expected := "ref: refs/heads/develop\n"
	if string(data) != expected {
		t.Errorf("HEAD content mismatch: expected %q, got %q", expected, string(data))
	}
}

func TestTimelineExists(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	if rm.TimelineExists("main", LocalTimeline) {
		t.Error("Expected timeline 'main' to not exist")
	}

	err = rm.CreateTimeline("main", LocalTimeline, makeTestHash(1), makeTestHash(2), "abc", "test")
	if err != nil {
		t.Fatalf("CreateTimeline failed: %v", err)
	}

	if !rm.TimelineExists("main", LocalTimeline) {
		t.Error("Expected timeline 'main' to exist")
	}

	// Different type should not exist
	if rm.TimelineExists("main", RemoteTimeline) {
		t.Error("Expected timeline 'main' to not exist as remote")
	}
}

func TestRenameTimeline(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	blake3 := makeTestHash(0x30)
	sha256 := makeTestHash(0x40)
	err = rm.CreateTimeline("old-branch", LocalTimeline, blake3, sha256, "aaa", "branch desc")
	if err != nil {
		t.Fatalf("CreateTimeline failed: %v", err)
	}

	// Set HEAD so rename updates it
	err = rm.SetCurrentTimeline("old-branch")
	if err != nil {
		t.Fatalf("SetCurrentTimeline failed: %v", err)
	}

	err = rm.RenameTimeline("old-branch", "new-branch", LocalTimeline, false)
	if err != nil {
		t.Fatalf("RenameTimeline failed: %v", err)
	}

	if rm.TimelineExists("old-branch", LocalTimeline) {
		t.Error("Old timeline should not exist after rename")
	}
	if !rm.TimelineExists("new-branch", LocalTimeline) {
		t.Error("New timeline should exist after rename")
	}

	// HEAD should be updated
	current, err := rm.GetCurrentTimeline()
	if err != nil {
		t.Fatalf("GetCurrentTimeline failed: %v", err)
	}
	if current != "new-branch" {
		t.Errorf("Expected HEAD to be 'new-branch', got %q", current)
	}

	// Verify content is preserved
	tl, err := rm.GetTimeline("new-branch", LocalTimeline)
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}
	if tl.Blake3Hash != blake3 {
		t.Error("Blake3Hash mismatch after rename")
	}
}

func TestRenameTimeline_Force(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Set HEAD to something else so force overwrite is allowed
	err = rm.SetCurrentTimeline("other")
	if err != nil {
		t.Fatalf("SetCurrentTimeline failed: %v", err)
	}

	err = rm.CreateTimeline("src", LocalTimeline, makeTestHash(1), makeTestHash(2), "aaa", "source")
	if err != nil {
		t.Fatalf("CreateTimeline(src) failed: %v", err)
	}
	err = rm.CreateTimeline("dst", LocalTimeline, makeTestHash(3), makeTestHash(4), "bbb", "destination")
	if err != nil {
		t.Fatalf("CreateTimeline(dst) failed: %v", err)
	}

	// Without force, should fail
	err = rm.RenameTimeline("src", "dst", LocalTimeline, false)
	if err == nil {
		t.Fatal("Expected error when renaming to existing timeline without force")
	}

	// With force, should succeed
	err = rm.RenameTimeline("src", "dst", LocalTimeline, true)
	if err != nil {
		t.Fatalf("RenameTimeline with force failed: %v", err)
	}

	if rm.TimelineExists("src", LocalTimeline) {
		t.Error("Source timeline should not exist after force rename")
	}
	if !rm.TimelineExists("dst", LocalTimeline) {
		t.Error("Destination timeline should exist after force rename")
	}
}

func TestRenameTimeline_NotExists(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	err = rm.RenameTimeline("nonexistent", "newname", LocalTimeline, false)
	if err == nil {
		t.Fatal("Expected error when renaming nonexistent timeline")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'does not exist' error, got: %v", err)
	}
}

func TestStoreSealName_And_GetSealByName(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	hash := makeTestHash(0xAA)
	message := "feat: add new feature"

	beforeStore := time.Now().Unix()
	err = rm.StoreSealName("v1.0.0", hash, message)
	if err != nil {
		t.Fatalf("StoreSealName failed: %v", err)
	}
	afterStore := time.Now().Unix()

	gotHash, gotTimestamp, gotMessage, err := rm.GetSealByName("v1.0.0")
	if err != nil {
		t.Fatalf("GetSealByName failed: %v", err)
	}

	if gotHash != hash {
		t.Errorf("Hash mismatch")
	}
	if gotMessage != message {
		t.Errorf("Message mismatch: expected %q, got %q", message, gotMessage)
	}

	ts := gotTimestamp.Unix()
	if ts < beforeStore || ts > afterStore {
		t.Errorf("Timestamp %d not in expected range [%d, %d]", ts, beforeStore, afterStore)
	}
}

func TestGetSealNameByHash(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	hash := makeTestHash(0xBB)
	err = rm.StoreSealName("my-seal", hash, "test message")
	if err != nil {
		t.Fatalf("StoreSealName failed: %v", err)
	}

	name, err := rm.GetSealNameByHash(hash)
	if err != nil {
		t.Fatalf("GetSealNameByHash failed: %v", err)
	}
	if name != "my-seal" {
		t.Errorf("Expected 'my-seal', got %q", name)
	}

	// Non-existent hash
	_, err = rm.GetSealNameByHash(makeTestHash(0xFF))
	if err == nil {
		t.Error("Expected error for non-existent hash")
	}
}

func TestListSealNames(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Empty initially
	names, err := rm.ListSealNames()
	if err != nil {
		t.Fatalf("ListSealNames failed: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("Expected 0 seal names, got %d", len(names))
	}

	// Add some seals
	for i, sealName := range []string{"seal-a", "seal-b", "seal-c"} {
		err = rm.StoreSealName(sealName, makeTestHash(byte(i+50)), "msg "+sealName)
		if err != nil {
			t.Fatalf("StoreSealName(%s) failed: %v", sealName, err)
		}
	}

	names, err = rm.ListSealNames()
	if err != nil {
		t.Fatalf("ListSealNames failed: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("Expected 3 seal names, got %d", len(names))
	}

	sort.Strings(names)
	expected := []string{"seal-a", "seal-b", "seal-c"}
	for i := range expected {
		if names[i] != expected[i] {
			t.Errorf("Seal name mismatch at %d: expected %q, got %q", i, expected[i], names[i])
		}
	}
}

func TestSealExists(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	if rm.SealExists("nonexistent") {
		t.Error("Expected seal 'nonexistent' to not exist")
	}

	err = rm.StoreSealName("exists", makeTestHash(0xCC), "message")
	if err != nil {
		t.Fatalf("StoreSealName failed: %v", err)
	}

	if !rm.SealExists("exists") {
		t.Error("Expected seal 'exists' to exist")
	}
}

func TestSetGitHubRepository_And_GetGitHubRepository(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	err = rm.SetGitHubRepository("javanhut", "Ivaldi-vcs")
	if err != nil {
		t.Fatalf("SetGitHubRepository failed: %v", err)
	}

	owner, repo, err := rm.GetGitHubRepository()
	if err != nil {
		t.Fatalf("GetGitHubRepository failed: %v", err)
	}
	if owner != "javanhut" {
		t.Errorf("Expected owner 'javanhut', got %q", owner)
	}
	if repo != "Ivaldi-vcs" {
		t.Errorf("Expected repo 'Ivaldi-vcs', got %q", repo)
	}
}

func TestRemoveGitHubRepository(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	err = rm.SetGitHubRepository("owner", "repo")
	if err != nil {
		t.Fatalf("SetGitHubRepository failed: %v", err)
	}

	err = rm.RemoveGitHubRepository()
	if err != nil {
		t.Fatalf("RemoveGitHubRepository failed: %v", err)
	}

	_, _, err = rm.GetGitHubRepository()
	if err == nil {
		t.Error("Expected error after removing GitHub repository config")
	}
}

func TestMapGitHashToBlake3_And_LookupByGitHash(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	blake3Hash := makeTestHash(0x50)
	sha256Hash := makeTestHash(0x60)
	gitSHA1 := "deadbeef12345678901234567890123456789012"

	err = rm.MapGitHashToBlake3(gitSHA1, blake3Hash, sha256Hash)
	if err != nil {
		t.Fatalf("MapGitHashToBlake3 failed: %v", err)
	}

	gotBlake3, gotSHA256, err := rm.LookupByGitHash(gitSHA1)
	if err != nil {
		t.Fatalf("LookupByGitHash failed: %v", err)
	}

	if gotBlake3 != blake3Hash {
		t.Errorf("Blake3Hash mismatch: expected %x, got %x", blake3Hash, gotBlake3)
	}
	if gotSHA256 != sha256Hash {
		t.Errorf("SHA256Hash mismatch: expected %x, got %x", sha256Hash, gotSHA256)
	}

	// Non-existent git hash
	_, _, err = rm.LookupByGitHash("0000000000000000000000000000000000000000")
	if err == nil {
		t.Error("Expected error for non-existent git hash")
	}
}

func TestResolveHashPrefix_NoSealsDir(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Remove the seals directory to simulate a fresh repo
	sealsDir := filepath.Join(ivaldiDir, "refs", "seals")
	os.RemoveAll(sealsDir)

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Test with any prefix
	_, err = rm.ResolveHashPrefix("abc123")
	if err == nil {
		t.Fatal("Expected error for missing seals dir, got nil")
	}

	if !strings.Contains(err.Error(), "no commits found") {
		t.Errorf("Expected 'no commits found' error, got: %v", err)
	}
}

func TestResolveHashPrefix_DistinguishablePrefix(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Create multiple seals with different prefixes (64 hex chars = 32 bytes)
	hash1 := "abc123def456789012345678901234567890123456789012345678901234abcd"
	hash2 := "def456789012345678901234567890123456789012345678901234567890abcd"

	createTestSeal(t, ivaldiDir, "test-seal-1", hash1, "Test commit 1")
	createTestSeal(t, ivaldiDir, "test-seal-2", hash2, "Test commit 2")

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Test that different prefixes resolve correctly
	hash, err := rm.ResolveHashPrefix("abc1")
	if err != nil {
		t.Fatalf("ResolveHashPrefix(abc1) failed: %v", err)
	}
	if hex.EncodeToString(hash[:]) != hash1 {
		t.Errorf("Expected hash1, got different hash")
	}

	hash, err = rm.ResolveHashPrefix("def4")
	if err != nil {
		t.Fatalf("ResolveHashPrefix(def4) failed: %v", err)
	}
	if hex.EncodeToString(hash[:]) != hash2 {
		t.Errorf("Expected hash2, got different hash")
	}
}

func TestGetTimeline_NonExistent(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	_, err = rm.GetTimeline("nonexistent", LocalTimeline)
	if err == nil {
		t.Fatal("Expected error for non-existent timeline, got nil")
	}
}

func TestGetCurrentTimeline_NoHEAD(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Don't create HEAD file
	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	_, err = rm.GetCurrentTimeline()
	if err == nil {
		t.Fatal("Expected error when HEAD file is missing, got nil")
	}
}

func TestGetCurrentTimeline_MalformedHEAD(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	// Write garbage to HEAD
	headPath := filepath.Join(ivaldiDir, "HEAD")
	os.WriteFile(headPath, []byte("garbage content\n"), 0644)

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	_, err = rm.GetCurrentTimeline()
	if err == nil {
		t.Fatal("Expected error for malformed HEAD, got nil")
	}
}

func TestGetSealByName_NonExistent(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	_, _, _, err = rm.GetSealByName("nonexistent-seal")
	if err == nil {
		t.Fatal("Expected error for non-existent seal, got nil")
	}
}

func TestRenameTimeline_ForceOverwriteCurrentHEAD(t *testing.T) {
	ivaldiDir, cleanup := setupTestRefsDir(t)
	defer cleanup()

	rm, err := NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("Failed to create refs manager: %v", err)
	}
	defer rm.Close()

	// Create two timelines
	rm.CreateTimeline("src", LocalTimeline, makeTestHash(1), makeTestHash(2), "", "source")
	rm.CreateTimeline("dst", LocalTimeline, makeTestHash(3), makeTestHash(4), "", "destination")

	// Set HEAD to dst (the destination)
	rm.SetCurrentTimeline("dst")

	// Force rename src -> dst should fail because dst is current HEAD
	err = rm.RenameTimeline("src", "dst", LocalTimeline, true)
	if err == nil {
		t.Fatal("Expected error when force renaming to current HEAD timeline, got nil")
	}
	if !strings.Contains(err.Error(), "current HEAD") {
		t.Errorf("Expected 'current HEAD' in error message, got: %v", err)
	}

	// Verify src still exists (rename was rejected)
	if !rm.TimelineExists("src", LocalTimeline) {
		t.Error("Source timeline should still exist after rejected rename")
	}

	// Verify dst still has its original data
	tl, err := rm.GetTimeline("dst", LocalTimeline)
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}
	if tl.Blake3Hash != makeTestHash(3) {
		t.Error("Expected dst to retain its original blake3 hash")
	}
}
