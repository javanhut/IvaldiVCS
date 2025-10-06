# Fuse Command

The `ivaldi fuse` command merges two timelines together, combining their commit histories and changes. It supports fast-forward merges, three-way merges, and conflict resolution.

## Overview

The fuse command provides:
- **Timeline merging**: Combine work from different timelines
- **Fast-forward detection**: Automatic optimization when possible
- **Three-way merge**: Intelligent merging using common ancestor
- **Conflict detection**: Identifies files that need manual resolution
- **Interactive workflow**: Review changes before merging
- **Conflict resolution**: Git-like conflict markers for manual fixes
- **Merge state management**: Support for multi-step merge workflows

## Basic Usage

### Merge Timelines

Merge source timeline into target timeline:

```bash
ivaldi fuse <source> to <target>
ivaldi fuse feature-auth to main
```

**Interactive workflow:**
```
Analyzing timelines for merge...

Source Timeline: feature-auth
  Last Seal: add-login-endpoint-abc123
  Commits ahead: 3

Target Timeline: main
  Last Seal: update-readme-def456
  Commits ahead: 1

Changes to be merged:
  3 files will be added
  2 files will be modified
  0 files will be removed

Proceed with merge? (yes/no): yes

[MERGE] Fast-forward merge detected
>> Updating main timeline...
>> Creating merge seal...

[OK] Merge completed successfully!
  Merge seal: merge-feature-auth-xyz789
  Timeline main updated
```

### Continue Merge After Resolving Conflicts

After manually resolving conflicts:

```bash
ivaldi fuse --continue
```

### Abort Merge

Cancel an in-progress merge:

```bash
ivaldi fuse --abort
```

## Command Options

### `<source> to <target>`

Specifies which timeline to merge into which:

```bash
ivaldi fuse feature-payment to main
```

- **source**: Timeline containing changes to merge
- **target**: Timeline that will receive the changes

### `--continue`

Continue merge after resolving conflicts:

```bash
ivaldi fuse --continue
```

**Use when:**
- Previous merge had conflicts
- You've manually edited conflicted files
- Ready to create merge commit

### `--abort`

Cancel in-progress merge:

```bash
ivaldi fuse --abort
```

**Effect:**
- Clears merge state
- Restores working directory to pre-merge state
- Removes conflict markers

## Merge Types

### Fast-Forward Merge

When target timeline is an ancestor of source timeline:

```
Before:
  main:    A---B
                \
  feature:       C---D

After:
  main:    A---B---C---D
  feature:             C---D
```

**Characteristics:**
- No merge commit created
- Timeline reference simply moves forward
- No conflicts possible
- Automatic and instant

**Example:**
```bash
$ ivaldi fuse feature-auth to main

[MERGE] Fast-forward merge detected
>> Updating main timeline...

[OK] Merge completed successfully!
  Timeline main updated
```

### Three-Way Merge

When both timelines have diverged:

```
Before:
  main:    A---B---C
                \
  feature:       D---E

After:
  main:    A---B---C---M
                \     /
  feature:       D---E
```

**Characteristics:**
- Finds common ancestor (A)
- Compares source (E) and target (C) against ancestor
- Creates merge commit (M) with both parents
- May have conflicts requiring resolution

**Example:**
```bash
$ ivaldi fuse feature-payment to main

Analyzing timelines for merge...

Changes to be merged:
  2 files will be modified

Proceed with merge? (yes/no): yes

>> Creating merge commit...

[OK] Merge completed successfully!
  Merge seal: merge-feature-payment-abc123
  Timeline main updated
```

## Conflict Resolution

### Understanding Conflicts

Conflicts occur when:
- Same file modified in both timelines
- Changes affect overlapping lines
- Cannot be automatically merged

### Conflict Markers

Conflicted files contain markers:

```
<<<<<<< TARGET (src/auth.go)
func Login(username string) error {
    return authenticate(username)
}
=======
func Login(user User) error {
    return validateAndAuthenticate(user)
}
>>>>>>> SOURCE (src/auth.go)
```

**Marker explanation:**
- `<<<<<<< TARGET`: Start of target timeline's version
- `=======`: Separator between versions
- `>>>>>>> SOURCE`: End of source timeline's version

### Resolution Workflow

1. **Identify conflicts:**
```bash
$ ivaldi fuse feature-auth to main

CONFLICT: Merge conflicts detected

[ERROR] Unresolved conflicts remaining:

  CONFLICT: src/auth.go
  CONFLICT: src/config.go

Please resolve conflicts and run: ivaldi fuse --continue
Or abort the merge with: ivaldi fuse --abort
```

2. **Edit conflicted files:**
```bash
# Open each file and manually resolve
vim src/auth.go
vim src/config.go
```

3. **Remove conflict markers:**
```go
// Before (with conflict):
<<<<<<< TARGET (src/auth.go)
func Login(username string) error {
    return authenticate(username)
}
=======
func Login(user User) error {
    return validateAndAuthenticate(user)
}
>>>>>>> SOURCE (src/auth.go)

// After (resolved):
func Login(user User) error {
    return authenticate(user)
}
```

