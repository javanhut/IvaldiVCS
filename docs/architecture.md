# Ivaldi VCS Architecture

## Overview

Ivaldi VCS is a modern version control system built from the ground up with a focus on performance, security, and developer experience. It uses content-addressable storage (CAS) with BLAKE3 hashing, Merkle Mountain Ranges (MMR) for commit history, and a timeline-based branching model.

## Core Components

### 1. Content-Addressable Storage (CAS)

Located in `internal/cas/`, the CAS system is the foundation of Ivaldi's object storage.

#### Key Features:
- **BLAKE3 Hashing**: Fast, secure cryptographic hashing
- **Deduplication**: Identical content stored only once
- **File-based Storage**: Objects stored in `.ivaldi/objects/`

#### Implementation:
```
CAS Interface:
├── Put(data []byte) → Hash
├── Get(hash Hash) → []byte
├── Has(hash Hash) → bool
└── Delete(hash Hash) → error
```

### 2. File Chunking System

Located in `internal/filechunk/`, handles large file storage efficiently.

#### Features:
- **Content-based Chunking**: Variable-size chunks based on content
- **Merkle Tree Structure**: Enables partial file retrieval
- **Configurable Parameters**: Chunk size limits and splitting

#### Structure:
```
FileChunk:
├── NodeRef (root of Merkle tree)
├── ChunkRef (individual chunks)
└── Parameters (min/max chunk sizes)
```

### 3. Merkle Mountain Range (MMR)

Located in `internal/history/`, provides append-only commit history.

#### Benefits:
- **Cryptographic Proofs**: Verify commit inclusion
- **Efficient Appends**: O(log n) complexity
- **Persistent Storage**: Backed by BoltDB

#### Components:
```
MMR System:
├── MMR (in-memory accumulator)
├── PersistentMMR (database-backed)
├── Timeline Store (HEAD tracking)
└── Binary Lifting (LCA queries)
```

### 4. HAMT Directory Trees

Located in `internal/hamtdir/`, implements Hash Array Mapped Trie for directories.

#### Features:
- **Efficient Updates**: O(log n) modifications
- **Structural Sharing**: Branches share unchanged data
- **Compact Storage**: Sparse array optimization

### 5. Commit System

Located in `internal/commit/`, manages commit objects and trees.

#### Commit Structure:
```go
type Commit struct {
    Tree         Hash      // Root directory hash
    Parents      []Hash    // Parent commit hashes
    Author       string    // Author information
    Committer    string    // Committer information
    Message      string    // Commit message
    AuthorTime   time.Time // Author timestamp
    CommitTime   time.Time // Commit timestamp
}
```

### 6. Timeline Management

Located in `internal/refs/`, handles branches (timelines) and references.

#### Timeline Types:
- **Local Timelines**: Regular branches
- **Remote Timelines**: Remote branch tracking
- **Tags**: Immutable references

#### Storage Layout:
```
.ivaldi/
├── refs/
│   ├── timelines/
│   │   ├── main
│   │   └── feature-branch
│   ├── remotes/
│   │   └── origin/
│   │       └── main
│   └── tags/
│       └── v1.0.0
└── HEAD (current timeline)
```

### 7. Workspace Management

Located in `internal/workspace/`, handles file materialization and changes.

#### Key Operations:
- **Materialization**: Extract files from commits to working directory
- **Scanning**: Create index of current workspace state
- **Auto-shelving**: Preserve uncommitted changes when switching

#### Workflow:
```
Switch Timeline:
1. Scan current workspace
2. Auto-shelf uncommitted changes
3. Compute diff to target state
4. Apply minimal file changes
5. Update HEAD reference
6. Restore auto-shelved changes
```

### 8. GitHub Integration

Located in `internal/github/`, provides GitHub API interaction.

#### Features:
- **Repository Download**: Clone GitHub repos
- **Timeline Discovery**: List remote branches
- **Content Upload**: Push changes to GitHub
- **Tree-based Operations**: Efficient bulk transfers

### 9. Database Layer

Uses BoltDB for persistent storage of metadata.

#### Databases:
- **objects.db**: CAS metadata and indices
- **refs.db**: Timeline and tag information
- **mmr.db**: Merkle Mountain Range data

## Data Flow

