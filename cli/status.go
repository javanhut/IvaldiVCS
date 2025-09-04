package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/objects"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

// FileStatus represents the status of a file
type FileStatus int

const (
	StatusUnknown FileStatus = iota
	StatusUntracked         // File exists but not in any previous commit
	StatusAdded             // File is staged for commit (new file)
	StatusModified          // File is modified from last commit
	StatusDeleted           // File was deleted from working directory
	StatusStaged            // File is staged for commit (modified)
	StatusIgnored           // File is ignored by .ivaldiignore
)

// FileStatusInfo holds information about a file's status
type FileStatusInfo struct {
	Path         string
	Status       FileStatus
	StagedStatus FileStatus // Status in staging area vs HEAD
	WorkStatus   FileStatus // Status in working directory vs staging area
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the working directory status",
	Long:  `Shows files that are staged, modified, deleted, untracked, or ignored`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we're in an Ivaldi repository
		ivaldiDir := ".ivaldi"
		if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
			return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
		}

		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		// Initialize refs manager
		refsManager, err := refs.NewRefsManager(ivaldiDir)
		if err != nil {
			return fmt.Errorf("failed to initialize refs manager: %w", err)
		}
		defer refsManager.Close()

		// Get current timeline
		currentTimeline, err := refsManager.GetCurrentTimeline()
		if err != nil {
			return fmt.Errorf("failed to get current timeline: %w", err)
		}

		// Load ignore patterns
		ignorePatterns, err := loadIgnorePatterns(workDir)
		if err != nil {
			log.Printf("Warning: Failed to load ignore patterns: %v", err)
		}

		// Get file statuses
		fileStatuses, err := getFileStatuses(workDir, ivaldiDir, ignorePatterns)
		if err != nil {
			return fmt.Errorf("failed to get file statuses: %w", err)
		}

		// Display status
		fmt.Printf("On timeline %s\n", currentTimeline)
		
		if len(fileStatuses) == 0 {
			fmt.Println("Working directory clean")
			return nil
		}

		// Group files by status
		var staged []FileStatusInfo
		var modified []FileStatusInfo
		var deleted []FileStatusInfo
		var untracked []FileStatusInfo
		var ignored []FileStatusInfo

		for _, fileInfo := range fileStatuses {
			switch fileInfo.Status {
			case StatusStaged, StatusAdded:
				staged = append(staged, fileInfo)
			case StatusModified:
				modified = append(modified, fileInfo)
			case StatusDeleted:
				deleted = append(deleted, fileInfo)
			case StatusUntracked:
				untracked = append(untracked, fileInfo)
			case StatusIgnored:
				ignored = append(ignored, fileInfo)
			}
		}

		// Display staged files
		if len(staged) > 0 {
			fmt.Println("\nFiles staged for seal:")
			for _, file := range staged {
				if file.Status == StatusAdded {
					fmt.Printf("  new file:   %s\n", file.Path)
				} else {
					fmt.Printf("  modified:   %s\n", file.Path)
				}
			}
		}

		// Display modified files
		if len(modified) > 0 {
			fmt.Println("\nFiles not staged for seal:")
			fmt.Println("  (use \"ivaldi gather <file>...\" to stage for seal)")
			for _, file := range modified {
				fmt.Printf("  modified:   %s\n", file.Path)
			}
		}

		// Display deleted files
		if len(deleted) > 0 {
			fmt.Println("\nDeleted files:")
			fmt.Println("  (use \"ivaldi gather <file>...\" to stage deletion)")
			for _, file := range deleted {
				fmt.Printf("  deleted:    %s\n", file.Path)
			}
		}

		// Display untracked files
		if len(untracked) > 0 {
			fmt.Println("\nUntracked files:")
			fmt.Println("  (use \"ivaldi gather <file>...\" to include in what will be sealed)")
			for _, file := range untracked {
				fmt.Printf("  %s\n", file.Path)
			}
		}

		// Display ignored files (only if verbose flag is set)
		verbose, _ := cmd.Flags().GetBool("ignored")
		if verbose && len(ignored) > 0 {
			fmt.Println("\nIgnored files:")
			for _, file := range ignored {
				fmt.Printf("  %s\n", file.Path)
			}
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().BoolP("ignored", "i", false, "Show ignored files")
}

