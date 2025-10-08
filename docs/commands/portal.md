---
layout: default
title: ivaldi portal
---

# ivaldi portal

Manage GitHub repository connections.

## Synopsis

```bash
ivaldi portal add <owner/repo>
ivaldi portal list
ivaldi portal remove <owner/repo>
```

## Description

Portals represent connections to GitHub repositories for remote operations.

## Subcommands

### add

Add a GitHub repository connection:

```bash
ivaldi portal add owner/repo
```

Example:
```bash
ivaldi portal add javanhut/my-project
```

### list

List all configured portals:

```bash
ivaldi portal list
```

### remove

Remove a portal:

```bash
ivaldi portal remove owner/repo
```

## Authentication

Portals require GitHub authentication:

### Option 1: GitHub Token

```bash
export GITHUB_TOKEN="your_token_here"
```

### Option 2: GitHub CLI

```bash
gh auth login
```

## Examples

### Connect to Repository

```bash
ivaldi portal add myusername/my-project
ivaldi upload
```

### List Connections

```bash
$ ivaldi portal list

Configured portals:
  javanhut/IvaldiVCS
  myusername/my-project
```

### Remove Connection

```bash
ivaldi portal remove old-username/old-project
```

## Common Workflows

### Initial Setup

```bash
ivaldi forge
ivaldi gather .
ivaldi seal "Initial commit"
ivaldi portal add username/repo
ivaldi upload
```

### Multiple Remotes

```bash
ivaldi portal add upstream/original
ivaldi portal add myusername/fork
ivaldi upload  # Pushes to first portal
```

## Related Commands

- [upload](upload.md) - Push to GitHub
- [download](download.md) - Clone from GitHub
- [scout](scout.md) - Discover remote timelines
- [harvest](harvest.md) - Fetch timelines

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git remote add origin url` | `ivaldi portal add owner/repo` |
| `git remote -v` | `ivaldi portal list` |
| `git remote remove` | `ivaldi portal remove` |

## Notes

- Portal format: `owner/repo` (not full URL)
- Requires GitHub token or CLI authentication
- First portal is default for upload/download
