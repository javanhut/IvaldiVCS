---
layout: default
title: ivaldi reset
---

# ivaldi reset

Unstage files or reset working directory changes.

## Synopsis

```bash
ivaldi reset [files...]
ivaldi reset --hard
```

## Description

Unstage files or discard changes in the working directory.

## Options

- `[files...]` - Unstage specific files
- `--hard` - Discard all uncommitted changes (dangerous!)

## Examples

### Unstage Files

```bash
ivaldi reset src/main.go
ivaldi reset .
```

### Discard All Changes

```bash
ivaldi reset --hard
```

Warning: This is destructive and cannot be undone!

## Use Cases

### Undo Staging

```bash
ivaldi gather file.txt
# Oops, didn't mean to stage
ivaldi reset file.txt
```

### Clean Workspace

```bash
ivaldi reset --hard
```

### Selective Unstaging

```bash
ivaldi gather .
ivaldi reset src/unwanted.go
```

## Related Commands

- [gather](gather.md) - Stage files
- [status](status.md) - Check staging state

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git reset file` | `ivaldi reset file` |
| `git reset --hard` | `ivaldi reset --hard` |
