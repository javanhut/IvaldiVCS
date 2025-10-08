---
layout: default
title: ivaldi exclude
---

# ivaldi exclude

Add patterns to `.ivaldiignore` file.

## Synopsis

```bash
ivaldi exclude <patterns...>
```

## Description

Quickly add file patterns to `.ivaldiignore` to exclude files from version control.

## Arguments

- `<patterns...>` - File patterns to ignore

## Examples

### Exclude Build Artifacts

```bash
ivaldi exclude build/ dist/ *.exe
```

### Exclude Temporary Files

```bash
ivaldi exclude *.tmp *.log .DS_Store
```

### Exclude Directories

```bash
ivaldi exclude node_modules/ .cache/ .venv/
```

## Manual .ivaldiignore

You can also manually create `.ivaldiignore`:

```bash
cat > .ivaldiignore <<EOF
# Build artifacts
build/
dist/
*.exe

# Dependencies
node_modules/
.venv/

# IDE
.vscode/
.idea/

# OS
.DS_Store
Thumbs.db

# Logs
*.log
logs/
EOF
```

## Pattern Syntax

- `*.log` - All .log files
- `build/` - Directory (trailing slash)
- `**/*.tmp` - Nested files
- `test/**/*.txt` - Specific subdirectories

## Auto-Excluded Files

These are always excluded:
- `.env`, `.env.*`
- `.venv`, `.venv/`

## Important Notes

- `.ivaldiignore` itself is NEVER ignored
- Can always gather and commit `.ivaldiignore`
- Patterns support glob matching
- Empty lines and `#` comments allowed

## Common Workflows

### New Project

```bash
ivaldi forge
ivaldi exclude node_modules/ dist/ *.log
ivaldi gather .
ivaldi seal "Initial commit"
```

### Add Language-Specific Ignores

Python:
```bash
ivaldi exclude __pycache__/ *.pyc .venv/
```

Node.js:
```bash
ivaldi exclude node_modules/ dist/ .npm/
```

Go:
```bash
ivaldi exclude bin/ *.exe *.test
```

## Check Ignored Files

```bash
# Try to gather ignored file
ivaldi gather build/output.exe
# Will be skipped

# Check status
ivaldi status
# Ignored files won't appear
```

## Related Commands

- [gather](gather.md) - Stage files (respects excludes)
- [status](status.md) - See untracked vs ignored

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| Edit `.gitignore` | `ivaldi exclude <pattern>` |
| `.gitignore` | `.ivaldiignore` |
| Glob patterns | Glob patterns |
