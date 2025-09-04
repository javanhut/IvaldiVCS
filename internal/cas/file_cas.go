// Package cas provides file-based content-addressable storage.
package cas

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileCAS implements CAS using file system storage.
type FileCAS struct {
	root string
}

// NewFileCAS creates a new file-based CAS in the given directory.
func NewFileCAS(root string) (*FileCAS, error) {
	// Create root directory if it doesn't exist
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("failed to create CAS directory: %w", err)
	}
	
	return &FileCAS{root: root}, nil
}

// getPath returns the file path for a given hash.
// Uses a two-level directory structure to avoid too many files in one directory.
func (f *FileCAS) getPath(hash Hash) string {
	hexStr := hex.EncodeToString(hash[:])
	// Use first 2 chars as directory, rest as filename
	// e.g., ab/cdef1234...
	dir := hexStr[:2]
	file := hexStr[2:]
	return filepath.Join(f.root, dir, file)
}

// Put implements CAS.Put.
func (f *FileCAS) Put(hash Hash, data []byte) error {
	// Verify the hash matches the data
	computed := SumB3(data)
	if computed != hash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", hash.String(), computed.String())
	}
	
	path := f.getPath(hash)
	
	// Create parent directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Check if file already exists (content-addressed, so no need to rewrite)
	if _, err := os.Stat(path); err == nil {
		return nil // Already exists, nothing to do
	}
	
	// Write to temporary file first, then rename (atomic operation)
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	
	_, err = file.Write(data)
	closeErr := file.Close()
	
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write data: %w", err)
	}
	
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close file: %w", closeErr)
	}
	
	// Rename temp file to final name
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename file: %w", err)
	}
	
	return nil
}

// Get implements CAS.Get.
func (f *FileCAS) Get(hash Hash) ([]byte, error) {
	path := f.getPath(hash)
	
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("hash not found: %s", hash.String())
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Verify the hash matches
	computed := SumB3(data)
	if computed != hash {
		return nil, fmt.Errorf("corrupted data: hash mismatch for %s", hash.String())
	}
	
	return data, nil
}

// Has implements CAS.Has.
func (f *FileCAS) Has(hash Hash) (bool, error) {
	path := f.getPath(hash)
	
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}
	
	return true, nil
}