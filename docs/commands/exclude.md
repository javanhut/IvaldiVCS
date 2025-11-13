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

- `*.log` - All .log files (anywhere in tree)
- `build/` - Directory and all contents (trailing slash recommended)
- `build` - Also matches directory (without trailing slash)
- `**/*.tmp` - Nested files matching pattern
- `test/**/*.txt` - Specific subdirectories

### Directory Exclusion

When excluding directories, **always use a trailing slash** for clarity:

```bash
ivaldi exclude node_modules/    # Excludes entire directory tree
ivaldi exclude dist/            # Excludes dist and all subdirectories
ivaldi exclude .cache/          # Excludes .cache directory
```

**Important**: Directory patterns exclude the entire directory tree. For example:
- `node_modules/` excludes `node_modules/`, `node_modules/package1/`, `node_modules/package1/src/index.js`, etc.
- The system will skip traversing into excluded directories, making `gather` much faster

## Auto-Excluded Files

These patterns are **always automatically excluded** for security:
- `.env` - Environment files
- `.env.*` - All environment file variants (`.env.local`, `.env.production`, etc.)
- `.venv` - Python virtual environment directory
- `.venv/` - Python virtual environment directory

**Note**: Auto-excluded directories are completely skipped during file gathering

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

## Performance Benefits

Directory exclusion provides significant performance improvements:

```bash
# Without exclusion: walks through 50,000+ files in node_modules
ivaldi gather .

# With exclusion: skips node_modules entirely
echo "node_modules/" >> .ivaldiignore
ivaldi gather .  # Much faster!
```

**Best Practice**: Always exclude large dependency directories:
- `node_modules/` for Node.js
- `.venv/` and `venv/` for Python
- `vendor/` for Go/PHP
- `target/` for Rust/Java
- `build/` and `dist/` for build outputs

## Related Commands

- [gather](gather.md) - Stage files (respects excludes)
- [status](status.md) - See untracked vs ignored

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| Edit `.gitignore` | `ivaldi exclude <pattern>` |
| `.gitignore` | `.ivaldiignore` |
| Glob patterns | Glob patterns |
