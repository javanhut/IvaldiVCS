---
layout: default
title: Command Reference
---

# Command Reference

Complete reference for all Ivaldi commands.

## Quick Reference Table

| Command | Purpose | Git Equivalent |
|---------|---------|----------------|
| [forge](forge.md) | Initialize repository | `git init` |
| [gather](gather.md) | Stage files | `git add` |
| [seal](seal.md) | Create commit | `git commit` |
| [status](status.md) | Show repository status | `git status` |
| [whereami](whereami.md) | Show current position | (custom) |
| [log](log.md) | View commit history | `git log` |
| [diff](diff.md) | Compare changes | `git diff` |
| [reset](reset.md) | Unstage or reset | `git reset` |
| [timeline](timeline.md) | Manage timelines | `git branch` / `git checkout` |
| [travel](travel.md) | Interactive time travel | (interactive `git log` + checkout) |
| [fuse](fuse.md) | Merge timelines | `git merge` |
| [auth](auth.md) | Authenticate with GitHub | (similar to `gh auth`) |
| [portal](portal.md) | Manage GitHub connections | `git remote` |
| [download](download.md) | Clone repository | `git clone` |
| [upload](upload.md) | Push to GitHub | `git push` |
| [scout](scout.md) | Discover remote branches | `git fetch` (metadata) |
| [harvest](harvest.md) | Fetch branches | `git fetch` (data) |
| [config](config.md) | Configure settings | `git config` |
| [exclude](exclude.md) | Ignore files | (edit `.gitignore`) |

## Commands by Category

### Repository Management
- [forge](forge.md) - Initialize a new Ivaldi repository
- [status](status.md) - Display working directory status
- [whereami](whereami.md) - Show current timeline and position
- [config](config.md) - View and modify configuration

### File Operations
- [gather](gather.md) - Stage files for the next seal
- [seal](seal.md) - Create a commit with staged files
- [reset](reset.md) - Unstage files or reset changes
- [exclude](exclude.md) - Add patterns to `.ivaldiignore`

### History and Inspection
- [log](log.md) - View commit history
- [diff](diff.md) - Compare file changes
- [travel](travel.md) - Interactively browse and navigate history

### Timeline Management
- [timeline](timeline.md) - Create, switch, list, and remove timelines
- [fuse](fuse.md) - Merge timelines together

### Remote Operations
- [auth](auth.md) - Authenticate with GitHub using OAuth
- [portal](portal.md) - Manage GitHub repository connections
- [download](download.md) - Clone a repository from GitHub
- [upload](upload.md) - Push commits to GitHub
- [scout](scout.md) - Discover available remote timelines
- [harvest](harvest.md) - Download specific remote timelines

## Command Details

Click on any command above to see detailed documentation including:
- Syntax and options
- Examples and use cases
- Related commands
- Troubleshooting tips

## Common Workflows

### Daily Development
```bash
ivaldi status              # Check what's changed
ivaldi gather .            # Stage changes
ivaldi seal "Description"  # Commit
ivaldi upload             # Push to GitHub
```

### Feature Development
```bash
ivaldi timeline create feature-name  # Create timeline
# ... make changes ...
ivaldi gather .                      # Stage
ivaldi seal "Add feature"           # Commit
ivaldi timeline switch main         # Switch to main
ivaldi fuse feature-name to main    # Merge
```

### Collaboration
```bash
ivaldi scout              # See remote branches
ivaldi harvest branch-name # Fetch branch
ivaldi timeline switch branch-name # Switch to it
# ... review or contribute ...
ivaldi upload            # Push changes
```

## Getting Help

Each command supports the `--help` flag:
```bash
ivaldi <command> --help
```

For additional help, see:
- [Getting Started Guide](../getting-started.md)
- [Workflow Guides](../guides/basic-workflow.md)
- [Core Concepts](../core-concepts.md)
