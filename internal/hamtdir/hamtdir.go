// Package hamtdir implements Hash Array Mapped Trie for scalable directory storage.
//
// Directories are represented as HAMTs where:
// - Keys are file/subdirectory names (strings)
// - Values are either file references (NodeRef) or subdirectory references (DirRef)
// - Internal nodes use 32-bit hash chunks with 5-bit fanout (32-way branching)
// - Canonical encoding ensures deterministic hashes
//
// Canonical Encoding:
// - Leaf: 0x00 | uvarint(entryCount) | (key_len | key | value_type | value_data)*
// - Internal: 0x01 | bitmap[4] | childHash[32] * popcount(bitmap)
// - Hash: BLAKE3(canonicalBytes)
package hamtdir

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/bits"
	"sort"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
)

// EntryType represents the type of directory entry.
type EntryType uint8

const (
	FileEntry EntryType = iota + 1
	DirEntry
)

// Entry represents a single directory entry.
type Entry struct {
	Name string
	Type EntryType
	File *filechunk.NodeRef // Set if Type == FileEntry
	Dir  *DirRef            // Set if Type == DirEntry
}

// DirRef represents a reference to a directory HAMT.
type DirRef struct {
	Hash cas.Hash // BLAKE3 hash of the directory
	Size int       // Number of entries in the directory tree
}

// Node represents a HAMT node.
type Node struct {
	// For leaf nodes
	IsLeaf  bool
	Entries []Entry // Only for leaf nodes

	// For internal nodes
	Bitmap   uint32            // 32-bit bitmap indicating which children exist
	Children map[int]cas.Hash  // Map from bit position to child hash
}

// Builder constructs directory HAMTs.
type Builder struct {
	CAS cas.CAS
}

// NewBuilder creates a new Builder with the given CAS.
func NewBuilder(casStore cas.CAS) *Builder {
	return &Builder{CAS: casStore}
}

// Build creates a HAMT directory from the given entries.
func (b *Builder) Build(entries []Entry) (DirRef, error) {
	if len(entries) == 0 {
		// Empty directory
		return b.buildLeaf(nil)
	}

	// Sort entries by name for deterministic ordering
	sortedEntries := make([]Entry, len(entries))
	copy(sortedEntries, entries)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Name < sortedEntries[j].Name
	})

	return b.buildNode(sortedEntries, 0)
}

// buildNode recursively builds HAMT nodes.
func (b *Builder) buildNode(entries []Entry, depth int) (DirRef, error) {
	if len(entries) <= 16 { // Leaf threshold
		return b.buildLeaf(entries)
	}

	// Group entries by hash chunk at current depth
	groups := make(map[uint32][]Entry)
	for _, entry := range entries {
		chunk := b.hashChunk(entry.Name, depth)
		groups[chunk] = append(groups[chunk], entry)
	}

	// Build children for each group
	var bitmap uint32
	children := make(map[int]cas.Hash)
	totalSize := 0

	for chunk, groupEntries := range groups {
		childRef, err := b.buildNode(groupEntries, depth+1)
		if err != nil {
			return DirRef{}, err
		}

		bitPos := int(chunk % 32)
		bitmap |= (1 << bitPos)
		children[bitPos] = childRef.Hash
		totalSize += childRef.Size
	}

	// Create internal node
	node := &Node{
		IsLeaf:   false,
		Bitmap:   bitmap,
		Children: children,
	}

	canonical := b.encodeInternal(node)
	hash := cas.SumB3(canonical)

	err := b.CAS.Put(hash, canonical)
	if err != nil {
		return DirRef{}, fmt.Errorf("failed to store internal node: %w", err)
	}

	return DirRef{
		Hash: hash,
		Size: totalSize,
	}, nil
}

// buildLeaf creates a leaf node from entries.
func (b *Builder) buildLeaf(entries []Entry) (DirRef, error) {
	node := &Node{
		IsLeaf:  true,
		Entries: entries,
	}

	canonical := b.encodeLeaf(node)
	hash := cas.SumB3(canonical)

	err := b.CAS.Put(hash, canonical)
	if err != nil {
		return DirRef{}, fmt.Errorf("failed to store leaf node: %w", err)
	}

	return DirRef{
		Hash: hash,
		Size: len(entries),
	}, nil
}

