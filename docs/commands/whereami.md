---
layout: default
title: ivaldi whereami
---

# ivaldi whereami

Show detailed information about the current timeline and position.

## Synopsis

```bash
ivaldi whereami
ivaldi wai
```

## Description

The `whereami` (or `wai`) command displays:
- Current timeline name and type
- Last seal with memorable name
- Remote sync status
- Workspace status
- Time since last seal

## Example Output

```bash
$ ivaldi whereami

Timeline: feature-auth
Type: Local Timeline
Last Seal: swift-eagle-flies-high-447abe9b (447abe9b)
  Message: "Add authentication feature"
  Author: Jane Doe <jane@example.com>
  Date: 2 hours ago
Remote: owner/repo (up to date)
Workspace: 3 files modified
```

## Shortened Version

Use the alias `wai` for quick checks:

```bash
$ ivaldi wai

Timeline: main
Last Seal: brave-wolf-runs-fast-abc12345 (just now)
Workspace: Clean
```

## Information Shown

### Timeline

Current timeline name:
```
Timeline: feature-auth
```

### Last Seal

Most recent commit with memorable name:
```
Last Seal: swift-eagle-flies-high-447abe9b (447abe9b)
  Message: "Add authentication feature"
  Date: 2 hours ago
```

### Remote Status

GitHub sync information:
```
Remote: owner/repo (up to date)
Remote: owner/repo (2 commits ahead)
Remote: Not configured
```

### Workspace Status

Current working directory state:
```
Workspace: Clean
Workspace: 3 files modified
Workspace: 5 files modified, 2 untracked
```

## Use Cases

### Quick Status Check

```bash
ivaldi wai
```

### Before Important Operations

```bash
# Before switching timelines
ivaldi whereami

# Before merging
ivaldi whereami
```

### Orientation

```bash
# After switching timelines
ivaldi timeline switch feature-auth
ivaldi wai
```

### Verify Location

```bash
# Confirm you're on right timeline
ivaldi whereami
```

## Common Workflows

### Daily Development

```bash
# Start of day
ivaldi wai  # See where you are

# Check after switching
ivaldi timeline switch feature-x
ivaldi wai
```

### Collaboration

```bash
# Check sync status
ivaldi wai

# See if remote has updates
ivaldi scout
ivaldi wai
```

## Related Commands

- [status](status.md) - Detailed file status
- [timeline](timeline.md) - Timeline management
- [log](log.md) - Commit history

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git branch --show-current` | `ivaldi whereami` |
| `git log -1` | (included in whereami) |
| `git status` | (workspace info included) |
| Multiple commands | Single command |

## Tips

- Use `wai` for quick checks (shorter alias)
- Run after switching timelines to verify
- Check before merges to confirm position
- Verify remote sync status regularly
