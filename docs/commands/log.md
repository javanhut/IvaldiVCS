---
layout: default
title: ivaldi log
---

# ivaldi log

View commit history with seal names and metadata.

## Synopsis

```bash
ivaldi log
ivaldi log [options]
```

## Description

Display commit history showing seals in reverse chronological order with human-friendly seal names.

## Options

- `--oneline` - Concise one-line format
- `--limit <n>` - Show only last n commits
- `--all` - Show commits from all timelines

## Examples

### Basic Log

```bash
ivaldi log
```

Output:
```
Seal: swift-eagle-flies-high-447abe9b (447abe9b)
Timeline: main
Author: Jane Smith <jane@example.com>
Date: 2025-10-05 14:30:22 -0700

    Add authentication feature

    Implemented JWT-based authentication

Seal: calm-river-flows-deep-2a1b3c4d (2a1b3c4d)
Timeline: main
Author: Jane Smith <jane@example.com>
Date: 2025-10-04 09:15:33 -0700

    Initial commit
```

### Oneline Format

```bash
ivaldi log --oneline
```

Output:
```
447abe9b swift-eagle-flies-high-447abe9b Add authentication feature
2a1b3c4d calm-river-flows-deep-2a1b3c4d Initial commit
```

### Limit Results

```bash
ivaldi log --limit 10
```

### All Timelines

```bash
ivaldi log --all
```

## Use Cases

### Review Recent Work

```bash
ivaldi log --limit 5
```

### Find Commit

```bash
ivaldi log | grep "authentication"
```

### Generate Changelog

```bash
ivaldi log --oneline > changelog.txt
```

## Related Commands

- [travel](travel.md) - Interactive history browser
- [diff](diff.md) - Compare commits
- [whereami](whereami.md) - Current position

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git log` | `ivaldi log` |
| `git log --oneline` | `ivaldi log --oneline` |
| `git log -n 5` | `ivaldi log --limit 5` |
