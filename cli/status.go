package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/objects"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

// FileStatus represents the status of a file
type FileStatus int

const (
	StatusUnknown   FileStatus = iota
	StatusUntracked            // File exists but not in any previous commit
	StatusAdded                // File is staged for commit (new file)
	StatusModified             // File is modified from last commit
	StatusDeleted              // File was deleted from working directory
	StatusStaged               // File is staged for commit (modified)
	StatusIgnored              // File is ignored by .ivaldiignore
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
		fmt.Printf("On timeline %s\n", colors.Bold(currentTimeline))

		// Show information about the last seal if available
		err = displayLastSealInfo(refsManager, currentTimeline, ivaldiDir)
		if err != nil {
			// Don't fail if we can't get seal info
		}

		if len(fileStatuses) == 0 {
			fmt.Println(colors.SuccessText("Working directory clean"))
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
			fmt.Printf("\n%s\n", colors.SectionHeader("Files staged for seal:"))
			for _, file := range staged {
				if file.Status == StatusAdded {
					fmt.Printf("  %s   %s\n", colors.Added("new file:"), colors.Green(file.Path))
				} else {
					fmt.Printf("  %s   %s\n", colors.Staged("modified:"), colors.Blue(file.Path))
				}
			}
		}

		// Display modified files
		if len(modified) > 0 {
			fmt.Printf("\n%s\n", colors.SectionHeader("Files not staged for seal:"))
			fmt.Printf("  %s\n", colors.Dim("(use \"ivaldi gather <file>...\" to stage for seal)"))
			for _, file := range modified {
				fmt.Printf("  %s   %s\n", colors.Modified("modified:"), colors.Blue(file.Path))
			}
		}

		// Display deleted files
		if len(deleted) > 0 {
			fmt.Printf("\n%s\n", colors.SectionHeader("Deleted files:"))
			fmt.Printf("  %s\n", colors.Dim("(use \"ivaldi gather <file>...\" to stage deletion)"))
			for _, file := range deleted {
				fmt.Printf("  %s    %s\n", colors.Deleted("deleted:"), colors.Red(file.Path))
			}
		}

		// Display untracked files
		if len(untracked) > 0 {
			fmt.Printf("\n%s\n", colors.SectionHeader("Untracked files:"))
			fmt.Printf("  %s\n", colors.Dim("(use \"ivaldi gather <file>...\" to include in what will be sealed)"))
			for _, file := range untracked {
				fmt.Printf("  %s\n", colors.Yellow(file.Path))
			}
		}

		// Display a summary
		fmt.Printf("\n%s ", colors.SectionHeader("Status summary:"))
		var parts []string
		if len(staged) > 0 {
			parts = append(parts, colors.Green(fmt.Sprintf("%d staged", len(staged))))
		}
		if len(modified) > 0 {
			parts = append(parts, colors.Blue(fmt.Sprintf("%d modified", len(modified))))
		}
		if len(untracked) > 0 {
			parts = append(parts, colors.Yellow(fmt.Sprintf("%d untracked", len(untracked))))
		}
		if len(deleted) > 0 {
			parts = append(parts, colors.Red(fmt.Sprintf("%d deleted", len(deleted))))
		}

		if len(parts) > 0 {
			fmt.Printf("%s\n", strings.Join(parts, ", "))
		} else {
			fmt.Printf("%s\n", colors.SuccessText("clean"))
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

// getKnownFiles reads files from the last commit/seal for proper status tracking
func getKnownFiles(ivaldiDir string) (map[string][32]byte, error) {
	knownFiles := make(map[string][32]byte)

	// Get the current timeline and its last commit
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return knownFiles, nil // No refs system, treat as empty
	}
	defer refsManager.Close()

	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return knownFiles, nil // No current timeline
	}

	timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return knownFiles, nil // Timeline doesn't exist
	}

	// If timeline has no commits (empty hash), return empty
	if timeline.Blake3Hash == [32]byte{} {
		return knownFiles, nil
	}

	// Initialize CAS to read commit
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return knownFiles, nil // Can't initialize CAS
	}

	// Read the commit object
	var commitHash cas.Hash
	copy(commitHash[:], timeline.Blake3Hash[:])

	commitReader := commit.NewCommitReader(casStore)
	commitObj, err := commitReader.ReadCommit(commitHash)
	if err != nil {
		return knownFiles, nil // Can't read commit
	}

	// Read the tree structure
	tree, err := commitReader.ReadTree(commitObj)
	if err != nil {
		return knownFiles, nil // Can't read tree
	}

	// List all files in the commit
	filePaths, err := commitReader.ListFiles(tree)
	if err != nil {
		return knownFiles, nil // Can't list files
	}

	// For each file, get its content and compute hash
	for _, filePath := range filePaths {
		content, err := commitReader.GetFileContent(tree, filePath)
		if err != nil {
			continue // Skip files we can't read
		}

		// Compute BLAKE3 hash of content
		hash := objects.HashBlobBLAKE3(content)
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

// displayLastSealInfo shows information about the last seal and its contents
func displayLastSealInfo(refsManager *refs.RefsManager, currentTimeline, ivaldiDir string) error {
	timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return err
	}

	// If timeline has no commits, don't show anything
	if timeline.Blake3Hash == [32]byte{} {
		return nil
	}

	// Try to get seal name
	sealName, err := refsManager.GetSealNameByHash(timeline.Blake3Hash)
	if err == nil && sealName != "" {
		fmt.Printf("Last seal: %s\n", colors.Cyan(sealName))
	} else {
		// Fallback to short hash
		shortHash := hex.EncodeToString(timeline.Blake3Hash[:])[:8]
		fmt.Printf("Last seal: %s\n", colors.Cyan(shortHash))
	}

	// Get files from the last commit
	knownFiles, err := getKnownFiles(ivaldiDir)
	if err != nil {
		return err
	}

	if len(knownFiles) > 0 {
		fmt.Printf("Files tracked in last seal: %s\n", colors.InfoText(fmt.Sprintf("%d", len(knownFiles))))
	}

	return nil
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
