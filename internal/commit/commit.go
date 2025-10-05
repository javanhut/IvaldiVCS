// Package commit implements the commit object system using Merkle trees.
//
// This package provides:
// - Commit objects that reference tree objects and parent commits
// - Tree objects built from directory HAMTs
// - Integration with the MMR history system
// - Commit creation, reading, and traversal
//
// Structure:
// - Commit: metadata + tree hash + parent hashes + MMR position
// - Tree: HAMT directory structure with file references
// - Integration with workspace index for efficient status tracking
package commit

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/hamtdir"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// CommitObject represents a commit in the repository.
type CommitObject struct {
	TreeHash    cas.Hash    // Hash of the root tree object
	Parents     []cas.Hash  // Hashes of parent commits
	Author      string      // Commit author
	Committer   string      // Commit committer (can be different from author)
	AuthorTime  time.Time   // When the change was authored
	CommitTime  time.Time   // When the commit was created
	Message     string      // Commit message
	MMRPosition uint64      // Position in the MMR history
}

// TreeObject represents a tree (directory) in the repository.
type TreeObject struct {
	Entries []TreeEntry // Sorted list of entries
	DirRef  hamtdir.DirRef // HAMT reference for efficient operations
}

// TreeEntry represents an entry in a tree object.
type TreeEntry struct {
	Mode uint32      // File mode (permissions)
	Name string      // Entry name
	Hash cas.Hash    // Hash of the object (file or subtree)
	Type ObjectType  // Type of the referenced object
}

// ObjectType represents the type of a Git-like object.
type ObjectType uint8

const (
	BlobObject ObjectType = iota + 1
	TreeObject_Type // Avoid conflict with TreeObject struct
	CommitObject_Type
)

// CommitBuilder creates commit objects from workspace state.
type CommitBuilder struct {
	CAS     cas.CAS
	History *history.MMR
}

// NewCommitBuilder creates a new CommitBuilder.
func NewCommitBuilder(casStore cas.CAS, mmr *history.MMR) *CommitBuilder {
	return &CommitBuilder{
		CAS:     casStore,
		History: mmr,
	}
}

// CreateCommit creates a new commit from workspace files.
func (cb *CommitBuilder) CreateCommit(
	workspaceFiles []wsindex.FileMetadata,
	parents []cas.Hash,
	author, committer, message string,
) (*CommitObject, error) {
	
	// Step 1: Build tree structure from workspace files
	treeHash, err := cb.buildTreeFromWorkspace(workspaceFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to build tree: %w", err)
	}

	// Step 2: Create commit object
	now := time.Now()
	commit := &CommitObject{
		TreeHash:   treeHash,
		Parents:    parents,
		Author:     author,
		Committer:  committer,
		AuthorTime: now,
		CommitTime: now,
		Message:    message,
	}

	// Step 3: Add to MMR history (if MMR is available)
	if cb.History != nil {
		// Determine PrevIdx from parent commits
		prevIdx := history.NoParent
		if len(parents) > 0 {
			// Read the first parent's commit to get its MMR position
			parentCommit, err := cb.readCommit(parents[0])
			if err == nil && parentCommit.MMRPosition > 0 {
				prevIdx = parentCommit.MMRPosition
			}
		}

		// Create leaf for MMR
		leaf := history.Leaf{
			TreeRoot:   commit.TreeHash,
			TimelineID: "main", // Default timeline for now
			PrevIdx:    prevIdx,
			Author:     commit.Author,
			TimeUnix:   commit.CommitTime.Unix(),
			Message:    commit.Message,
		}
		position, _, err := cb.History.AppendLeaf(leaf)
		if err != nil {
			return nil, fmt.Errorf("failed to add commit to MMR: %w", err)
		}
		commit.MMRPosition = position
	}

	// Step 4: Store commit object in CAS
	commitData := cb.encodeCommit(commit)
	commitHash := cas.SumB3(commitData)
	
	err = cb.CAS.Put(commitHash, commitData)
	if err != nil {
		return nil, fmt.Errorf("failed to store commit: %w", err)
	}

	return commit, nil
}

// buildTreeFromWorkspace builds a tree structure from workspace files.
func (cb *CommitBuilder) buildTreeFromWorkspace(files []wsindex.FileMetadata) (cas.Hash, error) {
	if len(files) == 0 {
		// Empty tree
		return cb.buildEmptyTree()
	}

	// Group files by directory
	dirStructure := cb.groupFilesByDirectory(files)
	
	// Build tree recursively
	return cb.buildTreeRecursive("", dirStructure)
}

