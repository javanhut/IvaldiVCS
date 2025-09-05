# Initial Commit Behavior in Ivaldi VCS

## Overview
When initializing a new Ivaldi repository with `ivaldi forge`, the system now automatically creates an initial commit if files exist in the working directory. This ensures timeline state always matches the workspace state.

## Behavior Details

### Empty Repository
When `ivaldi forge` is run in an empty directory:
- Creates the `main` timeline with zero hashes (empty state)
- No initial commit is created
- Workspace remains empty

### Repository with Existing Files
When `ivaldi forge` is run in a directory containing files:
- Creates the `main` timeline initially with zero hashes
- Snapshots all existing files as blob objects
- Creates an initial commit containing all workspace files
- Updates the `main` timeline to point to this initial commit
- Timeline state now matches workspace state

## Timeline Branching

### Branching from Non-Empty Timeline
When creating a new timeline from a timeline with commits:
- New timeline inherits the parent's commit hash
- Files are materialized from the parent timeline's committed state
- Workspace matches the parent timeline's state

### Branching from Empty Timeline
When creating a new timeline from an empty `main` timeline:
- If workspace has files, creates a commit from workspace state
- New timeline points to this commit
- Ensures files are preserved when switching between timelines

## File Materialization

### Timeline Switching
When switching between timelines:
- Current workspace changes are auto-shelved
- Target timeline's committed state is materialized
- Auto-shelved changes for the target timeline are restored if they exist

### Empty Timeline Handling
When switching to a timeline with zero hashes (truly empty):
- Workspace is cleared to match the empty state
- This only occurs for timelines that have never had any commits

## Benefits
1. **Consistency**: Timeline state always matches what's actually committed
2. **Predictability**: File materialization behaves as expected
3. **Safety**: No unexpected file loss when switching timelines
4. **Clarity**: Clear distinction between committed and uncommitted changes

## Technical Implementation
The fix involved:
1. Adding `createInitialCommit()` helper function in `cli/utils.go`
2. Modifying `forgeCommand` in `cli/cli.go` to create initial commit when files exist
3. Updating timeline creation logic to properly handle empty parent timelines
4. Ensuring workspace materialization correctly interprets empty vs non-empty timelines