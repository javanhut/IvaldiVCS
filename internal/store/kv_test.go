package store

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// helper: open a temporary DB and return it along with a cleanup function.
func openTestDB(t *testing.T) (*DB, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "kvtest-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	dbPath := filepath.Join(dir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("Open: %v", err)
	}
	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}
	return db, cleanup
}

// helper: build deterministic 32-byte arrays from a single seed byte.
func makeHash(seed byte) [32]byte {
	var h [32]byte
	for i := range h {
		h[i] = seed + byte(i)
	}
	return h
}

// ---------- Open / Close ----------

func TestOpen_CreatesDB(t *testing.T) {
	dir, err := os.MkdirTemp("", "kvtest-open-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "new.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	// The file should exist on disk.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created on disk")
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	// A path inside a non-existent directory should fail.
	_, err := Open("/nonexistent/dir/should/not/exist/test.db")
	if err == nil {
		t.Fatal("expected error when opening DB at an invalid path, got nil")
	}
}

// ---------- PutMapping / LookupByKey ----------

func TestPutMapping_And_LookupByKey(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	blake3 := makeHash(0x10)
	sha256 := makeHash(0x20)

	if err := db.PutMapping("file.txt", blake3, sha256); err != nil {
		t.Fatalf("PutMapping: %v", err)
	}

	gotB3, gotS2, err := db.LookupByKey("file.txt")
	if err != nil {
		t.Fatalf("LookupByKey: %v", err)
	}

	wantB3 := hex.EncodeToString(blake3[:])
	wantS2 := hex.EncodeToString(sha256[:])

	if gotB3 != wantB3 {
		t.Errorf("blake3 mismatch: got %s, want %s", gotB3, wantB3)
	}
	if gotS2 != wantS2 {
		t.Errorf("sha256 mismatch: got %s, want %s", gotS2, wantS2)
	}
}

func TestLookupByKey_NotFound(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	_, _, err := db.LookupByKey("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}

func TestPutMapping_MultipleMappings(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	keys := []string{"a.txt", "b.txt", "c.txt"}
	type entry struct{ b3, s2 [32]byte }
	entries := make(map[string]entry)

	for i, k := range keys {
		b3 := makeHash(byte(i * 10))
		s2 := makeHash(byte(i*10 + 1))
		entries[k] = entry{b3, s2}
		if err := db.PutMapping(k, b3, s2); err != nil {
			t.Fatalf("PutMapping(%s): %v", k, err)
		}
	}

	for _, k := range keys {
		gotB3, gotS2, err := db.LookupByKey(k)
		if err != nil {
			t.Fatalf("LookupByKey(%s): %v", k, err)
		}
		e := entries[k]
		if gotB3 != hex.EncodeToString(e.b3[:]) {
			t.Errorf("%s blake3 mismatch", k)
		}
		if gotS2 != hex.EncodeToString(e.s2[:]) {
			t.Errorf("%s sha256 mismatch", k)
		}
	}
}

func TestPutMapping_Overwrite(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	blake3A := makeHash(0xAA)
	sha256A := makeHash(0xBB)
	blake3B := makeHash(0xCC)
	sha256B := makeHash(0xDD)

	if err := db.PutMapping("overwrite.txt", blake3A, sha256A); err != nil {
		t.Fatalf("PutMapping (first): %v", err)
	}
	if err := db.PutMapping("overwrite.txt", blake3B, sha256B); err != nil {
		t.Fatalf("PutMapping (second): %v", err)
	}

	gotB3, gotS2, err := db.LookupByKey("overwrite.txt")
	if err != nil {
		t.Fatalf("LookupByKey: %v", err)
	}

	wantB3 := hex.EncodeToString(blake3B[:])
	wantS2 := hex.EncodeToString(sha256B[:])

	if gotB3 != wantB3 {
		t.Errorf("blake3 not overwritten: got %s, want %s", gotB3, wantB3)
	}
	if gotS2 != wantS2 {
		t.Errorf("sha256 not overwritten: got %s, want %s", gotS2, wantS2)
	}
}

// ---------- PutGitMapping / LookupByGitHash ----------

func TestPutGitMapping_And_LookupByGitHash(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	gitSHA := "abc123def456789012345678901234567890abcd"
	blake3 := makeHash(0x30)
	sha256 := makeHash(0x40)

	if err := db.PutGitMapping(gitSHA, blake3, sha256); err != nil {
		t.Fatalf("PutGitMapping: %v", err)
	}

	gotB3, gotS2, err := db.LookupByGitHash(gitSHA)
	if err != nil {
		t.Fatalf("LookupByGitHash: %v", err)
	}

	wantB3 := hex.EncodeToString(blake3[:])
	wantS2 := hex.EncodeToString(sha256[:])

	if gotB3 != wantB3 {
		t.Errorf("blake3 mismatch: got %s, want %s", gotB3, wantB3)
	}
	if gotS2 != wantS2 {
		t.Errorf("sha256 mismatch: got %s, want %s", gotS2, wantS2)
	}
}

func TestLookupByGitHash_NotFound(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	_, _, err := db.LookupByGitHash("0000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error for missing git hash, got nil")
	}
}

// ---------- GetAllGitHashes ----------

func TestGetAllGitHashes(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	gitHashes := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccc",
	}

	for i, g := range gitHashes {
		b3 := makeHash(byte(i))
		s2 := makeHash(byte(i + 100))
		if err := db.PutGitMapping(g, b3, s2); err != nil {
			t.Fatalf("PutGitMapping(%s): %v", g, err)
		}
	}

	got, err := db.GetAllGitHashes()
	if err != nil {
		t.Fatalf("GetAllGitHashes: %v", err)
	}

	sort.Strings(got)
	sort.Strings(gitHashes)

	if len(got) != len(gitHashes) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(gitHashes))
	}
	for i := range got {
		if got[i] != gitHashes[i] {
			t.Errorf("hash[%d] mismatch: got %s, want %s", i, got[i], gitHashes[i])
		}
	}
}

