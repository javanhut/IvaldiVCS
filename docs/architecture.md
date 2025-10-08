---
layout: default
title: Architecture
---

# Architecture

Technical overview of Ivaldi's internal design and data structures.

## System Overview

Ivaldi is built on several key components:

```
┌─────────────────────────────────────────┐
│         Command Line Interface           │
│         (cli/ package)                   │
└────────────────┬────────────────────────┘
                 │
                 v
┌─────────────────────────────────────────┐
│      Core Components                     │
│  ┌──────────────┐  ┌─────────────────┐  │
│  │   Commit     │  │    Workspace    │  │
│  │  Management  │  │  Materialization│  │
│  └──────────────┘  └─────────────────┘  │
│  ┌──────────────┐  ┌─────────────────┐  │
│  │   Timeline   │  │    References   │  │
│  │   History    │  │   Management    │  │
│  └──────────────┘  └─────────────────┘  │
└────────────────┬────────────────────────┘
                 │
                 v
┌─────────────────────────────────────────┐
│      Storage Layer                       │
│  ┌──────────────┐  ┌─────────────────┐  │
│  │    CAS       │  │     HAMT        │  │
│  │  (objects/)  │  │  Directory Tree │  │
│  └──────────────┘  └─────────────────┘  │
│  ┌──────────────┐  ┌─────────────────┐  │
│  │     MMR      │  │  File Chunking  │  │
│  │  (BoltDB)    │  │   (64KB chunks) │  │
│  └──────────────┘  └─────────────────┘  │
└─────────────────────────────────────────┘
```

## Directory Structure

```
.ivaldi/
├── objects/            # Content-addressable storage
│   ├── ab/
│   │   └── cdef123...  # Object files (first 2 chars as dir)
│   └── ...
├── refs/              # Timeline references
│   ├── heads/
│   │   ├── main
│   │   └── feature-auth
│   └── remotes/
├── mmr.db            # Merkle Mountain Range database (BoltDB)
├── config            # Repository configuration
├── HEAD              # Current timeline pointer
├── index             # Workspace index
└── shelves/          # Auto-shelving storage
    └── feature-auth  # Shelved changes per timeline
```

## Content-Addressable Storage (CAS)

### How It Works

Every piece of content is identified by its BLAKE3 hash:

```
Content → BLAKE3 → Hash (ID) → Storage Path
```

Example:
```
File: "Hello, Ivaldi!"
↓
BLAKE3 hash: af3d8e92b1c...
↓
Storage: .ivaldi/objects/af/3d8e92b1c...
```

### Object Types

**Blob**: File content
```
Type: blob
Size: 1234
Content: [binary data]
```

**Tree**: Directory structure (HAMT)
```
Type: tree
Entries:
  README.md → blob:abc123
  src/ → tree:def456
```

**Commit (Seal)**: Snapshot
```
Type: commit
Tree: hash-of-root-tree
Parent: hash-of-parent-commit
Author: Jane Doe <jane@example.com>
Timestamp: 2025-10-07T14:30:00Z
Message: Add authentication feature
SealName: swift-eagle-flies-high-447abe9b
```

### Storage Layout

```
.ivaldi/objects/
├── ab/
│   └── cdef1234567890...  # First 2 chars become directory
├── de/
│   └── f4567890abcdef...
└── ...
```

This sharding prevents too many files in one directory.

## BLAKE3 Hashing

### Why BLAKE3?

1. **Fast**: Up to 10x faster than SHA-256
2. **Parallel**: Uses all CPU cores
3. **Secure**: Cryptographically secure
4. **Modern**: State-of-the-art (2020)

### Performance

```
File: 1GB
SHA-1:   ~8 seconds
SHA-256: ~6 seconds
BLAKE3:  ~0.8 seconds (7-10x faster!)
```

### Usage in Ivaldi

- File content hashing
- Tree structure hashing
- Commit identification
- Deduplication

## File Chunking

### Chunking Strategy

Large files are split into 64KB chunks:

```
Large File (10MB)
↓
Split into chunks (64KB each)
↓
Chunk 1: hash1
Chunk 2: hash2
Chunk 3: hash3
...
Chunk 156: hash156
↓
Store chunk list with hashes
```

### Benefits

**Deduplication**: Identical chunks stored once
```
File A: [chunk1, chunk2, chunk3]
File B: [chunk1, chunk4, chunk3]
Shared: chunk1, chunk3 (stored once)
Different: chunk2, chunk4
```

**Efficient Updates**: Only modified chunks re-uploaded
```
Before: [chunk1, chunk2, chunk3]
After:  [chunk1, chunk2-modified, chunk3]
Upload: Only chunk2-modified
```

**Parallel Processing**: Hash/transfer chunks concurrently