// DirectoryNode represents a directory in the tree structure.
type DirectoryNode struct {
	Files       []wsindex.FileMetadata
	Subdirs     map[string]*DirectoryNode
}

// groupFilesByDirectory groups files into a directory tree structure.
func (cb *CommitBuilder) groupFilesByDirectory(files []wsindex.FileMetadata) *DirectoryNode {
	root := &DirectoryNode{
		Subdirs: make(map[string]*DirectoryNode),
	}

	for _, file := range files {
		parts := splitPath(file.Path)
		current := root

		// Navigate to the directory containing this file
		for _, part := range parts[:len(parts)-1] {
			if current.Subdirs[part] == nil {
				current.Subdirs[part] = &DirectoryNode{
					Subdirs: make(map[string]*DirectoryNode),
				}
			}
			current = current.Subdirs[part]
		}

		// Add file to the final directory
		fileName := parts[len(parts)-1]
		fileWithName := file
		fileWithName.Path = fileName // Store just the filename in the directory
		current.Files = append(current.Files, fileWithName)
	}

	return root
}

// buildTreeRecursive recursively builds trees for directories.
func (cb *CommitBuilder) buildTreeRecursive(path string, node *DirectoryNode) (cas.Hash, error) {
	var entries []hamtdir.Entry

	// Add files as blob entries
	for _, file := range node.Files {
		entry := hamtdir.Entry{
			Name: file.Path, // This is now just the filename
			Type: hamtdir.FileEntry,
			File: &file.FileRef,
		}
		entries = append(entries, entry)
	}

	// Add subdirectories as tree entries
	for dirName, subNode := range node.Subdirs {
		subPath := dirName
		if path != "" {
			subPath = path + "/" + dirName
		}

		subTreeHash, err := cb.buildTreeRecursive(subPath, subNode)
		if err != nil {
			return cas.Hash{}, fmt.Errorf("failed to build subtree %s: %w", subPath, err)
		}

		// Create a dummy DirRef for the subtree
		subDirRef := hamtdir.DirRef{
			Hash: subTreeHash,
			Size: len(subNode.Files), // Approximate size
		}

		entry := hamtdir.Entry{
			Name: dirName,
			Type: hamtdir.DirEntry,
			Dir:  &subDirRef,
		}
		entries = append(entries, entry)
	}

	// Build HAMT for this directory
	hamtBuilder := hamtdir.NewBuilder(cb.CAS)
	dirRef, err := hamtBuilder.Build(entries)
	if err != nil {
		return cas.Hash{}, fmt.Errorf("failed to build HAMT for directory %s: %w", path, err)
	}

	return dirRef.Hash, nil
}

// buildEmptyTree creates an empty tree object.
func (cb *CommitBuilder) buildEmptyTree() (cas.Hash, error) {
	hamtBuilder := hamtdir.NewBuilder(cb.CAS)
	dirRef, err := hamtBuilder.Build(nil)
	if err != nil {
		return cas.Hash{}, fmt.Errorf("failed to build empty tree: %w", err)
	}
	return dirRef.Hash, nil
}

// encodeCommit creates canonical encoding for a commit object.
func (cb *CommitBuilder) encodeCommit(commit *CommitObject) []byte {
	var buf bytes.Buffer

	// Write tree hash
	buf.WriteString("tree ")
	buf.WriteString(commit.TreeHash.String())
	buf.WriteByte('\n')

	// Write parent hashes
	for _, parent := range commit.Parents {
		buf.WriteString("parent ")
		buf.WriteString(parent.String())
		buf.WriteByte('\n')
	}

	// Write author
	buf.WriteString("author ")
	buf.WriteString(commit.Author)
	buf.WriteByte(' ')
	buf.WriteString(fmt.Sprintf("%d", commit.AuthorTime.Unix()))
	buf.WriteString(" +0000\n") // UTC timezone

	// Write committer
	buf.WriteString("committer ")
	buf.WriteString(commit.Committer)
	buf.WriteByte(' ')
	buf.WriteString(fmt.Sprintf("%d", commit.CommitTime.Unix()))
	buf.WriteString(" +0000\n") // UTC timezone

	// Write MMR position if available
	if commit.MMRPosition > 0 {
		buf.WriteString("mmr-position ")
		buf.WriteString(fmt.Sprintf("%d", commit.MMRPosition))
		buf.WriteByte('\n')
	}

	// Empty line before message
	buf.WriteByte('\n')

	// Write message
	buf.WriteString(commit.Message)
	if !bytes.HasSuffix([]byte(commit.Message), []byte{'\n'}) {
		buf.WriteByte('\n')
	}

	return buf.Bytes()
}