### Creating a Commit

```
1. User runs 'ivaldi gather files...'
   ├─→ Files listed in .ivaldi/stage/files

2. User runs 'ivaldi seal "message"'
   ├─→ Scan workspace files
   ├─→ Create file chunks (CAS)
   ├─→ Build HAMT directory tree
   ├─→ Create commit object
   ├─→ Append to MMR
   └─→ Update timeline reference
```

### Switching Timelines

```
1. User runs 'ivaldi timeline switch feature'
   ├─→ Save current workspace state
   ├─→ Auto-shelf uncommitted changes
   ├─→ Read target timeline commit
   ├─→ Extract file tree from commit
   ├─→ Compute workspace diff
   ├─→ Apply file changes
   └─→ Restore auto-shelved files
```

### Harvesting Remote Timelines

```
1. User runs 'ivaldi harvest branch-name'
   ├─→ Fetch branch info from GitHub
   ├─→ Create temporary workspace
   ├─→ Download files via GitHub API
   ├─→ Build Ivaldi objects (chunks, trees)
   ├─→ Create commit in MMR
   ├─→ Update timeline reference
   └─→ Clean up temp workspace
```

## Storage Format

### Object Types

1. **Blob Objects**: Raw file content chunks
2. **Tree Objects**: HAMT directory structures
3. **Commit Objects**: Commit metadata and references
4. **Index Objects**: Workspace file indices

### Hash Computation

All objects use BLAKE3 hashing:
```go
hash := blake3.Sum256(data)
```

### Object Storage Path

Objects stored by hash prefix:
```
.ivaldi/objects/
├── 00/
│   └── 00a1b2c3d4e5f6...
├── 01/
│   └── 01f2e3d4c5b6a7...
└── ff/
    └── ff9e8d7c6b5a4...
```

## Concurrency Model

### Parallel Operations

1. **File Scanning**: Concurrent workspace traversal
2. **Object Creation**: Parallel chunk generation
3. **Network Operations**: Concurrent GitHub API calls

### Locking Strategy

- **Database Locks**: BoltDB handles concurrent access
- **File Locks**: OS-level locking for ref updates
- **No Global Locks**: Operations isolated by design

## Security Considerations

### Cryptographic Integrity

- **BLAKE3**: Collision-resistant hashing
- **MMR Proofs**: Verifiable commit history
- **Content Verification**: All reads verify hashes

### GitHub Integration

- **Token Authentication**: Via GITHUB_TOKEN
- **HTTPS Only**: Secure communication
- **No Credential Storage**: Tokens never persisted

## Performance Optimizations

### Content Deduplication

- Same content stored once regardless of copies
- Chunk-level deduplication for large files

### Incremental Updates

- Only changed files transferred
- Minimal workspace modifications
- Efficient diff computation

### Caching

- In-memory object cache
- Workspace index caching
- GitHub API response caching

## Extension Points

### Custom Storage Backends

The CAS interface allows alternative implementations:
- Cloud storage (S3, GCS)
- Distributed storage (IPFS)
- Database storage (PostgreSQL)

### Additional VCS Features

Architecture supports future additions:
- Merge operations
- Conflict resolution
- Partial clones
- Shallow history

### Plugin System

Potential for hooks and extensions:
- Pre-commit hooks
- Post-checkout actions
- Custom merge strategies

## Comparison with Git

| Aspect | Ivaldi | Git |
|--------|---------|-----|
| Object Hashing | BLAKE3 | SHA-1/SHA-256 |
| History Structure | MMR | DAG |
| Directory Trees | HAMT | Tree objects |
| Branch Model | Timeline-based | Reference-based |
| Storage | CAS + BoltDB | Loose + Pack files |
| Uncommitted Changes | Auto-shelving | Manual stashing |

## Testing Strategy

### Unit Tests

Each component has comprehensive tests:
- `cas_test.go`: CAS operations
- `commit_test.go`: Commit creation
- `mmr_test.go`: MMR functionality
- `workspace_test.go`: Materialization

### Integration Tests

End-to-end workflow testing:
- Repository initialization
- Commit workflows
- Timeline operations
- GitHub synchronization

### Benchmarks

Performance testing for:
- Large file handling
- Many file repositories
- Concurrent operations
- Network efficiency