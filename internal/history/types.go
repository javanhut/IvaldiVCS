// Package history implements an append-only Merkle accumulator (MMR-style) for tracking
// timeline history. Each change (commit) is a Leaf referencing a filesystem tree root.
// Timelines are lightweight labels pointing to leaf indices.
//
// The package provides:
// - Leaf structure for commit records with stable canonical encoding
// - MMR (Merkle Mountain Range) accumulator for append-only history
// - Timeline management with LCA computation using binary lifting
// - Three-way merge support scaffolding
package history

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/javanhut/Ivaldi-vcs/internal/fsmerkle"
	"lukechampine.com/blake3"
)

// Hash is a BLAKE3-256 hash value, compatible with fsmerkle.
type Hash = fsmerkle.Hash

// Leaf represents a commit record in the history.
// Contains all information needed to reconstruct the commit state and lineage.
type Leaf struct {
	TreeRoot   Hash              // BLAKE3 root of directory tree (from fsmerkle)
	TimelineID string            // Timeline label (logical stream/branch name)
	PrevIdx    uint64            // Previous leaf index on this timeline; ^uint64(0) if none
	MergeIdxs  []uint64          // Additional parent indices for merges (usually empty or 1 entry)
	Author     string            // Commit author
	TimeUnix   int64             // Commit timestamp (Unix time)
	Message    string            // Commit message
	Meta       map[string]string // Additional metadata (e.g., "autoshelved": "1")
}

// NoParent represents the absence of a parent commit.
const NoParent = ^uint64(0)

// CanonicalBytes returns the stable byte encoding of the Leaf.
// This encoding is used for hashing and must be deterministic.
//
// Canonical encoding format (version 1):
//   uvarint(1)                    // version
//   32 bytes TreeRoot             // filesystem tree hash
//   uvarint(len(TimelineID))      // timeline ID length
//   bytes(TimelineID)             // timeline ID string
//   uvarint(PrevIdx)              // previous index (NoParent if none)
//   uvarint(len(MergeIdxs))       // number of merge parents
//   repeat len(MergeIdxs):
//     uvarint(index)              // merge parent index
//   uvarint(len(Author))          // author string length
//   bytes(Author)                 // author string
//   varint(TimeUnix)              // timestamp (signed)
//   uvarint(len(Message))         // message length
//   bytes(Message)                // message string
//   uvarint(len(Meta))            // metadata map size
//   repeat len(Meta):             // entries sorted by key
//     uvarint(len(key))           // key length
//     bytes(key)                  // key string
//     uvarint(len(value))         // value length
//     bytes(value)                // value string
func (l *Leaf) CanonicalBytes() []byte {
	var buf bytes.Buffer
	
	// Version
	version := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(version, 1)
	buf.Write(version[:n])
	
	// TreeRoot
	buf.Write(l.TreeRoot[:])
	
	// TimelineID
	timelineIDLen := make([]byte, binary.MaxVarintLen64)
	n = binary.PutUvarint(timelineIDLen, uint64(len(l.TimelineID)))
	buf.Write(timelineIDLen[:n])
	buf.WriteString(l.TimelineID)
	
	// PrevIdx
	prevIdx := make([]byte, binary.MaxVarintLen64)
	n = binary.PutUvarint(prevIdx, l.PrevIdx)
	buf.Write(prevIdx[:n])
	
	// MergeIdxs
	mergeCount := make([]byte, binary.MaxVarintLen64)
	n = binary.PutUvarint(mergeCount, uint64(len(l.MergeIdxs)))
	buf.Write(mergeCount[:n])
	
	for _, idx := range l.MergeIdxs {
		idxBytes := make([]byte, binary.MaxVarintLen64)
		n = binary.PutUvarint(idxBytes, idx)
		buf.Write(idxBytes[:n])
	}
	
	// Author
	authorLen := make([]byte, binary.MaxVarintLen64)
	n = binary.PutUvarint(authorLen, uint64(len(l.Author)))
	buf.Write(authorLen[:n])
	buf.WriteString(l.Author)
	
	// TimeUnix (signed varint)
	timeBytes := make([]byte, binary.MaxVarintLen64)
	n = binary.PutVarint(timeBytes, l.TimeUnix)
	buf.Write(timeBytes[:n])
	
	// Message
	messageLen := make([]byte, binary.MaxVarintLen64)
	n = binary.PutUvarint(messageLen, uint64(len(l.Message)))
	buf.Write(messageLen[:n])
	buf.WriteString(l.Message)
	
	// Meta (sorted by key for determinism)
	metaLen := make([]byte, binary.MaxVarintLen64)
	n = binary.PutUvarint(metaLen, uint64(len(l.Meta)))
	buf.Write(metaLen[:n])
	
	// Sort keys for deterministic encoding
	keys := make([]string, 0, len(l.Meta))
	for k := range l.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	for _, key := range keys {
		value := l.Meta[key]
		
		// Key
		keyLen := make([]byte, binary.MaxVarintLen64)
		n = binary.PutUvarint(keyLen, uint64(len(key)))
		buf.Write(keyLen[:n])
		buf.WriteString(key)
		
		// Value
		valueLen := make([]byte, binary.MaxVarintLen64)
		n = binary.PutUvarint(valueLen, uint64(len(value)))
		buf.Write(valueLen[:n])
		buf.WriteString(value)
	}
	
	return buf.Bytes()
}

