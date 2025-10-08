---
layout: default
title: Core Concepts
---

# Core Concepts

Understanding these core concepts will help you master Ivaldi VCS.

## Table of Contents
- [Timelines](#timelines)
- [Seals](#seals)
- [Content-Addressable Storage](#content-addressable-storage)
- [Auto-Shelving](#auto-shelving)
- [Merkle Mountain Range](#merkle-mountain-range)
- [HAMT Directory Trees](#hamt-directory-trees)

## Timelines

Timelines are Ivaldi's equivalent to Git branches, but with enhanced capabilities.

### What is a Timeline?

A timeline represents a linear sequence of commits (seals) in your project's history. Think of it as a parallel universe where your code can evolve independently.

### Key Features

**Auto-Shelving**: When you switch timelines, uncommitted changes are automatically saved and restored later. No more manual stashing!

```bash
# Working on main with uncommitted changes
echo "Work in progress" >> feature.txt

# Switch to another timeline
ivaldi timeline switch feature-branch
# Changes automatically shelved

# Switch back
ivaldi timeline switch main
# Changes automatically restored!
```

**Workspace Isolation**: Each timeline maintains its own workspace state, ensuring clean separation between different lines of development.

**Efficient Storage**: Shared content between timelines is stored only once via content-addressable storage.

### Timeline Operations

```bash
# Create a timeline
ivaldi timeline create feature-auth

# List all timelines
ivaldi timeline list

# Switch to a timeline
ivaldi timeline switch main

# Remove a timeline
ivaldi timeline remove old-feature
```

### Comparison with Git Branches

| Feature | Git Branch | Ivaldi Timeline |
|---------|-----------|-----------------|
| Switching | Manual stashing | Auto-shelving |
| Naming | Flexible | Flexible |
| Workspace | Shared | Isolated |
| Storage | Delta-based | Content-addressable |

## Seals

Seals are Ivaldi's commits, but with human-friendly names.

### What is a Seal?

A seal represents a snapshot of your repository at a specific point in time. Each seal has:
- **Unique Hash**: BLAKE3 content hash
- **Memorable Name**: Human-readable identifier
- **Message**: Description of changes
- **Author**: Who created the seal
- **Timestamp**: When it was created
- **Parent(s)**: Previous seal(s) in the timeline

### Seal Names

Every seal gets an automatically generated memorable name:

```
swift-eagle-flies-high-447abe9b
│     │     │    │    │
│     │     │    │    └─── Short hash (8 characters)
│     │     │    └─────── Adjective
│     │     └────────────── Verb
│     └────────────────────── Adjective
└──────────────────────────── Noun
```

### Why Seal Names?

**Easy to Remember**: "swift-eagle-flies-high" is easier than "447abe9b1234567890"

**Unique**: Each seal gets a guaranteed unique name

**Flexible References**: Use either the name or hash to reference a seal

```bash
# All of these work:
ivaldi seals show swift-eagle-flies-high-447abe9b
ivaldi seals show swift-eagle
ivaldi seals show 447abe9b
ivaldi seals show 447a
```

### Creating Seals

```bash
# Simple seal
ivaldi seal "Add authentication feature"
# Created seal: brave-wolf-runs-fast-abc12345

# View all seals
ivaldi seals list

# Show details
ivaldi seals show brave-wolf-runs-fast
```

## Content-Addressable Storage

Ivaldi uses content-addressable storage (CAS) for efficient and secure data management.

### How It Works

Every piece of content (file, directory, commit) is identified by its BLAKE3 hash:

```
File content → BLAKE3 hash → Storage key
```

**Identical content = Identical hash = Single storage**

### Benefits

**Automatic Deduplication**: Same content is stored only once, even across different timelines.

```bash
# README.md exists in multiple timelines
# But stored only once in CAS
# Massive space savings!
```

**Data Integrity**: Content hash proves data hasn't been corrupted or tampered with.

**Fast Comparison**: Comparing hashes is instant, even for large files.

**Efficient Storage**: Only unique chunks are stored, reducing disk usage.

### File Chunking

Large files are split into 64KB chunks:
- Each chunk is hashed independently
- Shared chunks between file versions are deduplicated
- Efficient storage for large repositories

### BLAKE3 Hashing

Ivaldi uses BLAKE3 instead of SHA-1 (Git) or SHA-256:
- **Faster**: Up to 10x faster than SHA-256
- **Secure**: Cryptographically secure
- **Parallel**: Can use multiple CPU cores
- **Modern**: State-of-the-art hash function

## Auto-Shelving

One of Ivaldi's killer features: never lose work when switching timelines.

### The Problem with Git

In Git:
```bash
# Working on feature
$ git status
Modified: important.txt

# Try to switch branches
$ git checkout main
error: Your local changes would be overwritten
# Must manually stash!
```

### The Ivaldi Solution

In Ivaldi:
```bash
# Working on feature
$ ivaldi status
Modified: important.txt

# Switch timelines - just works!
$ ivaldi timeline switch main
# Changes automatically shelved

# Switch back
$ ivaldi timeline switch feature
# Changes automatically restored!
```

### How It Works

1. **Before Switch**: Workspace state is saved
   - Staged files recorded
   - Modified files recorded
   - Untracked files handled

2. **During Switch**: Workspace is materialized to target timeline
   - Files from target timeline are restored
   - Previous state stored in shelf

3. **On Return**: Shelved state is restored
   - Staged files re-staged
   - Modified files restored
   - Everything as you left it

### Best Practices

Auto-shelving is automatic, but keep in mind:
- **Commit regularly**: Shelves are temporary
- **Check status**: Know what's shelved with `ivaldi status`
- **Clean workspace**: Easier to track changes

## Merkle Mountain Range

Ivaldi uses a Merkle Mountain Range (MMR) for commit history tracking.

### What is an MMR?

An MMR is an append-only data structure that provides:
- **Efficient verification**: Prove a commit exists in history
- **Cryptographic proofs**: Each commit is cryptographically linked
- **Fast appends**: Adding commits is O(log n)
- **Persistent storage**: Stored in BoltDB

### Structure

```
Height 3:           H₃
                   / \
Height 2:      H₁     H₂
              / \   / \
Height 1:    / \ / \ / \
Commits:    C₁ C₂ C₃ C₄ C₅ C₆
```

Each node contains:
- Hash of content below it
- Links to child nodes
- Metadata

### Benefits

**Tamper-Proof**: Changing any commit changes all subsequent hashes

**Efficient Proofs**: Prove commit inclusion with O(log n) hashes

**Fast Syncing**: Download only what's needed

**Append-Only**: History never modified, only extended

## HAMT Directory Trees

Ivaldi represents directories using Hash Array Mapped Tries (HAMTs).

### What is a HAMT?

A HAMT is an efficient tree structure for storing key-value pairs:
- **Immutable**: Creates new versions instead of modifying
- **Structural sharing**: Unchanged parts shared between versions
- **Fast lookups**: O(log n) time complexity
- **Space efficient**: Only stores differences

### Directory Representation

```
Directory Tree:
  src/
    auth/
      login.go
      logout.go
    database/
      models.go
```

Stored as HAMT:
```
Root HAMT
├─ "src" → HAMT
   ├─ "auth" → HAMT
   │  ├─ "login.go" → file hash
   │  └─ "logout.go" → file hash
   └─ "database" → HAMT
      └─ "models.go" → file hash
```

### Benefits

**Efficient Updates**: Changing one file only updates path to that file

**Structural Sharing**: Multiple timelines share unchanged directories

**Fast Comparison**: Compare entire directory trees by hash

**Version History**: Each commit has its own directory tree version

### Example

```bash
# Timeline A and B share most files
# Only differences stored
Timeline A: src/auth/login.go (modified)
Timeline B: src/auth/login.go (original)

# Shared: src/auth/logout.go, src/database/models.go
# Different: src/auth/login.go
# Result: Massive space savings!
```

## Workspace Management

Ivaldi intelligently manages your workspace when switching timelines.

### Materialization

When you switch timelines, Ivaldi:
1. **Determines changes**: Compare current and target trees
2. **Minimal updates**: Only modify files that differ
3. **Efficient I/O**: Batch file operations
4. **Preserve work**: Auto-shelf uncommitted changes

### Workspace Operations

```bash
# Check workspace status
ivaldi status

# See what's changed
ivaldi diff

# Reset to clean state
ivaldi reset --hard
```

### File States

Files in your workspace can be:
- **Untracked**: Not in version control
- **Unmodified**: Matches last seal
- **Modified**: Changed since last seal
- **Staged**: Ready for next seal
- **Ignored**: Excluded via `.ivaldiignore`

## Putting It Together

Here's how these concepts work together:

```bash
# Create timeline (auto-shelving ready)
ivaldi timeline create feature-x

# Make changes
echo "new code" >> src/main.go

# Create seal (BLAKE3 hash, memorable name)
ivaldi seal "Add feature X"
# Content → CAS
# Seal → MMR
# Directory → HAMT

# Switch timeline (auto-shelving activates)
ivaldi timeline switch main
# Workspace materialized from HAMT
# Changes shelved

# Switch back
ivaldi timeline switch feature-x
# Shelved changes restored
# Workspace ready to continue
```

## Next Steps

Now that you understand the core concepts:
- Explore [Commands](commands/index.md) to see how to use these features
- Read [Workflow Guides](guides/basic-workflow.md) for practical examples
- Compare with [Git](comparison.md) to understand differences
