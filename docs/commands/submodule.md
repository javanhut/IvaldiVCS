---
layout: default
title: Submodule Commands
---

# Submodule Commands

Ivaldi VCS supports Git-style submodules with enhanced features like timeline-awareness and automatic Git conversion.

## Overview

Submodules allow you to include external repositories within your main repository. Ivaldi's submodule system:

- **Automatically converts Git submodules** when cloning or initializing
- Uses **BLAKE3 hashes internally** for Ivaldi-native submodules
- Maintains **Git SHA-1 mapping** for GitHub compatibility
- Tracks submodules **per timeline** (not just per branch)
- **Auto-shelves** submodule changes when switching timelines

## Automatic Git Submodule Conversion

When you clone a Git repository with submodules or run `ivaldi forge` in a Git repository with submodules, Ivaldi automatically:

1. Detects `.gitmodules` file
2. Clones missing submodules
3. Converts Git objects to Ivaldi format
4. Creates `.ivaldimodules` configuration
5. Stores dual-hash mapping (BLAKE3 ↔ Git SHA-1)

### Example: Clone with Submodules

```bash
$ ivaldi download https://github.com/owner/repo-with-submodules

Cloning repository from GitHub...
✓ Cloned main repository

Converting Git repository to Ivaldi format...
✓ Converted 1,234 Git objects

📦 Detected Git submodules...
  Submodule 'external-lib' at libs/external-lib
    Cloning from https://github.com/owner/external-lib...
    ✓ Cloned (commit: abc123)
    Converting to Ivaldi format...
    ✓ Converted 456 objects

✓ Initialized 2 submodules
✓ Created .ivaldimodules

Repository ready! Current timeline: main
```

### Example: Initialize in Git Repo with Submodules

```bash
$ cd my-git-repo-with-submodules
$ ivaldi forge

Ivaldi repository initialized
Detecting existing Git repository...
✓ Converted 2,345 Git objects

📦 Detected Git submodules...
  Found 3 submodules in .gitmodules
  
  Submodule 'lib1' at libs/lib1 (commit: 789abc)
    ✓ Converted to Ivaldi format
  
✓ Converted 3 Git submodules
✓ Created .ivaldimodules
```

## Configuration File: `.ivaldimodules`

Ivaldi uses `.ivaldimodules` (similar to Git's `.gitmodules`) to track submodule configuration.

### Format

```ini
# .ivaldimodules - Ivaldi Submodule Configuration
# Version: 1

[submodule "library-name"]
    path = libs/external-lib
    url = https://github.com/owner/external-lib
    timeline = main
    commit = 1a2b3c4d5e6f...  # BLAKE3 hash (64 hex chars)
    git-commit = abc123def...  # Git SHA-1 (40 hex chars, optional)
    shallow = true             # Optional
    freeze = false             # Optional
```

### Fields

- **path** (required): Relative path in repository
- **url** (required): Repository URL (https, ssh, file)
- **timeline** (required): Timeline name to track
- **commit** (required): BLAKE3 hash of target commit (PRIMARY reference)
- **git-commit** (optional): Git SHA-1 for GitHub sync only
- **shallow** (optional): Use shallow clone
- **freeze** (optional): Prevent automatic updates
- **ignore** (optional): How to handle uncommitted changes in status

## Disabling Automatic Submodule Cloning

Use the `--recurse-submodules=false` flag to skip submodule initialization:

```bash
$ ivaldi download https://github.com/owner/repo --recurse-submodules=false

# Or
$ ivaldi forge --recurse-submodules=false
```

## Internal Architecture

### Storage

Ivaldi stores submodules using native mechanisms:

```
.ivaldi/
├── modules/
│   ├── metadata.db           # BoltDB: per-timeline state
│   ├── external-lib/.ivaldi/ # Full Ivaldi repo for submodule
│   └── vendor-tool/.ivaldi/  # Another submodule
├── objects/                  # SubmoduleNode objects in CAS
└── ...
```

### Node Types

**SubmoduleNode** (stored in CAS):
- URL, Path, Timeline
- **CommitHash** (BLAKE3) - primary reference
- Flags (shallow, freeze)

**HAMT Entry**:
- Name, Type (SubmoduleEntry)
- **SubmoduleRef** with BLAKE3 hashes

### Dual-Hash Mapping

BoltDB bucket `git-submodule-mappings`:
```
Key: "ivaldi-commit-" + blake3_hex
Value: git_sha1

Key: "git-commit-" + git_sha1
Value: blake3_hex
```

## Git Compatibility

### Push to GitHub

When pushing to GitHub, Ivaldi:
1. Converts `.ivaldimodules` → `.gitmodules`
2. Maps BLAKE3 → Git SHA-1 using dual-hash mapping
3. Creates Git gitlink entries (mode 160000)
4. Pushes submodule references

### Pull from GitHub

When pulling from GitHub, Ivaldi:
1. Parses `.gitmodules`
2. Converts gitlink entries to SubmoduleNodes
3. Maps Git SHA-1 → BLAKE3
4. Creates/updates `.ivaldimodules`

## Differences from Git Submodules

| Feature | Git | Ivaldi |
|---------|-----|--------|
| **Internal reference** | Git SHA-1 | BLAKE3 hash |
| **Configuration** | `.gitmodules` | `.ivaldimodules` |
| **Branch tracking** | Git branches | Ivaldi timelines |
| **Auto-shelving** | Manual (`git stash`) | Automatic |
| **GitHub push** | Native | Converts to Git format |
| **Missing submodules** | Error | Auto-clone during `forge` |

## Examples

### Clone Repo with Nested Submodules

```bash
$ ivaldi download https://github.com/owner/repo

# Automatically handles recursive submodules (depth limit: 10)
```

### Disable Submodule Cloning

```bash
$ ivaldi download https://github.com/owner/repo --recurse-submodules=false

# Clone parent only, skip submodules
```

### Check Submodule Commit References

```bash
$ cat .ivaldimodules

[submodule "lib"]
    path = libs/external
    url = https://github.com/owner/lib
    timeline = main
    commit = 1a2b3c4d5e6f7890...  # BLAKE3 (source of truth)
    git-commit = abc123def456...  # Git SHA-1 (for GitHub sync)
```

## Troubleshooting

### Missing Submodule Directories

If `.ivaldimodules` exists but submodule directories are missing:

```bash
# Will be auto-cloned on next forge
$ ivaldi forge
```

### Submodule URL Changed

If submodule URL was updated in remote `.ivaldimodules`:

```bash
# Manual update (future command)
$ ivaldi submodule sync
```

### Circular Dependencies

Ivaldi detects circular submodule references:

```
Error: Circular submodule reference detected: A → B → A
```

## Future Commands

The following submodule commands are planned for future versions:

```bash
ivaldi submodule add <url> [path]        # Add submodule
ivaldi submodule init [paths...]          # Initialize submodules
ivaldi submodule update [--remote]        # Update to latest/specific commit
ivaldi submodule status                   # Show submodule status
ivaldi submodule remove <path>            # Remove submodule
ivaldi submodule sync                     # Sync URLs from .ivaldimodules
```

## See Also

- [Getting Started](../getting-started.md)
- [Timeline Commands](timeline.md)
- [GitHub Integration](../guides/github-integration.md)