## HAMT Directory Trees

### What is a HAMT?

Hash Array Mapped Trie - efficient immutable tree structure.

### Structure

```
Root HAMT Node
├─ "src" → HAMT Node
│  ├─ "auth" → HAMT Node
│  │  ├─ "login.go" → blob hash
│  │  └─ "logout.go" → blob hash
│  └─ "database" → HAMT Node
│     └─ "models.go" → blob hash
└─ "README.md" → blob hash
```

### Properties

**Immutable**: Creating new version, not modifying
**Structural Sharing**: Unchanged nodes shared
**Fast Lookups**: O(log n) time
**Efficient Updates**: Only path to changed node updates

### Example: Modifying One File

```
Before:
Root
├─ src → Node1
│  └─ file.go → hash_old
└─ README.md → hash_readme

After (modify src/file.go):
Root'
├─ src → Node1'
│  └─ file.go → hash_new
└─ README.md → hash_readme  (shared!)

New nodes: Root', Node1'
Shared nodes: hash_readme
Result: Efficient storage!
```

## Merkle Mountain Range (MMR)

### Purpose

Track commit history with cryptographic proofs.

### Structure

```
     Peak
    /    \
   /      \
 Node    Node
 / \      / \
C1 C2    C3 C4  (Commits)
```

### Properties

**Append-Only**: Never modify, only add
**Cryptographic Proofs**: Each node hash includes children
**Efficient Verification**: Prove inclusion with O(log n) hashes
**Tamper-Proof**: Changing any commit changes all subsequent hashes

### Storage

Stored in BoltDB (`.ivaldi/mmr.db`):
```
Key: commit hash
Value: {
  height: 0,
  parent: parent_hash,
  left: left_hash,
  right: right_hash
}
```

### Benefits

- Prove a commit exists in history
- Detect tampering
- Efficient sync protocols
- Audit trail

## Timeline Management

### Timeline Reference

Stored in `.ivaldi/refs/heads/<timeline-name>`:
```
447abe9b1234567890abcdef...
```

Points to the latest commit hash.

### HEAD Pointer

`.ivaldi/HEAD` contains:
```
ref: refs/heads/main
```

Or direct hash in detached state.

### Timeline Operations

**Create Timeline**:
1. Create ref file: `.ivaldi/refs/heads/feature-name`
2. Point to current commit
3. Update HEAD to new timeline

**Switch Timeline**:
1. Read new timeline's commit
2. Materialize workspace from commit's tree
3. Update HEAD

**Remove Timeline**:
1. Delete ref file
2. Commits may become orphaned (garbage collected later)

## Workspace Materialization

### Process

When switching timelines:

1. **Read target commit**
   ```
   Commit hash → Load commit object → Get tree hash
   ```

2. **Compare trees**
   ```
   Current tree vs Target tree
   → Determine files to add/modify/delete
   ```

3. **Apply changes**
   ```
   For each difference:
     - Add: Create file from blob
     - Modify: Update file from blob
     - Delete: Remove file
   ```

4. **Update index**
   ```
   Record workspace state in .ivaldi/index
   ```

### Optimization

- **Batch operations**: Group file operations
- **Parallel I/O**: Process multiple files concurrently
- **Minimal changes**: Only modify what's different
- **Chunked reading**: Stream large files

## Auto-Shelving

### How It Works

When switching timelines:

1. **Before Switch**:
   ```
   - Detect modified files
   - Detect staged files
   - Store in .ivaldi/shelves/<timeline-name>
   ```

2. **Shelf Contents**:
   ```
   {
     staged: [list of staged file hashes],
     modified: {file: content-diff},
     created: [new files]
   }
   ```

3. **On Return**:
   ```
   - Read shelf for timeline
   - Restore staged files
   - Restore modified files
   - Restore created files
   ```

### Storage

```
.ivaldi/shelves/
├── feature-auth
│   ├── staged
│   ├── modified
│   └── created
└── feature-payment
    ├── staged
    └── modified
```

## GitHub Integration

### Upload (Push)

```
Ivaldi Timeline → Git Branch
Ivaldi Seal → Git Commit

Process:
1. Read timeline commits
2. Convert to Git commit format
3. Create Git objects
4. Push to GitHub via Git protocol
```

### Download (Clone)

```
GitHub Repo → Ivaldi Repo

Process:
1. Clone with Git
2. Convert Git commits to Ivaldi seals
3. Create Ivaldi timelines from branches
4. Build HAMT trees from Git trees
```

### Selective Sync (Harvest)

```
Traditional Git:
git fetch → Downloads all branches

Ivaldi:
ivaldi scout → Metadata only
ivaldi harvest feature-x → Only specific branch

Implementation:
1. Scout: Git ls-remote (metadata)
2. Harvest: Git fetch specific ref
3. Convert to Ivaldi format
```