func TestGetAllGitHashes_Empty(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	got, err := db.GetAllGitHashes()
	if err != nil {
		t.Fatalf("GetAllGitHashes: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(got))
	}
}

// ---------- Config ----------

func TestPutConfig_And_GetConfig(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	if err := db.PutConfig("remote.url", "https://example.com/repo.git"); err != nil {
		t.Fatalf("PutConfig: %v", err)
	}

	val, err := db.GetConfig("remote.url")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if val != "https://example.com/repo.git" {
		t.Errorf("config value mismatch: got %q, want %q", val, "https://example.com/repo.git")
	}
}

func TestGetConfig_NotFound(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	_, err := db.GetConfig("nonexistent.key")
	if err == nil {
		t.Fatal("expected error for missing config key, got nil")
	}
}

func TestRemoveConfig(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	if err := db.PutConfig("to.remove", "value"); err != nil {
		t.Fatalf("PutConfig: %v", err)
	}

	// Verify it exists first.
	if _, err := db.GetConfig("to.remove"); err != nil {
		t.Fatalf("GetConfig before remove: %v", err)
	}

	if err := db.RemoveConfig("to.remove"); err != nil {
		t.Fatalf("RemoveConfig: %v", err)
	}

	// After removal it should not be found.
	_, err := db.GetConfig("to.remove")
	if err == nil {
		t.Fatal("expected error after removal, got nil")
	}
}

func TestRemoveConfig_NonExistent(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	// bbolt Delete on a non-existent key returns nil, so this should not error.
	if err := db.RemoveConfig("never.existed"); err != nil {
		t.Fatalf("RemoveConfig for non-existent key: %v", err)
	}
}

// ---------- Close and Reopen ----------

func TestDBClose_And_Reopen(t *testing.T) {
	dir, err := os.MkdirTemp("", "kvtest-reopen-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "reopen.db")

	// Open, write data, close.
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open (first): %v", err)
	}

	blake3 := makeHash(0x50)
	sha256 := makeHash(0x60)
	if err := db.PutMapping("persist.txt", blake3, sha256); err != nil {
		t.Fatalf("PutMapping: %v", err)
	}
	if err := db.PutConfig("persist.key", "persist.value"); err != nil {
		t.Fatalf("PutConfig: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and verify data survived.
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open (second): %v", err)
	}
	defer db2.Close()

	gotB3, gotS2, err := db2.LookupByKey("persist.txt")
	if err != nil {
		t.Fatalf("LookupByKey after reopen: %v", err)
	}
	if gotB3 != hex.EncodeToString(blake3[:]) {
		t.Errorf("blake3 mismatch after reopen")
	}
	if gotS2 != hex.EncodeToString(sha256[:]) {
		t.Errorf("sha256 mismatch after reopen")
	}

	val, err := db2.GetConfig("persist.key")
	if err != nil {
		t.Fatalf("GetConfig after reopen: %v", err)
	}
	if val != "persist.value" {
		t.Errorf("config value mismatch after reopen: got %q", val)
	}
}
