# Simplified Upload Workflow

## Overview

Ivaldi VCS now provides a streamlined upload experience that eliminates the need to think about remote URLs, upstreams, or complex configurations. After downloading a repository from GitHub, uploading changes is as simple as running `ivaldi upload`.

## How It Works

### 1. Initial Setup (One Time)

When you download a repository from GitHub, Ivaldi automatically:
- Detects that it's a GitHub repository
- Stores the owner/repository information in the local configuration
- Sets up the repository for seamless uploads

```bash
# Download and auto-configure
ivaldi download owner/repo
cd repo
```

### 2. Making Changes

Work with your files normally:
```bash
# Edit files
vim README.md

# Stage changes
ivaldi gather README.md

# Create commit
ivaldi seal "Update documentation"
```

### 3. Uploading (The Magic)

Now uploading is effortless:
```bash
# Upload to the same repository and branch you're working on
ivaldi upload
```

That's it! No need to:
- Remember the GitHub URL
- Specify `github:owner/repo`
- Configure upstream branches
- Think about remotes

## Advanced Usage

### Different Branch

Upload to a different branch:
```bash
ivaldi upload main           # Upload current timeline to main branch
ivaldi upload feature-xyz    # Upload current timeline to feature-xyz branch
```

### Different Repository (Override)

Override the configured repository:
```bash
ivaldi upload github:different/repo           # Upload to different repo, current timeline
ivaldi upload github:different/repo main     # Upload to different repo and branch
```

### Timeline Management

Switch between different timelines (branches) and upload them:
```bash
# Switch to different timeline
ivaldi timeline switch feature-branch

# Upload the feature branch
ivaldi upload
```

## Benefits

1. **Zero Configuration**: No setup required after initial download
2. **Intuitive**: Works the way you expect it to
3. **Safe**: Uses the repository you downloaded from by default
4. **Flexible**: Can still override when needed
5. **Consistent**: Same pattern works for all repositories

## Error Handling

If you haven't downloaded from GitHub or the configuration is missing:
```bash
$ ivaldi upload
Error: no GitHub repository configured and none specified. Use 'ivaldi download' from GitHub first or specify 'github:owner/repo'
```

This ensures you always know what repository you're uploading to.

## Comparison with Git

| Operation | Git | Ivaldi |
|-----------|-----|---------|
| Clone | `git clone https://github.com/owner/repo` | `ivaldi download owner/repo` |
| Push | `git push origin main` | `ivaldi upload` |
| Push to different branch | `git push origin feature` | `ivaldi upload feature` |
| Setup required | Yes (remotes, upstream) | No (automatic) |

## Portal Commands

For managing repository connections, Ivaldi provides portal commands:

### View Current Connection
```bash
ivaldi portal list
```
Shows the currently configured GitHub repository and timeline.

### Add/Change Connection  
```bash
ivaldi portal add owner/repo
```
Configure or update the GitHub repository connection.

### Remove Connection
```bash
ivaldi portal remove
```
Remove the current GitHub repository connection.

## Migration

If you have existing Ivaldi repositories that weren't downloaded from GitHub:

1. **Option 1**: Re-download from GitHub to get automatic configuration
2. **Option 2**: Use `ivaldi portal add owner/repo` to configure the connection
3. **Option 3**: Use the explicit syntax: `ivaldi upload github:owner/repo`

The explicit syntax still works and can be used when you need full control.