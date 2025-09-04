// Package wsindex implements flat key-value Merkle index for workspace file tracking.
//
// The workspace index tracks all files in a workspace with their metadata:
// - Keys are file paths (strings)
// - Values contain file metadata (hash, size, modification time, etc.)
// - Implemented as a sorted Merkle tree for efficient global queries
// - Supports range queries, prefix matching, and difference computation
//
// Canonical Encoding:
// - Leaf: 0x00 | uvarint(entryCount) | (key_len | key | value_data)*
// - Internal: 0x01 | uvarint(childCount) | childHash[32] * childCount | separator_keys
// - Hash: BLAKE3(canonicalBytes)
package wsindex

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
)

// FileMetadata represents metadata for a file in the workspace.
type FileMetadata struct {
	Path     string                  // Full file path
	FileRef  filechunk.NodeRef       // Reference to file content
	ModTime  time.Time               // Last modification time
	Mode     uint32                  // File mode/permissions
	Size     int64                   // File size in bytes
	Checksum cas.Hash                // Content hash for quick comparison
}

// IndexRef represents a reference to a workspace index.
type IndexRef struct {
	Hash  cas.Hash // BLAKE3 hash of the index
	Count int      // Total number of files in the index
}

// Node represents a node in the flat Merkle index tree.
type Node struct {
	// For leaf nodes
	IsLeaf  bool
	Entries []FileMetadata // Only for leaf nodes

	// For internal nodes
	Children   []cas.Hash // Child hashes in sorted order
	Separators []string   // Separator keys (len = len(Children) - 1)
}

// Builder constructs flat key-value Merkle indexes.
type Builder struct {
	CAS        cas.CAS
	LeafSize   int // Maximum entries per leaf node
}

// NewBuilder creates a new Builder with the given CAS.
func NewBuilder(casStore cas.CAS) *Builder {
	return &Builder{
		CAS:      casStore,
		LeafSize: 64, // Default leaf size
	}
}

// Build creates a flat Merkle index from the given file metadata.
func (b *Builder) Build(files []FileMetadata) (IndexRef, error) {
	if len(files) == 0 {
		// Empty index
		return b.buildLeaf(nil)
	}

	// Sort files by path for deterministic ordering
	sortedFiles := make([]FileMetadata, len(files))
	copy(sortedFiles, files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].Path < sortedFiles[j].Path
	})

	return b.buildTree(sortedFiles)
}

// buildTree recursively builds the Merkle tree.
func (b *Builder) buildTree(files []FileMetadata) (IndexRef, error) {
	if len(files) <= b.LeafSize {
		return b.buildLeaf(files)
	}

	// Split files into chunks for child nodes
	var children []IndexRef
	var separators []string
	
	chunkSize := (len(files) + b.LeafSize - 1) / ((len(files) + b.LeafSize - 1) / b.LeafSize) // Balanced distribution
	if chunkSize < 1 {
		chunkSize = 1
	}

	for i := 0; i < len(files); i += chunkSize {
		end := i + chunkSize
		if end > len(files) {
			end = len(files)
		}
		
		chunk := files[i:end]
		childRef, err := b.buildTree(chunk)
		if err != nil {
			return IndexRef{}, err
		}
		
		children = append(children, childRef)
		
		// Add separator (first key of next chunk, or empty for last)
		if end < len(files) {
			separators = append(separators, files[end].Path)
		}
	}

	return b.buildInternal(children, separators)
}

// buildLeaf creates a leaf node from file metadata.
func (b *Builder) buildLeaf(files []FileMetadata) (IndexRef, error) {
	node := &Node{
		IsLeaf:  true,
		Entries: files,
	}

	canonical := b.encodeLeaf(node)
	hash := cas.SumB3(canonical)

	err := b.CAS.Put(hash, canonical)
	if err != nil {
		return IndexRef{}, fmt.Errorf("failed to store leaf node: %w", err)
	}

	return IndexRef{
		Hash:  hash,
		Count: len(files),
	}, nil
}

