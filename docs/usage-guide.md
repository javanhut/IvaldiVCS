# Ivaldi VCS Usage Guide

Ivaldi is a modern version control system designed as a Git alternative with enhanced features like timeline-based branching, content-addressable storage, and seamless GitHub integration.

## Table of Contents
- [Installation](#installation)
- [Getting Started](#getting-started)
- [Basic Workflow](#basic-workflow)
- [Timeline Management](#timeline-management)
- [Remote Repository Operations](#remote-repository-operations)
- [Advanced Features](#advanced-features)
- [Command Reference](#command-reference)

## Installation

Build from source:
```bash
git clone https://github.com/javanhut/IvaldiVCS
cd IvaldiVCS
make build
# Add to PATH or use ./ivaldi directly
```

## Getting Started

### Initialize a New Repository

```bash
# Create a new Ivaldi repository
ivaldi forge

# This creates:
# - .ivaldi directory for metadata
# - main timeline (branch) 
# - Initial snapshot of existing files
```

### Import from Git

If you're in an existing Git repository, `ivaldi forge` automatically:
- Imports Git refs as timelines
- Converts Git objects to Ivaldi format
- Preserves commit history

## Basic Workflow

### 1. Check Status

```bash
# View current timeline and file changes
ivaldi status

# Get detailed information about current timeline
ivaldi whereami
# or short form:
ivaldi wai
```

**Status** shows:
- Current timeline (branch)
- Modified files
- Staged files
- Untracked files

**Whereami** shows:
- Timeline name and type
- Last seal with human-friendly name (e.g., "swift-eagle-flies-high-447abe9b")
- Remote sync status
- Workspace status summary

### 2. Stage Files (Gather)

```bash
# Stage specific files
ivaldi gather file1.txt src/file2.js

# Stage all files in directory
ivaldi gather .

# Stage all modified files
ivaldi gather
```

### 3. Create Commit (Seal)

```bash
# Commit staged files with a message (generates unique seal name)
ivaldi seal "Add new feature"
# Output: Created seal: swift-eagle-flies-high-447abe9b (447abe9b)

# List all seals with their names
ivaldi seals list

# Show detailed information about a seal
ivaldi seals show swift-eagle-flies-high-447abe9b
# Or use partial matching:
ivaldi seals show swift-eagle
ivaldi seals show 447a
```

### 4. View History

```bash
# List all timelines
ivaldi timeline list
```

## Timeline Management

Timelines are Ivaldi's equivalent to Git branches, but with enhanced features.

### Create Timeline

```bash
# Branch from current timeline
ivaldi timeline create feature-auth

# Branch from specific timeline
ivaldi timeline create hotfix-bug main
```

### Switch Timeline

```bash
# Switch to another timeline
ivaldi timeline switch main

# Auto-shelving preserves uncommitted changes
```

### List Timelines

```bash
# Show all timelines with their status
ivaldi timeline list
```

### Remove Timeline

```bash
# Delete a timeline
ivaldi timeline remove old-feature
```

## Remote Repository Operations

### Connect to GitHub

```bash
# Add GitHub repository connection
ivaldi portal add owner/repo

# List configured repositories
ivaldi portal list

# Remove repository connection
ivaldi portal remove owner/repo
```

### Download from GitHub

```bash
# Clone a GitHub repository
ivaldi download owner/repo

# Clone to specific directory
ivaldi download owner/repo my-project
```

### Discover Remote Timelines (Scout)

```bash
# See available remote branches
ivaldi scout

# Refresh remote information
ivaldi scout --refresh
```

### Download Remote Timelines (Harvest)

```bash
# Download all new remote timelines
ivaldi harvest

# Download specific timelines
ivaldi harvest feature-auth bugfix-db

# Update existing + download new
ivaldi harvest --update
```

### Upload Changes

```bash
# Push current timeline to GitHub
ivaldi upload

# Upload creates/updates the branch on GitHub
```

## Advanced Features

### Auto-Shelving

When switching timelines, Ivaldi automatically:
1. Saves uncommitted changes to a shelf
2. Switches to the target timeline
3. Restores shelved changes when returning

This prevents work loss and eliminates manual stashing.

### Content-Addressable Storage

- Files are stored using BLAKE3 hashing
- Automatic deduplication
- Efficient storage of large repositories

### Ignore Files

Create `.ivaldiignore` file (similar to `.gitignore`):
```
# Ignore build artifacts
build/
dist/
*.exe

# Ignore temporary files
*.tmp
.DS_Store
```

## Command Reference

### Repository Management

| Command | Description |
|---------|-------------|
| `ivaldi forge` | Initialize new repository |
| `ivaldi status` | Show working directory status |
| `ivaldi whereami` or `ivaldi wai` | Show current timeline details |

### File Operations

| Command | Description |
|---------|-------------|
| `ivaldi gather [files...]` | Stage files for commit |
| `ivaldi seal <message>` | Create commit with auto-generated seal name |
| `ivaldi seals list` | List all seals with their names |
| `ivaldi seals show <name\|hash>` | Show detailed seal information |

### Timeline Operations

| Command | Description |
|---------|-------------|
| `ivaldi timeline create <name> [from]` | Create new timeline |
| `ivaldi timeline switch <name>` | Switch to timeline |
| `ivaldi timeline list` | List all timelines |
| `ivaldi timeline remove <name>` | Delete timeline |

### Remote Operations

| Command | Description |
|---------|-------------|
| `ivaldi portal add <owner/repo>` | Add GitHub connection |
| `ivaldi portal list` | List connections |
| `ivaldi portal remove <owner/repo>` | Remove connection |
| `ivaldi download <url> [dir]` | Clone repository |
| `ivaldi upload` | Push to GitHub |
| `ivaldi scout` | Discover remote timelines |
| `ivaldi harvest [names...]` | Download timelines |

## Workflow Examples

### Feature Development

```bash
# Check where you are
ivaldi whereami
# Output: Timeline: main
#         Type: Local Timeline
#         Last Commit: a1b2c3d4 (2 hours ago)
#         Message: "Initial commit"
#         Remote: owner/repo (up to date)
#         Workspace: Clean

# Start new feature
ivaldi timeline create feature-login
ivaldi gather src/auth.js src/login.js
ivaldi seal "Implement login functionality"

# Check your progress
ivaldi wai
# Output: Timeline: feature-login
#         Last Commit: e5f6g7h8 (just now)
#         Message: "Implement login functionality"
#         Workspace: Clean

# Switch back to main
ivaldi timeline switch main
# Your feature work is auto-shelved

# Return to feature
ivaldi timeline switch feature-login
# Work is restored automatically
```

### Collaborative Workflow

```bash
# Check for new remote branches
ivaldi scout

# Download teammate's branch
ivaldi harvest feature-payments

# Switch to review
ivaldi timeline switch feature-payments

# Make changes and push back
ivaldi gather src/payments.js
ivaldi seal "Fix payment validation"
ivaldi upload
```

### Repository Sync

```bash
# Update all timelines with remote
ivaldi harvest --update

# Check what changed
ivaldi timeline list
ivaldi status
```

## Best Practices

1. **Commit Frequently**: Small, focused commits are easier to manage
2. **Use Descriptive Timeline Names**: `feature-user-auth` not `feature1`
3. **Regular Scouting**: Check `ivaldi scout` to stay updated
4. **Selective Harvesting**: Only download timelines you need
5. **Clean Up**: Remove old timelines with `ivaldi timeline remove`

## Troubleshooting

### Not in Ivaldi Repository
```bash
Error: not in an Ivaldi repository
Solution: Run 'ivaldi forge' to initialize
```

### No GitHub Connection
```bash
Error: no GitHub repository configured
Solution: Use 'ivaldi portal add owner/repo'
```

### Timeline Already Exists
```bash
Error: timeline 'main' already exists
Solution: Use different name or remove existing
```

### Authentication Issues
```bash
Error: GitHub authentication failed
Solution: Set GITHUB_TOKEN environment variable
         or use 'gh auth login'
```

## Comparison with Git

| Feature | Git Command | Ivaldi Command |
|---------|------------|----------------|
| Initialize | `git init` | `ivaldi forge` |
| Stage files | `git add` | `ivaldi gather` |
| Commit | `git commit` | `ivaldi seal` |
| Branch | `git branch` | `ivaldi timeline create` |
| Switch branch | `git checkout` | `ivaldi timeline switch` |
| Clone | `git clone` | `ivaldi download` |
| Push | `git push` | `ivaldi upload` |
| Fetch branches | `git fetch` | `ivaldi harvest` |
| Status | `git status` | `ivaldi status` |

## Key Advantages

1. **Simplified Workflow**: Intuitive command names
2. **Auto-Shelving**: Never lose work when switching timelines
3. **Selective Sync**: Download only branches you need
4. **Modern Hashing**: BLAKE3 for better performance
5. **Clean Architecture**: Content-addressable storage