// hashChunk extracts a 5-bit chunk from the hash of a name at given depth.
func (b *Builder) hashChunk(name string, depth int) uint32 {
	hash := cas.SumB3([]byte(name))
	
	// Extract 5 bits starting at bit position depth*5
	bitOffset := depth * 5
	byteOffset := bitOffset / 8
	bitWithinByte := bitOffset % 8
	
	if byteOffset >= len(hash) {
		return 0
	}
	
	// Extract up to 16 bits to handle cross-byte boundaries
	var bits uint16
	if byteOffset+1 < len(hash) {
		bits = uint16(hash[byteOffset]) | (uint16(hash[byteOffset+1]) << 8)
	} else {
		bits = uint16(hash[byteOffset])
	}
	
	// Shift and mask to get our 5-bit chunk
	chunk := (bits >> bitWithinByte) & 0x1F // 0x1F = 31 = 2^5-1
	return uint32(chunk)
}

// encodeLeaf creates canonical encoding for a leaf node.
func (b *Builder) encodeLeaf(node *Node) []byte {
	var buf bytes.Buffer
	buf.WriteByte(0x00) // Leaf marker

	// Write entry count
	lenBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(lenBuf, uint64(len(node.Entries)))
	buf.Write(lenBuf[:n])

	// Write entries (sorted by name for deterministic encoding)
	sortedEntries := make([]Entry, len(node.Entries))
	copy(sortedEntries, node.Entries)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Name < sortedEntries[j].Name
	})

	for _, entry := range sortedEntries {
		// Write key length and key
		n = binary.PutUvarint(lenBuf, uint64(len(entry.Name)))
		buf.Write(lenBuf[:n])
		buf.WriteString(entry.Name)

		// Write value type
		buf.WriteByte(byte(entry.Type))

		// Write value data
		if entry.Type == FileEntry && entry.File != nil {
			buf.WriteByte(byte(entry.File.Kind))
			buf.Write(entry.File.Hash[:])
			n = binary.PutUvarint(lenBuf, uint64(entry.File.Size))
			buf.Write(lenBuf[:n])
		} else if entry.Type == DirEntry && entry.Dir != nil {
			buf.Write(entry.Dir.Hash[:])
			n = binary.PutUvarint(lenBuf, uint64(entry.Dir.Size))
			buf.Write(lenBuf[:n])
		}
	}

	return buf.Bytes()
}

// encodeInternal creates canonical encoding for an internal node.
func (b *Builder) encodeInternal(node *Node) []byte {
	var buf bytes.Buffer
	buf.WriteByte(0x01) // Internal marker

	// Write bitmap (4 bytes, little-endian)
	binary.Write(&buf, binary.LittleEndian, node.Bitmap)

	// Write child hashes in bit position order
	for bitPos := 0; bitPos < 32; bitPos++ {
		if (node.Bitmap & (1 << bitPos)) != 0 {
			if childHash, exists := node.Children[bitPos]; exists {
				buf.Write(childHash[:])
			}
		}
	}

	return buf.Bytes()
}

// Loader reads directory HAMTs.
type Loader struct {
	CAS cas.CAS
}

// NewLoader creates a new Loader with the given CAS.
func NewLoader(casStore cas.CAS) *Loader {
	return &Loader{CAS: casStore}
}

// Lookup finds an entry by name in the directory.
func (l *Loader) Lookup(dir DirRef, name string) (*Entry, error) {
	return l.lookupNode(dir.Hash, name, 0)
}

// ListAll returns all entries in the directory (recursively flattened).
func (l *Loader) ListAll(dir DirRef) ([]Entry, error) {
	return l.listNode(dir.Hash)
}

