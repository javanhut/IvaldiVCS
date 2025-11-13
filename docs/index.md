---
layout: default
title: Home
---

# Ivaldi VCS

**Modern version control for the modern developer**

Ivaldi is a next-generation version control system designed as a Git alternative with enhanced features like timeline-based branching, content-addressable storage, and seamless GitHub integration.

## Why Ivaldi?

- **Intuitive Commands**: Clear, descriptive command names (`forge`, `gather`, `seal`)
- **Timeline-Based Branching**: Enhanced branch management with auto-shelving
- **Butterfly Timelines**: Experimental sandboxes with bidirectional sync and auto-conflict resolution
- **Human-Friendly Commits**: Memorable seal names like "swift-eagle-flies-high-447abe9b"
- **Never Lose Work**: Auto-shelving preserves changes when switching timelines
- **Clean History**: Interactive commit squashing with safety confirmations
- **Selective Sync**: Download only the branches you need
- **Modern Cryptography**: BLAKE3 hashing for security and performance
- **GitHub Integration**: First-class GitHub support built-in
- **Submodule Support**: Automatic Git submodule conversion with dual-hash tracking
- **Interactive Time Travel**: Browse and navigate commits with arrow keys

## Quick Example

```bash
# Initialize repository
ivaldi forge

# Stage and commit
ivaldi gather README.md
ivaldi seal "Initial commit"
# Created seal: swift-eagle-flies-high-447abe9b

# Create experimental timeline (butterfly)
ivaldi timeline butterfly experiment
# Make changes, test safely
ivaldi timeline butterfly up  # Merge back to parent

# Clean up commits before pushing
ivaldi shift --last 3
# Squash into one clean commit

# Create and switch timelines
ivaldi timeline create feature-auth
ivaldi timeline switch main

# Connect to GitHub
ivaldi portal add owner/repo
ivaldi upload
```

## Documentation

### Getting Started
- [Installation & Quick Start](getting-started.md)
- [Core Concepts](core-concepts.md)
- [Git to Ivaldi Migration Guide](guides/migration.md)

### Command Reference
- [All Commands Overview](commands/index.md)
- **Repository**: [forge](commands/forge.md) • [status](commands/status.md) • [whereami](commands/whereami.md) • [config](commands/config.md)
- **Files**: [gather](commands/gather.md) • [seal](commands/seal.md) • [reset](commands/reset.md) • [exclude](commands/exclude.md)
- **History**: [log](commands/log.md) • [diff](commands/diff.md) • [travel](commands/travel.md) • [shift](commands/shift.md)
- **Timelines**: [timeline](commands/timeline.md) • [butterfly](commands/butterfly.md) • [fuse](commands/fuse.md)
- **Remote**: [portal](commands/portal.md) • [download](commands/download.md) • [upload](commands/upload.md) • [sync](sync-command.md) • [scout](commands/scout.md) • [harvest](commands/harvest.md)
- **Submodules**: [submodule](commands/submodule.md)

### Guides
- [Basic Workflow](guides/basic-workflow.md)
- [Timeline Branching](guides/branching.md)
- [Team Collaboration](guides/collaboration.md)
- [GitHub Integration](guides/github-integration.md)
- [Git Migration with Submodules](guides/git-migration-with-submodules.md)

### Reference
- [Comparison with Git](comparison.md)
- [Architecture](architecture.md)

## Feature Highlights

### Timelines Instead of Branches
Timelines are Ivaldi's enhanced version of Git branches:
- **Auto-shelving**: Changes are automatically preserved when switching
- **Workspace isolation**: Each timeline maintains its own state
- **Efficient storage**: Shared content via content-addressable storage

### Butterfly Timelines
Experimental sandboxes for safe development:
- **Lightweight branching**: Create experimental timelines from any parent
- **Bidirectional sync**: Merge changes up to parent or pull parent changes down
- **Nested butterflies**: Chain butterflies for complex feature development
- **Auto-conflict resolution**: Fast-forward merge strategy handles conflicts automatically
```bash
ivaldi timeline butterfly experiment
# Work safely in isolated sandbox
ivaldi timeline butterfly up  # Merge to parent when ready
```

### Human-Friendly Seal Names
Every commit gets a memorable name:
```
swift-eagle-flies-high-447abe9b
```
Much easier to remember than `447abe9b1234567890abcdef`!

### Interactive Commit Squashing
Clean up your history before pushing:
```bash
ivaldi shift --last 5
# Interactive arrow-key selection
# Safe multi-step confirmation
# Clean single commit result
```

### Interactive Time Travel
Browse commits with arrow keys and create branches from any point:
```bash
ivaldi travel
# Navigate with Up/Down arrows
# Press Enter to diverge or overwrite
```

### Intelligent Merging
Chunk-level conflict resolution without polluting your workspace:
```bash
ivaldi fuse feature-auth to main
# No conflict markers in files!
# Clean interactive resolution
```

### Automatic Submodule Support
Seamless Git submodule conversion:
- **Auto-detection**: Automatically finds and converts Git submodules
- **Dual-hash tracking**: BLAKE3 for Ivaldi, Git SHA-1 for GitHub compatibility
- **Timeline-aware**: Submodules track per-timeline state
```bash
ivaldi download owner/repo-with-submodules
# Automatically clones and converts all submodules
```

## Quick Reference

| Git Command | Ivaldi Command |
|-------------|----------------|
| `git init` | `ivaldi forge` |
| `git add` | `ivaldi gather` |
| `git commit` | `ivaldi seal` |
| `git branch` | `ivaldi timeline create` |
| `git checkout` | `ivaldi timeline switch` |
| `git merge` | `ivaldi fuse` |
| `git rebase -i` | `ivaldi shift` |
| `git clone` | `ivaldi download` |
| `git push` | `ivaldi upload` |
| `git pull` | `ivaldi sync` |
| `git fetch` | `ivaldi harvest` |
| `git status` | `ivaldi status` |
| `git log` | `ivaldi log` |
| `git submodule` | `ivaldi submodule` (auto) |

## Get Started

Ready to try Ivaldi? Head to the [Getting Started Guide](getting-started.md) to begin.

## Repository

View the source code and contribute on [GitHub](https://github.com/javanhut/IvaldiVCS).
