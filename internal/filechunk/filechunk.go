// Package filechunk implements chunked Merkle trees for efficient file storage.
//
// Files are represented as Merkle trees where:
// - Leaves contain fixed-size chunks of file data
// - Internal nodes contain hashes of their children
// - Root hash uniquely identifies the entire file content
//
// Canonical Encoding:
// - Leaf: 0x00 | uvarint(len(chunk)) | chunk
// - Internal: 0x01 | uvarint(childCount) | childHash[32] * childCount | uvarint(totalSize)
// - Hash: BLAKE3(canonicalBytes)
package filechunk

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
)

// Params defines chunking parameters.
type Params struct {
	LeafSize int // Size of leaf chunks in bytes
}

// DefaultParams returns sensible default parameters.
func DefaultParams() Params {
	return Params{
		LeafSize: 64 * 1024, // 64 KiB chunks
	}
}

// NodeKind represents the type of a Merkle tree node.
type NodeKind uint8

const (
	Leaf NodeKind = iota + 1
	Node
)

// NodeRef represents a reference to a node in the Merkle tree.
type NodeRef struct {
	Hash cas.Hash // BLAKE3 hash of the node
	Kind NodeKind  // Leaf or internal Node
	Size int64     // Total bytes covered by this subtree
}

// Builder constructs chunked Merkle trees.
type Builder struct {
	CAS    cas.CAS
	Params Params
}

// NewBuilder creates a new Builder with the given CAS and parameters.
func NewBuilder(casStore cas.CAS, params Params) *Builder {
	return &Builder{
		CAS:    casStore,
		Params: params,
	}
}

// Build creates a Merkle tree from the given content.
func (b *Builder) Build(content []byte) (NodeRef, error) {
	if len(content) == 0 {
		// Handle empty file as a single empty leaf
		return b.buildLeaf(nil)
	}

	// Split content into chunks
	var chunks [][]byte
	for i := 0; i < len(content); i += b.Params.LeafSize {
		end := i + b.Params.LeafSize
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, content[i:end])
	}

	return b.buildTree(chunks)
}

// BuildStreaming creates a Merkle tree from streaming input.
func (b *Builder) BuildStreaming(r io.Reader) (NodeRef, error) {
	var chunks [][]byte
	buf := make([]byte, b.Params.LeafSize)

	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			chunks = append(chunks, chunk)
		}

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return NodeRef{}, fmt.Errorf("read error: %w", err)
		}
	}

	if len(chunks) == 0 {
		// Handle empty stream
		return b.buildLeaf(nil)
	}

	return b.buildTree(chunks)
}

// buildTree constructs a Merkle tree from leaf chunks.
func (b *Builder) buildTree(chunks [][]byte) (NodeRef, error) {
	if len(chunks) == 0 {
		return NodeRef{}, fmt.Errorf("no chunks to build tree")
	}

	if len(chunks) == 1 {
		// Single chunk becomes a leaf
		return b.buildLeaf(chunks[0])
	}

	// Build leaf nodes
	var nodes []NodeRef
	for _, chunk := range chunks {
		leaf, err := b.buildLeaf(chunk)
		if err != nil {
			return NodeRef{}, err
		}
		nodes = append(nodes, leaf)
	}

	// Build internal nodes bottom-up
	for len(nodes) > 1 {
		var nextLevel []NodeRef
		
		// Group nodes and create internal nodes
		for i := 0; i < len(nodes); i += 2 {
			if i+1 < len(nodes) {
				// Two children
				internal, err := b.buildInternal([]NodeRef{nodes[i], nodes[i+1]})
				if err != nil {
					return NodeRef{}, err
				}
				nextLevel = append(nextLevel, internal)
			} else {
				// Odd child, promote to next level
				nextLevel = append(nextLevel, nodes[i])
			}
		}
		nodes = nextLevel
	}

	return nodes[0], nil
}

// buildLeaf creates a leaf node from chunk data.
func (b *Builder) buildLeaf(chunk []byte) (NodeRef, error) {
	canonical := b.encodeLeaf(chunk)
	hash := cas.SumB3(canonical)

	err := b.CAS.Put(hash, canonical)
	if err != nil {
		return NodeRef{}, fmt.Errorf("failed to store leaf: %w", err)
	}

	return NodeRef{
		Hash: hash,
		Kind: Leaf,
		Size: int64(len(chunk)),
	}, nil
}

// buildInternal creates an internal node from child nodes.
func (b *Builder) buildInternal(children []NodeRef) (NodeRef, error) {
	canonical, totalSize := b.encodeInternal(children)
	hash := cas.SumB3(canonical)

	err := b.CAS.Put(hash, canonical)
	if err != nil {
		return NodeRef{}, fmt.Errorf("failed to store internal node: %w", err)
	}

	return NodeRef{
		Hash: hash,
		Kind: Node,
		Size: totalSize,
	}, nil
}

