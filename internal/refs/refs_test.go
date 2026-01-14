package refs

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
