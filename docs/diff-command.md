# Diff Command

The `ivaldi diff` command shows differences between various states of your repository, including working directory, staged files, and commits.

## Overview

The diff command provides:
- **Working directory changes**: See what's modified but not staged
- **Staged changes**: Review what will be committed
- **Commit comparisons**: Compare any two commits
- **File-level details**: Line-by-line differences
- **Summary statistics**: Quick overview of changes

## Basic Usage

### Show Unstaged Changes

Compare working directory with staged files (or HEAD if nothing staged):

```bash
ivaldi diff
```

**Example output:**
```
Diff between staged and working directory:

+++ src/auth.go

File size: 1024 bytes

M   README.md

- Version 1.0.0
+ Version 1.1.0
```

### Show Staged Changes

Compare staged files with HEAD commit:

```bash
ivaldi diff --staged
```

**Example output:**
```
Diff between HEAD and staged:

+++ src/login.go

File size: 2048 bytes

M   src/auth.go

- func Login() {
+ func Login(username, password string) {
```

### Compare with Specific Commit

Compare working directory with a commit:

```bash
ivaldi diff <seal-name>
ivaldi diff swift-eagle-flies-high-447abe9b
ivaldi diff 447abe9b
```

### Compare Two Commits

Compare any two commits:

```bash
ivaldi diff <seal1> <seal2>
ivaldi diff abc123 def456
```

**Example output:**
```
Diff between abc123 and def456:

+++ src/payment.go

File size: 3072 bytes

---  src/old-feature.go

File size: 512 bytes

M    src/config.go

- timeout = 30
+ timeout = 60
```

### Summary Statistics Only

Show only change counts without details:

```bash
ivaldi diff --stat
```

**Example output:**
```
Diff between HEAD and working directory:

  5 files changed: 2 added, 2 modified, 1 removed
```

## Command Options

### `--staged`

Show differences between staged files and HEAD commit.

```bash
ivaldi diff --staged
```

**Use case:** Review what will be committed before running `ivaldi seal`

### `--stat`

Show only summary statistics, not detailed line changes.

```bash
ivaldi diff --stat
ivaldi diff --staged --stat
ivaldi diff abc123 def456 --stat
```

**Use case:** Quick overview of change scope

## Change Indicators

### File Status Markers

```
+++ filename    # File added
--- filename    # File removed
M   filename    # File modified
```

### Line Change Markers

```
- removed line  # Line deleted
+ added line    # Line added
  unchanged     # Context line
```

## Common Workflows

### Pre-Commit Review

Check what you're about to commit:

```bash
# Review unstaged changes
ivaldi diff

# Stage files
ivaldi gather src/

# Review staged changes
ivaldi diff --staged

# Create commit
ivaldi seal "Add new feature"
```

### Selective Staging

Review and stage files incrementally:

```bash
# See all changes
ivaldi diff

# Stage specific file
ivaldi gather src/auth.go

# Verify what's staged
ivaldi diff --staged

# See remaining unstaged changes
ivaldi diff

# Stage more files
ivaldi gather README.md

# Final review
ivaldi diff --staged
```

### Code Review Preparation

Compare feature timeline with main:

```bash
# Switch to feature timeline
ivaldi timeline switch feature-auth

# Get latest commit hash
ivaldi log --oneline --limit 1
# Shows: abc123 feature-seal ...

# Switch to main
ivaldi timeline switch main

# Get latest commit hash
ivaldi log --oneline --limit 1
# Shows: def456 main-seal ...

# Compare the timelines
ivaldi diff def456 abc123
```

### Verify Changes Before Push

Before uploading to remote:

```bash
# Get hash of remote tracking commit (last upload)
# Compare with current HEAD
ivaldi diff <last-uploaded-seal> <current-seal>

# Or review all commits since last upload
ivaldi log --oneline --limit 5
```

### Troubleshooting Merge Conflicts

Understand what changed in both timelines:

```bash
# On current timeline
ivaldi log --oneline --limit 1

# Compare with merge source
ivaldi diff <current-seal> <source-seal>
```

## Practical Examples

### Example 1: Quick Change Check

```bash
$ ivaldi diff --stat
Diff between HEAD and working directory:

  3 files changed: 1 added, 2 modified, 0 removed
```

### Example 2: Detailed Review

```bash
$ ivaldi diff

Diff between HEAD and working directory:

M   src/auth.go

- timeout := 30 * time.Second
+ timeout := 60 * time.Second

- return nil
+ return fmt.Errorf("authentication failed")

+++ tests/auth_test.go

File size: 856 bytes
```

### Example 3: Pre-Commit Check

```bash
$ ivaldi gather src/

$ ivaldi diff --staged

Diff between HEAD and staged:

M   src/auth.go

- timeout := 30 * time.Second
+ timeout := 60 * time.Second
```

### Example 4: Compare Releases

```bash
$ ivaldi diff release-1.0.0 release-1.1.0 --stat

Diff between release-1.0.0 and release-1.1.0:

  12 files changed: 3 added, 8 modified, 1 removed
```

### Example 5: Verify Unstaged Work

