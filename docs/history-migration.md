---
layout: default
title: Git History Migration
---

# Git History Migration

Ivaldi can now import full commit history from GitHub and GitLab repositories, preserving all commits, authors, timestamps, and tags. This allows you to travel through the complete history of a repository using Ivaldi's time-travel features.

## Overview

When downloading a repository from GitHub or GitLab, Ivaldi can:
- Import the entire commit history or a limited depth
- Preserve author and committer information
- Maintain original commit timestamps
- Import tags and releases
- Build the complete parent-child commit chain

## Usage

### Basic Download with Full History

```bash
ivaldi download owner/repo
```

By default, Ivaldi imports the full commit history from the repository.

### Limit Commit Depth

To import only the most recent commits:

```bash
ivaldi download owner/repo --depth=100
```

This imports the 100 most recent commits. Use `--depth=0` for full history (default).

### Skip History Migration

For backward compatibility or when you only need the latest snapshot:

```bash
ivaldi download owner/repo --skip-history
```

This downloads only the latest files without any commit history.

### Include Tags and Releases

To import tags along with commit history:

```bash
ivaldi download owner/repo --include-tags
```

Tags are created as Ivaldi timeline references under `tags/` namespace.

## Examples

### Clone Repository with Full History

```bash
ivaldi download torvalds/linux
```

Output:
```
Cloning torvalds/linux from GitHub...
Repository: torvalds/linux
Description: Linux kernel source tree
Default branch: master

Fetching commit history (depth: full history)...
Found 1234567 commits to import

Importing commit 1/1234567: abc1234 (Initial commit)
Importing commit 2/1234567: def5678 (Add feature X)
...
Successfully cloned torvalds/linux with 1234567 commits
```

### Clone with Limited Depth

```bash
ivaldi download facebook/react --depth=50
```

Output:
```
Cloning facebook/react from GitHub...
Repository: facebook/react
Default branch: main

Fetching commit history (depth: 50 commits)...
Found 50 commits to import

Importing commit 1/50: abc1234 (Fix bug in hooks)
...
Successfully cloned facebook/react with 50 commits
```

### Clone with Tags

```bash
ivaldi download golang/go --include-tags
```

Output:
```
Cloning golang/go from GitHub...
...
Successfully cloned golang/go with 125432 commits

Importing tags and releases...
Found 342 tags
Imported tag: go1.21.0
Imported tag: go1.20.5
...
Successfully imported 342/342 tags
```

### GitLab Repository with History

```bash
ivaldi download gitlab-org/gitlab --gitlab --depth=100 --include-tags
```

## How It Works

### Commit Import Process

1. **Fetch Commit List**: Ivaldi queries the GitHub/GitLab API for commit history
2. **Chronological Order**: Commits are imported oldest-first to maintain proper parent relationships
3. **Download Files**: For each commit, Ivaldi downloads the file tree at that point in time
4. **Create Ivaldi Commit**: Each Git commit is converted to an Ivaldi commit with:
   - Preserved author and committer metadata
   - Original commit timestamps
   - Original commit message
   - Parent commit references
5. **Store Mappings**: Git SHA-1 hashes are mapped to Ivaldi BLAKE3 hashes for traceability
6. **Update Timeline**: The timeline HEAD points to the latest commit

### Tag Import

When `--include-tags` is enabled:
1. Fetch all tags from the repository
2. For each tag, find the corresponding Ivaldi commit
3. Create an Ivaldi timeline reference under `tags/` namespace
4. Tags without corresponding commits are skipped with a warning

## Command Options

### download Command Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--depth` | int | 0 | Limit commit history depth (0 for full history) |
| `--skip-history` | bool | false | Skip history migration, download only latest snapshot |
| `--include-tags` | bool | false | Import tags and releases as Ivaldi references |
| `--recurse-submodules` | bool | true | Automatically clone and convert Git submodules |

### Platform-Specific Flags

#### GitLab
| Flag | Description |
|------|-------------|
| `--gitlab` | Force GitLab platform detection |
| `--url` | Custom GitLab instance URL |

## Viewing Imported History

