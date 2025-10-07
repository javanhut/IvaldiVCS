# Ivaldi Exclude Feature

The exclude feature in Ivaldi allows you to specify files and directories that should be ignored during gather (staging) operations, similar to `.gitignore` in Git.

## Overview

Ivaldi uses a `.ivaldiignore` file to define patterns for files and directories to exclude from version control. The exclude system integrates seamlessly with the `gather` command to prevent ignored files from being staged, even when explicitly specified.

### Security Features

Ivaldi includes built-in security protections:

1. **Auto-Excluded Files**: Certain sensitive files are automatically excluded (`.env`, `.env.*`, `.venv`)
2. **Interactive Dot File Prompts**: Hidden files (starting with `.`) require explicit confirmation before gathering
3. **Exception for `.ivaldiignore`**: The ignore file itself is always gatherable without prompting

## Using the Exclude Command

The `ivaldi exclude` command provides a convenient way to add patterns to `.ivaldiignore`:

```bash
# Add single pattern
ivaldi exclude build/

# Add multiple patterns at once
ivaldi exclude *.log *.tmp node_modules/

# Patterns are appended to .ivaldiignore
ivaldi exclude dist/
```

### Command Behavior

- Creates `.ivaldiignore` if it doesn't exist
- Appends patterns to existing `.ivaldiignore` file
- Each pattern is added on a new line
- Confirms successful addition of patterns

## Pattern Syntax

### Basic Patterns

```bash
# Ignore specific file
ivaldi exclude config.local.js

# Ignore all files with extension
ivaldi exclude *.log

# Ignore directory (trailing slash recommended)
ivaldi exclude build/
```

### Directory Patterns

```bash
# Ignore entire directory
ivaldi exclude node_modules/
ivaldi exclude dist/
ivaldi exclude .cache/

# Pattern matches any directory with this name
ivaldi exclude tmp/
```

### Wildcard Patterns

```bash
# Ignore all .log files anywhere
ivaldi exclude *.log

# Ignore all .tmp files in test directory and subdirectories
ivaldi exclude test/**/*.tmp

# Ignore all files in any build directory
ivaldi exclude **/build/
```

### Common Patterns

```bash
# Build artifacts
ivaldi exclude build/ dist/ out/ bin/

# Dependencies
ivaldi exclude node_modules/ vendor/

# Temporary files
ivaldi exclude *.tmp *.swp *~ .DS_Store

# Logs
ivaldi exclude *.log logs/

# IDE and editor files
ivaldi exclude .vscode/ .idea/ *.sublime-*

# OS files
ivaldi exclude .DS_Store Thumbs.db
```

## Manual .ivaldiignore Creation

You can also create or edit `.ivaldiignore` manually:

```
# Build outputs
build/
dist/
*.exe
*.dll

# Dependencies
node_modules/
vendor/

# Logs and temporary files
*.log
*.tmp
.cache/

# IDE files
.vscode/
.idea/
*.sublime-workspace

# OS files
.DS_Store
Thumbs.db

# Test coverage
coverage/
*.coverage
```

### Comments and Empty Lines

- Lines starting with `#` are comments and ignored
- Empty lines are ignored
- Use comments to organize your ignore patterns

## Integration with Gather Command

The `gather` command automatically respects `.ivaldiignore` patterns and includes security features:

### Auto-Excluded Files (Security)

Certain files are **always automatically excluded** for security:

```bash
# These files are auto-excluded and cannot be gathered
.env
.env.local
.env.production
.venv/
```

```bash
# Attempting to gather these will show a security warning
ivaldi gather .env
# Warning: File '.env' is auto-excluded for security, skipping
```

### Interactive Dot File Prompts

When gathering hidden files (except `.ivaldiignore`), Ivaldi prompts for confirmation:

```bash
# Gathering a dot file triggers an interactive prompt
ivaldi gather .vscode/settings.json

# Output:
# Warning: '.vscode/settings.json' is a hidden file.
# Do you want to gather this file? (y/N):
```

```bash
# User response 'y' or 'yes' will gather the file
# Any other response skips the file
```

### Bypassing Prompts with --allow-all

Use the `--allow-all` (or `-a`) flag to skip prompts but still show warnings:

```bash
# Gather all files including dot files without prompting
ivaldi gather --allow-all

# Output:
# Warning: Gathering hidden file: .vscode/settings.json
# Warning: Gathering hidden file: .editorconfig
# Auto-excluded for security: .env
```

