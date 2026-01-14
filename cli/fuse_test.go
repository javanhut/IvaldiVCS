package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
)

func TestWriteMergedFile(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "fuse_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a memory CAS store
	casStore := cas.NewMemoryCAS()

	// Create test chunks
	chunk1 := []byte("Hello, ")
	chunk2 := []byte("World!")

	hash1 := cas.SumB3(chunk1)
	hash2 := cas.SumB3(chunk2)

	// Store chunks in CAS
	if err := casStore.Put(hash1, chunk1); err != nil {
		t.Fatalf("Failed to store chunk1: %v", err)
	}
	if err := casStore.Put(hash2, chunk2); err != nil {
		t.Fatalf("Failed to store chunk2: %v", err)
	}

	// Test writing merged file
	testPath := "subdir/test.txt"
	err = writeMergedFile(casStore, tmpDir, testPath, []cas.Hash{hash1, hash2})
	if err != nil {
		t.Fatalf("writeMergedFile failed: %v", err)
	}

	// Verify the file was created
	fullPath := filepath.Join(tmpDir, testPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	expectedContent := "Hello, World!"
	if string(content) != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, string(content))
	}
}

func TestWriteMergedFileEmptyChunks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fuse_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	casStore := cas.NewMemoryCAS()

	// Test writing with no chunks (should create empty file)
	testPath := "empty.txt"
	err = writeMergedFile(casStore, tmpDir, testPath, []cas.Hash{})
	if err != nil {
		t.Fatalf("writeMergedFile failed: %v", err)
	}

	// Verify empty file was created
	fullPath := filepath.Join(tmpDir, testPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if len(content) != 0 {
		t.Errorf("Expected empty file, got %d bytes", len(content))
	}
}

func TestWriteMergedFileCreatesDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fuse_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	casStore := cas.NewMemoryCAS()

	chunk := []byte("test content")
	hash := cas.SumB3(chunk)
	if err := casStore.Put(hash, chunk); err != nil {
		t.Fatalf("Failed to store chunk: %v", err)
	}

	// Test with deeply nested path
	testPath := "a/b/c/d/file.txt"
	err = writeMergedFile(casStore, tmpDir, testPath, []cas.Hash{hash})
	if err != nil {
		t.Fatalf("writeMergedFile failed: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tmpDir, testPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Errorf("Expected file to be created at %s", fullPath)
	}
}
