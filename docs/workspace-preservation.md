# Workspace Preservation in Ivaldi VCS

## Overview

Ivaldi VCS automatically preserves your complete workspace state when switching between timelines, including all untracked files, modified files, and directory structures. This ensures that each timeline maintains its own independent working environment.

## How It Works

### Automatic Shelving

When you switch from one timeline to another, Ivaldi automatically:

1. **Preserves the current workspace state** - All files in your current workspace (tracked, untracked, and modified) are automatically shelved
2. **Restores the target timeline's state** - Any previously shelved changes for the target timeline are automatically restored

This process is completely transparent and requires no manual intervention.

### Example Workflow

```bash
# On main timeline, create a file
$ echo "main content" > main.txt
$ ivaldi timeline create feature

# On feature timeline, create feature-specific files
$ echo "feature work" > feature.txt
$ echo "more work" > temp.txt

# Switch back to main - feature files are automatically preserved
$ ivaldi timeline switch main
Auto-shelved 2 changes from timeline 'feature' (shelf: auto_feature_xxxxx)
# main.txt is here, but feature.txt and temp.txt are gone

# Switch back to feature - all files are restored
$ ivaldi timeline switch feature  
Restoring auto-shelved changes for timeline 'feature' (shelf: auto_feature_xxxxx)
# main.txt, feature.txt, and temp.txt are all present
```

## Key Features

### Complete Workspace Preservation

- **All files are preserved**: Both tracked and untracked files
- **Directory structures maintained**: Empty directories are preserved
- **File metadata preserved**: Timestamps and permissions are maintained

### Per-Timeline Workspace State

Each timeline maintains its own workspace state independently:
- Untracked files created on a timeline stay with that timeline
- Modified files are preserved in their modified state
- Deleted files remain deleted when you return to the timeline

### Transparent Operation

- No manual stashing required
- Automatic restoration when switching back
- Clear feedback about what's being preserved

## Technical Details

### Storage

Workspace states are stored in `.ivaldi/shelves/` as auto-shelves. Each auto-shelf contains:
- Complete workspace index
- Base timeline state for computing diffs
- Metadata about when and why it was created

### Clean Workspace Behavior

Even if your workspace appears "clean" (no changes from committed state), Ivaldi still preserves the exact workspace state to ensure consistency across timeline switches.

## Best Practices

1. **Let auto-shelving work for you** - Don't manually manage workspace state between timelines
2. **Commit regularly** - While workspace preservation protects your work, commits provide permanent history
3. **Use descriptive timeline names** - This helps track which workspace belongs to which timeline

## Limitations

- Auto-shelves are temporary and can be replaced by newer auto-shelves for the same timeline
- Very large workspaces may take longer to switch between
- Binary files are preserved but may consume significant storage

## Related Commands

- `ivaldi timeline switch <name>` - Switch timelines with automatic workspace preservation
- `ivaldi timeline create <name>` - Create new timeline (preserves current workspace if switching)
- `ivaldi status` - Check current timeline and workspace state