4. **Stage resolved files:**
```bash
ivaldi gather src/auth.go
ivaldi gather src/config.go
```

5. **Continue merge:**
```bash
ivaldi fuse --continue
```

6. **Verify completion:**
```bash
$ ivaldi fuse --continue

Creating merge commit...

[OK] Merge completed successfully!
  Merge seal: merge-feature-auth-xyz789
  Timeline main updated
```

## Common Workflows

### Feature Integration

Merge completed feature into main:

```bash
# Ensure feature is complete
ivaldi timeline switch feature-payment
ivaldi status  # Should be clean

# Switch to main
ivaldi timeline switch main

# Merge feature
ivaldi fuse feature-payment to main

# Review and confirm
# Proceed with merge? (yes/no): yes
```

### Sync Feature with Main

Update feature timeline with latest main changes:

```bash
# On feature timeline
ivaldi timeline switch feature-auth

# Merge main into feature
ivaldi fuse main to feature-auth

# Continue working on feature
```

### Release Preparation

Merge multiple features for release:

```bash
ivaldi timeline switch main

# Merge features one by one
ivaldi fuse feature-auth to main
ivaldi fuse feature-payment to main
ivaldi fuse feature-ui to main

# Verify all merged
ivaldi log --oneline
```

### Conflict Resolution Workflow

When merge has conflicts:

```bash
$ ivaldi fuse feature-auth to main

CONFLICT: Merge conflicts detected

  CONFLICT: src/auth.go

Please resolve conflicts and run: ivaldi fuse --continue

# Resolve conflicts
$ vim src/auth.go
# Edit file, remove markers, save

# Stage resolved file
$ ivaldi gather src/auth.go

# Complete merge
$ ivaldi fuse --continue

[OK] Merge completed successfully!
```

### Aborting Failed Merge

If merge becomes too complex:

```bash
$ ivaldi fuse feature-experimental to main

CONFLICT: Merge conflicts detected

  CONFLICT: src/core.go
  CONFLICT: src/utils.go
  CONFLICT: src/config.go

# Too many conflicts, abort
$ ivaldi fuse --abort

Merge aborted
Workspace restored to pre-merge state

# Try different approach or resolve timeline separately
```

## Practical Examples

### Example 1: Clean Fast-Forward

```bash
$ ivaldi timeline switch main

$ ivaldi fuse feature-login to main

[MERGE] Fast-forward merge detected
>> Updating main timeline...

[OK] Merge completed successfully!
  Timeline main updated
```

### Example 2: Three-Way Merge

```bash
$ ivaldi timeline switch main

$ ivaldi fuse feature-payment to main

Analyzing timelines for merge...

Changes to be merged:
  5 files will be added
  3 files will be modified

Proceed with merge? (yes/no): yes

>> Creating merge commit...

[OK] Merge completed successfully!
  Merge seal: merge-feature-payment-abc123
  Timeline main updated
```

### Example 3: Conflict Resolution

```bash
$ ivaldi fuse feature-auth to main

CONFLICT: Merge conflicts detected

  CONFLICT: src/auth.go

$ vim src/auth.go
# Resolve conflicts, save

$ ivaldi gather src/auth.go

$ ivaldi fuse --continue

[OK] Merge completed successfully!
  Merge seal: merge-feature-auth-xyz789
```

### Example 4: Merge Abort

```bash
$ ivaldi fuse feature-experimental to main

CONFLICT: Merge conflicts detected

  CONFLICT: src/core.go
  CONFLICT: src/utils.go

$ ivaldi fuse --abort

Merge aborted
Workspace restored to pre-merge state
```

## Integration with Other Commands

### With `ivaldi log`

Review timeline history before merging:

```bash
# Check what will be merged
ivaldi timeline switch feature-auth
ivaldi log --limit 5

# Check target timeline
ivaldi timeline switch main
ivaldi log --limit 5

# Perform merge
ivaldi fuse feature-auth to main
```

### With `ivaldi diff`

Compare timelines before merging:

```bash
# Get commit hashes
ivaldi timeline switch main
ivaldi log --oneline --limit 1  # abc123

ivaldi timeline switch feature-auth
ivaldi log --oneline --limit 1  # def456

# Compare
ivaldi diff abc123 def456

# Decide on merge
ivaldi timeline switch main
ivaldi fuse feature-auth to main
```

### With `ivaldi status`

Ensure clean working directory:

```bash
# Before merge
ivaldi status  # Should be clean

# Perform merge
ivaldi fuse feature-auth to main

# After merge
ivaldi status  # Verify state
```

### With `ivaldi gather`

Stage resolved conflicts:

```bash
# During conflict resolution
ivaldi gather src/auth.go src/config.go

# Continue merge
ivaldi fuse --continue
```

## Understanding Merge State

### Merge State Files

During a merge, Ivaldi creates temporary files:

- `.ivaldi/MERGE_HEAD`: Source timeline's commit hash
- `.ivaldi/MERGE_INFO`: Source and target timeline names
- `.ivaldi/MERGE_CONFLICTS`: List of conflicted file paths

### Checking Merge State