```bash
# Short form
ivaldi gather -a
```

### Automatic Filtering

```bash
# This will skip all ignored files
ivaldi gather

# This will skip ignored files, even if explicitly specified
ivaldi gather build/ node_modules/
# Warning: File 'build/app.js' is in .ivaldiignore, skipping
```

### Explicit File Handling

When you try to gather a specific ignored file:

```bash
# If test.log is in .ivaldiignore
ivaldi gather test.log
# Warning: File 'test.log' is in .ivaldiignore, skipping
```

### Directory Gathering

When gathering a directory containing ignored files:

```bash
ivaldi gather src/
# Skipping ignored file: src/build/output.js
# Skipping ignored file: src/temp/cache.tmp
# Gathered: src/main.js
# Gathered: src/utils.js
```

## Important: .ivaldiignore is Never Ignored

**The `.ivaldiignore` file itself is NEVER ignored**, regardless of patterns in the file. This ensures:

- You can always gather and commit `.ivaldiignore` changes
- The ignore file is tracked in version control
- Team members receive updates to ignore patterns

```bash
# This always works, even if *.ignore is in .ivaldiignore
ivaldi gather .ivaldiignore
# Gathered: .ivaldiignore
```

## Pattern Matching Details

### How Patterns are Matched

1. **Full Path Matching**: Pattern is matched against the full relative path
2. **Basename Matching**: Pattern is matched against just the filename
3. **Directory Patterns**: Patterns ending with `/` match directories and their contents
4. **Wildcard Support**: `*` matches any characters, `**` matches multiple directory levels

### Pattern Examples

| Pattern | Matches | Doesn't Match |
|---------|---------|---------------|
| `*.log` | `app.log`, `src/debug.log` | `log.txt`, `logger.js` |
| `build/` | `build/app.js`, `build/assets/img.png` | `dist/build.js` |
| `test/**/*.tmp` | `test/unit/temp.tmp`, `test/a/b/c.tmp` | `src/test.tmp` |
| `.DS_Store` | `.DS_Store`, `dir/.DS_Store` | `DS_Store` |
| `**/node_modules/` | `node_modules/`, `pkg/node_modules/` | `node_modules.bak/` |

## Checking Ignored Files

Use `ivaldi status --ignored` to see which files are being ignored:

```bash
ivaldi status --ignored

On timeline main
Last seal: swift-eagle-flies-high-447abe9b
Files tracked in last seal: 25

Ignored files:
  build/app.js
  node_modules/express/index.js
  test.log
  .DS_Store
```

## Workflow Examples

### Setting Up a New Project

```bash
# Initialize repository
ivaldi forge

# Set up common ignore patterns
ivaldi exclude node_modules/ dist/ build/
ivaldi exclude *.log *.tmp .DS_Store

# Note: .env files are auto-excluded, but you can add them to .ivaldiignore for documentation
ivaldi exclude .env .env.local

# Commit the ignore file (.ivaldiignore is never prompted)
ivaldi gather .ivaldiignore
ivaldi seal "Add ignore patterns"
```

### Gathering Files Interactively

```bash
# Gather all files - prompts for each dot file
ivaldi gather

# Output:
# Warning: '.eslintrc.json' is a hidden file.
# Do you want to gather this file? (y/N): y
# ✓ Gathering: .eslintrc.json
#
# Warning: '.vscode/settings.json' is a hidden file.
# Do you want to gather this file? (y/N): n
# ✗ Skipped: .vscode/settings.json
#
# Auto-excluded for security: .env
```

### Gathering Without Prompts

```bash
# Use --allow-all to skip prompts (for automation or batch operations)
ivaldi gather --allow-all

# Output:
# Warning: Gathering hidden file: .eslintrc.json
# Warning: Gathering hidden file: .prettierrc
# Auto-excluded for security: .env
# Gathered: src/main.js
# Gathered: src/utils.js
```

### Adding Build Directory to Ignore

```bash
# Add build directory to ignore
ivaldi exclude build/ dist/

# Verify it's ignored
ivaldi gather .
# (build/ and dist/ files won't be staged)

# Commit the updated ignore file
ivaldi gather .ivaldiignore
ivaldi seal "Ignore build directories"
```

### Ignoring IDE Files

```bash
# Ignore common IDE files
ivaldi exclude .vscode/ .idea/ *.sublime-*

# Commit the changes
ivaldi gather .ivaldiignore
ivaldi seal "Ignore IDE configuration files"
```

