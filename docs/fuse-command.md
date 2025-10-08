# Fuse Command

The `ivaldi fuse` command merges two timelines together using **intelligent chunk-level conflict resolution**. Unlike Git's line-based merging with conflict markers, Ivaldi uses content-addressable storage and BLAKE3 hashing to provide superior merge intelligence.

## Overview

The fuse command provides:
- **Intelligent chunk-level merging**: Merges at 64KB chunk granularity, not line-level
- **Automatic conflict resolution**: Multiple strategies (auto, ours, theirs, union, base)
- **Clean workspace**: NO conflict markers written to files
- **Content-hash based**: Identical changes auto-detected via BLAKE3 hashes
- **Fast-forward detection**: Automatic optimization when possible
- **Three-way merge**: Intelligent merging using common ancestor
- **Interactive resolution**: Clean CLI interface for remaining conflicts
- **Resolution tracking**: Merge decisions stored separately from workspace

## Why Ivaldi's Merge is Superior to Git

1. **Chunk-level intelligence**: Uses 64KB chunks with content hashes instead of line-based diff
2. **No false conflicts**: Whitespace/formatting changes don't cause conflicts
3. **Clean workspace**: Conflict markers never pollute your files
4. **Automatic strategies**: Resolve entire merges without manual editing
5. **Deterministic**: Same content = same hash = automatic merge
6. **Interactive UI**: When needed, use a clean CLI instead of editing markers

## Merge Strategies

Ivaldi provides five built-in strategies for automatic conflict resolution:

### 1. Auto (Default)
**Use case:** Most merges, especially when you want intelligent resolution

Performs chunk-level three-way merge:
- Automatically resolves non-conflicting changes
- Detects identical changes via content hashes
- Only flags truly conflicting chunks
- Best balance of intelligence and safety

```bash
ivaldi fuse feature to main
# Same as: ivaldi fuse --strategy=auto feature to main
```

### 2. Theirs
**Use case:** Accepting external changes, pulling updates, integrating trusted changes

Accepts ALL changes from source timeline:
- No conflicts possible
- Fast and deterministic
- Use when source is authoritative

```bash
ivaldi fuse --strategy=theirs upstream-main to main
```

### 3. Ours
**Use case:** Keeping local changes, rejecting external modifications

Keeps ALL changes from target timeline:
- No conflicts possible
- Useful for merging experimental branches you want to discard
- Source changes are ignored

```bash
ivaldi fuse --strategy=ours experimental-feature to main
```

### 4. Union
**Use case:** Append-only files like changelogs, combined documentation

Combines changes from both timelines:
- Merges non-duplicate chunks from both sides
- Useful for files where both changes should be kept
- May create duplicate content if not careful

```bash
ivaldi fuse --strategy=union feature-docs to main
```

### 5. Base
**Use case:** Reverting to known good state, undoing divergent changes

Reverts to common ancestor:
- Discards changes from both timelines
- Useful for resetting to last known-good state
- Rare use case

```bash
ivaldi fuse --strategy=base problematic-branch to main
```

## Basic Usage

### Merge Timelines with Auto Strategy (Default)

Merge source timeline into target timeline with intelligent chunk-level merging:

```bash
ivaldi fuse <source> to <target>
ivaldi fuse feature-auth to main
```

### Merge with Specific Strategy

Use a specific resolution strategy:

```bash
# Accept all source changes
ivaldi fuse --strategy=theirs feature-auth to main

# Keep all target changes
ivaldi fuse --strategy=ours feature-auth to main

# Combine both versions
ivaldi fuse --strategy=union feature-changelog to main

# Revert to common ancestor
ivaldi fuse --strategy=base feature-experimental to main
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

### `--strategy=<type>`

Specify the conflict resolution strategy:

```bash
ivaldi fuse --strategy=theirs feature to main
```

**Available strategies:**
- `auto` (default) - Intelligent chunk-level merge
- `ours` - Keep target timeline version
- `theirs` - Accept source timeline version
- `union` - Combine both versions
- `base` - Revert to common ancestor

### `--continue`

Continue merge after resolving conflicts:

```bash
ivaldi fuse --continue
```

**Use when:**
- Previous merge had conflicts with auto strategy
- Ready to use interactive resolver or choose different strategy

### `--abort`

Cancel in-progress merge:

```bash
ivaldi fuse --abort
```

**Effect:**
- Clears merge state
- Removes resolution tracking
- Workspace remains clean (no files were modified)

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

## Intelligent Conflict Resolution

### Understanding Ivaldi's Approach

**Key Difference from Git:** Ivaldi NEVER writes conflict markers to your workspace files. All conflict resolution happens through strategies or an interactive UI, keeping your workspace clean.

Conflicts are detected at the **chunk level** (64KB granularity) using content hashes:
- Same chunk hash: No conflict (identical content)
- Different hashes: Check if only one side changed
- Both changed differently: Real conflict

### Automatic Resolution with Strategies

Most merges can be resolved automatically without any manual intervention:

#### 1. Auto Strategy (Default - Chunk-Level Intelligence)

```bash
$ ivaldi fuse feature-auth to main

