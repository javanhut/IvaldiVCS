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

- `<url>` - Repository URL (GitHub, GitLab, or any Git server)
- `[directory]` - Optional target directory (defaults to repo name)

## Flags

### General Flags
- `--depth N` - Limit commit history depth (0 for full history, default: 0)
- `--skip-history` - Download only latest snapshot without commit history
- `--include-tags` - Include tags and releases in the import
- `--recurse-submodules` - Automatically clone and convert Git submodules (default: true)

### Platform-Specific Flags
- `--gitlab` - Force GitLab handling (for GitLab instances)
- `--url <URL>` - Custom GitLab instance URL

### Authentication Flags (for Generic Git Servers)
- `--username <user>` - Username for HTTP basic auth
- `--password <pass>` - Password for HTTP basic auth
- `--token <token>` - Personal access token
- `--ssh-key <path>` - Path to SSH private key (default: ~/.ssh/id_rsa)

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

## Generic Git Repository Support

Ivaldi can clone from **any** Git server, not just GitHub and GitLab:

### Supported Protocols

- ✅ **HTTPS** - `https://git.example.com/repo.git`
- ✅ **HTTP** - `http://git.example.com/repo.git`
- ✅ **SSH** - `git@git.example.com:user/repo.git`
- ✅ **Git Protocol** - `git://git.example.com/repo`

### Examples

#### Public Repository (HTTPS)

```bash
# Gitea
ivaldi download https://gitea.com/user/repo.git

# Gogs
ivaldi download https://try.gogs.io/user/repo.git

# Self-hosted GitLab
ivaldi download https://gitlab.mycompany.com/team/project.git

# Bitbucket
ivaldi download https://bitbucket.org/user/repo.git
```

#### Private Repository with Token

```bash
# Using environment variable
export GIT_TOKEN="your_access_token"
ivaldi download https://git.example.com/private/repo.git

# Using CLI flag
ivaldi download https://git.example.com/private/repo.git --token your_access_token
```

#### Private Repository with Basic Auth

```bash
# Using environment variables
export GIT_USERNAME="username"
export GIT_PASSWORD="password"
ivaldi download https://git.example.com/private/repo.git

# Using CLI flags
ivaldi download https://git.example.com/private/repo.git \
  --username myuser --password mypass
```

#### SSH Clone

```bash
# Using default SSH key (~/.ssh/id_rsa)
ivaldi download git@git.example.com:user/repo.git

# Using custom SSH key
ivaldi download git@git.example.com:user/repo.git \
  --ssh-key ~/.ssh/custom_key
```

### Shallow Clone (Generic Git)

```bash
# Clone only last 50 commits
ivaldi download https://git.example.com/large/repo.git --depth 50

# Clone only latest files (no history)
ivaldi download https://git.example.com/huge/repo.git --skip-history
```

### Authentication Methods

#### 1. Environment Variables (Recommended)

```bash
# For token-based auth
export GIT_TOKEN="ghp_xxxxxxxxxxxx"

# For basic auth
export GIT_USERNAME="myusername"
export GIT_PASSWORD="mypassword"

# Then clone
ivaldi download https://git.example.com/repo.git
```

#### 2. Command-Line Flags

```bash
# Token
ivaldi download URL --token TOKEN

# Basic auth
ivaldi download URL --username USER --password PASS

# SSH key
ivaldi download URL --ssh-key /path/to/key
```

#### 3. SSH Keys (Automatic)

Ivaldi automatically tries these SSH keys:
- `~/.ssh/id_rsa`
- `~/.ssh/id_ed25519`

### Supported Git Servers

Tested and working with:

- ✅ **Gitea** - Open-source Git hosting
- ✅ **Gogs** - Lightweight Git service
- ✅ **GitLab (self-hosted)** - Community & Enterprise
- ✅ **Bitbucket** - Cloud and Server
- ✅ **cgit** - Fast web interface
- ✅ **Gerrit** - Code review platform
- ✅ **AWS CodeCommit** - AWS Git hosting
- ✅ **Azure DevOps** - Microsoft Git hosting
- ✅ **Any Git server** - Standard Git protocol support

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

### Authentication Failed (Generic Git)

```
Error: authentication failed - use --token, --username/--password, or --ssh-key
```

Solutions:

**For HTTPS with token:**
```bash
ivaldi download URL --token YOUR_TOKEN
```

**For HTTPS with username/password:**
```bash
ivaldi download URL --username user --password pass
```

**For SSH:**
```bash
ivaldi download git@server.com:user/repo.git --ssh-key ~/.ssh/id_rsa
```

### SSH Key Not Found

```
Error: SSH key not found - use --ssh-key to specify path
```

Solutions:
```bash
# Generate SSH key if you don't have one
ssh-keygen -t ed25519 -C "your_email@example.com"

# Add to Git server (copy public key)
cat ~/.ssh/id_ed25519.pub

# Then clone
ivaldi download git@server.com:user/repo.git
```

### Connection Timeout

```
Error: clone timeout - try using --depth to limit history
```

For large repositories:
```bash
ivaldi download URL --depth 100
```

### Self-Signed Certificate

For Git servers with self-signed SSL certificates:
```bash
# Set environment variable
export GIT_SSL_NO_VERIFY=true
ivaldi download https://git.internal.com/repo.git
```
