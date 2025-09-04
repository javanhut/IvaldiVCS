// Package fsmerkle implements a Merkle DAG for filesystem trees.
//
// This package represents the working tree as a Merkle DAG with:
// - BlobNode for file content
// - TreeNode for directories (sorted entries, structural sharing)
// - Stable canonical encodings for hashing and storage
// - Efficient diff that short-circuits identical subtrees by hash
//
// All hashing uses BLAKE3-256 for consistency and performance.
package fsmerkle

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"lukechampine.com/blake3"
)

// Hash represents a BLAKE3-256 hash value.
type Hash = [32]byte

// Kind represents the type of a filesystem node.
type Kind uint8

const (
	KindBlob Kind = 1 // Regular file content
	KindTree Kind = 2 // Directory
)

// String returns a human-readable representation of the Kind.
func (k Kind) String() string {
	switch k {
	case KindBlob:
		return "blob"
	case KindTree:
		return "tree"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// BlobNode represents a file with content stored separately in the CAS.
// The actual content is fetched from the CAS using the node's hash.
type BlobNode struct {
	Size int // Size of the file content in bytes
}

// Entry represents a single entry in a directory tree.
// Entries are sorted lexicographically by Name and must be unique.
type Entry struct {
	Name string // UTF-8 filename, following POSIX rules
	Mode uint32 // File permissions (0100644 for regular files, 040000 for directories)
	Kind Kind   // Type of the referenced node (blob or tree)
	Hash Hash   // BLAKE3 hash of the child node
}

// TreeNode represents a directory containing sorted, unique entries.
// Entries are maintained in lexicographic order for canonical representation.
type TreeNode struct {
	Entries []Entry // Lexicographically sorted by Name, no duplicates
}

// Canonical encodings for hashing and CAS storage:
//
// Blob canonical bytes:
//   header := "blob <len>\x00" (ASCII)
//   content := raw file bytes
//   BlobHash = BLAKE3(header || content)
//
// Tree canonical bytes:
//   uvarint(entry_count)
//   for each entry in sorted order:
//     uvarint(mode)
//     uvarint(len(name))
//     name bytes (no NUL terminator)
//     1 byte kind
//     32 bytes child hash
//   TreeHash = BLAKE3(tree_canonical_bytes)

// validateName checks if a filename is valid according to POSIX rules.
// Rejects empty names, ".", "..", and names containing path separators.
func validateName(name string) error {
	if name == "" {
		return errors.New("empty filename")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid filename: %q", name)
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("filename cannot contain path separator: %q", name)
	}
	return nil
}

// validateMode checks if a file mode is valid.
// Currently supports only regular files (0100644) and directories (040000).
func validateMode(mode uint32, kind Kind) error {
	switch kind {
	case KindBlob:
		if mode != 0100644 {
			return fmt.Errorf("invalid mode %o for blob, expected 0100644", mode)
		}
	case KindTree:
		if mode != 040000 {
			return fmt.Errorf("invalid mode %o for tree, expected 040000", mode)
		}
	default:
		return fmt.Errorf("unknown kind: %d", kind)
	}
	return nil
}

// CanonicalBytes returns the canonical byte representation of a BlobNode.
// Format: "blob <size>\x00"
func (b *BlobNode) CanonicalBytes() []byte {
	header := fmt.Sprintf("blob %d\x00", b.Size)
	return []byte(header)
}

// Hash computes the BLAKE3 hash of the blob's canonical representation plus content.
func (b *BlobNode) Hash(content []byte) Hash {
	if len(content) != b.Size {
		panic(fmt.Sprintf("content size mismatch: expected %d, got %d", b.Size, len(content)))
	}
	
	var buf bytes.Buffer
	buf.Write(b.CanonicalBytes())
	buf.Write(content)
	
	return blake3.Sum256(buf.Bytes())
}

// CanonicalBytes returns the canonical byte representation of a TreeNode.
// Format: uvarint(count) + entries in sorted order
func (t *TreeNode) CanonicalBytes() ([]byte, error) {
	if err := t.validate(); err != nil {
		return nil, fmt.Errorf("invalid tree: %w", err)
	}
	
	var buf bytes.Buffer
	
	// Write entry count
	count := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(count, uint64(len(t.Entries)))
	buf.Write(count[:n])
	
	// Write each entry
	for _, entry := range t.Entries {
		// Write mode
		mode := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(mode, uint64(entry.Mode))
		buf.Write(mode[:n])
		
		// Write name length and name
		nameLen := make([]byte, binary.MaxVarintLen64)
		n = binary.PutUvarint(nameLen, uint64(len(entry.Name)))
		buf.Write(nameLen[:n])
		buf.WriteString(entry.Name)
		
		// Write kind
		buf.WriteByte(byte(entry.Kind))
		
		// Write hash
		buf.Write(entry.Hash[:])
	}
	
	return buf.Bytes(), nil
}

// Hash computes the BLAKE3 hash of the tree's canonical representation.
func (t *TreeNode) Hash() (Hash, error) {
	canonical, err := t.CanonicalBytes()
	if err != nil {
		return Hash{}, err
	}
	return blake3.Sum256(canonical), nil
}

// validate checks that the TreeNode is well-formed.
func (t *TreeNode) validate() error {
	names := make(map[string]bool, len(t.Entries))
	
	for i, entry := range t.Entries {
		// Validate name
		if err := validateName(entry.Name); err != nil {
			return fmt.Errorf("entry %d: %w", i, err)
		}
		
		// Check for duplicates
		if names[entry.Name] {
			return fmt.Errorf("duplicate name: %q", entry.Name)
		}
		names[entry.Name] = true
		
		// Validate mode
		if err := validateMode(entry.Mode, entry.Kind); err != nil {
			return fmt.Errorf("entry %d (%s): %w", i, entry.Name, err)
		}
		
		// Check sorting (entries must be in lexicographic order)
		if i > 0 && entry.Name <= t.Entries[i-1].Name {
			return fmt.Errorf("entries not sorted: %q should come before %q", t.Entries[i-1].Name, entry.Name)
		}
	}
	
	return nil
}

// SortEntries sorts the entries in lexicographic order by name.
// This is called automatically by Builder implementations.
func (t *TreeNode) SortEntries() {
	sort.Slice(t.Entries, func(i, j int) bool {
		return t.Entries[i].Name < t.Entries[j].Name
	})
}

// FindEntry finds an entry by name, returning the entry and true if found.
func (t *TreeNode) FindEntry(name string) (Entry, bool) {
	for _, entry := range t.Entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return Entry{}, false
}

// splitPath splits a POSIX path into directory and filename components.
// Returns ("", filename) for paths with no directory separator.
func splitPath(filepath string) (dir, name string) {
	filepath = path.Clean(filepath)
	if filepath == "." {
		return "", ""
	}
	
	dir, name = path.Split(filepath)
	if dir != "" && dir != "/" {
		dir = strings.TrimSuffix(dir, "/")
	}
	if dir == "/" {
		dir = ""
	}
	
	return dir, name
}