// CommitReader reads commit objects and trees.
type CommitReader struct {
	CAS cas.CAS
}

// NewCommitReader creates a new CommitReader.
func NewCommitReader(casStore cas.CAS) *CommitReader {
	return &CommitReader{CAS: casStore}
}

// ReadCommit reads a commit object by hash.
func (cr *CommitReader) ReadCommit(commitHash cas.Hash) (*CommitObject, error) {
	data, err := cr.CAS.Get(commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	return cr.parseCommit(data)
}

// ReadTree reads the tree object for a commit.
func (cr *CommitReader) ReadTree(commit *CommitObject) (*TreeObject, error) {
	// Load the HAMT directory
	loader := hamtdir.NewLoader(cr.CAS)
	
	dirRef := hamtdir.DirRef{
		Hash: commit.TreeHash,
		Size: 0, // Size will be determined when loading
	}

	entries, err := loader.List(dirRef)
	if err != nil {
		return nil, fmt.Errorf("failed to read tree: %w", err)
	}

	// Convert HAMT entries to tree entries
	var treeEntries []TreeEntry
	for _, entry := range entries {
		var objType ObjectType
		var hash cas.Hash

		switch entry.Type {
		case hamtdir.FileEntry:
			objType = BlobObject
			hash = entry.File.Hash
		case hamtdir.DirEntry:
			objType = TreeObject_Type
			hash = entry.Dir.Hash
		}

		treeEntry := TreeEntry{
			Mode: 0644, // Default file mode
			Name: entry.Name,
			Hash: hash,
			Type: objType,
		}
		treeEntries = append(treeEntries, treeEntry)
	}

	// Sort entries by name for deterministic ordering
	sort.Slice(treeEntries, func(i, j int) bool {
		return treeEntries[i].Name < treeEntries[j].Name
	})

	return &TreeObject{
		Entries: treeEntries,
		DirRef:  dirRef,
	}, nil
}

// GetFileContent reads the content of a file from the tree.
func (cr *CommitReader) GetFileContent(tree *TreeObject, filePath string) ([]byte, error) {
	parts := splitPath(filePath)
	
	// Navigate through the tree structure using HAMT
	hamtLoader := hamtdir.NewLoader(cr.CAS)
	currentDirRef := tree.DirRef
	
	// Navigate through directories
	for i, part := range parts {
		entries, err := hamtLoader.List(currentDirRef)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory entries: %w", err)
		}
		
		if i == len(parts)-1 {
			// This is the final file
			for _, entry := range entries {
				if entry.Name == part && entry.Type == hamtdir.FileEntry {
					// Use the original NodeRef which has all the correct information
					loader := filechunk.NewLoader(cr.CAS)
					return loader.ReadAll(*entry.File)
				}
			}
			return nil, fmt.Errorf("file not found: %s", part)
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
				return nil, fmt.Errorf("directory not found: %s", part)
			}
		}
	}
	
	return nil, fmt.Errorf("unexpected error in GetFileContent")
}

// ListFiles lists all files in the tree recursively.
func (cr *CommitReader) ListFiles(tree *TreeObject) ([]string, error) {
	var files []string
	err := cr.listFilesRecursive(tree, "", &files)
	return files, err
}

