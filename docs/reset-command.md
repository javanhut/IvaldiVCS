# Reset Command

The `ivaldi reset` command unstages files from the staging area, reverting them to their state in the working directory. It can also discard all changes when used with `--hard`.

## Overview

The reset command provides:
- **Unstage files**: Remove files from staging area without losing changes
- **Selective unstaging**: Choose which files to unstage
- **Unstage all**: Clear entire staging area at once
- **Hard reset**: Discard all uncommitted changes (dangerous)
- **Safe by default**: Preserves working directory changes

## Basic Usage

### Unstage Specific Files

Remove specific files from staging area:

```bash
ivaldi reset <file1> <file2> ...
ivaldi reset src/auth.go
ivaldi reset src/auth.go src/config.go
```

**Effect:** Files remain modified in working directory but are no longer staged for commit.

### Unstage All Files

Remove all files from staging area:

```bash
ivaldi reset
```

**Effect:** Clears staging area completely, preserving working directory changes.

### Discard All Changes (Dangerous)

Reset working directory to match HEAD:

```bash
ivaldi reset --hard
```

**Warning:** This permanently deletes all uncommitted changes!

## Command Options

### No Arguments

Unstage all currently staged files:

```bash
ivaldi reset
```

### Specific Files

Unstage only the specified files:

```bash
ivaldi reset file1.go file2.go
```

### `--hard`

Discard all uncommitted changes and reset to HEAD:

```bash
ivaldi reset --hard
```

**Danger:** This operation is irreversible!

## Common Workflows

### Undo Staging Mistakes

After staging wrong files:

```bash
# Accidentally staged everything
ivaldi gather .

# Check what's staged
ivaldi status

# Unstage specific files
ivaldi reset tests/debug.go
ivaldi reset config/secrets.yaml

# Verify
ivaldi status
```

### Selective Commit Preparation

Stage files incrementally:

```bash
# Stage everything
ivaldi gather .

# Review
ivaldi diff --staged

# Unstage files not ready for commit
ivaldi reset src/experimental.go
ivaldi reset TODO.md

# Commit only what's left
ivaldi seal "Stable changes only"
```

### Start Over with Staging

Clear staging area and reselect files:

```bash
# Clear all staging
ivaldi reset

# Re-stage selectively
ivaldi gather src/auth.go
ivaldi gather src/user.go

# Commit
ivaldi seal "Authentication updates"
```

### Abandon Work in Progress

Discard all changes and start fresh:

```bash
# See what will be lost
ivaldi diff
ivaldi status

# Confirm you want to lose changes
ivaldi reset --hard

# Working directory now matches HEAD
```

### Recover from Bad Edits

After making unwanted changes:

```bash
# Made mistakes in multiple files
ivaldi status
# Shows: modified auth.go, config.go, user.go

# Keep changes to some files, discard others
ivaldi gather auth.go user.go    # Stage files to keep
ivaldi reset --hard               # Discard everything
# auth.go and user.go are now staged
# config.go reverted to HEAD

# Restore staged files to working directory
ivaldi reset
# auth.go and user.go now have your changes
# config.go is clean
```

**Note:** This workflow is complex. For simpler cases, manually revert specific files.

## Practical Examples

### Example 1: Unstage One File

```bash
$ ivaldi gather src/auth.go src/config.go

$ ivaldi status
Staged files:
  src/auth.go
  src/config.go

$ ivaldi reset src/config.go
Unstaged: src/config.go

$ ivaldi status
Staged files:
  src/auth.go

Modified files:
  src/config.go
```

### Example 2: Clear Staging Area

```bash
$ ivaldi gather .

$ ivaldi status
Staged files:
  src/auth.go
  src/config.go
  README.md

$ ivaldi reset
Unstaged all files

$ ivaldi status
Staged files: none

Modified files:
  src/auth.go
  src/config.go
  README.md
```

### Example 3: Hard Reset

