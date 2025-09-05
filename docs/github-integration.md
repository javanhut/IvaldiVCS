# GitHub Integration for Ivaldi VCS

## Overview

Ivaldi VCS provides native GitHub integration that operates independently from Git while leveraging existing Git credentials for authentication. This allows you to interact with GitHub repositories directly through the GitHub API without requiring Git to be installed.

GitHub functionality is now integrated directly into the standard `ivaldi download` and `ivaldi upload` commands for a more streamlined experience.

## Features

- **Clone repositories** without Git
- **Pull updates** directly from GitHub
- **Push changes** using GitHub API
- **Automatic URL detection** for GitHub repositories
- **Automatic authentication** using existing Git credentials
- **Built-in rate limiting** management
- **Concurrent downloads** for fast cloning

## Authentication

Ivaldi automatically discovers GitHub credentials from multiple sources (in order of priority):

1. **Environment Variable**: `GITHUB_TOKEN`
2. **Git Config**: `git config github.token`
3. **Git Credential Helper**: Standard git credentials
4. **.netrc file**: Machine-specific credentials
5. **GitHub CLI**: `gh` command line tool config

### Setting Up Authentication

Choose one of these methods:

```bash
# Option 1: Environment variable
export GITHUB_TOKEN=your_personal_access_token

# Option 2: Git config
git config --global github.token your_personal_access_token

# Option 3: GitHub CLI
gh auth login

# Option 4: .netrc file
echo "machine github.com password your_token" >> ~/.netrc
```

## Commands

### Portal Commands

Manage GitHub repository connections with portal commands:

```bash
# List current repository connections
ivaldi portal list

# Add or update repository connection
ivaldi portal add owner/repo
ivaldi portal add github:owner/repo        # github: prefix optional

# Remove repository connection
ivaldi portal remove
```

These commands help you:
- See which repository you're connected to
- Configure connections for existing repositories  
- Switch between different repositories
- Remove connections when no longer needed

### Clone/Download from GitHub

The `download` command automatically detects GitHub URLs and uses the GitHub API:

```bash
# Clone using full HTTPS URL
ivaldi download https://github.com/owner/repo [directory]

# Clone using SSH-style URL
ivaldi download git@github.com:owner/repo.git [directory]

# Clone using short format
ivaldi download owner/repo [directory]

# Clone using just the GitHub path
ivaldi download github.com/owner/repo [directory]

# Examples
ivaldi download torvalds/linux linux-kernel
ivaldi download https://github.com/microsoft/vscode
ivaldi download git@github.com:rust-lang/rust.git
```

This will:
- Automatically detect the GitHub URL format
- Create an Ivaldi repository
- Download all files from the default branch
- Create an initial commit with the imported files
- Configure the repository for automatic uploads (no need to specify URLs again)
- Use 8 concurrent workers for fast downloads

### Push/Upload to GitHub

The `upload` command automatically detects the GitHub repository and uploads to it:

```bash
# Push to configured GitHub repository (current timeline/branch)
ivaldi upload

# Push to specific branch  
ivaldi upload main

# Override repository (optional)
ivaldi upload github:owner/repo
ivaldi upload github:owner/repo main
```

**How it works:**
- When you clone a repository with `ivaldi download`, the GitHub repository information is automatically stored
- `ivaldi upload` uses this stored configuration, so you don't need to specify the repository URL again
- The current timeline (branch) is used as the target branch unless specified otherwise
- No need to think about upstreams or remote configurations - it just works!

This will:
- Push the current timeline's latest commit to GitHub
- Create or update the specified branch
- Upload files individually through the GitHub API

Note: This method is suitable for smaller repositories due to API limitations.

## Technical Details

### How It Works

1. **URL Detection**: Automatically recognizes various GitHub URL formats
2. **Authentication Discovery**: Finds credentials from Git config, environment, or credential helpers
3. **API-Based Operations**: Uses GitHub REST API v3 for all operations
4. **Content Addressing**: Files are stored using Ivaldi's BLAKE3-based CAS
5. **Concurrent Processing**: Downloads use 8 parallel workers for speed
6. **Rate Limit Management**: Automatic waiting when rate limited