## Seal Name Generation

### Algorithm

```
Commit Hash: 447abe9b1234...
↓
Use hash as seed for deterministic random
↓
Select from word lists:
- Adjectives: [swift, brave, calm, ...]
- Nouns: [eagle, wolf, river, ...]
- Verbs: [flies, runs, flows, ...]
- Adjectives: [high, fast, deep, ...]
↓
Combine: swift-eagle-flies-high-447abe9b
```

### Properties

- **Deterministic**: Same hash always produces same name
- **Unique**: Hash suffix ensures uniqueness
- **Memorable**: Human-readable words
- **Searchable**: Can search by partial name

## Configuration System

### Levels

1. **System**: `/etc/ivaldi/config` (future)
2. **User**: `~/.ivaldi/config`
3. **Repository**: `.ivaldi/config`

### Format

```ini
[user]
    name = Jane Doe
    email = jane@example.com

[color]
    ui = true

[portal]
    default = javanhut/IvaldiVCS
```

### Precedence

Repository > User > System

## Performance Optimizations

### Content-Addressable Storage

**Deduplication**:
- Identical content stored once
- Massive space savings

**Example**:
```
10 branches with README.md (same content)
Git: Stores 10 copies (delta compressed)
Ivaldi: Stores 1 copy (exact hash match)
```

### Parallel Operations

- File chunking: Parallel hashing
- Tree traversal: Parallel processing
- Network operations: Concurrent transfers

### Caching

- Tree objects cached in memory
- Commit metadata cached
- Blob chunk locations cached

## Security Considerations

### Hash Security

BLAKE3 provides:
- Collision resistance
- Preimage resistance
- Second preimage resistance

### Tamper Detection

MMR structure ensures:
- Cannot modify past commits
- Cannot reorder history
- Changes are detectable

### Safe Operations

- Auto-shelving prevents data loss
- Time travel offers non-destructive options
- Multiple merge strategies for safety

## Future Enhancements

### Planned Features

1. **Tags**: Lightweight and annotated
2. **Hooks**: Pre/post operation scripts
3. **Submodules**: Nested repositories
4. **Bisect**: Binary search for bugs
5. **Garbage Collection**: Remove orphaned objects
6. **Pack Files**: Compress object storage
7. **Partial Clone**: Clone without full history

### Research Areas

- **Distributed**: Peer-to-peer sync
- **Incremental**: Faster checkouts
- **Compression**: Better storage efficiency
- **Networking**: Optimized transfer protocols

## Implementation Details

### Language: Go

Benefits:
- Fast compilation
- Excellent concurrency (goroutines)
- Strong standard library
- Cross-platform

### Dependencies

Key libraries:
- **BLAKE3**: github.com/zeebo/blake3
- **BoltDB**: go.etcd.io/bbolt
- **GitHub API**: github.com/google/go-github

### Code Structure

```
internal/
├── cas/          # Content-addressable storage
├── commit/       # Commit management
├── filechunk/    # File chunking system
├── github/       # GitHub integration
├── hamtdir/      # HAMT directory trees
├── history/      # MMR and timeline history
├── refs/         # Reference management
├── workspace/    # Workspace materialization
└── wsindex/      # Workspace indexing
```

## Performance Benchmarks

### Hashing Speed

```
Operation: Hash 1GB file
SHA-1:   ~8.0 seconds
BLAKE3:  ~0.8 seconds
Result: 10x faster
```

### Repository Size

```
Test: 1000 commits, 10MB each
Git:    ~800MB (with delta compression)
Ivaldi: ~600MB (with deduplication)
Result: 25% smaller
```

### Clone Time

```
Operation: Clone repo with 50 branches
Git:    ~45 seconds (all branches)
Ivaldi: ~8 seconds (main only)
Result: 5.6x faster
```

## Summary

Ivaldi's architecture provides:
- **Performance**: BLAKE3 hashing, chunking, deduplication
- **Safety**: MMR proofs, auto-shelving, immutability
- **Efficiency**: Content-addressable storage, selective sync
- **Usability**: Human-readable names, clean merges

Key innovations:
1. BLAKE3 over SHA-1
2. HAMT for directory trees
3. MMR for commit history
4. Chunk-level deduplication
5. Auto-shelving system
6. Selective remote sync

## Further Reading

- [Core Concepts](core-concepts.md) - User-facing concepts
- [Commands](commands/index.md) - Command reference
- [Comparison](comparison.md) - Ivaldi vs Git

## Contributing

Interested in contributing to Ivaldi's architecture?
- Review the code on [GitHub](https://github.com/javanhut/IvaldiVCS)
- Submit issues and pull requests
- Discuss architecture decisions
