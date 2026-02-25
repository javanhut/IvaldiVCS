package engine

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/ignore"
	"github.com/javanhut/Ivaldi-vcs/internal/objects"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
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

// StatusResult holds the complete status information for the repository
type StatusResult struct {
	Timeline   string
	SealName   string
	FileCount  int // files tracked in last seal
	Files      []FileStatusInfo
	Staged     []FileStatusInfo
	Modified   []FileStatusInfo
	Deleted    []FileStatusInfo
	Untracked  []FileStatusInfo
	Ignored    []FileStatusInfo
}

// GetFileStatuses computes file statuses for the working directory
func GetFileStatuses(workDir, ivaldiDir string) (*StatusResult, error) {
	// Initialize refs manager
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, err
	}
	defer refsManager.Close()

	// Get current timeline
	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return nil, err
	}

	// Load ignore patterns
	patternCache, err := ignore.LoadPatternCache(workDir)
	if err != nil {
		log.Printf("Warning: Failed to load ignore patterns: %v", err)
	}

	// Get known files
	knownFiles, err := GetKnownFiles(ivaldiDir, refsManager)
	if err != nil {
		log.Printf("Warning: Failed to get known files: %v", err)
		knownFiles = make(map[string][32]byte)
	}

	// Get file statuses
	fileStatuses, err := computeFileStatuses(workDir, ivaldiDir, patternCache, knownFiles)
	if err != nil {
		return nil, err
	}

	// Get seal name
	sealName := getSealName(refsManager, currentTimeline)

	result := &StatusResult{
		Timeline:  currentTimeline,
		SealName:  sealName,
		FileCount: len(knownFiles),
		Files:     fileStatuses,
	}

	// Group files by status
	for _, f := range fileStatuses {
		switch f.Status {
		case StatusStaged, StatusAdded:
			result.Staged = append(result.Staged, f)
		case StatusModified:
			result.Modified = append(result.Modified, f)
		case StatusDeleted:
			result.Deleted = append(result.Deleted, f)
		case StatusUntracked:
			result.Untracked = append(result.Untracked, f)
		case StatusIgnored:
			result.Ignored = append(result.Ignored, f)
		}
	}

	return result, nil
}

// getSealName returns the seal name for the current timeline head
func getSealName(refsManager *refs.RefsManager, currentTimeline string) string {
	timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return ""
	}
	if timeline.Blake3Hash == [32]byte{} {
		return ""
	}
	sealName, err := refsManager.GetSealNameByHash(timeline.Blake3Hash)
	if err != nil || sealName == "" {
		return ""
	}
	return sealName
}

// computeFileStatuses analyzes the working directory and returns file status information
func computeFileStatuses(workDir, ivaldiDir string, patternCache *ignore.PatternCache, knownFiles map[string][32]byte) ([]FileStatusInfo, error) {
	var fileStatuses []FileStatusInfo

	// Get staged files
	stagedFiles, err := GetStagedFiles(ivaldiDir)
	if err != nil {
		log.Printf("Warning: Failed to get staged files: %v", err)
	}

	// Build staged map for O(1) lookups
	stagedMap := make(map[string]bool, len(stagedFiles))
	for _, f := range stagedFiles {
		stagedMap[f] = true
	}

	// Walk the working directory
	err = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		relPath, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			if relPath == ".ivaldi" || strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) {
				return filepath.SkipDir
			}
			if patternCache != nil && patternCache.IsDirIgnored(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(relPath, ".ivaldi") {
			return nil
		}

		if patternCache != nil && patternCache.IsIgnored(relPath) {
			fileStatuses = append(fileStatuses, FileStatusInfo{
				Path:   relPath,
				Status: StatusIgnored,
			})
			return nil
		}

		isStaged := stagedMap[relPath]
		knownHash, wasKnown := knownFiles[relPath]

		if isStaged {
			if wasKnown {
				fileStatuses = append(fileStatuses, FileStatusInfo{
					Path:   relPath,
					Status: StatusStaged,
				})
			} else {
				fileStatuses = append(fileStatuses, FileStatusInfo{
					Path:   relPath,
					Status: StatusAdded,
				})
			}
		} else {
			if wasKnown {
				currentHash, err := computeFileHash(path)
				if err != nil {
					log.Printf("Warning: Failed to compute hash for %s: %v", relPath, err)
					return nil
				}
				if currentHash != knownHash {
					fileStatuses = append(fileStatuses, FileStatusInfo{
						Path:   relPath,
						Status: StatusModified,
					})
				}
			} else {
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

	// Check for deleted files
	for filePath := range knownFiles {
		fullPath := filepath.Join(workDir, filePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			isStaged := stagedMap[filePath]
			if isStaged {
				fileStatuses = append(fileStatuses, FileStatusInfo{
					Path:   filePath,
					Status: StatusStaged,
				})
			} else {
				fileStatuses = append(fileStatuses, FileStatusInfo{
					Path:   filePath,
					Status: StatusDeleted,
				})
			}
		}
	}

	return fileStatuses, nil
}

// GetStagedFiles returns a list of files that are currently staged
func GetStagedFiles(ivaldiDir string) ([]string, error) {
	stageFile := filepath.Join(ivaldiDir, "stage", "files")
	if _, err := os.Stat(stageFile); os.IsNotExist(err) {
		return []string{}, nil
	}

	data, err := os.ReadFile(stageFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// GetKnownFiles reads files from the last commit/seal for proper status tracking.
func GetKnownFiles(ivaldiDir string, refsManager *refs.RefsManager) (map[string][32]byte, error) {
	knownFiles := make(map[string][32]byte)

	rm := refsManager
	if rm == nil {
		var err error
		rm, err = refs.NewRefsManager(ivaldiDir)
		if err != nil {
			return knownFiles, nil
		}
		defer rm.Close()
	}

	currentTimeline, err := rm.GetCurrentTimeline()
	if err != nil {
		return knownFiles, nil
	}

	timeline, err := rm.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return knownFiles, nil
	}

	if timeline.Blake3Hash == [32]byte{} {
		return knownFiles, nil
	}

	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return knownFiles, nil
	}

	var commitHash cas.Hash
	copy(commitHash[:], timeline.Blake3Hash[:])

	commitReader := commit.NewCommitReader(casStore)
	commitObj, err := commitReader.ReadCommit(commitHash)
	if err != nil {
		return knownFiles, nil
	}

	tree, err := commitReader.ReadTree(commitObj)
	if err != nil {
		return knownFiles, nil
	}

	filePaths, err := commitReader.ListFiles(tree)
	if err != nil {
		return knownFiles, nil
	}

	for _, filePath := range filePaths {
		content, err := commitReader.GetFileContent(tree, filePath)
		if err != nil {
			continue
		}
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