// List returns direct entries in the directory (non-recursive).
func (l *Loader) List(dir DirRef) ([]Entry, error) {
	data, err := l.CAS.Get(dir.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get directory node: %w", err)
	}

	node, err := l.decodeNode(data)
	if err != nil {
		return nil, err
	}

	if node.IsLeaf {
		return node.Entries, nil
	}

	// For internal nodes, collect entries from all children
	var allEntries []Entry
	for bitPos := 0; bitPos < 32; bitPos++ {
		if (node.Bitmap & (1 << bitPos)) != 0 {
			if childHash, exists := node.Children[bitPos]; exists {
				childEntries, err := l.listNode(childHash)
				if err != nil {
					return nil, err
				}
				allEntries = append(allEntries, childEntries...)
			}
		}
	}

	return allEntries, nil
}

// lookupNode recursively searches for an entry by name.
func (l *Loader) lookupNode(nodeHash cas.Hash, name string, depth int) (*Entry, error) {
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
			if entry.Name == name {
				return &entry, nil
			}
		}
		return nil, nil // Not found
	}

	// Internal node: find the right child
	chunk := l.hashChunk(name, depth)
	bitPos := int(chunk % 32)

	if (node.Bitmap & (1 << bitPos)) == 0 {
		return nil, nil // Child doesn't exist
	}

	childHash, exists := node.Children[bitPos]
	if !exists {
		return nil, fmt.Errorf("bitmap indicates child exists but hash not found")
	}

	return l.lookupNode(childHash, name, depth+1)
}

