---
layout: default
title: ivaldi forge
---

# ivaldi forge

Initialize a new Ivaldi repository or import an existing Git repository.

## Synopsis

```bash
ivaldi forge
```

## Description

The `forge` command creates a new Ivaldi repository in the current directory. It:
- Creates a `.ivaldi` directory for metadata and object storage
- Initializes the main timeline
- Sets up content-addressable storage
- Creates initial workspace index

If run in an existing Git repository, it automatically imports:
- Git refs as Ivaldi timelines
- Git objects to Ivaldi format
- Commit history with preserved metadata

## Examples

### Initialize New Repository

```bash
mkdir my-project
cd my-project
ivaldi forge
```

Output:
```
Initialized empty Ivaldi repository in /path/to/my-project/.ivaldi
Created timeline: main
```

### Import Existing Git Repository

```bash
cd existing-git-repo
ivaldi forge
```

Output:
```
Importing from Git repository...
Imported 3 branches as timelines
Converted 42 commits
Initialized Ivaldi repository in /path/to/existing-git-repo/.ivaldi
```

## What Gets Created

After running `ivaldi forge`, your directory contains:

```
.ivaldi/
├── objects/        # Content-addressable storage
├── refs/          # Timeline references
├── mmr.db         # Merkle Mountain Range database
├── config         # Repository configuration
└── HEAD          # Current timeline pointer
```

## Configuration

After initialization, configure your identity:

```bash
ivaldi config --set user.name "Your Name"
ivaldi config --set user.email "your.email@example.com"
```

Or use interactive mode:
```bash
ivaldi config
```

## Common Workflows

### Start Fresh Project

```bash
mkdir new-project
cd new-project
ivaldi forge
ivaldi config
echo "# New Project" > README.md
ivaldi gather README.md
ivaldi seal "Initial commit"
```

### Convert Git Repository

```bash
cd existing-git-project
ivaldi forge  # Automatically imports Git history
ivaldi status  # Verify import
ivaldi log --limit 5  # View imported commits
```

## Related Commands

- [config](config.md) - Configure user settings
- [status](status.md) - Check repository state
- [gather](gather.md) - Stage files
- [seal](seal.md) - Create commit

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git init` | `ivaldi forge` |
| Creates `.git/` | Creates `.ivaldi/` |
| Uses SHA-1 | Uses BLAKE3 |

## Notes

- Only run `forge` once per repository
- Safe to run in Git repositories (non-destructive)
- Git repository remains usable alongside Ivaldi
- Default timeline name is "main"
