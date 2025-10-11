---
layout: default
title: Sync Command
---

# Sync Command

The `ivaldi sync` command performs incremental synchronization between your local timeline and the remote repository, downloading only the changes (delta) that have been made on the remote.

## Overview

Unlike traditional Git pull operations that fetch entire objects, `ivaldi sync` is optimized to:
- Download only files that have changed on the remote
- Display a clear summary of added, modified, and deleted files
- Update your local timeline to match the remote state
- Minimize network bandwidth and storage usage

## Usage

```bash
ivaldi sync [timeline-name]
```

If no timeline name is provided, syncs the current timeline.

## Examples

### Sync Current Timeline

```bash
ivaldi sync
```

Output:
```
Syncing timeline 'main' with javanhut/myrepo...

Fetching remote state for branch 'main'...
Downloading 3 changed file(s)...

++ test.txt
++ new_magic_wand.md
-- vulnerabilities.py

✓ Synced 3 file(s) from remote
  • Added: 2
  • Deleted: 1
```

### Sync Specific Timeline

```bash
ivaldi sync feature-branch
```

### When Already Up to Date

```bash
ivaldi sync
```

Output:
```
Syncing timeline 'main' with javanhut/myrepo...

Fetching remote state for branch 'main'...

✓ Timeline 'main' is already up to date
```

## Output Format

The sync command displays changes using a diff-style format:

- `++ filename` - File was added or modified on remote
- `-- filename` - File was deleted on remote

### Symbol Meanings

| Symbol | Meaning |
|--------|---------|
| `++` | File added or modified (displayed in green) |
| `--` | File deleted (displayed in red) |

## Prerequisites

Before using `ivaldi sync`, ensure:

1. You have a configured GitHub repository connection:
   ```bash
   ivaldi portal add owner/repo
   ```

2. You have previously cloned or downloaded from the repository:
   ```bash
   ivaldi download github:owner/repo
   ```

## How It Works

1. **Fetch Remote State**: Connects to GitHub and retrieves the current state of the remote branch
2. **Compute Delta**: Compares local commit with remote tree to identify changes
3. **Download Changes**: Downloads only files that were added or modified
4. **Delete Removed**: Removes files that were deleted on remote
5. **Create Commit**: Creates a new local commit representing the synced state
6. **Update Timeline**: Updates the local timeline reference

## Related Commands

- `ivaldi scout` - Discover available remote timelines before syncing
- `ivaldi harvest` - Download entire remote timelines (full clone)
- `ivaldi download` - Clone a repository from GitHub
- `ivaldi upload` - Push local changes to remote
- `ivaldi portal` - Manage repository connections

## Comparison with Other Commands

| Command | Purpose | Use Case |
|---------|---------|----------|
| `sync` | Incremental update of current timeline | Regular updates to existing timeline |
| `harvest` | Full download of remote timeline | First-time download of a branch |
| `download` | Clone entire repository | Initial repository setup |

## Error Handling

### No GitHub Repository Configured

```
Error: no GitHub repository configured. Use 'ivaldi portal add owner/repo' or download from GitHub first
```

**Solution**: Configure a GitHub repository connection:
```bash
ivaldi portal add owner/repo
```

### Timeline Not Found

```
Error: failed to get timeline 'branch-name': timeline not found
```

**Solution**: Ensure the timeline exists locally. Use `ivaldi timeline list` to see available timelines.

### Network Errors

If sync fails due to network issues, the command will report the error and your local state remains unchanged. Simply retry the sync once network connectivity is restored.

## Best Practices

1. **Sync Regularly**: Run `ivaldi sync` frequently to keep your local timeline up to date
2. **Check Status First**: Use `ivaldi status` before syncing to see local changes
3. **Commit Before Syncing**: Create a seal (commit) of your local changes before syncing to avoid conflicts
4. **Use Scout**: Run `ivaldi scout` to discover new remote timelines before syncing

## Performance

The sync command is optimized for performance:
- Only changed files are downloaded
- Local file comparison uses BLAKE3 hashing
- Concurrent file downloads when multiple files need updating
- Efficient tree comparison algorithms

## Technical Details

### Delta Computation

The sync algorithm:
1. Reads local commit tree structure
2. Fetches remote tree from GitHub API
3. Compares file lists and content hashes
4. Identifies additions, modifications, and deletions
5. Downloads only necessary file content

### Storage Efficiency

All downloaded content is stored in Ivaldi's content-addressable storage (CAS), providing:
- Automatic deduplication across timelines
- Efficient storage of file versions
- Fast retrieval of content by hash

## See Also

- [Core Concepts](core-concepts.md) - Understanding timelines and seals
- [Getting Started](getting-started.md) - Basic Ivaldi workflow
- [Architecture](architecture.md) - How Ivaldi stores data
