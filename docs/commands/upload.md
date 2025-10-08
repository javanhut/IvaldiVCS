---
layout: default
title: ivaldi upload
---

# ivaldi upload

Push commits to GitHub.

## Synopsis

```bash
ivaldi upload
```

## Description

Upload the current timeline to GitHub, creating or updating the corresponding branch.

## Prerequisites

1. Portal configured: `ivaldi portal add owner/repo`
2. GitHub authentication (token or CLI)

## Examples

### Basic Upload

```bash
ivaldi upload
```

### Complete Workflow

```bash
ivaldi portal add username/my-repo
ivaldi gather .
ivaldi seal "Add feature"
ivaldi upload
```

## What Happens

1. Converts Ivaldi seals to Git commits
2. Pushes to GitHub repository
3. Creates/updates branch matching timeline name

## Authentication

### GitHub Token

```bash
export GITHUB_TOKEN="ghp_your_token"
ivaldi upload
```

### GitHub CLI

```bash
gh auth login
ivaldi upload
```

## Common Workflows

### Daily Workflow

```bash
# Make changes
ivaldi gather .
ivaldi seal "Daily progress"
ivaldi upload
```

### Feature Branch

```bash
ivaldi timeline create feature-x
# ... work ...
ivaldi gather .
ivaldi seal "Add feature X"
ivaldi upload  # Creates 'feature-x' branch on GitHub
```

## Related Commands

- [portal](portal.md) - Configure GitHub connection
- [download](download.md) - Clone repository
- [seal](seal.md) - Create commits to upload

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git push` | `ivaldi upload` |
| `git push -u origin branch` | `ivaldi upload` (automatic) |

## Troubleshooting

### No Portal Configured

```
Error: no GitHub repository configured
```

Solution:
```bash
ivaldi portal add owner/repo
```

### Authentication Failed

```
Error: GitHub authentication failed
```

Solution:
```bash
export GITHUB_TOKEN="your_token"
# or
gh auth login
```
