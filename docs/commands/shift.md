---
layout: default
title: ivaldi shift
---

# ivaldi shift

Interactively squash multiple commits into a single commit for cleaner history.

## Synopsis

```bash
ivaldi shift                 # Interactive mode - select range with arrow keys
ivaldi shift --last N        # Squash last N commits
ivaldi shift <start> <end>   # Squash specific range by seal name or hash
```

## Description

The `shift` command allows you to combine multiple commits into a single commit, creating a cleaner commit history. This is particularly useful before pushing to a remote repository or when you have many small WIP commits that should be combined.

Unlike Git's interactive rebase, `shift` provides a simple, guided interface for squashing commits.

## Features

- Interactive commit range selection with arrow keys
- Automatic combined commit message generation
- Safety confirmations before rewriting history
- Force push warnings and guidance
- Support for multiple selection modes

## Options

- `--last N` - Squash the last N commits (must be at least 2)
- No options - Interactive mode with visual selection

## Usage Modes

### Interactive Mode (Recommended)

The interactive mode provides a two-phase selection process:

```bash
ivaldi shift
```

**Phase 1: Select START commit (oldest)**
- Navigate through commits using arrow keys
- Press Enter to select the oldest commit in range
- Press Q to cancel

**Phase 2: Select END commit (newest)**
- Navigate through filtered commits
- Start commit is marked with [START]
- Press Enter to select the newest commit
- Press Q to cancel

**Phase 3: Review and Confirm**
- Review all commits to be squashed
- Enter custom commit message or use suggested one
- Confirm the operation with 'yes'

### Last N Commits Mode

Quickly squash the most recent commits:

```bash
# Squash last 3 commits
ivaldi shift --last 3

# Squash last 5 commits
ivaldi shift --last 5
```

### Specific Range Mode

Squash a specific range using seal names or hashes:

```bash
# Using seal names
ivaldi shift swift-eagle bold-hawk

# Using partial hashes
ivaldi shift 447abe9b 7bb05886

# Mixing seal names and hashes
ivaldi shift swift-eagle 7bb05886
```

## Examples

### Example 1: Clean Up WIP Commits

```bash
# You have 3 WIP commits to clean up
$ ivaldi log --oneline
447abe9b swift-eagle-flies-high WIP: Add authentication
7bb05886 empty-phoenix-attacks WIP: Fix login bug
3c4d5e6f brave-wolf-runs-fast  WIP: Update tests

# Squash them interactively
$ ivaldi shift

⏱ Select START of commit range (oldest):

→ 1. swift-eagle-flies-high-447abe9b (447abe9b)
     WIP: Add authentication
  
  2. empty-phoenix-attacks-7bb05886 (7bb05886)
     WIP: Fix login bug
  
  3. brave-wolf-runs-fast-3c4d5e6f (3c4d5e6f)
     WIP: Update tests

[Select commit 3 with Enter]

⏱ Select END of commit range (newest):

→ 1. swift-eagle-flies-high-447abe9b (447abe9b)
     WIP: Add authentication
  
  2. empty-phoenix-attacks-7bb05886 (7bb05886)
     WIP: Fix login bug
  
  3. brave-wolf-runs-fast-3c4d5e6f (3c4d5e6f)  [START]
     WIP: Update tests

[Select commit 1 with Enter]

✓ Range selected: 3 commits will be squashed

📋 Review commits to squash:

[✓] 1. 3c4d5e6f - WIP: Update tests
[✓] 2. 7bb05886 - WIP: Fix login bug
[✓] 3. 447abe9b - WIP: Add authentication

Suggested commit message:
Squashed 3 commits:

WIP: Update tests
WIP: Fix login bug
WIP: Add authentication

Enter new commit message (or press Enter to use suggested): 
Add authentication with login fixes and tests

⚠ WARNING: This will rewrite commit history!
  • 3 commits will be replaced with 1 commit
  • You will need to force push: ivaldi upload --force

Confirm squash? (yes/no): yes

🔨 Squashing commits...
✓ Created squashed commit: bold-hawk-soars-9a8b7c6d (9a8b7c6d)
✓ Timeline updated
✓ 3 commits squashed into 1

⚠ Remote history differs. Push with --force to update:
  ivaldi upload --force
```

### Example 2: Squash Last N Commits

```bash
# Quick squash of last 4 commits
$ ivaldi shift --last 4

✓ Range selected: 4 commits will be squashed

📋 Review commits to squash:

[✓] 1. 1a2b3c4d - Fix typo
[✓] 2. 5e6f7g8h - Add comment
[✓] 3. 9i0j1k2l - Refactor function
[✓] 4. 3m4n5o6p - Add feature X

Enter new commit message: Implement feature X with refactoring

Confirm squash? (yes/no): yes

✓ Created squashed commit: clever-fox-jumps-high-8h9i0j1k
✓ 4 commits squashed into 1
```

### Example 3: Force Push After Squash

```bash
# After squashing, update remote
$ ivaldi upload --force

⚠ WARNING: Force push will OVERWRITE remote history!
This is a destructive operation that:
  • Rewrites commit history on the remote
  • Can cause issues for collaborators
  • Cannot be undone easily

💡 Tip: Consider creating a backup branch first:
  ivaldi timeline create backup-before-force-push
  ivaldi upload github:owner/repo backup-before-force-push

Type 'force push' to confirm: force push

Uploading to GitHub: owner/repo (branch: main)...
✓ Force pushed to GitHub
⚠ Make sure to notify collaborators about the history rewrite
```