>> Fusing feature-auth into main...

[MERGE] Three-way merge required

Changes to be merged:
  + 3 files
  ~ 2 files

Apply merge from feature-auth to main? (y/N)> y

>> Creating merge commit...

[OK] Changes from feature-auth fused into main!
  Merge seal: merge-feature-auth-xyz789
```

The auto strategy uses chunk-level intelligence to automatically resolve:
- Changes on only one side (takes that change)
- Identical changes on both sides (same hash, no conflict)
- Non-overlapping chunks in same file

#### 2. Theirs Strategy (Accept Source)

```bash
$ ivaldi fuse --strategy=theirs feature-auth to main

>> Using merge strategy: theirs
  No manual resolution needed

[OK] Changes from feature-auth fused into main!
```

Automatically accepts ALL changes from source timeline.

#### 3. Ours Strategy (Keep Target)

```bash
$ ivaldi fuse --strategy=ours feature-auth to main

>> Using merge strategy: ours
  No manual resolution needed

[OK] Changes from feature-auth fused into main!
```

Automatically keeps ALL changes from target timeline.

#### 4. Union Strategy (Combine Both)

```bash
$ ivaldi fuse --strategy=union feature-changelog to main

>> Using merge strategy: union
  Combining changes from both timelines

[OK] Changes from feature-changelog fused into main!
```

Combines changes from both sides (useful for append-only files like changelogs).

### When Auto Strategy Finds Conflicts

If auto strategy encounters truly conflicting chunks:

```bash
$ ivaldi fuse feature-auth to main

>> Fusing feature-auth into main...

[CONFLICTS] Merge conflicts detected:

  CONFLICT: src/auth.go
  CONFLICT: src/config.go

>> 2 file(s) with conflicts

Resolution options:
  ivaldi fuse --continue - Use interactive resolver
  ivaldi fuse --strategy=theirs feature-auth - Accept all source changes
  ivaldi fuse --strategy=ours feature-auth - Keep all target changes
  ivaldi fuse --abort - Abort merge

Note: Workspace files are NOT modified - conflicts are resolved separately
```

**Important:** Your workspace files remain untouched. No conflict markers are written.

### Interactive Resolution Workflow

When conflicts exist, you have clean options:

**Option 1: Choose a strategy**
```bash
# Just accept source changes
ivaldi fuse --strategy=theirs feature-auth to main

# Or keep target changes
ivaldi fuse --strategy=ours feature-auth to main
```

**Option 2: Interactive resolver (future)**
```bash
ivaldi fuse --continue

# Interactive CLI shows each conflict with options:
# - View OURS, THEIRS, BASE versions
# - Choose which version to keep
# - No manual file editing needed
```

**Option 3: Abort and reconsider**
```bash
ivaldi fuse --abort
```

### Comparison: Ivaldi vs Git

| Aspect | Git | Ivaldi |
|--------|-----|--------|
| Conflict markers | Written to files | Never written to files |
| Resolution method | Manual file editing | Strategy selection or interactive UI |
| Granularity | Line-based | Chunk-based (64KB) |
| False conflicts | Common (whitespace) | Rare (content-hash based) |
| Workspace during conflict | Polluted with markers | Always clean |
| Identical changes | May conflict | Auto-merged (same hash) |

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

When merge has conflicts with auto strategy:

```bash
$ ivaldi fuse feature-auth to main

[CONFLICTS] Merge conflicts detected:

  CONFLICT: src/auth.go

>> 1 file(s) with conflicts

Resolution options:
  ivaldi fuse --continue - Use interactive resolver
  ivaldi fuse --strategy=theirs feature-auth - Accept all source changes
  ivaldi fuse --strategy=ours feature-auth - Keep all target changes
  ivaldi fuse --abort - Abort merge

# Option 1: Just accept source changes
$ ivaldi fuse --strategy=theirs feature-auth to main
[OK] Changes from feature-auth fused into main!

# Option 2: Or keep target changes
$ ivaldi fuse --strategy=ours feature-auth to main
[OK] Changes from feature-auth fused into main!
```

**Note:** No file editing required! Workspace stays clean.

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
