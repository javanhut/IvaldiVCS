# Whereami Command

The `whereami` command (alias: `wai`) provides detailed information about your current position in the Ivaldi repository. Unlike the basic `status` command that focuses on file changes, `whereami` gives you a comprehensive overview of your timeline context.

## Usage

```bash
# Full command
ivaldi whereami

# Short alias
ivaldi wai
```

## Output Format

The command displays information in the following format:

```
Timeline: feature-auth
Type: Local Timeline
Last Commit: 3d4e5f6a (2 hours ago)
Message: "Add authentication middleware"
Remote: owner/repo (up to date)
Workspace: 2 modified, 1 added
Staged: 3 files ready for seal
```

## Information Displayed

### Timeline Information
- **Timeline Name**: Current timeline (branch) you're working on
- **Type**: Always "Local Timeline" for local branches

### Last Commit Details
- **Commit Hash**: Short 8-character hash of the last commit
- **Time**: Human-readable time since the commit ("just now", "2 hours ago", etc.)
- **Message**: First line of the commit message (truncated if too long)

### Remote Status
- **No GitHub Repository**: Shows "(no GitHub repository configured)"
- **Not Tracked**: Shows "owner/repo (not tracked)" if local timeline has no remote
- **Sync Status**: Shows sync status like "up to date" or "needs comparison"

### Workspace Status
- **Clean**: No uncommitted changes
- **With Changes**: Shows count of added, modified, and deleted files
- **Staged Files**: Shows count of files ready for commit with `seal`

## Examples

### Empty Repository
```bash
$ ivaldi wai
Timeline: main
Type: Local Timeline
Last Commit: (no commits yet)
Remote: (no GitHub repository configured)
Workspace: Clean
```

### Repository with Commits
```bash
$ ivaldi wai
Timeline: feature-login
Type: Local Timeline
Last Commit: a1b2c3d4 (5 minutes ago)
Message: "Implement user authentication"
Remote: myorg/myproject (up to date)
Workspace: Clean
```

### Repository with Uncommitted Changes
```bash
$ ivaldi wai
Timeline: bugfix-validation
Type: Local Timeline
Last Commit: e5f6g7h8 (1 hour ago)
Message: "Fix input validation logic"
Remote: myorg/myproject (needs comparison)
Workspace: 2 modified, 1 added
Staged: 1 files ready for seal
```

## Comparison with Other Commands

| Command | Purpose | Information Shown |
|---------|---------|-------------------|
| `ivaldi status` | File changes | Detailed file-by-file status |
| `ivaldi whereami` | Timeline context | High-level position and summary |
| `ivaldi timeline list` | All timelines | List of all available timelines |

## Use Cases

### 1. Quick Orientation
When returning to a project after time away:
```bash
$ ivaldi wai
# Instantly see what timeline you're on and recent work
```

### 2. Before Making Changes
Check your current position before starting new work:
```bash
$ ivaldi wai
# Verify you're on the right timeline with clean workspace
```

### 3. Workflow Status Check
During development to see progress:
```bash
$ ivaldi wai
# See what's staged, what's changed, and last commit
```

### 4. Remote Sync Verification
Check if you're in sync with remote:
```bash
$ ivaldi wai
# See remote status without doing a full scout/harvest
```

## Integration with Other Commands

The `whereami` command works well with other Ivaldi commands:

```bash
# Check position, then switch timelines
ivaldi wai
ivaldi timeline switch main

# Check position, then gather and seal changes
ivaldi wai
ivaldi gather .
ivaldi seal "Complete feature"

# Check position, then sync with remote
ivaldi wai
ivaldi scout
ivaldi harvest
```

## Time Display Format

The command uses human-readable time formats:
- **just now**: Less than 1 minute ago
- **X minutes ago**: Less than 1 hour ago
- **X hours ago**: Less than 1 day ago
- **X days ago**: Less than 1 week ago
- **X weeks ago**: Less than 1 month ago
- **X months ago**: Less than 1 year ago
- **X years ago**: More than 1 year ago

## Error Handling

The command gracefully handles various error conditions:

- **Not in Ivaldi repository**: Clear error message
- **No commits**: Shows "(no commits yet)"
- **Missing commit data**: Shows hash but indicates read error
- **No remote configured**: Shows appropriate message
- **Workspace scan failures**: Shows error but continues with other info

## Implementation Notes

The `whereami` command:
- Reads timeline information from `.ivaldi/refs/`
- Accesses commit objects from `.ivaldi/objects/`
- Scans workspace for changes using the same logic as other commands
- Checks staging area for prepared commits
- Queries remote configuration and sync status

The command is designed to be fast and informative, providing essential context without overwhelming detail.