```bash
# After accidentally running 'ivaldi reset'
$ ivaldi diff

Diff between HEAD and working directory:

M   important-file.go

- // TODO: implement
+ func NewFeature() { ... }

# Good! Changes are still in working directory
```

## Integration with Other Commands

### With `ivaldi gather`

```bash
# See what needs staging
ivaldi diff

# Stage files
ivaldi gather src/

# Verify staging
ivaldi diff --staged
```

### With `ivaldi seal`

```bash
# Review before committing
ivaldi diff --staged

# Commit if looks good
ivaldi seal "Implement new feature"
```

### With `ivaldi reset`

```bash
# See staged changes
ivaldi diff --staged

# Unstage if needed
ivaldi reset src/config.go

# Verify
ivaldi diff --staged
```

### With `ivaldi log`

```bash
# Find commits to compare
ivaldi log --oneline

# Compare specific commits
ivaldi diff abc123 def456
```

### With `ivaldi fuse`

```bash
# Before merging, compare timelines
ivaldi diff <target-seal> <source-seal>

# Proceed with merge
ivaldi fuse feature-auth to main
```

## Understanding Output

### Added Files

```
+++ src/new-feature.go

File size: 2048 bytes
```

Indicates a completely new file.

### Removed Files

```
--- src/old-feature.go

File size: 512 bytes
```

Indicates a deleted file.

### Modified Files

```
M   src/config.go

- old_value = 10
+ new_value = 20
- removed_line
+ added_line
```

Shows line-by-line changes within the file.

### Binary Files

```
M   image.png

  (binary file or read error)
```

Binary files show modification marker but not line diff.

## Comparison with Git

| Feature | Git | Ivaldi |
|---------|-----|--------|
| Unstaged changes | `git diff` | `ivaldi diff` |
| Staged changes | `git diff --staged` | `ivaldi diff --staged` |
| Commit vs working | `git diff <commit>` | `ivaldi diff <seal>` |
| Two commits | `git diff <a> <b>` | `ivaldi diff <a> <b>` |
| Summary only | `git diff --stat` | `ivaldi diff --stat` |
| File status | Same markers | Same markers |

## Tips and Tricks

### 1. Pipe to Pager

For large diffs:

```bash
ivaldi diff | less
ivaldi diff --staged | more
```

### 2. Save Diff for Review

```bash
ivaldi diff > changes.diff
ivaldi diff --staged > staged-changes.diff
```

### 3. Count Changed Lines

```bash
ivaldi diff | grep -c "^+"
ivaldi diff | grep -c "^-"
```

### 4. Filter to Specific Files

```bash
ivaldi diff | grep -A 10 "auth.go"
```

### 5. Combine with Stat for Overview

```bash
# Quick overview
ivaldi diff --stat

# If interesting, view details
ivaldi diff
```

### 6. Use with Watch

Monitor changes in real-time:

```bash
watch -n 2 'ivaldi diff --stat'
```

## Troubleshooting

### No Output

If `ivaldi diff` shows nothing:

```bash
# Check if there are any changes
ivaldi status

# Verify working directory
ls -la
```

**Possible reasons:**
- No changes in working directory
- All changes are staged (use `--staged`)
- Working directory matches HEAD

### "No differences" Message

```
No differences.
```

**Solutions:**
- Make sure you have uncommitted changes
- Try `ivaldi diff --staged` if changes are staged
- Verify you're comparing the right commits

### Binary File Warnings

```
(binary file or read error)
```

**Explanation:** Binary files don't have meaningful line-by-line diffs.

**Alternative:** Use `--stat` to see which binary files changed.

### Commit Not Found

```
Error: commit not found: abc123
```

**Solutions:**
- Verify seal name or hash: `ivaldi log --oneline`
- Use full seal name: `swift-eagle-flies-high-447abe9b`
- Check if commit exists: `ivaldi seals show <name>`

### Overwhelming Output

If diff is too large:

```bash
# Use summary
ivaldi diff --stat

# Or paginate
ivaldi diff | less

# Or save to file
ivaldi diff > review.txt
```

## Advanced Usage

### Scripted Diff Analysis

```bash
#!/bin/bash
# Check if changes are safe to commit

ADDED=$(ivaldi diff --staged | grep -c "^+")
REMOVED=$(ivaldi diff --staged | grep -c "^-")

echo "Lines added: $ADDED"
echo "Lines removed: $REMOVED"

if [ $ADDED -gt 1000 ]; then
    echo "Warning: Large commit!"
fi
```

### Automated Code Review

```bash
#!/bin/bash
# Review script for pre-commit hook

if ivaldi diff --staged | grep -q "console.log"; then
    echo "Error: Found console.log in staged changes"
    exit 1
fi

if ivaldi diff --staged | grep -q "TODO"; then
    echo "Warning: Found TODO in staged changes"
fi
```

### Change Metrics

```bash
# Calculate churn
ivaldi diff abc123 def456 | grep "^+" | wc -l
ivaldi diff abc123 def456 | grep "^-" | wc -l
```

## Related Commands

- `ivaldi status` - Show which files are modified
- `ivaldi gather` - Stage files for commit
- `ivaldi reset` - Unstage files
- `ivaldi seal` - Create commit
- `ivaldi log` - View commit history
- `ivaldi seals show` - Show commit details
