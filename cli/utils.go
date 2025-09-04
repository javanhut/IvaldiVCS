package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/objects"
)

// updateLastSnapshot updates the snapshot file with current file hashes for status tracking
func updateLastSnapshot(workDir, ivaldiDir string) error {
	snapshotFile := filepath.Join(ivaldiDir, "last_snapshot")
	f, err := os.Create(snapshotFile)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}
	defer f.Close()

	// Walk the working directory and record all files
	err = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}

		// Skip .ivaldi directory
		if strings.HasPrefix(relPath, ".ivaldi") {
			return nil
		}

		// Compute file hash
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		hash := objects.HashBlobBLAKE3(content)
		hashHex := fmt.Sprintf("%x", hash[:])

		// Write to snapshot file in format: "path:hash\n"
		if _, err := f.WriteString(fmt.Sprintf("%s:%s\n", relPath, hashHex)); err != nil {
			return err
		}

		return nil
	})

	return err
}