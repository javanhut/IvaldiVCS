package history

import (
	"fmt"

	"lukechampine.com/blake3"
)

// MMR hashing rules:
// - Leaf node hash = BLAKE3(0x00 || LeafHash)
// - Internal node hash = BLAKE3(0x01 || LeftChildHash || RightChildHash)

const (
	leafPrefix     = 0x00
	internalPrefix = 0x01
)

// Proof represents an inclusion proof for a leaf in the MMR.
type Proof struct {
	LeafIndex uint64   // Index of the leaf being proven
	Siblings  []Hash   // Sibling hashes needed for verification
	Peaks     []Hash   // Current peak hashes
}

// Accumulator provides append-only Merkle accumulator functionality.
type Accumulator interface {
	// AppendLeaf appends a leaf to the MMR and returns its index and new root.
	AppendLeaf(l Leaf) (leafIdx uint64, root Hash, err error)
	
	// Root returns the current MMR root hash.
	Root() Hash
	
	// GetLeaf retrieves a leaf by its index.
	GetLeaf(idx uint64) (Leaf, error)
	
	// Size returns the number of leaves in the MMR.
	Size() uint64
	
	// Proof generates an inclusion proof for a leaf at the given index.
	Proof(idx uint64) (Proof, error)
	
	// Verify verifies an inclusion proof against a root hash.
	Verify(leafHash Hash, proof Proof, root Hash) bool
}

// MMR implements the Accumulator interface using a Merkle Mountain Range.
// This is an append-only structure that maintains peaks and efficiently
// computes roots as new leaves are added.
type MMR struct {
	leaves []Leaf           // All leaves in order
	nodes  map[uint64]Hash  // Internal node hashes by position
	peaks  []uint64         // Positions of current peaks
}

// NewMMR creates a new empty MMR accumulator.
func NewMMR() *MMR {
	return &MMR{
		leaves: make([]Leaf, 0),
		nodes:  make(map[uint64]Hash),
		peaks:  make([]uint64, 0),
	}
}

// AppendLeaf implements Accumulator.AppendLeaf.
func (m *MMR) AppendLeaf(l Leaf) (uint64, Hash, error) {
	leafIdx := uint64(len(m.leaves))
	m.leaves = append(m.leaves, l)
	
	// Compute leaf hash with prefix
	leafHash := computeLeafHash(l.Hash())
	
	// Add leaf to MMR structure - use proper MMR position
	pos := m.leafIndexToPos(leafIdx)
	m.nodes[pos] = leafHash
	
	// Update peaks by merging where possible
	m.updatePeaks(pos)
	
	// Recompute root
	root := m.computeRoot()
	
	return leafIdx, root, nil
}

// Root implements Accumulator.Root.
func (m *MMR) Root() Hash {
	return m.computeRoot()
}

// GetLeaf implements Accumulator.GetLeaf.
func (m *MMR) GetLeaf(idx uint64) (Leaf, error) {
	if idx >= uint64(len(m.leaves)) {
		return Leaf{}, fmt.Errorf("leaf index %d out of range (size: %d)", idx, len(m.leaves))
	}
	return m.leaves[idx], nil
}

// Size implements Accumulator.Size.
func (m *MMR) Size() uint64 {
	return uint64(len(m.leaves))
}

// Proof implements Accumulator.Proof.
func (m *MMR) Proof(idx uint64) (Proof, error) {
	if idx >= uint64(len(m.leaves)) {
		return Proof{}, fmt.Errorf("leaf index %d out of range", idx)
	}
	
	proof := Proof{
		LeafIndex: idx,
		Siblings:  make([]Hash, 0),
		Peaks:     make([]Hash, len(m.peaks)),
	}
	
	// Copy current peaks
	for i, peakPos := range m.peaks {
		proof.Peaks[i] = m.nodes[peakPos]
	}
	
	// Collect sibling hashes along the path to a peak
	pos := m.leafIndexToPos(idx) // Convert leaf index to MMR position
	
	// If this position is already a peak, no siblings needed
	if m.isPeak(pos) {
		return proof, nil
	}
	
	for {
		siblingPos := m.getSibling(pos)

		// Check if sibling exists in nodes map
		siblingHash, exists := m.nodes[siblingPos]
		if !exists {
			break
		}
		proof.Siblings = append(proof.Siblings, siblingHash)

		pos = m.getParent(pos)

		if m.isPeak(pos) {
			break
		}
	}
	
	return proof, nil
}

// Verify implements Accumulator.Verify.
func (m *MMR) Verify(leafHash Hash, proof Proof, root Hash) bool {
	// Start with the leaf hash (already with prefix)
	currentHash := computeLeafHash(leafHash)
	
	// Climb up using siblings
	pos := m.leafIndexToPos(proof.LeafIndex)
	for _, sibling := range proof.Siblings {
		if m.isLeftChild(pos) {
			currentHash = computeInternalHash(currentHash, sibling)
		} else {
			currentHash = computeInternalHash(sibling, currentHash)
		}
		pos = m.getParent(pos)
	}
	
	// The final hash should be one of the peaks
	for _, peak := range proof.Peaks {
		if currentHash == peak {
			// Verify that these peaks produce the claimed root
			return computeRootFromPeaks(proof.Peaks) == root
		}
	}
	
	return false
}

