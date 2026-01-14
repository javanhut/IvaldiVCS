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
- `<seal>` - Compare with specific seal (full hash or prefix)

## Hash Prefix Support

You can use short hash prefixes instead of full 64-character hashes:

```bash
# Full hash
ivaldi diff 447abe9b1234567890abcdef1234567890abcdef1234567890abcdef12345678

# Short prefix (minimum 4 characters)
ivaldi diff 447a
ivaldi diff 447abe9b
```

The prefix must be unique - if multiple commits match, you'll be prompted to use a longer prefix.

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
