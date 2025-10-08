---
layout: default
title: ivaldi download
---

# ivaldi download

Clone a repository from GitHub.

## Synopsis

```bash
ivaldi download <owner/repo> [directory]
```

## Description

Clone a GitHub repository to your local machine.

## Arguments

- `<owner/repo>` - GitHub repository to clone
- `[directory]` - Optional target directory (defaults to repo name)

## Examples

### Basic Clone

```bash
ivaldi download javanhut/IvaldiVCS
cd IvaldiVCS
```

### Clone to Specific Directory

```bash
ivaldi download javanhut/IvaldiVCS my-project
cd my-project
```

## Authentication

Requires GitHub authentication for private repositories:

```bash
export GITHUB_TOKEN="your_token"
# or
gh auth login
```

## What Gets Downloaded

- Default branch (usually main)
- Commit history
- Portal configuration (automatic)

## After Cloning

```bash
ivaldi download owner/repo
cd repo

# See status
ivaldi whereami

# List available remote timelines
ivaldi scout

# Download other branches
ivaldi harvest feature-branch
```

## Common Workflows

### Clone and Contribute

```bash
ivaldi download username/project
cd project
ivaldi timeline create my-feature
# ... make changes ...
ivaldi gather .
ivaldi seal "Add feature"
ivaldi upload
```

### Clone and Explore

```bash
ivaldi download username/project
cd project
ivaldi log
ivaldi scout
ivaldi harvest --all
```

## Related Commands

- [portal](portal.md) - Manage connections
- [upload](upload.md) - Push changes
- [scout](scout.md) - Discover branches
- [harvest](harvest.md) - Fetch branches

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git clone url` | `ivaldi download owner/repo` |
| Full URL required | Short format: owner/repo |

## Troubleshooting

### Repository Not Found

```
Error: repository not found
```

Solutions:
- Check repository name spelling
- Verify you have access
- Authenticate for private repos