// buildInternal creates an internal node from child references.
func (b *Builder) buildInternal(children []IndexRef, separators []string) (IndexRef, error) {
	childHashes := make([]cas.Hash, len(children))
	totalCount := 0
	
	for i, child := range children {
		childHashes[i] = child.Hash
		totalCount += child.Count
	}

	node := &Node{
		IsLeaf:     false,
		Children:   childHashes,
		Separators: separators,
	}

	canonical := b.encodeInternal(node)
	hash := cas.SumB3(canonical)

	err := b.CAS.Put(hash, canonical)
	if err != nil {
		return IndexRef{}, fmt.Errorf("failed to store internal node: %w", err)
	}

	return IndexRef{
		Hash:  hash,
		Count: totalCount,
	}, nil
}

// encodeLeaf creates canonical encoding for a leaf node.
func (b *Builder) encodeLeaf(node *Node) []byte {
	var buf bytes.Buffer
	buf.WriteByte(0x00) // Leaf marker

	// Write entry count
	lenBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(lenBuf, uint64(len(node.Entries)))
	buf.Write(lenBuf[:n])

	// Write entries in sorted order
	sortedEntries := make([]FileMetadata, len(node.Entries))
	copy(sortedEntries, node.Entries)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Path < sortedEntries[j].Path
	})

	for _, entry := range sortedEntries {
		// Write path length and path
		n = binary.PutUvarint(lenBuf, uint64(len(entry.Path)))
		buf.Write(lenBuf[:n])
		buf.WriteString(entry.Path)

		// Write file reference
		buf.WriteByte(byte(entry.FileRef.Kind))
		buf.Write(entry.FileRef.Hash[:])
		n = binary.PutUvarint(lenBuf, uint64(entry.FileRef.Size))
		buf.Write(lenBuf[:n])

		// Write modification time (Unix nanoseconds)
		n = binary.PutUvarint(lenBuf, uint64(entry.ModTime.UnixNano()))
		buf.Write(lenBuf[:n])

		// Write file mode
		n = binary.PutUvarint(lenBuf, uint64(entry.Mode))
		buf.Write(lenBuf[:n])

		// Write size
		n = binary.PutUvarint(lenBuf, uint64(entry.Size))
		buf.Write(lenBuf[:n])

		// Write content checksum
		buf.Write(entry.Checksum[:])
	}

	return buf.Bytes()
}

// encodeInternal creates canonical encoding for an internal node.
func (b *Builder) encodeInternal(node *Node) []byte {
	var buf bytes.Buffer
	buf.WriteByte(0x01) // Internal marker

	// Write child count
	lenBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(lenBuf, uint64(len(node.Children)))
	buf.Write(lenBuf[:n])

	// Write child hashes
	for _, childHash := range node.Children {
		buf.Write(childHash[:])
	}

	// Write separators
	for _, sep := range node.Separators {
		n = binary.PutUvarint(lenBuf, uint64(len(sep)))
		buf.Write(lenBuf[:n])
		buf.WriteString(sep)
	}

	return buf.Bytes()
}

// Loader reads flat key-value Merkle indexes.
type Loader struct {
	CAS cas.CAS
}

// NewLoader creates a new Loader with the given CAS.
func NewLoader(casStore cas.CAS) *Loader {
	return &Loader{CAS: casStore}
}

// Lookup finds a file by exact path.
func (l *Loader) Lookup(index IndexRef, path string) (*FileMetadata, error) {
	return l.lookupNode(index.Hash, path)
}

// ListAll returns all files in the index.
func (l *Loader) ListAll(index IndexRef) ([]FileMetadata, error) {
	return l.listNode(index.Hash)
}

// ListPrefix returns all files with the given path prefix.
func (l *Loader) ListPrefix(index IndexRef, prefix string) ([]FileMetadata, error) {
	var result []FileMetadata
	err := l.walkNode(index.Hash, func(file FileMetadata) error {
		if strings.HasPrefix(file.Path, prefix) {
			result = append(result, file)
		}
		return nil
	})
	return result, err
}