// getFileStatuses analyzes the working directory and returns file status information
func getFileStatuses(workDir, ivaldiDir string, ignorePatterns []string) ([]FileStatusInfo, error) {
	var fileStatuses []FileStatusInfo

	// Get staged files
	stagedFiles, err := getStagedFiles(ivaldiDir)
	if err != nil {
		log.Printf("Warning: Failed to get staged files: %v", err)
	}

	// Get known files from last snapshot (if any)
	knownFiles, err := getKnownFiles(ivaldiDir)
	if err != nil {
		log.Printf("Warning: Failed to get known files: %v", err)
	}

	// Walk the working directory
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

		// Check if file is ignored
		if isIgnored(relPath, ignorePatterns) {
			fileStatuses = append(fileStatuses, FileStatusInfo{
				Path:   relPath,
				Status: StatusIgnored,
			})
			return nil
		}

		// Check if file is staged
		isStaged := false
		for _, stagedFile := range stagedFiles {
			if stagedFile == relPath {
				isStaged = true
				break
			}
		}

		// Check if file was known in previous snapshot
		wasKnown := false
		var knownHash [32]byte
		for filePath, hash := range knownFiles {
			if filePath == relPath {
				wasKnown = true
				knownHash = hash
				break
			}
		}

		if isStaged {
			// File is staged - determine if it's new or modified
			if wasKnown {
				fileStatuses = append(fileStatuses, FileStatusInfo{
					Path:   relPath,
					Status: StatusStaged, // Modified and staged
				})
			} else {
				fileStatuses = append(fileStatuses, FileStatusInfo{
					Path:   relPath,
					Status: StatusAdded, // New file staged
				})
			}
		} else {
			// File is not staged
			if wasKnown {
				// Check if file has been modified since last snapshot
				currentHash, err := computeFileHash(path)
				if err != nil {
					log.Printf("Warning: Failed to compute hash for %s: %v", relPath, err)
					return nil
				}

				if currentHash != knownHash {
					fileStatuses = append(fileStatuses, FileStatusInfo{
						Path:   relPath,
						Status: StatusModified, // Modified but not staged
					})
				}
				// If hashes match, file is unchanged (don't add to status)
			} else {
				// File is new and not staged
				fileStatuses = append(fileStatuses, FileStatusInfo{
					Path:   relPath,
					Status: StatusUntracked,
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Check for deleted files (files that were known but no longer exist)
	for filePath := range knownFiles {
		fullPath := filepath.Join(workDir, filePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// File was deleted
			isStaged := false
			for _, stagedFile := range stagedFiles {
				if stagedFile == filePath {
					isStaged = true
					break
				}
			}

			if isStaged {
				// Deletion is staged
				fileStatuses = append(fileStatuses, FileStatusInfo{
					Path:   filePath,
					Status: StatusStaged, // Deletion staged
				})
			} else {
				// Deletion not staged
				fileStatuses = append(fileStatuses, FileStatusInfo{
					Path:   filePath,
					Status: StatusDeleted,
				})
			}
		}
	}

	return fileStatuses, nil
}

// getStagedFiles returns a list of files that are currently staged
func getStagedFiles(ivaldiDir string) ([]string, error) {
	stageFile := filepath.Join(ivaldiDir, "stage", "files")
	if _, err := os.Stat(stageFile); os.IsNotExist(err) {
		return []string{}, nil // No staged files
	}

	data, err := os.ReadFile(stageFile)
	if err != nil {
		return nil, err
	}

	files := strings.Fields(string(data))
	return files, nil
}

// loadIgnorePatterns loads patterns from .ivaldiignore file
func loadIgnorePatterns(workDir string) ([]string, error) {
	ignoreFile := filepath.Join(workDir, ".ivaldiignore")
	if _, err := os.Stat(ignoreFile); os.IsNotExist(err) {
		return []string{}, nil // No ignore file
	}

	file, err := os.Open(ignoreFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns, scanner.Err()
}

// getKnownFiles returns a map of files from the last snapshot with their BLAKE3 hashes
func getKnownFiles(ivaldiDir string) (map[string][32]byte, error) {
	knownFiles := make(map[string][32]byte)
	
	// Create a snapshot file to track known files
	// In a full implementation, this would come from the last commit/tree object
	snapshotFile := filepath.Join(ivaldiDir, "last_snapshot")
	if _, err := os.Stat(snapshotFile); os.IsNotExist(err) {
		return knownFiles, nil // No previous snapshot
	}

	data, err := os.ReadFile(snapshotFile)
	if err != nil {
		return knownFiles, nil // Can't read snapshot, treat as empty
	}

	// Parse snapshot format: "path:hash\n"
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		filePath := parts[0]
		hashHex := parts[1]

		// Decode hex hash
		hashBytes, err := hex.DecodeString(hashHex)
		if err != nil || len(hashBytes) != 32 {
			continue
		}

		var hash [32]byte
		copy(hash[:], hashBytes)
		knownFiles[filePath] = hash
	}

	return knownFiles, nil
}

// computeFileHash computes the BLAKE3 hash of a file
func computeFileHash(filePath string) ([32]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return [32]byte{}, err
	}

	return objects.HashBlobBLAKE3(content), nil
}

// isIgnored checks if a file path matches any ignore patterns
func isIgnored(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// Simple pattern matching - in a full implementation, 
		// this would support full glob patterns
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}