## Workflow

### Complete Squash and Push Workflow

```bash
# 1. Check your commits
ivaldi log

# 2. Squash commits interactively
ivaldi shift

# 3. Verify the result
ivaldi log

# 4. Create backup (optional but recommended)
ivaldi timeline create backup-before-push

# 5. Force push to remote
ivaldi upload --force
```

### Before Pushing to Remote

Always squash before pushing if you have:
- Multiple WIP commits
- Debugging commits
- Experimental commits
- "Fix typo" commits

```bash
# Make your changes
ivaldi gather .
ivaldi seal "WIP: Feature progress"
ivaldi seal "WIP: More work"
ivaldi seal "WIP: Almost done"
ivaldi seal "WIP: Final touches"

# Clean up before pushing
ivaldi shift --last 4
# Enter: "Implement feature X"

# Now push clean history
ivaldi upload --force
```

## Safety Features

### Confirmation Requirements

`shift` requires explicit confirmation:
1. Interactive selection confirmation
2. Commit message entry
3. "yes" typed to confirm squash
4. "force push" typed to confirm force push

### Pre-Squash Checks

Before squashing, `shift`:
- Validates commit range
- Ensures commits are connected
- Checks for cycles in history
- Verifies all commits exist

### Post-Squash Warnings

After squashing, you'll see:
- Force push requirement warning
- Suggestion to notify collaborators
- Backup branch creation tip

## Common Use Cases

### 1. Clean Up Before Pull Request

```bash
# You have messy local commits
ivaldi log --oneline

# Squash into logical commits
ivaldi shift --last 10
# Message: "Add user authentication feature"

ivaldi upload --force
# Create clean pull request
```

### 2. Combine Related Changes

```bash
# Multiple commits for one feature
$ ivaldi shift swift-eagle brave-wolf
# Combines all commits between these seals
```

### 3. Remove Debugging Commits

```bash
# After debugging session with many commits
ivaldi shift --last 15
# Message: "Fix authentication bug"
```

## Best Practices

### DO:
- Squash before pushing to remote
- Create descriptive commit messages
- Review commits before confirming
- Create backup branches for safety
- Notify team before force pushing

### DON'T:
- Squash after pushing (requires force push)
- Squash commits others have based work on
- Skip reviewing the commit list
- Force push without warning team
- Squash across merge commits

## Safety Recommendations

### 1. Create Backup Branch

```bash
ivaldi timeline create backup-$(date +%Y%m%d)
ivaldi shift --last 5
```

### 2. Test Locally First

```bash
# Squash locally
ivaldi shift --last 3

# Test thoroughly
ivaldi status
ivaldi log

# If good, then force push
ivaldi upload --force
```

### 3. Coordinate with Team

Before force pushing to shared branches:
1. Notify team in chat/email
2. Ensure no one is working on the branch
3. Ask team to re-fetch after push

## Troubleshooting

### Error: Timeline has no commits yet

```
Error: timeline has no commits yet
```

Solution: Create at least one commit first:
```bash
ivaldi gather .
ivaldi seal "Initial commit"
ivaldi shift  # Now works
```

### Error: Need at least 2 commits

```
Error: need at least 2 commits to squash
```

Solution: You can't squash a single commit. Make more commits:
```bash
ivaldi seal "Second commit"
ivaldi shift --last 2
```

### Error: End commit is not a descendant

```
Error: end commit is not a descendant of start commit
```

Solution: Verify commit order. Start must be older than end:
```bash
# Wrong order:
ivaldi shift bold-hawk swift-eagle  # bold-hawk is newer

# Correct order:
ivaldi shift swift-eagle bold-hawk   # swift-eagle is older
```

### Error: Invalid commit range

```
Error: invalid commit range: end commit is not a descendant of start commit
```

Solution: Selected commits are not connected. Use `ivaldi log` to find correct range.

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git rebase -i HEAD~3` | `ivaldi shift --last 3` |
| Edit, pick, squash in editor | Interactive arrow key selection |
| Manual conflict resolution | Automatic handling |
| Reword/edit/squash/fixup | Simple squash with message |
| `git push --force` | `ivaldi upload --force` |

## Advanced Usage

### Squash Non-Adjacent Commits

To squash non-adjacent commits, create a new timeline:

```bash
# Can't directly squash commit 1 and 3 (skipping 2)
# Instead:

# 1. Travel to before unwanted commits
ivaldi travel
# Select commit before commit 2, diverge to 'cleaned'

# 2. Cherry-pick wanted changes (manual)
# Apply changes from commit 1 and 3

# 3. Create new commit
ivaldi seal "Combined commits 1 and 3"
```

### Squash Across Branches

```bash
# Merge first
ivaldi fuse feature-branch to main

# Then squash
ivaldi shift --last 5
```

## Related Commands

- [seal](seal.md) - Create commits
- [travel](travel.md) - Time travel through history
- [timeline](timeline.md) - Manage timelines/branches
- [upload](upload.md) - Push to remote (with --force)
- [log](log.md) - View commit history

## See Also

- [Force Push Safety](../guides/force-push-safety.md)
- [Collaboration Guide](../guides/collaboration.md)
- [GitHub Integration](../guides/github-integration.md)
