---
layout: default
title: ivaldi diff
---

# ivaldi diff

Compare changes between commits, staged files, or working directory.

## Synopsis

```bash
ivaldi diff
ivaldi diff [options]
ivaldi diff <seal>
```

## Description

Show differences between:
- Working directory and last seal
- Staged files and last seal
- Two specific seals

## Options

- `--staged` - Show staged changes
- `--stat` - Show summary statistics
- `<seal>` - Compare with specific seal

## Examples

### Working Directory Changes

```bash
ivaldi diff
```

### Staged Changes

```bash
ivaldi diff --staged
```

### Compare with Seal

```bash
ivaldi diff swift-eagle-flies-high
ivaldi diff 447abe9b
```

### Statistics Only

```bash
ivaldi diff --stat
```

## Use Cases

### Review Before Commit

```bash
ivaldi diff
ivaldi gather .
ivaldi diff --staged
ivaldi seal "Changes"
```

### Compare Versions

```bash
ivaldi log --oneline
ivaldi diff abc123 def456
```

## Related Commands

- [status](status.md) - See which files changed
- [log](log.md) - Find seals to compare

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git diff` | `ivaldi diff` |
| `git diff --staged` | `ivaldi diff --staged` |
| `git diff <commit>` | `ivaldi diff <seal>` |
