---
layout: default
title: Getting Started
---

# Getting Started with Ivaldi

This guide will help you install Ivaldi and create your first repository.

## Installation

### Build from Source

Ivaldi is written in Go. To build from source:

```bash
# Clone the repository
git clone https://github.com/javanhut/IvaldiVCS
cd IvaldiVCS

# Build the binary
go build -o ivaldi .

# Optionally, add to PATH
sudo mv ivaldi /usr/local/bin/
```

### Verify Installation

```bash
ivaldi --version
# Ivaldi VCS
```

## Your First Repository

### Initialize a New Repository

```bash
# Create a new directory
mkdir my-project
cd my-project

# Initialize Ivaldi repository
ivaldi forge
```

This creates a `.ivaldi` directory and sets up the main timeline.

### Configure Your Identity

```bash
# Set your name and email
ivaldi config
# Interactive prompts will guide you
```

Or set directly:
```bash
ivaldi config --set user.name "Your Name"
ivaldi config --set user.email "your.email@example.com"
```

### Create Your First Seal (Commit)

```bash
# Create a README file
echo "# My Project" > README.md

# Stage the file
ivaldi gather README.md

# Create a commit (seal)
ivaldi seal "Initial commit"
# Output: Created seal: swift-eagle-flies-high-447abe9b (447abe9b)
```

Congratulations! You've created your first seal with a memorable name.

### Check Status

```bash
# View repository status
ivaldi status

# See current position
ivaldi whereami
# or shorter:
ivaldi wai
```

## Basic Workflow

### 1. Make Changes

```bash
# Edit files
echo "Hello Ivaldi!" >> README.md

# Check what changed
ivaldi status
```

### 2. Stage Changes

```bash
# Stage specific files
ivaldi gather README.md

# Or stage all changes
ivaldi gather .

# Or gather all (prompts for hidden files)
ivaldi gather
```

### 3. Create a Seal (Commit)

```bash
ivaldi seal "Update README with greeting"
# Created seal: brave-wolf-runs-fast-abc12345
```

### 4. View History

```bash
# See all seals
ivaldi log

# Concise view
ivaldi log --oneline

# Recent seals only
ivaldi log --limit 5
```

## Working with Timelines (Branches)

### Create a Timeline

```bash
# Create a new timeline for a feature
ivaldi timeline create feature-login

# This automatically switches to the new timeline
```

### Switch Timelines

```bash
# Switch back to main
ivaldi timeline switch main

# Your changes are auto-shelved and restored when you return!
```

### List Timelines

```bash
ivaldi timeline list
```

### Merge Timelines

```bash
# Merge feature into main
ivaldi timeline switch main
ivaldi fuse feature-login to main
```

## GitHub Integration

### Connect to GitHub

```bash
# Add GitHub repository
ivaldi portal add owner/repo
```

You'll need a GitHub token with appropriate permissions. Set it as:
```bash
export GITHUB_TOKEN="your_token_here"
```

Or use GitHub CLI:
```bash
gh auth login
```

### Upload (Push) to GitHub

```bash
ivaldi upload
```

### Download (Clone) from GitHub

```bash
ivaldi download owner/repo my-project
cd my-project
```

### Discover and Fetch Remote Timelines

```bash
# See what's available
ivaldi scout

# Download specific timelines
ivaldi harvest feature-payments

# Download all new timelines
ivaldi harvest
```

## Essential Commands Summary

| Command | Purpose | Example |
|---------|---------|---------|
| `ivaldi forge` | Initialize repository | `ivaldi forge` |
| `ivaldi gather` | Stage files | `ivaldi gather README.md` |
| `ivaldi seal` | Create commit | `ivaldi seal "Add feature"` |
| `ivaldi status` | Check status | `ivaldi status` |
| `ivaldi whereami` | Current position | `ivaldi whereami` |
| `ivaldi log` | View history | `ivaldi log --limit 10` |
| `ivaldi timeline create` | New timeline | `ivaldi timeline create feature-x` |
| `ivaldi timeline switch` | Change timeline | `ivaldi timeline switch main` |
| `ivaldi fuse` | Merge timelines | `ivaldi fuse feature to main` |
| `ivaldi portal add` | Connect GitHub | `ivaldi portal add owner/repo` |
| `ivaldi upload` | Push to GitHub | `ivaldi upload` |
| `ivaldi download` | Clone from GitHub | `ivaldi download owner/repo` |

## Quick Start Example

Complete workflow from initialization to pushing to GitHub:

```bash
# Initialize repository
ivaldi forge

# Configure user
ivaldi config --set user.name "Your Name"
ivaldi config --set user.email "you@example.com"

# Create initial files
echo "# My Project" > README.md
echo "This is a test project using Ivaldi VCS" >> README.md

# Stage and commit
ivaldi gather README.md
ivaldi seal "Initial commit"

# Create a feature timeline
ivaldi timeline create add-documentation

# Add more files
echo "# Documentation" > DOCS.md
ivaldi gather DOCS.md
ivaldi seal "Add documentation file"

# Switch back to main
ivaldi timeline switch main

# Merge the feature
ivaldi fuse add-documentation to main

# Connect to GitHub
ivaldi portal add yourusername/your-repo

# Push to GitHub
ivaldi upload
```

## Next Steps

Now that you understand the basics:

- Learn about [Core Concepts](core-concepts.md) behind Ivaldi
- Explore the [Command Reference](commands/index.md) for detailed command documentation
- Read [Workflow Guides](guides/basic-workflow.md) for best practices
- See the [Git Comparison](comparison.md) to understand differences from Git

## Getting Help

- View command help: `ivaldi <command> --help`
- Check current status: `ivaldi status`
- View configuration: `ivaldi config --list`
- Report issues on [GitHub](https://github.com/javanhut/IvaldiVCS/issues)