// ListRange returns files in the given path range [start, end).
func (l *Loader) ListRange(index IndexRef, start, end string) ([]FileMetadata, error) {
	var result []FileMetadata
	err := l.walkNode(index.Hash, func(file FileMetadata) error {
		if file.Path >= start && file.Path < end {
			result = append(result, file)
		}
		return nil
	})
	return result, err
}

// Walk calls the given function for each file in the index.
func (l *Loader) Walk(index IndexRef, walkFn func(FileMetadata) error) error {
	return l.walkNode(index.Hash, walkFn)
}

// lookupNode recursively searches for a file by path.
func (l *Loader) lookupNode(nodeHash cas.Hash, path string) (*FileMetadata, error) {
	data, err := l.CAS.Get(nodeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	node, err := l.decodeNode(data)
	if err != nil {
		return nil, err
	}

	if node.IsLeaf {
		// Search in leaf entries
		for _, entry := range node.Entries {
			if entry.Path == path {
				return &entry, nil
			}
		}
		return nil, nil // Not found
	}

	// Internal node: find the right child
	childIndex := l.findChildIndex(node, path)
	if childIndex < 0 || childIndex >= len(node.Children) {
		return nil, nil // Not found
	}

	return l.lookupNode(node.Children[childIndex], path)
}

// listNode recursively lists all files in a node.
func (l *Loader) listNode(nodeHash cas.Hash) ([]FileMetadata, error) {
	var result []FileMetadata
	err := l.walkNode(nodeHash, func(file FileMetadata) error {
		result = append(result, file)
		return nil
	})
	return result, err
}

// walkNode recursively walks all files in a node.
func (l *Loader) walkNode(nodeHash cas.Hash, walkFn func(FileMetadata) error) error {
	data, err := l.CAS.Get(nodeHash)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	node, err := l.decodeNode(data)
	if err != nil {
		return err
	}

	if node.IsLeaf {
		for _, entry := range node.Entries {
			err := walkFn(entry)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Internal node: walk all children
	for _, childHash := range node.Children {
		err := l.walkNode(childHash, walkFn)
		if err != nil {
			return err
		}
	}

	return nil
}

// findChildIndex finds which child should contain the given path.
func (l *Loader) findChildIndex(node *Node, path string) int {
	// Binary search through separators to find the right child
	for i, sep := range node.Separators {
		if path < sep {
			return i
		}
	}
	return len(node.Separators) // Last child
}

// decodeNode decodes canonical bytes into a Node.
func (l *Loader) decodeNode(data []byte) (*Node, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty node data")
	}

	if data[0] == 0x00 {
		return l.decodeLeaf(data)
	} else if data[0] == 0x01 {
		return l.decodeInternal(data)
	}

	return nil, fmt.Errorf("invalid node encoding: unknown marker %02x", data[0])
}

// decodeLeaf decodes a leaf node.
func (l *Loader) decodeLeaf(data []byte) (*Node, error) {
	buf := bytes.NewReader(data[1:]) // Skip marker

	// Read entry count
	entryCount, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read entry count: %w", err)
	}

	entries := make([]FileMetadata, 0, entryCount)

	for i := uint64(0); i < entryCount; i++ {
		// Read path length and path
		pathLen, err := binary.ReadUvarint(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read path length: %w", err)
		}

		pathBytes := make([]byte, pathLen)
		n, err := buf.Read(pathBytes)
		if err != nil || uint64(n) != pathLen {
			return nil, fmt.Errorf("failed to read path")
		}

		// Read file reference
		var nodeKind byte
		err = binary.Read(buf, binary.LittleEndian, &nodeKind)
		if err != nil {
			return nil, fmt.Errorf("failed to read node kind: %w", err)
		}

		var fileHash cas.Hash
		n, err = buf.Read(fileHash[:])
		if err != nil || n != 32 {
			return nil, fmt.Errorf("failed to read file hash")
		}

		fileSize, err := binary.ReadUvarint(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read file size: %w", err)
		}

		// Read modification time
		modTimeNanos, err := binary.ReadUvarint(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read mod time: %w", err)
		}

		// Read file mode
		mode, err := binary.ReadUvarint(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read mode: %w", err)
		}

		// Read size
		size, err := binary.ReadUvarint(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read size: %w", err)
		}

		// Read checksum
		var checksum cas.Hash
		n, err = buf.Read(checksum[:])
		if err != nil || n != 32 {
			return nil, fmt.Errorf("failed to read checksum")
		}

		entry := FileMetadata{
			Path: string(pathBytes),
			FileRef: filechunk.NodeRef{
				Hash: fileHash,
				Kind: filechunk.NodeKind(nodeKind),
				Size: int64(fileSize),
			},
			ModTime:  time.Unix(0, int64(modTimeNanos)),
			Mode:     uint32(mode),
			Size:     int64(size),
			Checksum: checksum,
		}

		entries = append(entries, entry)
	}

	return &Node{
		IsLeaf:  true,
		Entries: entries,
	}, nil
}

// decodeInternal decodes an internal node.
func (l *Loader) decodeInternal(data []byte) (*Node, error) {
	buf := bytes.NewReader(data[1:]) // Skip marker

	// Read child count
	childCount, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read child count: %w", err)
	}

	// Read child hashes
	children := make([]cas.Hash, childCount)
	for i := uint64(0); i < childCount; i++ {
		n, err := buf.Read(children[i][:])
		if err != nil || n != 32 {
			return nil, fmt.Errorf("failed to read child hash %d", i)
		}
	}

	// Read separators (childCount - 1 separators)
	var separators []string
	if childCount > 1 {
		separators = make([]string, 0, childCount-1)
		for i := uint64(0); i < childCount-1; i++ {
			sepLen, err := binary.ReadUvarint(buf)
			if err != nil {
				return nil, fmt.Errorf("failed to read separator length: %w", err)
			}

			sepBytes := make([]byte, sepLen)
			n, err := buf.Read(sepBytes)
			if err != nil || uint64(n) != sepLen {
				return nil, fmt.Errorf("failed to read separator")
			}

			separators = append(separators, string(sepBytes))
		}
	}

	return &Node{
		IsLeaf:     false,
		Children:   children,
		Separators: separators,
	}, nil
}

