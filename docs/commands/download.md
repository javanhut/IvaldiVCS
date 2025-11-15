---
layout: default
title: ivaldi download
---

# ivaldi download

Clone a repository from GitHub.

## Synopsis

```bash
ivaldi download <owner/repo> [directory] [flags]
```

## Description

Clone a GitHub repository to your local machine with full commit history and convert it to Ivaldi format.

## Arguments

- `<owner/repo>` - GitHub repository to clone
- `[directory]` - Optional target directory (defaults to repo name)

## Flags

- `--depth N` - Limit commit history depth (0 for full history, default: 0)
- `--skip-history` - Download only latest snapshot without commit history
- `--include-tags` - Include tags and releases in the import
- `--recurse-submodules` - Automatically clone and convert Git submodules (default: true)

## Examples

### Basic Clone (Full History)

```bash
ivaldi download javanhut/IvaldiVCS
cd IvaldiVCS
```

This will download the complete commit history and convert it to Ivaldi format with progress bars showing the status.

### Clone to Specific Directory

```bash
ivaldi download javanhut/IvaldiVCS my-project
cd my-project
```

### Clone with Limited History

```bash
# Download only the last 50 commits
ivaldi download javanhut/IvaldiVCS --depth 50
```

### Clone Latest Snapshot Only

```bash
# Skip history, download only current state
ivaldi download javanhut/IvaldiVCS --skip-history
```

### Clone with Tags and Releases

```bash
# Include all tags and releases
ivaldi download javanhut/IvaldiVCS --include-tags
```

## Authentication

Requires GitHub authentication for private repositories:

```bash
export GITHUB_TOKEN="your_token"
# or
gh auth login
```

## What Gets Downloaded

- **Full commit history** - All commits from the default branch (unless limited by --depth)
- **Complete file tree** - All files from the latest commit
- **Commit metadata** - Author, committer, timestamps, and messages preserved
- **Git SHA mappings** - Bidirectional mapping between Git SHA1 and Ivaldi BLAKE3 hashes
- **Portal configuration** - Automatic remote configuration for upload/sync
- **Tags and releases** - When --include-tags is specified

## Progress Tracking

The download command provides real-time progress bars for:

- **Commit history fetching** - Shows pagination progress for large repositories
- **Commit processing** - Visual progress for converting commits to Ivaldi format
- **File downloads** - Parallel download progress with count and completion percentage

## Performance Features

- **Parallel downloads** - Up to 32 concurrent file downloads
- **Delta detection** - Skips files that already exist locally
- **Rate limit handling** - Automatic waiting when GitHub API limits are reached
- **Optimized pagination** - Efficient fetching of commit history (100 commits per page)

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