## Best Practices

1. **Commit `.ivaldiignore` Early**: Set up ignore patterns at project start
2. **Use Comments**: Organize patterns with comments for clarity
3. **Be Specific**: Use precise patterns to avoid accidentally ignoring needed files
4. **Test Patterns**: Use `ivaldi status` to verify patterns work as expected
5. **Share Patterns**: Commit `.ivaldiignore` so team members have consistent ignore rules
6. **Directory Trailing Slash**: Use trailing `/` for directories for clarity
7. **Avoid Over-Ignoring**: Don't ignore files that should be in version control
8. **Be Careful with Dot Files**: Review each dot file prompt carefully before accepting
9. **Use --allow-all for Automation**: In scripts/CI, use `--allow-all` to avoid interactive prompts
10. **Never Commit Secrets**: The auto-exclude for `.env` files helps prevent accidental secret commits

## Comparison with Git

| Feature | Git | Ivaldi |
|---------|-----|--------|
| Ignore file | `.gitignore` | `.ivaldiignore` |
| Add patterns | Manual edit or echo >> | `ivaldi exclude` or manual |
| Command integration | `git add` respects ignore | `gather` respects ignore |
| Ignore file tracking | Can be ignored | NEVER ignored |
| Pattern syntax | Git patterns | Glob patterns |
| Explicit add override | `git add -f` | Not supported (by design) |
| Auto-exclude sensitive files | No | Yes (.env, .venv) |
| Dot file prompts | No | Yes (interactive) |
| Skip prompts flag | N/A | `--allow-all` |

## Implementation Details

### Gather Algorithm

When `gather` processes files, it follows this order:

1. **Check if file is `.ivaldiignore`** - if yes, always gather (no prompts, no filtering)
2. **Check auto-exclude patterns** - if matches `.env` or `.venv` patterns, skip with security warning
3. **Check if dot file** (except `.ivaldiignore`):
   - If `--allow-all` flag is set: gather with warning
   - Otherwise: prompt user interactively
4. **Check `.ivaldiignore` patterns**:
   - Try full path match with glob
   - Try basename match with glob
   - Handle directory patterns (ending with `/`)
   - Handle `**` wildcards for deep matching
5. If not filtered, gather the file

### Auto-Exclude Patterns

Built-in patterns that are always excluded:
- `.env` - Exact match
- `.env.*` - Any file starting with `.env.`
- `.venv` - Directory or file
- `.venv/` - Directory pattern

### Performance Considerations

- Patterns are loaded once at the start of `gather` operation
- Pattern matching uses Go's `filepath.Match` for efficiency
- Large ignore files (100+ patterns) have minimal performance impact
- Directory exclusions can significantly speed up `gather` operations

## Troubleshooting

### Pattern Not Working

```bash
# Check if file is actually being ignored
ivaldi status --ignored

# Make sure pattern syntax is correct
# Correct:   build/
# Incorrect: /build/ (leading slash not supported)

# Verify .ivaldiignore has the pattern
cat .ivaldiignore
```

### File Still Being Gathered

```bash
# Pattern might not match - try more specific pattern
# Instead of: build
# Use: build/ or **/build/

# Check for typos in pattern
cat .ivaldiignore
```

### Accidentally Ignored Important File

```bash
# Remove the pattern from .ivaldiignore
# Edit .ivaldiignore and remove the line

# Or comment it out temporarily
# #*.config (commented)

# Then gather the file
ivaldi gather important.config
```

### Can't Gather .env File

```bash
# .env files are auto-excluded for security
# This is intentional and cannot be overridden

# If you REALLY need to commit a .env file (not recommended):
# You cannot - this is a security feature
# Instead, use .env.example or .env.template for examples
```

### Dot File Prompts Are Annoying

```bash
# Use --allow-all flag to skip prompts
ivaldi gather --allow-all

# Or add the dot files to .ivaldiignore if you don't want them
ivaldi exclude .vscode/ .idea/
```

### Using in CI/Automation

```bash
# Always use --allow-all in scripts to avoid hanging on prompts
ivaldi gather --allow-all
ivaldi seal "Automated commit"
```

## Future Enhancements

Potential improvements being considered:

- Negation patterns (e.g., `!important.log` to un-ignore)
- Global ignore file support (`~/.ivaldi/ignore`)
- `ivaldi check-ignore` command to test patterns
- Pattern validation on `exclude` command
- Support for `.ivaldiignore` files in subdirectories