// listNode recursively lists all entries in a node.
func (l *Loader) listNode(nodeHash cas.Hash) ([]Entry, error) {
	data, err := l.CAS.Get(nodeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	node, err := l.decodeNode(data)
	if err != nil {
		return nil, err
	}

	if node.IsLeaf {
		return node.Entries, nil
	}

	// Internal node: collect from all children
	var allEntries []Entry
	for bitPos := 0; bitPos < 32; bitPos++ {
		if (node.Bitmap & (1 << bitPos)) != 0 {
			if childHash, exists := node.Children[bitPos]; exists {
				childEntries, err := l.listNode(childHash)
				if err != nil {
					return nil, err
				}
				allEntries = append(allEntries, childEntries...)
			}
		}
	}

	return allEntries, nil
}

// hashChunk extracts a 5-bit chunk from the hash (same as Builder).
func (l *Loader) hashChunk(name string, depth int) uint32 {
	hash := cas.SumB3([]byte(name))
	
	bitOffset := depth * 5
	byteOffset := bitOffset / 8
	bitWithinByte := bitOffset % 8
	
	if byteOffset >= len(hash) {
		return 0
	}
	
	var bits uint16
	if byteOffset+1 < len(hash) {
		bits = uint16(hash[byteOffset]) | (uint16(hash[byteOffset+1]) << 8)
	} else {
		bits = uint16(hash[byteOffset])
	}
	
	chunk := (bits >> bitWithinByte) & 0x1F
	return uint32(chunk)
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

	entries := make([]Entry, 0, entryCount)
	
	for i := uint64(0); i < entryCount; i++ {
		// Read key length and key
		keyLen, err := binary.ReadUvarint(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read key length: %w", err)
		}

		key := make([]byte, keyLen)
		n, err := buf.Read(key)
		if err != nil || uint64(n) != keyLen {
			return nil, fmt.Errorf("failed to read key")
		}

		// Read value type
		var valueType byte
		err = binary.Read(buf, binary.LittleEndian, &valueType)
		if err != nil {
			return nil, fmt.Errorf("failed to read value type: %w", err)
		}

		entry := Entry{
			Name: string(key),
			Type: EntryType(valueType),
		}

		// Read value data
		if entry.Type == FileEntry {
			var nodeKind byte
			err = binary.Read(buf, binary.LittleEndian, &nodeKind)
			if err != nil {
				return nil, fmt.Errorf("failed to read node kind: %w", err)
			}

			var hash cas.Hash
			n, err := buf.Read(hash[:])
			if err != nil || n != 32 {
				return nil, fmt.Errorf("failed to read file hash")
			}

			size, err := binary.ReadUvarint(buf)
			if err != nil {
				return nil, fmt.Errorf("failed to read file size: %w", err)
			}

			entry.File = &filechunk.NodeRef{
				Hash: hash,
				Kind: filechunk.NodeKind(nodeKind),
				Size: int64(size),
			}
		} else if entry.Type == DirEntry {
			var hash cas.Hash
			n, err := buf.Read(hash[:])
			if err != nil || n != 32 {
				return nil, fmt.Errorf("failed to read dir hash")
			}

			size, err := binary.ReadUvarint(buf)
			if err != nil {
				return nil, fmt.Errorf("failed to read dir size: %w", err)
			}

			entry.Dir = &DirRef{
				Hash: hash,
				Size: int(size),
			}
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
	if len(data) < 5 { // marker + 4-byte bitmap
		return nil, fmt.Errorf("internal node data too short")
	}

	buf := bytes.NewReader(data[1:]) // Skip marker

	// Read bitmap
	var bitmap uint32
	err := binary.Read(buf, binary.LittleEndian, &bitmap)
	if err != nil {
		return nil, fmt.Errorf("failed to read bitmap: %w", err)
	}

	// Count children
	childCount := bits.OnesCount32(bitmap)
	children := make(map[int]cas.Hash)

	// Read child hashes in bit position order
	for bitPos := 0; bitPos < 32; bitPos++ {
		if (bitmap & (1 << bitPos)) != 0 {
			var hash cas.Hash
			n, err := buf.Read(hash[:])
			if err != nil || n != 32 {
				return nil, fmt.Errorf("failed to read child hash at bit %d", bitPos)
			}
			children[bitPos] = hash
		}
	}

	if len(children) != childCount {
		return nil, fmt.Errorf("child count mismatch: expected %d, got %d", childCount, len(children))
	}

	return &Node{
		IsLeaf:   false,
		Bitmap:   bitmap,
		Children: children,
	}, nil
}

// WalkEntries recursively walks all entries in the directory.
func (l *Loader) WalkEntries(dir DirRef, walkFn func(path string, entry Entry) error) error {
	return l.walkNode(dir.Hash, "", walkFn)
}

// walkNode recursively walks a node with path prefix.
func (l *Loader) walkNode(nodeHash cas.Hash, pathPrefix string, walkFn func(string, Entry) error) error {
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
			fullPath := entry.Name
			if pathPrefix != "" {
				fullPath = pathPrefix + "/" + entry.Name
			}
			
			err := walkFn(fullPath, entry)
			if err != nil {
				return err
			}
			
			// If it's a directory, recurse into it
			if entry.Type == DirEntry && entry.Dir != nil {
				err = l.walkNode(entry.Dir.Hash, fullPath, walkFn)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Internal node: walk all children
	for bitPos := 0; bitPos < 32; bitPos++ {
		if (node.Bitmap & (1 << bitPos)) != 0 {
			if childHash, exists := node.Children[bitPos]; exists {
				err = l.walkNode(childHash, pathPrefix, walkFn)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// PathLookup performs a path-based lookup (e.g., "dir1/dir2/file.txt").
func (l *Loader) PathLookup(root DirRef, path string) (*Entry, error) {
	if path == "" || path == "/" {
		return nil, fmt.Errorf("invalid path")
	}
	
	// Clean and split path
	path = strings.Trim(path, "/")
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	
	components := strings.Split(path, "/")
	currentDir := root
	
	for i, component := range components {
		entry, err := l.Lookup(currentDir, component)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			return nil, nil // Not found
		}
		
		// If this is the last component, return it
		if i == len(components)-1 {
			return entry, nil
		}
		
		// Must be a directory to continue
		if entry.Type != DirEntry || entry.Dir == nil {
			return nil, fmt.Errorf("path component '%s' is not a directory", component)
		}
		
		currentDir = *entry.Dir
	}
	
	return nil, fmt.Errorf("path traversal failed")
}