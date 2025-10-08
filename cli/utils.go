package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/config"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/objects"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/seals"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
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

// createInitialCommit creates an initial commit from the current workspace state
// and returns the commit hash. This is used during forge to capture initial files.
func createInitialCommit(ivaldiDir, workDir string) (*[32]byte, error) {
	// Initialize storage system
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Create materializer to scan workspace
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)

	// Scan the current workspace
	wsIndex, err := materializer.ScanWorkspace()
	if err != nil {
		return nil, fmt.Errorf("failed to scan workspace: %w", err)
	}

	// Get workspace files
	wsLoader := wsindex.NewLoader(casStore)
	workspaceFiles, err := wsLoader.ListAll(wsIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace files: %w", err)
	}

	// If no files, return nil (no commit needed)
	if len(workspaceFiles) == 0 {
		return nil, nil
	}

	// Initialize MMR for commit tracking
	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		// Fall back to in-memory MMR if persistent fails
		mmr = &history.PersistentMMR{MMR: history.NewMMR()}
	}
	defer mmr.Close()

	// Create commit builder
	commitBuilder := commit.NewCommitBuilder(casStore, mmr.MMR)

	// Create initial commit with no parents
	commitObj, err := commitBuilder.CreateCommit(
		workspaceFiles,
		nil, // No parent commits for initial commit
		"ivaldi-system",
		"ivaldi-system",
		"Initial commit",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial commit: %w", err)
	}

	// Get commit hash
	commitHash := commitBuilder.GetCommitHash(commitObj)

	// Convert to array
	var hashArray [32]byte
	copy(hashArray[:], commitHash[:])

	// Generate and store seal name for the initial commit
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err == nil {
		defer refsManager.Close()
		sealName := seals.GenerateSealName(hashArray)
		err = refsManager.StoreSealName(sealName, hashArray, "Initial commit")
		if err != nil {
			// Log but don't fail - seal name is nice to have but not critical
			fmt.Printf("Warning: Failed to store seal name for initial commit: %v\n", err)
		}
	}

	return &hashArray, nil
}

// getAuthorFromConfig retrieves the author string from configuration
// Returns "Name <email>" format or error if not configured
func getAuthorFromConfig() (string, error) {
	return config.GetAuthor()
}
