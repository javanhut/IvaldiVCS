# history Package

The `history` package implements an append-only Merkle accumulator (MMR-style) for tracking timeline history. Each commit is represented as a **Leaf** that references a filesystem tree root and maintains lineage information.

## Overview

This package provides:
- **Leaf structure** for commit records with stable canonical encoding
- **MMR (Merkle Mountain Range)** accumulator for append-only history
- **Timeline management** with lightweight branch pointers
- **LCA computation** using binary lifting for efficient ancestry queries
- **Merge support** scaffolding for three-way merges

## Core Concepts

### Leaf (Commit Record)

Each commit is represented as a `Leaf` containing:
- `TreeRoot`: BLAKE3 hash of the filesystem tree (from fsmerkle)
- `TimelineID`: Timeline/branch name
- `PrevIdx`: Previous commit index on this timeline
- `MergeIdxs`: Additional parent indices for merge commits
- `Author`, `TimeUnix`, `Message`: Standard commit metadata
- `Meta`: Key-value pairs for flags like autoshelving

### MMR (Merkle Mountain Range)

An append-only structure that maintains:
- **Peaks**: Top-level nodes of complete binary trees
- **Efficient root computation**: O(log n) root updates
- **Inclusion proofs**: Verify any leaf belongs to a specific root
- **Structural sharing**: Unchanged portions require no recomputation

### Timeline Management

Timelines are lightweight pointers to leaf indices:
- **Branch creation**: O(1) fork by pointing to existing leaf
- **LCA computation**: Binary lifting for O(log n) ancestry queries
- **Cross-timeline ancestry**: Support for merge scenarios

## Key Types

```go
type Leaf struct {
    TreeRoot   Hash              // Filesystem tree root
    TimelineID string            // Branch/timeline name
    PrevIdx    uint64            // Previous commit (NoParent if first)
    MergeIdxs  []uint64          // Merge parent indices
    Author     string            // Commit author
    TimeUnix   int64             // Unix timestamp
    Message    string            // Commit message
    Meta       map[string]string // Metadata flags
}

type Accumulator interface {
    AppendLeaf(l Leaf) (leafIdx uint64, root Hash, err error)
    Root() Hash
    GetLeaf(idx uint64) (Leaf, error)
    Proof(idx uint64) (Proof, error)
    Verify(leafHash Hash, proof Proof, root Hash) bool
}

type Manager interface {
    Commit(timeline string, leaf Leaf) (idx uint64, root Hash, err error)
    GetTimelineHead(name string) (uint64, bool)
    SetTimelineHead(name string, idx uint64) error
    LCA(aIdx, bIdx uint64) (uint64, error)
}
```

## Canonical Encoding

Leaf canonical encoding (version 1):
```
uvarint(1)                    // version
32 bytes TreeRoot             // filesystem tree hash
uvarint(len(TimelineID))      // timeline ID length
bytes(TimelineID)             // timeline ID string
uvarint(PrevIdx)              // previous index (NoParent if none)
uvarint(len(MergeIdxs))       // number of merge parents
repeat len(MergeIdxs):
  uvarint(index)              // merge parent index
uvarint(len(Author))          // author string length
bytes(Author)                 // author string
varint(TimeUnix)              // timestamp (signed)
uvarint(len(Message))         // message length
bytes(Message)                // message string
uvarint(len(Meta))            // metadata map size
repeat len(Meta):             // entries sorted by key
  uvarint(len(key))           // key length
  bytes(key)                  // key string
  uvarint(len(value))         // value length
  bytes(value)                // value string
```

## MMR Hashing Rules

- **Leaf node hash**: `BLAKE3(0x00 || LeafHash)`
- **Internal node hash**: `BLAKE3(0x01 || LeftChildHash || RightChildHash)`

The `0x00`/`0x01` prefixes prevent collision attacks between leaf and internal node hashes.

## Usage Examples

### Basic Timeline Operations

```go
// Create manager
mmr := NewMMR()
timelineStore := NewMemoryTimelineStore()
manager := NewHistoryManager(mmr, timelineStore)

// Create initial commit
leaf := Leaf{
    TreeRoot:   rootHash,      // from fsmerkle
    TimelineID: "main",        // will be set automatically
    Author:     "Alice",
    TimeUnix:   time.Now().Unix(),
    Message:    "Initial commit",
}

idx, root, err := manager.Commit("main", leaf)
// Creates first commit on 'main' timeline

// Create branch
err = manager.SetTimelineHead("feature", idx)
// 'feature' now points to same commit as 'main'

// Commit to feature branch
featureLeaf := Leaf{
    TreeRoot: newRootHash,
    Author:   "Alice", 
    TimeUnix: time.Now().Unix(),
    Message:  "Add feature",
}

featureIdx, _, err := manager.Commit("feature", featureLeaf)
// Creates new commit on 'feature', with PrevIdx = idx
```

### LCA and Merge Preparation

```go
// Find common base for merge
mainHead, _ := manager.GetTimelineHead("main")
featureHead, _ := manager.GetTimelineHead("feature")

baseIdx, err := manager.LCA(mainHead, featureHead)
// Returns the commit where the branches diverged

// Prepare three-way merge context
baseLeaf, _ := manager.Accumulator().GetLeaf(baseIdx)
mainLeaf, _ := manager.Accumulator().GetLeaf(mainHead)  
featureLeaf, _ := manager.Accumulator().GetLeaf(featureHead)

// Use tree roots for three-way merge in fsmerkle
// mergedTree := fsmerkle.ThreeWayMerge(baseLeaf.TreeRoot, mainLeaf.TreeRoot, featureLeaf.TreeRoot)
```

### Inclusion Proofs

```go
// Generate proof that a commit exists in the history
proof, err := mmr.Proof(commitIdx)

// Verify the proof against a known root
leafHash := leaf.Hash()
isValid := mmr.Verify(leafHash, proof, knownRoot)
```

## Performance Characteristics

- **Append**: O(log n) - MMR peak updates
- **Root computation**: O(log n) - combine peaks  
- **LCA query**: O(log n) - binary lifting
- **Proof generation**: O(log n) - path to peak
- **Proof verification**: O(log n) - climb to root

## Timeline Semantics

- **Linear history**: Each commit has one parent via `PrevIdx`
- **Merge commits**: Additional parents via `MergeIdxs`
- **Branch creation**: Point new timeline to existing leaf index
- **Autoshelving**: Mark commits with `Meta["autoshelved"] = "1"`

## Thread Safety

The core structures are not thread-safe. Concurrent access requires external synchronization or immutable snapshots.