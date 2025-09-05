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
2. **Creates New Commit**: Builds a commit object that references the parent's tree
3. **Copies File References**: All files from parent are referenced (copy-on-write semantics)
4. **Stores in MMR**: Appends the new commit to the persistent MMR history
5. **Materializes Workspace**: Extracts all files to the working directory

## Usage

### Creating a New Timeline

Create a timeline that branches from the current timeline:

```bash
ivaldi timeline create feature-branch
```

This will:
- Create a new timeline branched from the current timeline
- Copy all files from the parent timeline (via commit references)
- Materialize the files in your workspace
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

Automatically preserves workspace files when switching timelines by comparing against the target timeline's state:

#### How It Works
- **Intelligent Preservation**: Compares current workspace with TARGET timeline's state (not source)
- **Comprehensive Shelving**: Preserves ALL files that would be lost in the switch, including:
  - Uncommitted new files
  - Modified files  
  - Files that exist in current timeline but not in target timeline
- **Automatic Restoration**: Restores shelved files when returning to timeline
- **Per-Timeline Shelves**: Each timeline maintains its own auto-shelf

#### Example
```bash
# Empty main timeline scenario
ivaldi forge                    # Creates empty main
touch main.txt                  # Uncommitted file
ivaldi tl create feature        # Captures main.txt in feature
echo "test" > feature.txt       # Add another file  
ivaldi tl switch main           # Auto-shelves BOTH files
# Workspace becomes empty (main has no files)
ivaldi tl switch feature        # Restores both files
```

This ensures no files are lost during timeline switches, regardless of their commit status.

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