// encodeLeaf creates canonical encoding for a leaf node.
func (b *Builder) encodeLeaf(chunk []byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte(0x00) // Leaf marker
	
	// Write chunk length as uvarint
	lenBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(lenBuf, uint64(len(chunk)))
	buf.Write(lenBuf[:n])
	
	// Write chunk data
	buf.Write(chunk)
	
	return buf.Bytes()
}

// encodeInternal creates canonical encoding for an internal node.
func (b *Builder) encodeInternal(children []NodeRef) ([]byte, int64) {
	var buf bytes.Buffer
	var totalSize int64
	
	buf.WriteByte(0x01) // Internal node marker
	
	// Write child count as uvarint
	lenBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(lenBuf, uint64(len(children)))
	buf.Write(lenBuf[:n])
	
	// Write child hashes
	for _, child := range children {
		buf.Write(child.Hash[:])
		totalSize += child.Size
	}
	
	// Write total size as uvarint
	n = binary.PutUvarint(lenBuf, uint64(totalSize))
	buf.Write(lenBuf[:n])
	
	return buf.Bytes(), totalSize
}

// Loader reads chunked Merkle trees.
type Loader struct {
	CAS cas.CAS
}

// NewLoader creates a new Loader with the given CAS.
func NewLoader(casStore cas.CAS) *Loader {
	return &Loader{CAS: casStore}
}

// ReadAll reads the entire content of a file tree.
func (l *Loader) ReadAll(root NodeRef) ([]byte, error) {
	if root.Size == 0 {
		return nil, nil
	}

	var result bytes.Buffer
	err := l.readNode(root, &result)
	if err != nil {
		return nil, err
	}

	return result.Bytes(), nil
}

// Reader returns a streaming reader for a file tree.
func (l *Loader) Reader(root NodeRef) (io.ReadCloser, error) {
	data, err := l.ReadAll(root)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// readNode recursively reads node content.
func (l *Loader) readNode(node NodeRef, w io.Writer) error {
	data, err := l.CAS.Get(node.Hash)
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", node.Hash, err)
	}

	if node.Kind == Leaf {
		return l.readLeaf(data, w)
	}
	return l.readInternal(data, w)
}

// readLeaf reads content from a leaf node.
func (l *Loader) readLeaf(data []byte, w io.Writer) error {
	if len(data) == 0 || data[0] != 0x00 {
		return fmt.Errorf("invalid leaf node encoding")
	}

	buf := bytes.NewReader(data[1:])
	
	// Read chunk length
	chunkLen, err := binary.ReadUvarint(buf)
	if err != nil {
		return fmt.Errorf("failed to read chunk length: %w", err)
	}

	// Read chunk data
	chunk := make([]byte, chunkLen)
	n, err := buf.Read(chunk)
	if err != nil || uint64(n) != chunkLen {
		return fmt.Errorf("failed to read chunk data: expected %d, got %d", chunkLen, n)
	}

	_, err = w.Write(chunk)
	return err
}

// readInternal reads content from an internal node.
func (l *Loader) readInternal(data []byte, w io.Writer) error {
	if len(data) == 0 || data[0] != 0x01 {
		return fmt.Errorf("invalid internal node encoding")
	}

	buf := bytes.NewReader(data[1:])
	
	// Read child count
	childCount, err := binary.ReadUvarint(buf)
	if err != nil {
		return fmt.Errorf("failed to read child count: %w", err)
	}

	// Read child hashes
	children := make([]cas.Hash, childCount)
	for i := uint64(0); i < childCount; i++ {
		n, err := buf.Read(children[i][:])
		if err != nil || n != 32 {
			return fmt.Errorf("failed to read child hash %d", i)
		}
	}

	// Read total size (for validation)
	_, err = binary.ReadUvarint(buf)
	if err != nil {
		return fmt.Errorf("failed to read total size: %w", err)
	}

	// Recursively read children
	for _, childHash := range children {
		// We need to determine the child's kind and size
		// For simplicity, we'll peek at the stored data
		childData, err := l.CAS.Get(childHash)
		if err != nil {
			return fmt.Errorf("failed to get child %s: %w", childHash, err)
		}

		var childNode NodeRef
		childNode.Hash = childHash
		
		if len(childData) > 0 && childData[0] == 0x00 {
			childNode.Kind = Leaf
		} else if len(childData) > 0 && childData[0] == 0x01 {
			childNode.Kind = Node
		} else {
			return fmt.Errorf("invalid child node encoding")
		}

		err = l.readNode(childNode, w)
		if err != nil {
			return err
		}
	}

	return nil
}