After importing history, you can use Ivaldi's log command to view commits:

```bash
ivaldi log
```

Output:
```
seal: brave-mountain-7a3c (7a3c4ef1)
message: Fix critical bug in authentication
author: Jane Doe <jane@example.com>
date: 2025-11-14 10:30:15

seal: silent-river-2b1d (2b1d9af3)
message: Add new user dashboard feature
author: John Smith <john@example.com>
date: 2025-11-13 14:22:08
...
```

## Traveling Through History

With full commit history imported, you can use Ivaldi's time-travel features:

### Jump to Specific Commit

```bash
ivaldi jump <seal-name>
```

### View Differences Between Commits

```bash
ivaldi log --verbose
```

### Timeline Management

```bash
# List all timelines (including tags)
ivaldi timeline list

# Create new timeline from tag
ivaldi timeline create my-feature --from tags/v1.0.0

# Switch to a tag
ivaldi timeline switch tags/v1.0.0
```

## Performance Considerations

### Large Repositories

For repositories with extensive history:
- **Use depth limiting**: `--depth=100` for recent commits only
- **Increased timeout**: Large imports may take 10-30 minutes
- **Network bandwidth**: Full history requires downloading all file versions
- **Storage space**: Each commit's files are stored (with deduplication)

### Optimization Tips

1. **Start with shallow clone**: Use `--depth=50` initially, then fetch more history if needed
2. **Skip history for archives**: Use `--skip-history` for read-only reference repositories
3. **Incremental imports**: Import recent history first, older history can be added later
4. **Use tags strategically**: Only import tags if you need to reference specific releases

## Storage and Deduplication

Ivaldi's content-addressable storage (CAS) automatically deduplicates:
- Files that don't change between commits
- Identical content across different paths
- Content shared across timelines

This means importing full history is more storage-efficient than it might appear.

## Git Mapping

Ivaldi maintains a mapping between Git SHA-1 hashes and Ivaldi BLAKE3 hashes:

```
Git SHA-1: abc123def456... → Ivaldi BLAKE3: 7a3c4ef1...
```

This mapping enables:
- Traceability back to original Git commits
- Incremental synchronization with remote
- Compatibility with Git-based workflows

## Troubleshooting

### Import Fails Midway

If import fails partway through:
```
Error: failed to import commit abc1234: network timeout
```

**Solution**: Re-run the download command. Ivaldi will skip already-imported commits and continue from where it left off.

### Tag Import Warnings

```
Warning: tag 'v1.0.0-beta' points to commit abc1234 which was not imported, skipping
```

**Cause**: The tag points to a commit outside the imported range (when using `--depth`).

**Solution**: Import with greater depth or use `--depth=0` for full history.

### Rate Limiting

GitHub API has rate limits:
- **Unauthenticated**: 60 requests/hour
- **Authenticated**: 5000 requests/hour

**Solution**: Run `ivaldi auth login` to authenticate and increase rate limits.

## Best Practices

1. **Authentication First**: Always authenticate before importing large repositories
   ```bash
   ivaldi auth login
   ```

2. **Start Shallow**: Begin with `--depth=100`, expand if needed

3. **Use Tags Selectively**: Only use `--include-tags` if you need release references

4. **Check Disk Space**: Large histories require significant storage

5. **Incremental Approach**:
   - First import: `--depth=100`
   - Later: Fetch full history if needed

6. **Documentation Repositories**: Use `--skip-history` for static documentation

## Comparison with Git Clone

| Feature | Ivaldi | Git |
|---------|--------|-----|
| Hashing | BLAKE3 | SHA-1 |
| Storage | CAS with deduplication | Packfiles |
| History | Optional depth control | Supports shallow clones |
| Metadata | Preserved | Preserved |
| Tags | Optional import | Always imported |
| Submodules | Auto-converted to Ivaldi | Requires --recurse-submodules |

## See Also

- [Getting Started](getting-started.md) - Basic Ivaldi workflow
- [Sync Command](sync-command.md) - Incremental updates
- [Core Concepts](core-concepts.md) - Understanding timelines and seals
- [Architecture](architecture.md) - How Ivaldi stores data