### Supported URL Formats

The following GitHub URL formats are automatically detected:

- `https://github.com/owner/repo`
- `http://github.com/owner/repo`  
- `git@github.com:owner/repo.git`
- `git@github.com:owner/repo`
- `github.com/owner/repo`
- `owner/repo` (simple format)

All formats can optionally include the `.git` suffix.

### Differences from Git

| Feature | Ivaldi GitHub | Git |
|---------|--------------|-----|
| Protocol | HTTPS REST API | Git protocol/HTTPS |
| Authentication | Token-based | SSH keys/HTTPS |
| Large Files | API limitations | LFS support |
| Performance | Good for small-medium repos | Optimized for all sizes |
| Dependencies | None | Requires Git |
| Incremental Updates | Full tree comparison | Smart protocol |

### API Limitations

- **File Size**: Individual files limited to 100MB via API
- **Rate Limits**: 5000 requests/hour for authenticated users
- **Repository Size**: Best for repositories under 1GB
- **Binary Files**: Base64 encoding adds overhead

### Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Ivaldi    │────▶│ GitHub API   │────▶│   GitHub    │
│   Commands  │     │   Client     │     │   Servers   │
└─────────────┘     └──────────────┘     └─────────────┘
       │                    │
       ▼                    ▼
┌─────────────┐     ┌──────────────┐
│   Ivaldi    │     │     Auth     │
│     CAS     │     │   Discovery  │
└─────────────┘     └──────────────┘
```

## Use Cases

### When to Use Ivaldi GitHub Integration

✅ **Good for:**
- Cloning repositories without installing Git
- Working with small to medium repositories
- Quick file downloads
- Environments where Git cannot be installed
- Automated workflows using GitHub API
- Read-heavy operations

❌ **Consider alternatives for:**
- Very large repositories (>1GB)
- Repositories with many large binary files
- High-frequency push operations
- Complex merge scenarios

## Advanced Usage

### Working with Private Repositories

Ensure your token has appropriate permissions:
- `repo` scope for private repositories
- `public_repo` scope for public repositories only

### Handling Rate Limits

Ivaldi automatically handles rate limiting by:
1. Checking remaining requests before operations
2. Waiting for reset when limit reached
3. Showing wait time to user

### Concurrent Operations

Clone operations use 8 concurrent workers by default for optimal performance. This balances speed with API rate limits.

## Troubleshooting

### Authentication Issues

```bash
# Set up authentication if not configured
export GITHUB_TOKEN=your_token

# Or use GitHub CLI
gh auth login
```

### Rate Limit Issues

If you encounter rate limit issues:
- Wait for the reset time shown in error message
- Use authentication to get higher limits (5000 vs 60 requests/hour)
- Consider using a different token

### Connection Issues

- Verify internet connectivity
- Check GitHub API status: https://www.githubstatus.com/
- Try with curl to isolate issues

## Security Considerations

- **Token Storage**: Store tokens securely, never commit them
- **Permissions**: Use minimum required scopes
- **Credential Priority**: Environment variables override config files
- **HTTPS Only**: All communications use HTTPS

## Command Reference

All GitHub functionality is integrated into the main Ivaldi commands:

| Function | Command | Description |
|----------|---------|-------------|
| Clone/Download | `ivaldi download owner/repo` | Download from GitHub and auto-configure |
| Upload/Push | `ivaldi upload` | Upload to configured GitHub repository |
| Upload/Push | `ivaldi upload github:owner/repo` | Upload to specific repository |
| View Connection | `ivaldi portal list` | See current repository connections |
| Configure Connection | `ivaldi portal add owner/repo` | Configure repository connection |
| Remove Connection | `ivaldi portal remove` | Remove repository connection |

## Future Enhancements

- Pull functionality integration into standard commands
- Incremental pull (only changed files)
- Shallow cloning support
- GraphQL API integration for better performance
- Caching layer for frequently accessed content
- Support for GitHub Enterprise
- Webhook integration for real-time updates