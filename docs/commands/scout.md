---
layout: default
title: ivaldi scout
---

# ivaldi scout

Discover available remote timelines (branches) on GitHub.

## Synopsis

```bash
ivaldi scout
ivaldi scout --refresh
```

## Description

Scout discovers what timelines (branches) are available on the remote GitHub repository without downloading them.

## Options

- `--refresh` - Force refresh of remote information

## Examples

### Discover Remote Timelines

```bash
$ ivaldi scout

Remote timelines available:
  main
  feature-auth
  feature-payment
  bugfix-security

Use 'ivaldi harvest <name>' to download
```

### Force Refresh

```bash
ivaldi scout --refresh
```

## Use Cases

### Before Harvesting

```bash
ivaldi scout
ivaldi harvest feature-auth
```

### Check for Updates

```bash
ivaldi scout
# See if teammates pushed new branches
```

### Team Collaboration

```bash
# Daily standup
ivaldi scout
# See what branches team is working on
```

## Common Workflows

### Discover and Fetch

```bash
ivaldi scout
ivaldi harvest feature-x feature-y
```

### Check Remote State

```bash
ivaldi scout
ivaldi timeline list
# Compare local vs remote
```

## Related Commands

- [harvest](harvest.md) - Download timelines
- [portal](portal.md) - Configure GitHub connection
- [upload](upload.md) - Push timelines

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git fetch` (metadata) | `ivaldi scout` |
| `git ls-remote` | `ivaldi scout` |

## Notes

- Lightweight operation (no data download)
- Requires portal configuration
- Shows all branches on remote
