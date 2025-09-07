# Timeline Branching in Ivaldi VCS

## Overview

Ivaldi VCS implements Git-like branching through its timeline system, providing isolated workspaces with advanced storage features including Merkle Mountain Range (MMR) for commit history, content-addressable storage (CAS), and persistent state management.

## Architecture

### Storage Components

1. **Content-Addressable Storage (CAS)**: Stores all file content using BLAKE3 hashing
2. **Merkle Mountain Range (MMR)**: Maintains append-only commit history with cryptographic proofs
3. **HAMT Directory Trees**: Efficient directory structure management
4. **Bolt DB**: Persistent storage for metadata, MMR state, and timeline heads
5. **Workspace Index**: Tracks file states and changes

### Timeline Branching Model

When creating a new timeline (branch), Ivaldi:

1. **Reads Parent State**: Retrieves the parent timeline's HEAD commit
2. **Inherits Parent Commit**: The new timeline points to the same commit as its parent
3. **Shares File References**: All committed files from parent are referenced (copy-on-write semantics)
4. **Clean Workspace**: Only committed files are materialized, untracked files are NOT carried over
5. **Isolated Development**: Each timeline maintains its own workspace state independently

## Usage

### Creating a New Timeline

Create a timeline that branches from the current timeline:

```bash
ivaldi timeline create feature-branch
```

This will:
- Create a new timeline branched from the current timeline
- Inherit the parent timeline's last commit (no new commit created)
- Materialize ONLY the committed files from the parent timeline
- Untracked files in current workspace are preserved via auto-shelving
- Set up isolated tracking for the new timeline

### Switching Between Timelines

Switch to a different timeline:

```bash
ivaldi timeline switch main
```

Features:
- **Auto-shelving**: Automatically stashes uncommitted changes
- **File Materialization**: Restores exact file state from timeline's HEAD
- **Shelf Restoration**: Restores any previously shelved changes for that timeline

### Listing Timelines

View all available timelines:

```bash
ivaldi timeline list
```

Shows:
- Timeline name
- Type (local/remote/tag)
- Last commit hash
- Description

## Implementation Details

### Persistent MMR Storage

The MMR implementation provides:
- Persistent storage using Bolt DB
- Cryptographic accumulator for commit history
- Efficient inclusion proofs
- Timeline head tracking

Key components:
- `PersistentMMR`: MMR with database backing
- `PersistentTimelineStore`: Manages timeline HEAD pointers
- Binary lifting for efficient LCA (Lowest Common Ancestor) queries

### Commit Structure

Each commit contains:
- Tree hash (root of file tree)
- Parent commit hashes
- Author/committer information
- Timestamp
- MMR position (for history tracking)
- Commit message

### File Copying Strategy

Ivaldi uses copy-on-write semantics:
1. **Reference Sharing**: New timelines reference parent's file chunks
2. **Lazy Copying**: Files are only duplicated when modified
3. **Deduplication**: Identical content shares storage via CAS

### Workspace Materialization

When switching timelines:
1. Computes diff between current and target states
2. Applies minimal changes to workspace
3. Preserves file permissions and timestamps
4. Handles directory creation/removal

## Advanced Features

### Auto-Shelving

Automatically preserves workspace changes when switching timelines:

#### How It Works
- **Smart Preservation**: Saves current workspace state before switching
- **Timeline Isolation**: Each timeline's workspace changes are preserved separately
- **Automatic Restoration**: Restores previously shelved changes when returning to a timeline
- **Per-Timeline Shelves**: Each timeline maintains its own auto-shelf

#### Behavior
- When switching FROM a timeline: Current workspace state is auto-shelved
- When switching TO a timeline: 
  1. If timeline has auto-shelved changes, they are restored
  2. Otherwise, the timeline's committed state is materialized
- Untracked files are preserved with their respective timelines

#### Example
```bash
# Timeline isolation example
ivaldi forge                    # Creates empty main
ivaldi seal                     # Empty initial commit
touch main.txt                  # Untracked file in main
ivaldi tl create feature        # Creates feature from main's commit
# feature workspace is empty (main.txt stays with main)
touch feature.txt               # Untracked file in feature
ivaldi tl switch main           # Auto-shelves feature.txt
# main workspace has main.txt (from auto-shelf)
ivaldi tl switch feature        # Auto-shelves main.txt, restores feature.txt
# feature workspace has feature.txt only
```

This ensures complete isolation between timelines - untracked files stay with their respective timelines.

### Merkle Mountain Range Benefits

1. **Append-only**: Immutable history
2. **Efficient Proofs**: O(log n) inclusion proofs
3. **Incremental Updates**: Fast additions
4. **Cryptographic Security**: Tamper-evident history

### Storage Efficiency

- **Content Deduplication**: Via CAS
- **Incremental Storage**: Only changed blocks stored
- **Compressed Objects**: Reduced disk usage
- **Shared References**: Branches share unchanged content

## Comparison with Git

| Feature | Ivaldi | Git |
|---------|--------|-----|
| Branching Model | Timeline-based | Reference-based |
| History Storage | MMR accumulator | DAG of commits |
| Content Hashing | BLAKE3 | SHA-1/SHA-256 |
| Directory Structure | HAMT | Tree objects |
| Uncommitted Changes | Auto-shelving | Manual stashing |
| Storage Backend | Bolt DB + CAS | Loose objects + packs |

## Best Practices

1. **Branch Naming**: Use descriptive names for timelines
2. **Regular Commits**: Commit changes before switching timelines
3. **Timeline Hygiene**: Remove unused timelines to save space
4. **Backup Strategy**: Regular backups of `.ivaldi` directory

## Troubleshooting

### Timeline Creation Fails

If timeline creation fails:
1. Check disk space
2. Verify `.ivaldi` directory integrity
3. Ensure no file permission issues
4. Check for database locks

### Materialization Issues

If files don't materialize correctly:
1. Run `ivaldi timeline switch <name>` again
2. Check for uncommitted changes blocking materialization
3. Verify CAS object integrity
4. Review workspace permissions

### MMR Synchronization

If history appears corrupted:
1. Database may need recovery
2. Check `objects.db` integrity
3. Verify MMR metadata consistency
4. Consider rebuilding from refs

## Future Enhancements

- Remote timeline synchronization
- Partial materialization for large repositories
- Timeline merge operations
- Garbage collection for unreferenced objects
- Timeline-specific hooks
- Signed commits with MMR verification