// Hash computes the BLAKE3 hash of the leaf's canonical representation.
func (l *Leaf) Hash() Hash {
	return blake3.Sum256(l.CanonicalBytes())
}

// HasParent returns true if this leaf has a previous commit on its timeline.
func (l *Leaf) HasParent() bool {
	return l.PrevIdx != NoParent
}

// IsMerge returns true if this leaf has merge parents (is a merge commit).
func (l *Leaf) IsMerge() bool {
	return len(l.MergeIdxs) > 0
}

// AllParents returns all parent indices (previous + merges) for this leaf.
func (l *Leaf) AllParents() []uint64 {
	parents := make([]uint64, 0, 1+len(l.MergeIdxs))
	
	if l.HasParent() {
		parents = append(parents, l.PrevIdx)
	}
	
	parents = append(parents, l.MergeIdxs...)
	return parents
}

// IsAutoshelved returns true if this leaf is marked as autoshelved.
func (l *Leaf) IsAutoshelved() bool {
	return l.Meta["autoshelved"] == "1"
}

// SetAutoshelved marks this leaf as autoshelved or not.
func (l *Leaf) SetAutoshelved(autoshelved bool) {
	if l.Meta == nil {
		l.Meta = make(map[string]string)
	}
	
	if autoshelved {
		l.Meta["autoshelved"] = "1"
	} else {
		delete(l.Meta, "autoshelved")
	}
}

// parseLeafCanonical parses canonical bytes back into a Leaf structure.
func parseLeafCanonical(canonical []byte) (*Leaf, error) {
	buf := bytes.NewReader(canonical)
	
	// Version
	version, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}
	
	leaf := &Leaf{}
	
	// TreeRoot
	if n, err := buf.Read(leaf.TreeRoot[:]); err != nil || n != 32 {
		return nil, fmt.Errorf("failed to read tree root: %w", err)
	}
	
	// TimelineID
	timelineIDLen, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read timeline ID length: %w", err)
	}
	timelineIDBytes := make([]byte, timelineIDLen)
	if n, err := buf.Read(timelineIDBytes); err != nil || uint64(n) != timelineIDLen {
		return nil, fmt.Errorf("failed to read timeline ID: %w", err)
	}
	leaf.TimelineID = string(timelineIDBytes)
	
	// PrevIdx
	leaf.PrevIdx, err = binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read prev index: %w", err)
	}
	
	// MergeIdxs
	mergeCount, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read merge count: %w", err)
	}
	leaf.MergeIdxs = make([]uint64, mergeCount)
	for i := uint64(0); i < mergeCount; i++ {
		leaf.MergeIdxs[i], err = binary.ReadUvarint(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read merge index %d: %w", i, err)
		}
	}
	
	// Author
	authorLen, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read author length: %w", err)
	}
	authorBytes := make([]byte, authorLen)
	if n, err := buf.Read(authorBytes); err != nil || uint64(n) != authorLen {
		return nil, fmt.Errorf("failed to read author: %w", err)
	}
	leaf.Author = string(authorBytes)
	
	// TimeUnix
	leaf.TimeUnix, err = binary.ReadVarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read time: %w", err)
	}
	
	// Message
	messageLen, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}
	messageBytes := make([]byte, messageLen)
	if n, err := buf.Read(messageBytes); err != nil || uint64(n) != messageLen {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}
	leaf.Message = string(messageBytes)
	
	// Meta
	metaCount, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read meta count: %w", err)
	}
	
	if metaCount > 0 {
		leaf.Meta = make(map[string]string)
		for i := uint64(0); i < metaCount; i++ {
			// Key
			keyLen, err := binary.ReadUvarint(buf)
			if err != nil {
				return nil, fmt.Errorf("failed to read meta key length %d: %w", i, err)
			}
			keyBytes := make([]byte, keyLen)
			if n, err := buf.Read(keyBytes); err != nil || uint64(n) != keyLen {
				return nil, fmt.Errorf("failed to read meta key %d: %w", i, err)
			}
			key := string(keyBytes)
			
			// Value
			valueLen, err := binary.ReadUvarint(buf)
			if err != nil {
				return nil, fmt.Errorf("failed to read meta value length %d: %w", i, err)
			}
			valueBytes := make([]byte, valueLen)
			if n, err := buf.Read(valueBytes); err != nil || uint64(n) != valueLen {
				return nil, fmt.Errorf("failed to read meta value %d: %w", i, err)
			}
			value := string(valueBytes)
			
			leaf.Meta[key] = value
		}
	}
	
	// Verify no extra data
	if buf.Len() > 0 {
		return nil, fmt.Errorf("unexpected extra data after leaf")
	}
	
	return leaf, nil
}