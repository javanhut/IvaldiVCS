---
layout: default
title: ivaldi status
---

# ivaldi status

Display the current state of the working directory and staging area.

## Synopsis

```bash
ivaldi status
```

## Description

The `status` command shows:
- Current timeline
- Staged files (ready for seal)
- Modified files (not staged)
- Untracked files (not in version control)
- Workspace state

## Example Output

```bash
$ ivaldi status

Timeline: feature-auth
Last seal: swift-eagle-flies-high-447abe9b

Staged changes:
  modified: src/auth.go
  new file: src/login.go

Unstaged changes:
  modified: src/config.go
  modified: README.md

Untracked files:
  tests/auth_test.go
  .env.local

Working directory: 3 files modified, 1 untracked
```

## File States

### Staged

Files ready for next seal:
```
Staged changes:
  modified: src/auth.go
  new file: src/login.go
```

### Unstaged

Modified but not staged:
```
Unstaged changes:
  modified: src/config.go
```

### Untracked

New files not in version control:
```
Untracked files:
  tests/new_test.go
```

## Common Workflows

### Check Status Before Commit

```bash
ivaldi status
ivaldi gather .
ivaldi seal "Update features"
```

### Verify Staging

```bash
ivaldi gather src/
ivaldi status  # Verify correct files staged
ivaldi seal "Update source files"
```

### Review Changes

```bash
ivaldi status  # See what changed
ivaldi diff   # See specific changes
```

## Clean Working Directory

When everything is committed:

```bash
$ ivaldi status

Timeline: main
Last seal: brave-wolf-runs-fast-abc12345

Working directory: clean
```

## Related Commands

- [gather](gather.md) - Stage files
- [seal](seal.md) - Create commit
- [diff](diff.md) - See specific changes
- [whereami](whereami.md) - Detailed timeline info

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git status` | `ivaldi status` |
| Shows branch | Shows timeline |
| Staged/unstaged | Staged/unstaged |
| Untracked | Untracked |

## Tips

- Run `status` frequently to stay aware of changes
- Verify `status` shows correct files before `seal`
- Clean working directory makes timeline switching easier
