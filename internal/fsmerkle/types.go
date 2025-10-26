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
	KindBlob      Kind = 1 // Regular file content
	KindTree      Kind = 2 // Directory
	KindSubmodule Kind = 3 // Submodule reference
)

// String returns a human-readable representation of the Kind.
func (k Kind) String() string {
	switch k {
	case KindBlob:
		return "blob"
	case KindTree:
		return "tree"
	case KindSubmodule:
		return "submodule"
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

// SubmoduleNode represents a reference to an external repository.
// Submodules are tracked by BLAKE3 commit hash internally, with optional
// Git SHA-1 mapping for GitHub compatibility.
type SubmoduleNode struct {
	URL        string // Repository URL (https, ssh, file)
	Path       string // Relative path in parent repository
	Timeline   string // Timeline name to track
	CommitHash Hash   // BLAKE3 hash of target commit
	Shallow    bool   // Shallow clone flag
	Freeze     bool   // Prevent automatic updates
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

// CanonicalBytes returns the canonical byte representation of a SubmoduleNode.
// Format (Version 1):
//   Version byte:     0x01
//   URL length:       uvarint(len(url))
//   URL bytes:        UTF-8 string
//   Path length:      uvarint(len(path))
//   Path bytes:       UTF-8 string
//   Timeline length:  uvarint(len(timeline))
//   Timeline bytes:   UTF-8 string
//   Commit hash:      32 bytes (BLAKE3)
//   Flags:            uvarint(flags)
//     bit 0: shallow
//     bit 1: freeze
//     bits 2-63: reserved
func (s *SubmoduleNode) CanonicalBytes() []byte {
	var buf bytes.Buffer
	
	buf.WriteByte(0x01)
	
	urlBytes := []byte(s.URL)
	lenBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(lenBuf, uint64(len(urlBytes)))
	buf.Write(lenBuf[:n])
	buf.Write(urlBytes)
	
	pathBytes := []byte(s.Path)
	n = binary.PutUvarint(lenBuf, uint64(len(pathBytes)))
	buf.Write(lenBuf[:n])
	buf.Write(pathBytes)
	
	timelineBytes := []byte(s.Timeline)
	n = binary.PutUvarint(lenBuf, uint64(len(timelineBytes)))
	buf.Write(lenBuf[:n])
	buf.Write(timelineBytes)
	
	buf.Write(s.CommitHash[:])
	
	var flags uint64
	if s.Shallow {
		flags |= 1
	}
	if s.Freeze {
		flags |= 2
	}
	n = binary.PutUvarint(lenBuf, flags)
	buf.Write(lenBuf[:n])
	
	return buf.Bytes()
}

// Hash computes the BLAKE3 hash of the submodule's canonical representation.
func (s *SubmoduleNode) Hash() Hash {
	return blake3.Sum256(s.CanonicalBytes())
}

// DecodeSubmoduleNode decodes a SubmoduleNode from its canonical bytes.
func DecodeSubmoduleNode(data []byte) (*SubmoduleNode, error) {
	buf := bytes.NewReader(data)
	
	version, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != 0x01 {
		return nil, fmt.Errorf("unsupported submodule version: %d", version)
	}
	
	readString := func() (string, error) {
		length, err := binary.ReadUvarint(buf)
		if err != nil {
			return "", err
		}
		strBytes := make([]byte, length)
		if _, err := buf.Read(strBytes); err != nil {
			return "", err
		}
		return string(strBytes), nil
	}
	
	url, err := readString()
	if err != nil {
		return nil, fmt.Errorf("read URL: %w", err)
	}
	
	pathStr, err := readString()
	if err != nil {
		return nil, fmt.Errorf("read path: %w", err)
	}
	
	timeline, err := readString()
	if err != nil {
		return nil, fmt.Errorf("read timeline: %w", err)
	}
	
	var commitHash Hash
	if _, err := buf.Read(commitHash[:]); err != nil {
		return nil, fmt.Errorf("read commit hash: %w", err)
	}
	
	flags, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("read flags: %w", err)
	}
	
	return &SubmoduleNode{
		URL:        url,
		Path:       pathStr,
		Timeline:   timeline,
		CommitHash: commitHash,
		Shallow:    (flags & 1) != 0,
		Freeze:     (flags & 2) != 0,
	}, nil
}
