package fsmerkle

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	
	"lukechampine.com/blake3"
)

// CAS (Content Addressable Store) provides persistent storage for tree nodes.
// Raw canonical bytes are stored keyed by their BLAKE3 hash.
type CAS interface {
	// Put stores raw canonical bytes keyed by their BLAKE3 hash.
	Put(hash Hash, raw []byte) error
	
	// Get retrieves raw canonical bytes by hash.
	Get(hash Hash) ([]byte, error)
	
	// Has checks if content exists for the given hash.
	Has(hash Hash) (bool, error)
}

// Builder creates and stores tree nodes in the CAS.
type Builder interface {
	// PutBlob creates a blob from content, stores canonical bytes in CAS, returns hash.
	PutBlob(content []byte) (Hash, int, error)
	
	// PutTree builds a TreeNode from entries, canonicalizes, hashes, and stores.
	PutTree(entries []Entry) (Hash, error)
}

// Loader retrieves and parses tree nodes from the CAS.
type Loader interface {
	// LoadTree parses and loads a TreeNode from CAS by hash.
	LoadTree(hash Hash) (*TreeNode, error)
	
	// LoadBlob parses and loads a BlobNode from CAS by hash, returning both node and content.
	LoadBlob(hash Hash) (*BlobNode, []byte, error)
}

// MemoryCAS is an in-memory implementation of the CAS interface for testing.
type MemoryCAS struct {
	data map[Hash][]byte
}

// NewMemoryCAS creates a new in-memory CAS.
func NewMemoryCAS() *MemoryCAS {
	return &MemoryCAS{
		data: make(map[Hash][]byte),
	}
}

// Put implements CAS.Put.
func (m *MemoryCAS) Put(hash Hash, raw []byte) error {
	// Verify that the content actually hashes to the provided hash
	computed := blake3.Sum256(raw)
	if computed != hash {
		return fmt.Errorf("hash mismatch: expected %x, got %x", hash, computed)
	}
	
	m.data[hash] = make([]byte, len(raw))
	copy(m.data[hash], raw)
	return nil
}

// Get implements CAS.Get.
func (m *MemoryCAS) Get(hash Hash) ([]byte, error) {
	raw, exists := m.data[hash]
	if !exists {
		return nil, fmt.Errorf("hash not found: %x", hash)
	}
	
	result := make([]byte, len(raw))
	copy(result, raw)
	return result, nil
}

// Has implements CAS.Has.
func (m *MemoryCAS) Has(hash Hash) (bool, error) {
	_, exists := m.data[hash]
	return exists, nil
}

// Len returns the number of objects stored in the CAS.
func (m *MemoryCAS) Len() int {
	return len(m.data)
}

// Store combines Builder and Loader functionality with a CAS backend.
type Store struct {
	cas CAS
}

// NewStore creates a new Store with the given CAS backend.
func NewStore(cas CAS) *Store {
	return &Store{cas: cas}
}

// PutBlob implements Builder.PutBlob.
func (s *Store) PutBlob(content []byte) (Hash, int, error) {
	blob := &BlobNode{Size: len(content)}
	hash := blob.Hash(content)
	
	// Create canonical representation (header + content)
	var buf bytes.Buffer
	buf.Write(blob.CanonicalBytes())
	buf.Write(content)
	canonical := buf.Bytes()
	
	if err := s.cas.Put(hash, canonical); err != nil {
		return Hash{}, 0, fmt.Errorf("failed to store blob: %w", err)
	}
	
	return hash, blob.Size, nil
}

// PutTree implements Builder.PutTree.
func (s *Store) PutTree(entries []Entry) (Hash, error) {
	// Create and validate tree
	tree := &TreeNode{Entries: make([]Entry, len(entries))}
	copy(tree.Entries, entries)
	tree.SortEntries()
	
	// Generate canonical bytes and hash
	canonical, err := tree.CanonicalBytes()
	if err != nil {
		return Hash{}, fmt.Errorf("failed to canonicalize tree: %w", err)
	}
	
	hash := blake3.Sum256(canonical)
	
	if err := s.cas.Put(hash, canonical); err != nil {
		return Hash{}, fmt.Errorf("failed to store tree: %w", err)
	}
	
	return hash, nil
}

// LoadTree implements Loader.LoadTree.
func (s *Store) LoadTree(hash Hash) (*TreeNode, error) {
	canonical, err := s.cas.Get(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to load tree: %w", err)
	}
	
	return parseTreeCanonical(canonical)
}

// LoadBlob implements Loader.LoadBlob.
func (s *Store) LoadBlob(hash Hash) (*BlobNode, []byte, error) {
	canonical, err := s.cas.Get(hash)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load blob: %w", err)
	}
	
	return parseBlobCanonical(canonical)
}

// parseTreeCanonical parses canonical tree bytes back into a TreeNode.
func parseTreeCanonical(canonical []byte) (*TreeNode, error) {
	buf := bytes.NewReader(canonical)
	
	// Read entry count
	count, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read entry count: %w", err)
	}
	
	tree := &TreeNode{
		Entries: make([]Entry, count),
	}
	
	// Read each entry
	for i := uint64(0); i < count; i++ {
		entry := &tree.Entries[i]
		
		// Read mode
		mode, err := binary.ReadUvarint(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read mode for entry %d: %w", i, err)
		}
		entry.Mode = uint32(mode)
		
		// Read name length and name
		nameLen, err := binary.ReadUvarint(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read name length for entry %d: %w", i, err)
		}
		
		nameBytes := make([]byte, nameLen)
		if n, err := buf.Read(nameBytes); err != nil || uint64(n) != nameLen {
			return nil, fmt.Errorf("failed to read name for entry %d: %w", i, err)
		}
		entry.Name = string(nameBytes)
		
		// Read kind
		kindByte, err := buf.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read kind for entry %d: %w", i, err)
		}
		entry.Kind = Kind(kindByte)
		
		// Read hash
		if n, err := buf.Read(entry.Hash[:]); err != nil || n != 32 {
			return nil, fmt.Errorf("failed to read hash for entry %d: %w", i, err)
		}
	}
	
	// Verify there's no extra data
	if buf.Len() > 0 {
		return nil, fmt.Errorf("unexpected extra data after tree entries")
	}
	
	return tree, nil
}

// parseBlobCanonical parses canonical blob bytes back into a BlobNode and content.
func parseBlobCanonical(canonical []byte) (*BlobNode, []byte, error) {
	// Find the null terminator in the header
	nullIdx := bytes.IndexByte(canonical, 0)
	if nullIdx == -1 {
		return nil, nil, errors.New("invalid blob: no null terminator in header")
	}
	
	header := string(canonical[:nullIdx])
	content := canonical[nullIdx+1:]
	
	// Parse header: "blob <size>"
	var size int
	n, err := fmt.Sscanf(header, "blob %d", &size)
	if err != nil || n != 1 {
		return nil, nil, fmt.Errorf("invalid blob header %q: %w", header, err)
	}
	
	if len(content) != size {
		return nil, nil, fmt.Errorf("content size mismatch: header says %d, got %d bytes", size, len(content))
	}
	
	blob := &BlobNode{Size: size}
	
	// Make a copy of content to avoid sharing the underlying array
	contentCopy := make([]byte, len(content))
	copy(contentCopy, content)
	
	return blob, contentCopy, nil
}