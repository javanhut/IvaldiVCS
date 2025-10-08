---
layout: default
title: ivaldi gather
---

# ivaldi gather

Stage files for the next seal (commit).

## Synopsis

```bash
ivaldi gather [<files>...]
ivaldi gather [options]
```

## Description

The `gather` command stages files to be included in the next seal. It:
- Adds files to the staging area
- Respects `.ivaldiignore` patterns
- Prompts for confirmation when staging hidden files
- Provides security warnings for sensitive files

## Options

- `--allow-all` - Skip interactive prompts for hidden files (useful for automation)

## Examples

### Stage Specific Files

```bash
ivaldi gather README.md src/main.go
```

### Stage Directory

```bash
ivaldi gather src/
```

### Stage All Changes

```bash
ivaldi gather .
```

### Stage All (Interactive)

```bash
ivaldi gather
```

Prompts for:
- Hidden files (files starting with `.`)
- Directories to include

### Automation Mode

```bash
ivaldi gather --allow-all
```

Skips prompts but shows warnings for sensitive files.

## Security Features

### Auto-Excluded Files

These files are always excluded for security:
- `.env`, `.env.*` - Environment files
- `.venv`, `.venv/` - Python virtual environments

### Interactive Prompts

Hidden files require confirmation:
```bash
$ ivaldi gather .

Found hidden files:
  .config
  .secrets

Include hidden files? (y/n):
```

Exception: `.ivaldiignore` never prompts and can always be staged.

### Warnings

Sensitive files trigger warnings:
```bash
$ ivaldi gather --allow-all

WARNING: Staging hidden file: .config
WARNING: Consider adding to .ivaldiignore
```

## Ignore Files

Create `.ivaldiignore` to exclude files:

```bash
# Create ignore file
ivaldi exclude build/ dist/ *.exe

# Or manually create
cat > .ivaldiignore <<EOF
node_modules/
*.log
.DS_Store
EOF
```

See [exclude](exclude.md) for details.

## Common Workflows

### Daily Work

```bash
# Check what changed
ivaldi status

# Stage modified files
ivaldi gather src/

# Create seal
ivaldi seal "Update feature"
```

### Selective Staging

```bash
# Stage specific changes
ivaldi gather src/auth.go src/login.go

# Review staged
ivaldi status

# Seal when ready
ivaldi seal "Fix authentication"
```

### Gathering Everything

```bash
# Interactive
ivaldi gather

# Or non-interactive
ivaldi gather .
```

## File States

After gathering, files can be:
- **Staged**: Ready for next seal
- **Modified but unstaged**: Changed but not gathered
- **Untracked**: New files not yet gathered

Check with:
```bash
ivaldi status
```

## Related Commands

- [seal](seal.md) - Create commit with staged files
- [status](status.md) - Show staged and unstaged changes
- [reset](reset.md) - Unstage files
- [exclude](exclude.md) - Add to `.ivaldiignore`

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git add file.txt` | `ivaldi gather file.txt` |
| `git add .` | `ivaldi gather .` |
| `git add -A` | `ivaldi gather` |
| Interactive add | Prompts for hidden files |
| No security checks | Auto-excludes `.env`, warns on hidden |

## Troubleshooting

### File Not Staged

If a file isn't staging:
```bash
# Check if ignored
cat .ivaldiignore

# Check status
ivaldi status
```

### Hidden File Skipped

Use `--allow-all` or respond "yes" to prompts:
```bash
ivaldi gather --allow-all
```

### Sensitive File Warning

Add to `.ivaldiignore`:
```bash
ivaldi exclude .env .secrets
```
