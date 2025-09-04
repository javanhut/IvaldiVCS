# fsmerkle Package

The `fsmerkle` package implements a Merkle DAG (Directed Acyclic Graph) for filesystem trees, providing efficient content-addressable storage and diff computation.

## Overview

This package represents filesystem trees as immutable Merkle structures where:
- **BlobNode** represents file content
- **TreeNode** represents directories with sorted entries
- All content is identified by BLAKE3-256 hashes
- Structural sharing enables efficient storage and comparison

## Key Features

- **Content Addressable Storage (CAS)**: Pluggable storage interface for persistence
- **Canonical Encodings**: Stable byte representations for consistent hashing
- **Structural Sharing**: Unchanged subtrees share the same hash
- **Efficient Diff**: Short-circuits on identical hashes, only examines changed portions
- **POSIX Compatibility**: Follows POSIX filesystem semantics

## Core Types

### Node Types

```go
type BlobNode struct {
    Size int  // File size in bytes
}

type TreeNode struct {
    Entries []Entry  // Sorted directory entries
}

type Entry struct {
    Name string   // Filename (UTF-8, POSIX rules)
    Mode uint32   // File permissions
    Kind Kind     // blob or tree
    Hash Hash     // BLAKE3 hash of child node
}
```

### Storage Interface

```go
type CAS interface {
    Put(hash Hash, raw []byte) error
    Get(hash Hash) ([]byte, error)
    Has(hash Hash) (bool, error)
}
```

## Canonical Encodings

### Blob Encoding
```
header := "blob <size>\x00"
content := raw_file_bytes
canonical := header || content
hash := BLAKE3(canonical)
```

### Tree Encoding
```
uvarint(entry_count)
for each entry (sorted by name):
  uvarint(mode)
  uvarint(name_length)
  name_bytes
  kind_byte
  32_byte_hash
hash := BLAKE3(tree_canonical_bytes)
```

## Usage Example

```go
// Create filesystem from map
files := map[string][]byte{
    "README.md": []byte("# Project"),
    "src/main.go": []byte("package main"),
    "src/lib.go": []byte("package main"),
}

rootHash, nodeCount, err := BuildTreeFromMap(files)
if err != nil {
    log.Fatal(err)
}

// Later, diff two trees
store := NewStore(NewMemoryCAS())
changes, err := DiffTrees(oldHash, newHash, store)
if err != nil {
    log.Fatal(err)
}

for _, change := range changes {
    fmt.Printf("%s: %s\n", change.Kind, change.Path)
}
```

## Performance Characteristics

- **Hash Computation**: O(content_size) for blobs, O(entries) for trees
- **Structural Sharing**: Unchanged subtrees have identical hashes
- **Diff Computation**: O(changed_nodes) due to hash-based short-circuiting
- **Storage**: Only modified nodes require new storage

## Validation Rules

- **Filenames**: Must be non-empty, cannot be "." or "..", cannot contain "/"
- **Entry Ordering**: TreeNode entries must be sorted lexicographically
- **Uniqueness**: No duplicate names within a directory
- **Mode Validation**: Only 0100644 (files) and 040000 (directories) supported

## Thread Safety

The core types are immutable after creation. The CAS interface implementations must handle their own thread safety requirements.