// updatePeaks maintains the list of peak positions after adding a new leaf.
func (m *MMR) updatePeaks(newPos uint64) {
	m.peaks = append(m.peaks, newPos)
	
	// Merge adjacent peaks of the same height
	for len(m.peaks) >= 2 {
		last := len(m.peaks) - 1
		secondLast := last - 1
		
		lastHeight := m.getHeight(m.peaks[last])
		secondLastHeight := m.getHeight(m.peaks[secondLast])
		
		if lastHeight == secondLastHeight {
			// Merge these peaks
			leftPos := m.peaks[secondLast]
			rightPos := m.peaks[last]
			
			leftHash := m.nodes[leftPos]
			rightHash := m.nodes[rightPos]
			
			// Create parent node
			parentPos := m.getParent(rightPos)
			parentHash := computeInternalHash(leftHash, rightHash)
			m.nodes[parentPos] = parentHash
			
			// Replace the two peaks with the parent
			m.peaks = m.peaks[:secondLast]
			m.peaks = append(m.peaks, parentPos)
		} else {
			break
		}
	}
}

// computeRoot computes the MMR root from current peaks.
func (m *MMR) computeRoot() Hash {
	if len(m.peaks) == 0 {
		return Hash{} // Empty MMR
	}
	
	peakHashes := make([]Hash, len(m.peaks))
	for i, pos := range m.peaks {
		peakHashes[i] = m.nodes[pos]
	}
	
	return computeRootFromPeaks(peakHashes)
}

// computeRootFromPeaks computes a root hash from peak hashes.
func computeRootFromPeaks(peaks []Hash) Hash {
	if len(peaks) == 0 {
		return Hash{}
	}
	if len(peaks) == 1 {
		return peaks[0]
	}
	
	// Combine peaks from right to left
	result := peaks[len(peaks)-1]
	for i := len(peaks) - 2; i >= 0; i-- {
		result = computeInternalHash(peaks[i], result)
	}
	
	return result
}

// Helper functions for MMR position calculations
//
// Standard 0-indexed MMR layout (post-order binary tree):
//
//              14 (h=3)
//           /         \
//      6 (h=2)       13 (h=2)
//      / \           / \
//   2(h1) 5(h1)  9(h1) 12(h1)
//   /\    /\     /\    /\
//  0  1  3  4   7  8  10 11
//
// Leaf positions: 0, 1, 3, 4, 7, 8, 10, 11, 15, 16, ...
// leafIndexToPos(i) = 2*i - popcount(i)

// leafIndexToPos converts a leaf index to its MMR position.
func (m *MMR) leafIndexToPos(leafIdx uint64) uint64 {
	return 2*leafIdx - popcount(leafIdx)
}

// popcount returns the number of set bits in x.
func popcount(x uint64) uint64 {
	count := uint64(0)
	for x > 0 {
		count += x & 1
		x >>= 1
	}
	return count
}

// getHeight returns the height of a node at the given MMR position.
// Uses the "peel off mountains" algorithm for post-order binary tree positions.
func (m *MMR) getHeight(pos uint64) uint64 {
	n := pos
	var h uint64
	for {
		treeSize := (uint64(1) << (h + 1)) - 1 // 2^(h+1) - 1
		if n < treeSize {
			if n == treeSize-1 {
				return h // At the root of a subtree of this height
			}
			// Recurse into left or right child subtree
			childSize := (treeSize - 1) / 2
			if n < childSize {
				h = 0 // In left subtree, restart height search
			} else {
				n -= childSize // In right subtree, offset position
				h = 0
			}
			continue
		}
		h++
	}
}

// getSibling returns the sibling position of the given position.
func (m *MMR) getSibling(pos uint64) uint64 {
	h := m.getHeight(pos)
	subtreeSize := (uint64(1) << (h + 1)) - 1

	// Right child test: if pos+1 has height h+1, we are a right child
	if m.getHeight(pos+1) == h+1 {
		// Right child — sibling is to the left
		return pos - subtreeSize
	}
	// Left child — sibling is to the right
	return pos + subtreeSize
}

// getParent returns the parent position of the given position.
func (m *MMR) getParent(pos uint64) uint64 {
	h := m.getHeight(pos)
	subtreeSize := (uint64(1) << (h + 1)) - 1

	// Right child test: if pos+1 has height h+1, parent is pos+1
	if m.getHeight(pos+1) == h+1 {
		return pos + 1
	}
	// Left child — parent is pos + subtreeSize + 1
	return pos + subtreeSize + 1
}

// isLeftChild returns true if the position is a left child.
func (m *MMR) isLeftChild(pos uint64) bool {
	h := m.getHeight(pos)
	// Right child: parent is at pos+1 with height h+1
	return m.getHeight(pos+1) != h+1
}

// isPeak returns true if the position is currently a peak.
func (m *MMR) isPeak(pos uint64) bool {
	for _, peak := range m.peaks {
		if peak == pos {
			return true
		}
	}
	return false
}

// Hash computation functions

// computeLeafHash computes the hash of a leaf node with the leaf prefix.
func computeLeafHash(leafHash Hash) Hash {
	data := make([]byte, 1+32)
	data[0] = leafPrefix
	copy(data[1:], leafHash[:])
	return blake3.Sum256(data)
}

// computeInternalHash computes the hash of an internal node with the internal prefix.
func computeInternalHash(left, right Hash) Hash {
	data := make([]byte, 1+32+32)
	data[0] = internalPrefix
	copy(data[1:33], left[:])
	copy(data[33:65], right[:])
	return blake3.Sum256(data)
}