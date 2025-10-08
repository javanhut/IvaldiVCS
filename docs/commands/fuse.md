---
layout: default
title: ivaldi fuse
---

# ivaldi fuse

Merge timelines together using intelligent chunk-level conflict resolution.

## Synopsis

```bash
ivaldi fuse <source> to <target>
ivaldi fuse --strategy=<type> <source> to <target>
ivaldi fuse --continue
ivaldi fuse --abort
```

## Description

The `fuse` command merges two timelines using intelligent chunk-level resolution. Unlike Git's line-based merging, Ivaldi uses:
- **Chunk-level intelligence**: 64KB chunk granularity
- **Content-hash based**: BLAKE3 hashes detect identical changes
- **Clean workspace**: No conflict markers in files
- **Multiple strategies**: auto, ours, theirs, union, base

## Options

- `--strategy=<type>` - Conflict resolution strategy (default: auto)
- `--continue` - Continue merge after resolving conflicts
- `--abort` - Abandon current merge

## Merge Strategies

### auto (Default)

Intelligent three-way merge:
- Automatically resolves non-conflicting changes
- Detects identical changes via content hashes
- Only flags truly conflicting chunks

```bash
ivaldi fuse feature to main
```

### theirs

Accept all changes from source timeline:
- No conflicts possible
- Use when source is authoritative

```bash
ivaldi fuse --strategy=theirs upstream-main to main
```

### ours

Keep all changes from target timeline:
- No conflicts possible
- Source changes ignored

```bash
ivaldi fuse --strategy=ours experimental to main
```

### union

Combine changes from both timelines:
- Merges non-duplicate chunks
- Useful for append-only files

```bash
ivaldi fuse --strategy=union feature-docs to main
```

### base

Revert to common ancestor:
- Discards changes from both timelines
- Rare use case

```bash
ivaldi fuse --strategy=base problematic to main
```

## Examples

### Basic Merge

```bash
ivaldi fuse feature-auth to main
```

### Fast-Forward Merge

```bash
$ ivaldi fuse feature-login to main

[MERGE] Fast-forward merge detected
>> Updating main timeline...

[OK] Merge completed successfully!
  Timeline main updated
```

### Three-Way Merge

```bash
$ ivaldi fuse feature-payment to main

Analyzing timelines for merge...

Changes to be merged:
  2 files will be modified

Proceed with merge? (yes/no): yes

>> Creating merge commit...

[OK] Merge completed successfully!
  Merge seal: merge-feature-payment-abc123
```

### Merge with Strategy

```bash
# Accept all source changes
ivaldi fuse --strategy=theirs feature to main

# Keep all target changes
ivaldi fuse --strategy=ours feature to main

# Combine both
ivaldi fuse --strategy=union changelog to main
```

## Conflict Resolution

### When Conflicts Occur

```bash
$ ivaldi fuse feature-auth to main

[CONFLICTS] Merge conflicts detected:

  CONFLICT: src/auth.go
  CONFLICT: src/config.go

>> 2 file(s) with conflicts

Resolution options:
  ivaldi fuse --continue - Use interactive resolver
  ivaldi fuse --strategy=theirs feature-auth to main
  ivaldi fuse --strategy=ours feature-auth to main
  ivaldi fuse --abort - Abort merge
```

Important: Your workspace files remain clean. No conflict markers written.

### Option 1: Choose Strategy

```bash
# Accept source changes
ivaldi fuse --strategy=theirs feature-auth to main

# Or keep target changes
ivaldi fuse --strategy=ours feature-auth to main
```

### Option 2: Manual Resolution

```bash
# Edit conflicted files
vim src/auth.go

# Stage resolved files
ivaldi gather src/auth.go

# Continue merge
ivaldi fuse --continue
```

### Option 3: Abort

```bash
ivaldi fuse --abort
```

## Common Workflows

### Feature Integration

```bash
# Ensure feature is complete
ivaldi timeline switch feature-payment
ivaldi status

# Switch to main
ivaldi timeline switch main

# Merge feature
ivaldi fuse feature-payment to main
```

### Sync Feature with Main

```bash
# On feature timeline
ivaldi timeline switch feature-auth

# Merge main into feature
ivaldi fuse main to feature-auth
```

### Release Preparation

```bash
ivaldi timeline switch main

# Merge multiple features
ivaldi fuse feature-auth to main
ivaldi fuse feature-payment to main
ivaldi fuse feature-ui to main
```

## Merge Types

### Fast-Forward

When target is ancestor of source:
```
Before:
  main:    A---B
                \
  feature:       C---D

After:
  main:    A---B---C---D
```

No merge commit created.

### Three-Way

When both timelines diverged:
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

Creates merge commit (M).

## Why Ivaldi's Merge is Superior

| Aspect | Git | Ivaldi |
|--------|-----|--------|
| Conflict markers | Written to files | Never written |
| Resolution | Manual file editing | Strategy or interactive |
| Granularity | Line-based | Chunk-based (64KB) |
| False conflicts | Common | Rare (hash-based) |
| Workspace | Polluted with markers | Always clean |
| Identical changes | May conflict | Auto-merged (same hash) |

## Best Practices

### Keep Working Directory Clean

```bash
ivaldi status  # Should be clean
ivaldi fuse feature to main
```

### Review Before Merging

```bash
ivaldi log --all
ivaldi diff <target-hash> <source-hash>
ivaldi fuse feature to main
```

### Merge Regularly

- Merge main into feature frequently
- Keep feature timelines small and focused

### Test After Merging

```bash
ivaldi fuse feature to main
# Run tests
make test
```

## Related Commands

- [timeline](timeline.md) - Create and switch timelines
- [log](log.md) - View timeline history
- [diff](diff.md) - Compare changes
- [status](status.md) - Check working directory

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git merge branch` | `ivaldi fuse branch to main` |
| `git merge --continue` | `ivaldi fuse --continue` |
| `git merge --abort` | `ivaldi fuse --abort` |
| `git merge --strategy` | `--strategy` option |

## Troubleshooting

### Timeline Not Found

```
Error: timeline not found: feature-xyz
```

Solution:
```bash
ivaldi timeline list
```

### Unresolved Conflicts

```
Error: unresolved conflicts
```

Solution:
```bash
# Resolve conflicts
vim src/file.go
ivaldi gather src/file.go
ivaldi fuse --continue
```

### Merge Already in Progress

```
Error: merge already in progress
```

Solutions:
```bash
# Complete merge
ivaldi fuse --continue

# Or abort
ivaldi fuse --abort
```