```bash
$ ivaldi status
Modified files:
  src/auth.go (modified)
  src/config.go (modified)

$ ivaldi reset --hard
Reset to HEAD: 2 files restored

$ ivaldi status
Working directory clean
```

### Example 4: Selective Reset

```bash
$ ivaldi gather src/*.go

$ ivaldi status
Staged files:
  src/auth.go
  src/config.go
  src/user.go

$ ivaldi reset src/user.go
Unstaged: src/user.go

$ ivaldi seal "Auth and config updates"
```

## Understanding Reset Behavior

### Default Reset (Soft)

```bash
ivaldi reset <files>
```

**What happens:**
1. Files are removed from staging area
2. Working directory remains unchanged
3. File modifications are preserved
4. Safe operation - no data loss

**Before:**
- Staging: `auth.go` (modified)
- Working: `auth.go` (modified)

**After:**
- Staging: empty
- Working: `auth.go` (modified)

### Hard Reset

```bash
ivaldi reset --hard
```

**What happens:**
1. Staging area is cleared
2. Working directory is reset to HEAD
3. All uncommitted changes are lost
4. Dangerous operation - permanent data loss

**Before:**
- Staging: `auth.go` (modified)
- Working: `auth.go` (modified), `config.go` (modified)

**After:**
- Staging: empty
- Working: All files match HEAD (clean)

## Safety Considerations

### Safe Operations

These are always safe:

```bash
ivaldi reset                # Unstage all
ivaldi reset file.go        # Unstage specific file
ivaldi diff --staged        # Preview before reset
ivaldi status               # Check state before reset
```

### Dangerous Operations

**Always double-check before running:**

```bash
ivaldi reset --hard
```

**Before running `--hard`, ask yourself:**
- Do I have uncommitted work I need?
- Have I backed up important changes?
- Can I recreate these changes easily?
- Is there a safer alternative?

**Safer alternatives:**
- `ivaldi reset` (without --hard) to unstage only
- Manual file revert for specific files
- `ivaldi timeline switch` with auto-shelving
- Commit work first, then reset to previous commit

## Integration with Other Commands

### With `ivaldi gather`

```bash
# Stage files
ivaldi gather src/

# Review
ivaldi diff --staged

# Unstage if needed
ivaldi reset src/debug.go

# Re-review
ivaldi diff --staged

# Commit
ivaldi seal "Production code only"
```

### With `ivaldi status`

```bash
# Check current state
ivaldi status

# Unstage files
ivaldi reset

# Verify
ivaldi status
```

### With `ivaldi diff`

```bash
# See what's staged
ivaldi diff --staged

# Unstage everything
ivaldi reset

# See working directory changes
ivaldi diff
```

### With `ivaldi seal`

```bash
# Stage files
ivaldi gather .

# Review
ivaldi diff --staged

# Unstage unwanted files
ivaldi reset secrets.yaml

# Create clean commit
ivaldi seal "Clean commit"
```

## Comparison with Git

| Feature | Git | Ivaldi |
|---------|-----|--------|
| Unstage files | `git reset <file>` | `ivaldi reset <file>` |
| Unstage all | `git reset` | `ivaldi reset` |
| Hard reset | `git reset --hard` | `ivaldi reset --hard` |
| Mixed reset | `git reset --mixed` | (default behavior) |
| Soft reset | `git reset --soft` | Not available |
| Reset to commit | `git reset <commit>` | Not available yet |

**Note:** Ivaldi currently doesn't support resetting to a specific commit (only HEAD).

## Tips and Tricks

### 1. Always Preview Before Hard Reset

```bash
# See what will be lost
ivaldi diff
ivaldi status

# Then decide
ivaldi reset --hard
```

### 2. Use Status to Verify

```bash
ivaldi reset src/auth.go
ivaldi status  # Confirm unstaging worked
```

### 3. Unstage Patterns

