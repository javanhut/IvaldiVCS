# Portal Commands

Portal commands in Ivaldi VCS allow you to manage GitHub repository connections for your local Ivaldi repositories. These commands provide visibility and control over which remote repositories your local repository is connected to.

## Overview

Portal commands solve the problem of "Where will my uploads go?" by providing:
- **Visibility**: See which repository you're connected to
- **Configuration**: Set up connections for existing repositories
- **Management**: Change or remove connections as needed
- **Safety**: Always know where your code will be uploaded

## Commands

### `ivaldi portal list`

Shows the current repository connections and status.

```bash
ivaldi portal list
```

**Example Output:**
```
Repository Connections:
  GitHub: myuser/myproject
  Current Timeline: feature-branch
  Upload Command: ivaldi upload
```

**When no connections exist:**
```
No GitHub repository connections configured.
Use 'ivaldi portal add owner/repo' to add one.
```

### `ivaldi portal add <owner/repo>`

Adds or updates a GitHub repository connection.

```bash
# Add repository connection
ivaldi portal add myuser/myproject

# GitHub prefix is optional
ivaldi portal add github:myuser/myproject
```

**Features:**
- Accepts both `owner/repo` and `github:owner/repo` formats
- Updates existing connections (overwrites previous configuration)
- Validates repository format before saving
- Provides confirmation when successful

**Example:**
```bash
$ ivaldi portal add myuser/awesome-project
[OK] Added GitHub repository connection: myuser/awesome-project
```

### `ivaldi portal remove`

Removes the current GitHub repository connection.

```bash
ivaldi portal remove
```

**Features:**
- Shows which connection is being removed
- Safe to run even when no connection exists
- Provides guidance on what to do next

**Example:**
```bash
$ ivaldi portal remove
[OK] Removed GitHub repository connection: myuser/myproject
You can now use 'ivaldi portal add owner/repo' to configure a new connection.
```

## Use Cases

### 1. Setting Up Existing Repositories

If you have an Ivaldi repository that wasn't created via `ivaldi download`:

```bash
# In your existing Ivaldi repository
ivaldi portal add myuser/myproject
ivaldi upload  # Now works automatically
```

### 2. Switching Between Repositories

If you need to upload to a different repository:

```bash
# Check current connection
ivaldi portal list

# Change to different repository
ivaldi portal add myuser/different-repo

# Upload to new repository
ivaldi upload
```

### 3. Temporary Repository Override

For one-time uploads to a different repository without changing configuration:

```bash
# Upload to different repo without changing config
ivaldi upload github:temp/repo

# Verify your normal config is still intact
ivaldi portal list
```

### 4. Repository Cleanup

When you no longer want a repository connection:

```bash
# Remove connection
ivaldi portal remove

# Verify it's gone
ivaldi portal list
```

## Integration with Other Commands

### Download Command

When you use `ivaldi download` with a GitHub URL, the portal connection is automatically configured:

```bash
ivaldi download owner/repo
cd repo
ivaldi portal list  # Shows: GitHub: owner/repo
```

### Upload Command

The upload command uses portal configuration by default:

```bash
ivaldi upload         # Uses portal configuration
ivaldi upload main    # Uses portal config, specific branch
```

You can still override the repository:

```bash
ivaldi upload github:other/repo      # Override repository
ivaldi upload github:other/repo main # Override repository and branch
```

## Best Practices

### 1. Always Check Connections

Before uploading, especially in new repositories:

```bash
ivaldi portal list
ivaldi upload
```

### 2. Use Descriptive Repository Names

Choose clear repository names that make it obvious where code is going:

```bash
ivaldi portal add company/production-api     # Clear
ivaldi portal add company/api                # Less clear
```

### 3. Clean Up Unused Connections

Remove connections for repositories you no longer use:

```bash
ivaldi portal remove
```

### 4. Verify Before Major Uploads

For important changes, double-check your configuration:

```bash
ivaldi portal list
# Verify the repository and timeline are correct
ivaldi upload
```

## Troubleshooting

### "Not in an Ivaldi repository"

Make sure you're in a directory with a `.ivaldi` folder:

```bash
$ ivaldi portal list
Error: not in an Ivaldi repository (no .ivaldi directory found)

# Solution: Navigate to an Ivaldi repository or create one
cd my-repo        # or
ivaldi forge      # to create new repository
```

### Invalid Repository Format

Repository must be in `owner/repo` format:

```bash
$ ivaldi portal add invalid-format
Error: invalid repository format. Use: owner/repo

# Correct formats:
ivaldi portal add owner/repo
ivaldi portal add github:owner/repo
```

### Configuration Persistence

Portal configurations are stored locally in the `.ivaldi/objects.db` file. If this file is deleted or corrupted, you'll need to reconfigure:

```bash
ivaldi portal add owner/repo
```

## Security Considerations

- Portal configurations are stored locally and not uploaded with your code
- Repository connections only affect where uploads go, not authentication
- Use GitHub authentication methods (tokens, SSH keys) for actual access control
- Portal commands never transmit data to GitHub - they only manage local configuration

## Advanced Usage

### Multiple Repositories Workflow

For projects that need to push to multiple repositories:

```bash
# Work on main repository
ivaldi portal add company/main-repo
ivaldi upload

# Push to fork for PR
ivaldi upload github:myuser/main-repo feature-branch

# Push to staging
ivaldi upload github:company/staging-repo

# Configuration remains unchanged
ivaldi portal list  # Still shows: company/main-repo
```

### Scripting with Portal Commands

Portal commands are designed to be script-friendly:

```bash
#!/bin/bash
# Script to ensure proper repository connection

EXPECTED_REPO="company/production"
CURRENT_REPO=$(ivaldi portal list 2>/dev/null | grep "GitHub:" | cut -d: -f2 | xargs)

if [[ "$CURRENT_REPO" != "$EXPECTED_REPO" ]]; then
    echo "Configuring repository: $EXPECTED_REPO"
    ivaldi portal add "$EXPECTED_REPO"
fi

ivaldi upload
```