// Diff computes the differences between two workspace indexes.
type DiffResult struct {
	Added    []FileMetadata // Files in new but not in old
	Modified []FileMetadata // Files in both but with different content/metadata
	Removed  []FileMetadata // Files in old but not in new
}

// Diff computes differences between two indexes.
func (l *Loader) Diff(oldIndex, newIndex IndexRef) (*DiffResult, error) {
	oldFiles, err := l.ListAll(oldIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to list old index: %w", err)
	}

	newFiles, err := l.ListAll(newIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to list new index: %w", err)
	}

	// Create maps for efficient lookup
	oldMap := make(map[string]FileMetadata)
	for _, file := range oldFiles {
		oldMap[file.Path] = file
	}

	newMap := make(map[string]FileMetadata)
	for _, file := range newFiles {
		newMap[file.Path] = file
	}

	result := &DiffResult{}

	// Find added and modified files
	for _, newFile := range newFiles {
		if oldFile, exists := oldMap[newFile.Path]; exists {
			// File exists in both - check if modified
			if !l.filesEqual(oldFile, newFile) {
				result.Modified = append(result.Modified, newFile)
			}
		} else {
			// File is new
			result.Added = append(result.Added, newFile)
		}
	}

	// Find removed files
	for _, oldFile := range oldFiles {
		if _, exists := newMap[oldFile.Path]; !exists {
			result.Removed = append(result.Removed, oldFile)
		}
	}

	return result, nil
}

// filesEqual checks if two file metadata entries are equal.
func (l *Loader) filesEqual(a, b FileMetadata) bool {
	return a.Path == b.Path &&
		a.FileRef.Hash == b.FileRef.Hash &&
		a.FileRef.Kind == b.FileRef.Kind &&
		a.FileRef.Size == b.FileRef.Size &&
		a.ModTime.Equal(b.ModTime) &&
		a.Mode == b.Mode &&
		a.Size == b.Size &&
		a.Checksum == b.Checksum
}