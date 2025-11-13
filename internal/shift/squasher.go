package shift

import (
	"fmt"
	"strings"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/hamtdir"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// CommitSquasher handles the logic for squashing multiple commits into one.
type CommitSquasher struct {
	CAS     cas.CAS
	Builder *commit.CommitBuilder
	Reader  *commit.CommitReader
}

// NewCommitSquasher creates a new CommitSquasher.
func NewCommitSquasher(casStore cas.CAS, builder *commit.CommitBuilder) *CommitSquasher {
	return &CommitSquasher{
		CAS:     casStore,
		Builder: builder,
		Reader:  commit.NewCommitReader(casStore),
	}
}

// CommitInfo holds information about a commit in the range.
type CommitInfo struct {
	Hash      cas.Hash
	Message   string
	Author    string
	Timestamp time.Time
	TreeHash  cas.Hash
}

// GetCommitRange retrieves all commits between start (oldest) and end (newest) inclusive.
// The commits are returned in chronological order (oldest first).
func (cs *CommitSquasher) GetCommitRange(start, end cas.Hash) ([]CommitInfo, error) {
	var commits []CommitInfo
	visited := make(map[cas.Hash]bool)

	// Walk backwards from end to start, collecting commits
	currentHash := end
	for {
		if visited[currentHash] {
			return nil, fmt.Errorf("cycle detected in commit history")
		}
		visited[currentHash] = true

		// Read commit
		commitObj, err := cs.Reader.ReadCommit(currentHash)
		if err != nil {
			return nil, fmt.Errorf("failed to read commit %x: %w", currentHash[:4], err)
		}

		// Add to commits list
		commits = append(commits, CommitInfo{
			Hash:      currentHash,
			Message:   commitObj.Message,
			Author:    commitObj.Author,
			Timestamp: commitObj.CommitTime,
			TreeHash:  commitObj.TreeHash,
		})

		// If we've reached the start commit, we're done
		if currentHash == start {
			break
		}

		// Move to parent
		if len(commitObj.Parents) == 0 {
			return nil, fmt.Errorf("reached root commit before finding start commit")
		}

		// Use first parent for linear history
		currentHash = commitObj.Parents[0]
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	return commits, nil
}

// ExtractFinalState extracts the final workspace state from the end commit.
// This is the state we want to preserve in the squashed commit.
func (cs *CommitSquasher) ExtractFinalState(endCommit cas.Hash) ([]wsindex.FileMetadata, error) {
	// Read the end commit
	commitObj, err := cs.Reader.ReadCommit(endCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to read end commit: %w", err)
	}

	// Read the tree
	tree, err := cs.Reader.ReadTree(commitObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read tree: %w", err)
	}

	// List all files in the tree
	filePaths, err := cs.Reader.ListFiles(tree)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// Create file metadata for each file
	var files []wsindex.FileMetadata
	for _, filePath := range filePaths {
		// Get file content to determine checksum
		content, err := cs.Reader.GetFileContent(tree, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get content for %s: %w", filePath, err)
		}

		// Get file ref by navigating tree structure
		fileRef, err := cs.getFileRefFromTree(tree, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get file ref for %s: %w", filePath, err)
		}

		fileMetadata := wsindex.FileMetadata{
			Path:     filePath,
			FileRef:  fileRef,
			ModTime:  commitObj.CommitTime,
			Mode:     0644,
			Size:     int64(len(content)),
			Checksum: cas.SumB3(content),
		}
		files = append(files, fileMetadata)
	}

	return files, nil
}

// getFileRefFromTree extracts the file reference from a tree by path.
func (cs *CommitSquasher) getFileRefFromTree(tree *commit.TreeObject, filePath string) (filechunk.NodeRef, error) {
	// Split path into parts
	parts := strings.Split(filePath, "/")
	if len(parts) == 0 {
		return filechunk.NodeRef{}, fmt.Errorf("invalid file path: %s", filePath)
	}

	// Navigate through the HAMT structure
	hamtLoader := hamtdir.NewLoader(cs.CAS)
	currentDirRef := tree.DirRef

	for i, part := range parts {
		entries, err := hamtLoader.List(currentDirRef)
		if err != nil {
			return filechunk.NodeRef{}, fmt.Errorf("failed to read directory entries: %w", err)
		}

		if i == len(parts)-1 {
			// This is the final file
			for _, entry := range entries {
				if entry.Name == part && entry.Type == hamtdir.FileEntry {
					return *entry.File, nil
				}
			}
			return filechunk.NodeRef{}, fmt.Errorf("file not found: %s", part)
		} else {
			// Navigate to subdirectory
			found := false
			for _, entry := range entries {
				if entry.Name == part && entry.Type == hamtdir.DirEntry {
					currentDirRef = *entry.Dir
					found = true
					break
				}
			}
			if !found {
				return filechunk.NodeRef{}, fmt.Errorf("directory not found: %s", part)
			}
		}
	}

	return filechunk.NodeRef{}, fmt.Errorf("unexpected error in getFileRefFromTree")
}

// CreateSquashedCommit creates a new commit with the final state and combined metadata.
func (cs *CommitSquasher) CreateSquashedCommit(
	files []wsindex.FileMetadata,
	parent cas.Hash,
	author, message string,
) (*commit.CommitObject, cas.Hash, error) {
	// Determine parents
	var parents []cas.Hash
	if parent != (cas.Hash{}) {
		parents = append(parents, parent)
	}

	// Create the commit
	commitObj, err := cs.Builder.CreateCommit(
		files,
		parents,
		author,
		author,
		message,
	)
	if err != nil {
		return nil, cas.Hash{}, fmt.Errorf("failed to create commit: %w", err)
	}

	// Get commit hash
	commitHash := cs.Builder.GetCommitHash(commitObj)

	return commitObj, commitHash, nil
}

// GetCombinedMessage creates a combined commit message from a range of commits.
// Format: "Squashed N commits: <first message> | <second message> | ..."
func (cs *CommitSquasher) GetCombinedMessage(commits []CommitInfo) string {
	if len(commits) == 0 {
		return "Empty squash"
	}

	if len(commits) == 1 {
		return commits[0].Message
	}

	var messages []string
	for _, c := range commits {
		// Trim message to first line only
		firstLine := strings.Split(c.Message, "\n")[0]
		messages = append(messages, firstLine)
	}

	return fmt.Sprintf("Squashed %d commits:\n\n%s",
		len(commits),
		strings.Join(messages, "\n"))
}

// ValidateRange validates that the commit range is valid.
func (cs *CommitSquasher) ValidateRange(start, end cas.Hash) error {
	// Ensure both commits exist
	if _, err := cs.Reader.ReadCommit(start); err != nil {
		return fmt.Errorf("start commit not found: %w", err)
	}

	if _, err := cs.Reader.ReadCommit(end); err != nil {
		return fmt.Errorf("end commit not found: %w", err)
	}

	// Verify end is a descendant of start
	currentHash := end
	visited := make(map[cas.Hash]bool)

	for {
		if currentHash == start {
			return nil // Valid range
		}

		if visited[currentHash] {
			return fmt.Errorf("cycle detected in commit history")
		}
		visited[currentHash] = true

		commitObj, err := cs.Reader.ReadCommit(currentHash)
		if err != nil {
			return fmt.Errorf("failed to read commit: %w", err)
		}

		if len(commitObj.Parents) == 0 {
			return fmt.Errorf("end commit is not a descendant of start commit")
		}

		currentHash = commitObj.Parents[0]
	}
}

// GetParentOfStart returns the parent commit of the start commit.
// This will be the parent of the squashed commit.
func (cs *CommitSquasher) GetParentOfStart(start cas.Hash) (cas.Hash, error) {
	commitObj, err := cs.Reader.ReadCommit(start)
	if err != nil {
		return cas.Hash{}, fmt.Errorf("failed to read start commit: %w", err)
	}

	if len(commitObj.Parents) == 0 {
		// No parent (root commit)
		return cas.Hash{}, nil
	}

	return commitObj.Parents[0], nil
}