// listFilesRecursive recursively lists files in a tree.
func (cr *CommitReader) listFilesRecursive(tree *TreeObject, prefix string, files *[]string) error {
	for _, entry := range tree.Entries {
		fullPath := entry.Name
		if prefix != "" {
			fullPath = prefix + "/" + entry.Name
		}

		switch entry.Type {
		case BlobObject:
			*files = append(*files, fullPath)
		case TreeObject_Type:
			// Load and recurse into subtree
			subDirRef := hamtdir.DirRef{
				Hash: entry.Hash,
				Size: 0,
			}

			loader := hamtdir.NewLoader(cr.CAS)
			subEntries, err := loader.List(subDirRef)
			if err != nil {
				return fmt.Errorf("failed to read subtree %s: %w", entry.Name, err)
			}

			// Convert to TreeObject
			var subTreeEntries []TreeEntry
			for _, subEntry := range subEntries {
				var objType ObjectType
				var hash cas.Hash

				switch subEntry.Type {
				case hamtdir.FileEntry:
					objType = BlobObject
					hash = subEntry.File.Hash
				case hamtdir.DirEntry:
					objType = TreeObject_Type
					hash = subEntry.Dir.Hash
				}

				subTreeEntries = append(subTreeEntries, TreeEntry{
					Mode: 0644,
					Name: subEntry.Name,
					Hash: hash,
					Type: objType,
				})
			}

			subTree := &TreeObject{
				Entries: subTreeEntries,
				DirRef:  subDirRef,
			}

			err = cr.listFilesRecursive(subTree, fullPath, files)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// parseCommit parses commit object data.
func (cr *CommitReader) parseCommit(data []byte) (*CommitObject, error) {
	lines := bytes.Split(data, []byte{'\n'})
	commit := &CommitObject{}
	
	var messageStart int
	for i, line := range lines {
		if len(line) == 0 {
			// Empty line indicates start of message
			messageStart = i + 1
			break
		}

		parts := bytes.SplitN(line, []byte{' '}, 2)
		if len(parts) < 2 {
			continue
		}

		key := string(parts[0])
		value := string(parts[1])

		switch key {
		case "tree":
			hash, err := parseHash(value)
			if err != nil {
				return nil, fmt.Errorf("invalid tree hash: %w", err)
			}
			commit.TreeHash = hash

		case "parent":
			hash, err := parseHash(value)
			if err != nil {
				return nil, fmt.Errorf("invalid parent hash: %w", err)
			}
			commit.Parents = append(commit.Parents, hash)

		case "author":
			parts := bytes.Fields(parts[1])
			if len(parts) >= 2 {
				commit.Author = string(bytes.Join(parts[:len(parts)-2], []byte{' '}))
				if timestamp, err := parseTimestamp(string(parts[len(parts)-2])); err == nil {
					commit.AuthorTime = timestamp
				}
			}

		case "committer":
			parts := bytes.Fields(parts[1])
			if len(parts) >= 2 {
				commit.Committer = string(bytes.Join(parts[:len(parts)-2], []byte{' '}))
				if timestamp, err := parseTimestamp(string(parts[len(parts)-2])); err == nil {
					commit.CommitTime = timestamp
				}
			}

		case "mmr-position":
			if pos, err := parseUint64(value); err == nil {
				commit.MMRPosition = pos
			}
		}
	}

	// Extract message
	if messageStart < len(lines) {
		messageLines := lines[messageStart:]
		messageBytes := bytes.Join(messageLines, []byte{'\n'})
		messageBytes = bytes.TrimSuffix(messageBytes, []byte{'\n'})
		commit.Message = string(messageBytes)
	}

	return commit, nil
}

// Helper functions

func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	// Remove leading and trailing slashes, then split
	pathBytes := bytes.Trim([]byte(path), "/")
	if len(pathBytes) == 0 {
		return []string{}
	}
	parts := bytes.Split(pathBytes, []byte{'/'})
	result := make([]string, len(parts))
	for i, part := range parts {
		result[i] = string(part)
	}
	return result
}

func parseHash(s string) (cas.Hash, error) {
	if len(s) != 64 { // 32 bytes * 2 hex chars
		return cas.Hash{}, fmt.Errorf("invalid hash length: %d", len(s))
	}
	
	var hash cas.Hash
	for i := 0; i < 32; i++ {
		b, err := parseHexByte(s[i*2 : i*2+2])
		if err != nil {
			return cas.Hash{}, err
		}
		hash[i] = b
	}
	return hash, nil
}

func parseHexByte(s string) (byte, error) {
	if len(s) != 2 {
		return 0, fmt.Errorf("invalid hex byte: %s", s)
	}
	
	var result byte
	for _, c := range []byte(s) {
		var digit byte
		switch {
		case c >= '0' && c <= '9':
			digit = c - '0'
		case c >= 'a' && c <= 'f':
			digit = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			digit = c - 'A' + 10
		default:
			return 0, fmt.Errorf("invalid hex character: %c", c)
		}
		result = (result << 4) | digit
	}
	return result, nil
}

func parseTimestamp(s string) (time.Time, error) {
	var timestamp int64
	n, err := fmt.Sscanf(s, "%d", &timestamp)
	if err != nil || n != 1 {
		return time.Time{}, fmt.Errorf("invalid timestamp: %s", s)
	}
	return time.Unix(timestamp, 0), nil
}

func parseUint64(s string) (uint64, error) {
	var value uint64
	n, err := fmt.Sscanf(s, "%d", &value)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("invalid uint64: %s", s)
	}
	return value, nil
}

// GetCommitHash computes the hash of a commit object.
func (cb *CommitBuilder) GetCommitHash(commit *CommitObject) cas.Hash {
	data := cb.encodeCommit(commit)
	return cas.SumB3(data)
}

// readCommit reads a commit from CAS (helper for internal use)
func (cb *CommitBuilder) readCommit(hash cas.Hash) (*CommitObject, error) {
	reader := NewCommitReader(cb.CAS)
	return reader.ReadCommit(hash)
}