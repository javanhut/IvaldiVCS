# Scout and Harvest Commands

## Overview

Ivaldi VCS provides a powerful workflow for discovering and downloading remote timelines (branches) from GitHub repositories. The `scout` and `harvest` commands work together to give you full visibility and control over which remote branches you want to work with locally.

## The Scout-Harvest Workflow

### 1. **Scout** - Discover What's Available
Scan the remote repository to see what new timelines (branches) are available without downloading them.

### 2. **Harvest** - Download What You Need
Selectively download specific timelines or all new ones, bringing them into your local Ivaldi repository with full history preservation.

### 3. **Switch** - Start Working
Use the existing timeline commands to switch to and work with harvested timelines.

## Commands

### `ivaldi scout`

Discovers remote timelines available for harvest.

```bash
# Basic usage
ivaldi scout

# Refresh remote timeline information
ivaldi scout --refresh
```

**What it shows:**
- 🆕 **New timelines** - Available on remote but not harvested locally
- ✅ **Existing timelines** - Available both locally and remotely  
- 📱 **Local-only timelines** - Exist locally but not on remote
- 📊 **Summary statistics** - Overview of timeline status

**Example Output:**
```
Scouting GitHub repository: owner/awesome-project

📡 Remote Timelines:
  🆕 feature-auth        (new, ready to harvest)
  🆕 bugfix-database     (new, ready to harvest)
  🆕 experimental-ui     (new, ready to harvest)

🏠 Local Timelines:
  ✅ main                (exists locally and remotely)
  ✅ develop             (exists locally and remotely)
  📱 local-experiment    (local only, not on remote)

📊 Summary:
  • Current timeline: main
  • Remote timelines available to harvest: 3
  • Timelines that exist both locally and remotely: 2
  • Local-only timelines: 1
  • Total remote timelines discovered: 5

💡 Next steps:
  • Use 'ivaldi harvest <timeline-name>' to download specific timelines
  • Use 'ivaldi harvest' to download all new timelines
```

### `ivaldi harvest`

Downloads remote timelines into your local repository.

```bash
# Harvest all new remote timelines
ivaldi harvest

# Harvest specific timelines
ivaldi harvest feature-auth
ivaldi harvest feature-auth bugfix-database

# Harvest all new + update existing timelines
ivaldi harvest --update
```

**What it does:**
- Downloads all files and history for the specified timeline(s)
- Creates local timeline references with proper hashes
- Preserves GitHub commit SHAs for reference
- Only downloads the minimal data needed (no redundant files)
- Maintains workspace integrity during the process

**Example Output:**
```
Harvesting from GitHub repository: owner/awesome-project
Discovering remote timelines...

📦 Harvesting timeline: feature-auth
Fetching timeline 'feature-auth' from owner/awesome-project...
Downloaded 23 files
✅ Harvested new timeline: feature-auth

📦 Harvesting timeline: bugfix-database
Fetching timeline 'bugfix-database' from owner/awesome-project...
Downloaded 15 files
✅ Harvested new timeline: bugfix-database

📊 Harvest Summary:
  • Successfully harvested: 2 timelines

💡 Next steps:
  • Use 'ivaldi timeline list' to see all available timelines
  • Use 'ivaldi timeline switch <name>' to switch to a harvested timeline
```

## Usage Patterns

### 1. Discovering New Work

Check what new branches teammates have created:

```bash
# See what's new
ivaldi scout

# If there are interesting branches, harvest them
ivaldi harvest feature-auth experimental-ui

# Switch to work on one
ivaldi timeline switch feature-auth
```

### 2. Staying Up to Date

Regularly sync with remote branches:

```bash
# Check for new branches
ivaldi scout

# Harvest everything new and update existing
ivaldi harvest --update

# See what you have now
ivaldi timeline list
```

### 3. Selective Collaboration

Only work with branches you care about:

```bash
# Discover what's available
ivaldi scout

# Only harvest the branches you need
ivaldi harvest critical-bugfix important-feature

# Ignore experimental branches you don't need locally
```

### 4. Branch Exploration

Try out different branches easily:

```bash
# Scout for interesting branches
ivaldi scout

# Harvest one to try
ivaldi harvest experimental-ui

# Switch to it and explore
ivaldi timeline switch experimental-ui

# Switch back when done
ivaldi timeline switch main
```

## Integration with Other Commands

### Portal Commands
Scout and harvest require a configured GitHub repository:

```bash
# Check current repository configuration
ivaldi portal list

# Configure if needed
ivaldi portal add owner/repo
```

### Timeline Commands
After harvesting, use timeline commands normally:

```bash
# List all timelines (including harvested ones)
ivaldi timeline list

# Switch to harvested timeline
ivaldi timeline switch feature-auth

# Create new timeline based on harvested one
ivaldi timeline create my-feature feature-auth
```

### Upload Commands
Upload works normally with harvested timelines:

```bash
# Switch to harvested timeline
ivaldi timeline switch feature-auth

# Make changes
echo "console.log('hello')" > test.js
ivaldi gather test.js
ivaldi seal "Add test file"

# Upload changes back
ivaldi upload
```

## Advanced Usage

### Batch Harvesting with Filtering

Harvest multiple specific timelines:
```bash
# Harvest all feature branches
ivaldi harvest feature-auth feature-payments feature-ui

# Harvest all bug fixes
ivaldi harvest bugfix-login bugfix-database bugfix-cache
```

### Update Existing Timelines

Keep existing timelines synchronized:
```bash
# Update all timelines that exist both locally and remotely
ivaldi harvest --update

# Update specific timeline
ivaldi harvest --update main
```

### Scripting and Automation

Scout and harvest are designed to be script-friendly:

```bash
#!/bin/bash
# Auto-harvest all feature branches

echo "Checking for new feature branches..."
ivaldi scout

# This could be enhanced to parse output and filter
ivaldi harvest --update

echo "All timelines synchronized!"
```

## Technical Details

### Data Efficiency
- **Incremental Downloads**: Only downloads new or changed files
- **Content Addressing**: Uses Ivaldi's CAS to avoid storing duplicate content
- **Minimal Network**: Leverages GitHub's tree API for efficient bulk operations

### Timeline Integrity
- **Hash Preservation**: Maintains proper BLAKE3 hashes for all content
- **SHA Mapping**: Stores GitHub commit SHAs for reference and comparison
- **Workspace Safety**: Protects your current working directory during harvesting

### Conflict Handling
- **Safe Overrides**: Won't overwrite local timelines without `--update` flag
- **Clean Rollback**: If harvest fails, cleans up partial state automatically
- **Status Tracking**: Maintains clear status of local vs remote timelines

## Troubleshooting

### Authentication Issues
```bash
$ ivaldi scout
Error: failed to get remote timelines: authentication required

# Solution: Configure GitHub authentication
export GITHUB_TOKEN=your_token
# or
gh auth login
```

### Repository Configuration
```bash
$ ivaldi scout
Error: no GitHub repository configured

# Solution: Configure repository connection
ivaldi portal add owner/repo
# or download from GitHub first
ivaldi download owner/repo
```

### Network Timeouts
```bash
$ ivaldi harvest
Error: context deadline exceeded

# Large repositories may need more time - this is handled automatically
# but if you consistently see timeouts, check your network connection
```

### Timeline Conflicts
```bash
$ ivaldi harvest main
⚠️  Timeline 'main' already exists locally, skipping (use --update to force)

# Solution: Use --update flag to update existing timelines
ivaldi harvest --update main
```

### Partial Harvest Failures
```bash
$ ivaldi harvest feature-a feature-b
✅ Harvested new timeline: feature-a
❌ Failed to harvest timeline 'feature-b': branch not found

# Solution: Check branch exists with scout first
ivaldi scout
```

## Best Practices

### 1. Regular Scouting
Make scouting part of your regular workflow:
```bash
# At the start of each work session
ivaldi scout
```

### 2. Selective Harvesting
Don't harvest everything - only what you need:
```bash
# Be specific about what you harvest
ivaldi harvest feature-payments feature-auth
# Rather than
ivaldi harvest  # (everything)
```

### 3. Clean Timeline Management
Use descriptive timeline names and clean up when done:
```bash
# After working on harvested timeline
ivaldi timeline remove old-feature
```

### 4. Backup Important Work
Before major harvesting operations:
```bash
# Check current state
ivaldi portal list
ivaldi timeline list

# Then proceed with harvest
ivaldi harvest --update
```

### 5. Verify After Harvesting
Always verify harvest results:
```bash
ivaldi harvest feature-branch
ivaldi timeline list  # Confirm it's there
ivaldi timeline switch feature-branch  # Test switching
```

## Comparison with Git

| Operation | Git | Ivaldi |
|-----------|-----|--------|
| Discover remote branches | `git ls-remote --heads` | `ivaldi scout` |
| Fetch all branches | `git fetch --all` | `ivaldi harvest` |
| Fetch specific branch | `git fetch origin branch:branch` | `ivaldi harvest branch` |
| List branches | `git branch -r` | `ivaldi scout` |
| Switch branch | `git checkout branch` | `ivaldi timeline switch branch` |
| Update tracking | `git remote update` | `ivaldi scout --refresh` |

### Key Advantages

1. **Clarity**: Scout shows exactly what's available before downloading
2. **Selectivity**: Harvest only what you need, when you need it
3. **Safety**: Won't overwrite local work without explicit permission
4. **Efficiency**: Uses content addressing to minimize storage and network usage
5. **Visibility**: Always know what's local vs remote vs new

The scout and harvest workflow gives you complete control over which remote timelines you work with, making collaboration more intentional and efficient.