```bash
# Check if merge is in progress
ls .ivaldi/MERGE_HEAD

# View merge info
cat .ivaldi/MERGE_INFO
```

### Cleanup

Merge state is automatically cleaned up after:
- Successful merge (`--continue`)
- Merge abort (`--abort`)

## Comparison with Git

| Feature | Git | Ivaldi |
|---------|-----|--------|
| Merge branches | `git merge <branch>` | `ivaldi fuse <source> to <target>` |
| Fast-forward | `git merge --ff` | (automatic detection) |
| Three-way merge | `git merge --no-ff` | (automatic when needed) |
| Conflict markers | `<<<<<<< HEAD` | `<<<<<<< TARGET` |
| Continue merge | `git merge --continue` | `ivaldi fuse --continue` |
| Abort merge | `git merge --abort` | `ivaldi fuse --abort` |
| Merge commit | (automatic) | (automatic for three-way) |

## Tips and Tricks

### 1. Preview Before Merge

```bash
# Compare timelines first
ivaldi log --all
ivaldi diff <target-hash> <source-hash>

# Then merge
ivaldi fuse <source> to <target>
```

### 2. Clean Working Directory

```bash
# Ensure clean state
ivaldi status

# If dirty, commit or reset
ivaldi gather .
ivaldi seal "WIP before merge"

# Now safe to merge
ivaldi fuse feature to main
```

### 3. Backup Before Merge

```bash
# Create safety timeline
ivaldi timeline create backup-before-merge

# Perform merge on main
ivaldi timeline switch main
ivaldi fuse feature-risky to main

# If problems, restore from backup
```

### 4. Incremental Conflict Resolution

```bash
# Resolve and stage files one at a time
vim src/auth.go
ivaldi gather src/auth.go

vim src/config.go
ivaldi gather src/config.go

# Continue when all resolved
ivaldi fuse --continue
```

### 5. Document Merge Commits

Use descriptive messages for merge commits:

```bash
# Merge commit messages are auto-generated
# Format: "Merge <source> into <target>"
```

## Troubleshooting

### "Not in an Ivaldi repository"

```
Error: not in an Ivaldi repository
```

**Solution:** Navigate to Ivaldi repository:
```bash
cd my-ivaldi-repo
ivaldi fuse feature to main
```

### "Timeline not found"

```
Error: timeline not found: feature-xyz
```

**Solution:** Verify timeline exists:
```bash
ivaldi timeline list
```

### "No merge in progress"

```
Error: no merge in progress
```

**Solution:** Start a merge first:
```bash
ivaldi fuse <source> to <target>
```

### "Unresolved conflicts"

```
[ERROR] Unresolved conflicts remaining:
  CONFLICT: src/auth.go
```

**Solution:** Resolve conflicts:
```bash
vim src/auth.go  # Remove conflict markers
ivaldi gather src/auth.go
ivaldi fuse --continue
```

### "Merge already in progress"

```
Error: merge already in progress
```

**Solutions:**
- Complete current merge: `ivaldi fuse --continue`
- Abort current merge: `ivaldi fuse --abort`

## Advanced Usage

### Scripted Merges

```bash
#!/bin/bash
# Automated merge script with error handling

SOURCE="feature-auth"
TARGET="main"

ivaldi timeline switch "$TARGET" || exit 1

if ivaldi fuse "$SOURCE" to "$TARGET"; then
    echo "Merge successful"
else
    echo "Merge failed or has conflicts"
    echo "Please resolve manually"
    exit 1
fi
```

### Batch Merging

```bash
#!/bin/bash
# Merge multiple features

FEATURES=("feature-auth" "feature-payment" "feature-ui")

ivaldi timeline switch main

for feature in "${FEATURES[@]}"; do
    echo "Merging $feature..."
    if ! ivaldi fuse "$feature" to main; then
        echo "Failed to merge $feature"
        break
    fi
done
```

### Conflict Detection

```bash
#!/bin/bash
# Pre-check for potential conflicts

if ivaldi fuse feature to main 2>&1 | grep -q "CONFLICT"; then
    echo "Conflicts detected, aborting"
    ivaldi fuse --abort
    exit 1
fi
```

## Best Practices

### 1. Keep Working Directory Clean

Before merging:
```bash
ivaldi status  # Should show "Working directory clean"
```

### 2. Review Before Merging

```bash
ivaldi log --all
ivaldi diff <target> <source>
```

### 3. Merge Regularly

Avoid long-lived feature timelines:
- Merge main into feature frequently
- Keep feature timelines small and focused

### 4. Test After Merging

```bash
ivaldi fuse feature to main
# Run tests to verify merge didn't break anything
make test
```

### 5. Document Conflicts

When resolving conflicts, document your decisions:
```bash
# After resolving
ivaldi seal "Resolved merge conflicts: chose source implementation for auth"
```

## Related Commands

- `ivaldi timeline create` - Create new timeline
- `ivaldi timeline switch` - Switch between timelines
- `ivaldi timeline list` - List all timelines
- `ivaldi log` - View commit history
- `ivaldi diff` - Compare changes
- `ivaldi status` - Check working directory state
- `ivaldi gather` - Stage files
- `ivaldi seal` - Create commit
