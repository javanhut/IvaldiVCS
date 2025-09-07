# Staging Area Preservation

## Overview

Ivaldi VCS preserves the staging area (files gathered with `ivaldi gather`) when switching between timelines. This ensures that work-in-progress staged files are not lost during timeline operations.

## How It Works

### Staging Files

When you use `ivaldi gather <files>` to stage files for the next seal (commit), the files are recorded in `.ivaldi/stage/files`. This staging area is timeline-independent but preserved during timeline switches.

### Timeline Switching with Staged Files

When switching timelines with staged files:

1. **Auto-shelving**: The current timeline's workspace state AND staged files are automatically shelved
2. **Timeline Switch**: The workspace is updated to match the target timeline
3. **Restoration**: If the target timeline has previously shelved staged files, they are restored

### Example Workflow

```bash
# On timeline 'main'
$ ivaldi gather file1.txt
Gathered: file1.txt

$ ivaldi status
Files staged for seal:
  new file:   file1.txt

# Switch to another timeline
$ ivaldi timeline switch feature
Auto-shelved 1 changes from timeline 'main' (shelf: auto_main_xxxxx)
Switched to timeline 'feature'

# Work on feature timeline...
$ ivaldi gather feature.txt

# Switch back to main
$ ivaldi timeline switch main  
Auto-shelved changes from timeline 'feature' (shelf: auto_feature_xxxxx)
Restoring auto-shelved changes for timeline 'main' (shelf: auto_main_xxxxx)
Switched to timeline 'main'

# Staged files are preserved
$ ivaldi status
Files staged for seal:
  new file:   file1.txt
```

## Implementation Details

### Shelf Structure

Each auto-shelf now includes:
- `workspace_index`: The workspace file state
- `staged_files`: List of files staged for commit
- `timeline_name`: The timeline this shelf belongs to
- `auto_created`: Boolean indicating automatic creation

### Storage Location

- Staging area: `.ivaldi/stage/files`
- Auto-shelves: `.ivaldi/shelves/auto_<timeline>_<timestamp>.json`

## Benefits

1. **No Lost Work**: Staged files are never lost when switching timelines
2. **Context Preservation**: Each timeline maintains its own staging context
3. **Seamless Workflow**: Switch between timelines without worrying about staging state
4. **Automatic Management**: No manual intervention required

## Notes

- Staged files are cleared after a successful `ivaldi seal` command
- Auto-shelves are automatically removed when restored
- Only the most recent auto-shelf per timeline is kept