```bash
# Unstage all .go files
ivaldi reset src/*.go

# Unstage all test files
ivaldi reset **/*_test.go
```

### 4. Create Alias for Safety

```bash
# In ~/.bashrc or ~/.zshrc
alias ireset-hard='ivaldi status && echo "Really reset? (Ctrl-C to cancel)" && sleep 3 && ivaldi reset --hard'
```

### 5. Backup Before Hard Reset

```bash
# Create safety commit on temp timeline
ivaldi timeline create safety-backup
ivaldi gather .
ivaldi seal "Backup before reset"
ivaldi timeline switch main
ivaldi reset --hard

# If needed, restore from backup:
# ivaldi timeline switch safety-backup
```

## Troubleshooting

### File Not Staged

```bash
$ ivaldi reset src/auth.go
Warning: src/auth.go is not staged
```

**Solution:** Check staging status first:
```bash
ivaldi status
```

### No Staged Files

```bash
$ ivaldi reset
No staged files to reset
```

**Explanation:** Staging area is already empty.

### Permission Denied

```
Error: failed to remove stage file: permission denied
```

**Solutions:**
- Check file permissions on `.ivaldi/stage/files`
- Ensure you own the repository
- Run with appropriate permissions

### Can't Undo Hard Reset

**Problem:** Ran `ivaldi reset --hard` and lost work.

**Solutions:**
- Check if you have recent commits: `ivaldi log`
- Look for auto-shelved changes: Check `.ivaldi/shelves/`
- Restore from backup if available
- Recreate the changes manually

**Prevention:** Always commit or backup before `--hard` reset.

## Advanced Usage

### Scripted Unstaging

```bash
#!/bin/bash
# Unstage all test files

for file in $(ivaldi status | grep "_test.go"); do
    ivaldi reset "$file"
done
```

### Conditional Reset

```bash
#!/bin/bash
# Reset only if staging area has more than 10 files

STAGED_COUNT=$(ivaldi status | grep -c "Staged:")
if [ "$STAGED_COUNT" -gt 10 ]; then
    echo "Too many staged files, resetting..."
    ivaldi reset
fi
```

### Interactive Unstaging

```bash
#!/bin/bash
# Interactively choose files to unstage

ivaldi status | grep "Staged:" | while read -r file; do
    read -p "Unstage $file? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        ivaldi reset "$file"
    fi
done
```

### Safe Hard Reset with Confirmation

```bash
#!/bin/bash
# Require explicit confirmation before hard reset

echo "WARNING: This will discard all uncommitted changes!"
echo "Modified files:"
ivaldi status

read -p "Type 'YES' to confirm: " -r
if [[ $REPLY == "YES" ]]; then
    ivaldi reset --hard
    echo "Reset complete"
else
    echo "Reset cancelled"
fi
```

## Best Practices

### 1. Check Before Unstaging

Always verify what's staged:
```bash
ivaldi status
ivaldi diff --staged
```

### 2. Use --hard Sparingly

Only use `--hard` when you're absolutely certain:
- You don't need uncommitted changes
- You've backed up important work
- You understand it's irreversible

### 3. Unstage Incrementally

Rather than reset everything:
```bash
# Selective unstaging
ivaldi reset unwanted-file.go
ivaldi reset debug-code.go
```

### 4. Verify After Reset

Confirm the operation worked:
```bash
ivaldi reset
ivaldi status  # Should show no staged files
```

### 5. Consider Alternatives

Before using `reset --hard`, consider:
- Creating a commit you can revert later
- Using timeline switching with auto-shelving
- Manually reverting specific files

## Related Commands

- `ivaldi gather` - Stage files for commit
- `ivaldi status` - Show staging and working directory state
- `ivaldi diff` - Show changes in working directory
- `ivaldi diff --staged` - Show staged changes
- `ivaldi seal` - Create commit with staged files
- `ivaldi timeline switch` - Switch timelines (with auto-shelving)
