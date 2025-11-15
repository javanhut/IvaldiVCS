---
layout: default
title: ivaldi upload
---

# ivaldi upload

Push commits to GitHub.

## Synopsis

```bash
ivaldi upload [branch] [flags]
```

## Description

Upload the current timeline to GitHub, creating or updating the corresponding branch. Ivaldi automatically converts your commits to Git format and pushes them with visual progress tracking.

## Flags

- `--force` - Force push to remote (overwrites remote history - use with caution!)

## Prerequisites

1. Portal configured: `ivaldi portal add owner/repo`
2. GitHub authentication (token or CLI)

## Examples

### Basic Upload

```bash
ivaldi upload
```

Uploads the current timeline to GitHub with automatic branch creation and progress tracking.

### Upload to Specific Branch

```bash
ivaldi upload main
```

### Force Push (Overwrite Remote History)

```bash
ivaldi upload --force
```

**Warning:** Force push requires confirmation and will overwrite remote history. Use with extreme caution!

### Complete Workflow

```bash
ivaldi portal add username/my-repo
ivaldi gather .
ivaldi seal "Add feature"
ivaldi upload
```

## What Happens

1. **Detects changes** - Compares local commits with remote state
2. **Optimizes transfer** - Uses delta uploads when possible (only changed files)
3. **Uploads blobs** - Parallel upload of file content to GitHub (with progress bar)
4. **Creates tree** - Constructs Git tree structure
5. **Creates commit** - Generates Git commit with preserved metadata
6. **Updates reference** - Creates or updates the branch on GitHub

## Progress Tracking

The upload command provides real-time progress bars for:

- **File uploads** - Visual progress for uploading blobs to GitHub
- **Parallel processing** - Shows concurrent upload status (up to 32 workers)
- **Completion status** - Clear indication of successful upload

## Performance Features

- **Delta uploads** - Only uploads changed files when updating existing branches
- **Parallel uploads** - Up to 32 concurrent file uploads
- **Smart detection** - Automatically detects if files have changed
- **Rate limit handling** - Respects GitHub API rate limits

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
