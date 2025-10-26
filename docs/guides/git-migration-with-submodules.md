---
layout: default
title: Migrating Git Repositories with Submodules
---

# Migrating Git Repositories with Submodules

This guide explains how Ivaldi automatically handles Git submodules during migration.

## Automatic Detection and Conversion

Ivaldi **automatically detects and converts** Git submodules when you:
- Clone a repository with `ivaldi download`
- Initialize Ivaldi in an existing Git repo with `ivaldi forge`

**No manual intervention required!**

## What Happens During Migration

### 1. Detection

Ivaldi checks for `.gitmodules` file:

```bash
$ ivaldi forge

Ivaldi repository initialized
Detecting existing Git repository...

📦 Detected Git submodules...
```

### 2. Parsing

Reads `.gitmodules` configuration:

```ini
[submodule "external-lib"]
    path = libs/external-lib
    url = https://github.com/owner/external-lib
    branch = main
```

### 3. Cloning

Clones missing submodules automatically:

```bash
  Submodule 'external-lib' at libs/external-lib
    Cloning from https://github.com/owner/external-lib...
    ✓ Cloned (commit: abc123)
```

### 4. Conversion

Converts each submodule to Ivaldi format:

```bash
    Converting to Ivaldi format...
    ✓ Converted 456 objects
```

### 5. Configuration

Creates `.ivaldimodules` with BLAKE3 hashes:

```ini
[submodule "external-lib"]
    path = libs/external-lib
    url = https://github.com/owner/external-lib
    timeline = main
    commit = 1a2b3c4d5e6f7890...  # BLAKE3 hash
    git-commit = abc123def456...  # Git SHA-1 (preserved)
```

### 6. Dual-Hash Mapping

Stores bidirectional BLAKE3 ↔ Git SHA-1 mapping in BoltDB for GitHub compatibility.

## Migration Scenarios

### Scenario 1: Clone from GitHub with Submodules

```bash
$ ivaldi download https://github.com/tensorflow/tensorflow

Cloning repository from GitHub...
✓ Cloned main repository

Converting Git repository to Ivaldi format...
✓ Converted 45,678 Git objects

📦 Detected Git submodules...
  Submodule 'third_party/eigen' at third_party/eigen
    Cloning from https://github.com/eigenteam/eigen-git-mirror...
    ✓ Cloned (commit: 12a3456)
    ✓ Converted 234 objects
  
  Submodule 'third_party/protobuf' at third_party/protobuf
    Already cloned
    ✓ Converted 567 objects

✓ Initialized 2 submodules
✓ Created .ivaldimodules

Repository ready! Current timeline: main
```

### Scenario 2: Existing Git Repo with Uninitialized Submodules

```bash
$ git clone --recurse-submodules=false https://github.com/owner/repo
$ cd repo
$ ivaldi forge

Ivaldi repository initialized
Detecting existing Git repository...

📦 Detected Git submodules...
  Found 3 submodules in .gitmodules
  
  Submodule 'lib1' at libs/lib1
    Cloning from https://github.com/owner/lib1...
    ✓ Cloned and converted
  
  Submodule 'lib2' at libs/lib2
    Cloning from https://github.com/owner/lib2...
    ✓ Cloned and converted

✓ Converted 3 Git submodules
```

### Scenario 3: Nested Submodules

```bash
$ ivaldi download https://github.com/owner/parent-repo

📦 Detected Git submodules...
  Submodule 'lib1' at libs/lib1
    ✓ Converted
    📦 Detecting nested submodules in libs/lib1
      Submodule 'lib2' at nested/lib2
        ✓ Converted

✓ Initialized 2 submodules (1 nested)
```

## Mapping Git Concepts to Ivaldi

### Branch → Timeline

Git submodule branches become Ivaldi timelines:

**Git**:
```ini
[submodule "lib"]
    branch = develop
```

**Ivaldi**:
```ini
[submodule "lib"]
    timeline = develop
```

### Commit Hash → BLAKE3 + Git SHA-1

Ivaldi maintains both hashes:

- **BLAKE3**: Internal Ivaldi operations (primary)
- **Git SHA-1**: GitHub sync only (stored for compatibility)

### Detached HEAD State

If Git submodule is in detached HEAD:

```bash
Warning: Submodule 'lib' in detached HEAD state
Using current commit: abc123
Timeline set to: main (default)
```

## Preserving Git History

Ivaldi preserves:

- ✅ Exact commit references (via dual-hash mapping)
- ✅ Submodule URLs
- ✅ Branch/timeline associations
- ✅ Nested submodule structure
- ✅ Shallow clone flags

## Handling Edge Cases

### Missing Submodule Repositories

If submodule URL is inaccessible:

```bash
Warning: Submodule 'deleted-lib' not accessible (404)
Options:
  1. Skip this submodule
  2. Provide alternate URL
  3. Abort conversion

Choose [1-3]:
```

### Changed Submodule URLs

If `.gitmodules` URL differs from cloned URL:

```bash
Warning: Submodule 'lib' URL changed
  Old: https://github.com/old-owner/lib
  New: https://github.com/new-owner/lib
Using new URL from .gitmodules
```

### Orphaned Submodule Commits

If referenced commit doesn't exist in remote:

```bash
Warning: Commit abc123 not found in submodule 'lib'
Using current submodule state (def456)
```

## Post-Migration Workflow

After migration, Ivaldi submodules work seamlessly:

### Push to GitHub

```bash
$ ivaldi upload

# Ivaldi automatically:
# 1. Converts .ivaldimodules → .gitmodules
# 2. Maps BLAKE3 → Git SHA-1
# 3. Creates Git gitlink entries
# 4. Pushes to GitHub
```

### Pull from GitHub

```bash
$ ivaldi sync

# Ivaldi automatically:
# 1. Reads .gitmodules from GitHub
# 2. Converts Git SHA-1 → BLAKE3
# 3. Updates .ivaldimodules
```

### Timeline Switches

```bash
$ ivaldi timeline switch feature

# Submodules automatically:
# 1. Shelve uncommitted changes
# 2. Switch to timeline-specific commits
# 3. Restore on return to original timeline
```

## Disabling Automatic Conversion

To skip submodule conversion during migration:

```bash
$ ivaldi forge --recurse-submodules=false

# Or for download:
$ ivaldi download <url> --recurse-submodules=false
```

You can manually initialize later if needed.

## Troubleshooting

### Conversion Failed for Some Submodules

Check the error log:

```bash
⚠ Skipped 2 submodules due to errors
  - libs/fail1: network timeout
  - libs/fail2: invalid URL

# Manually retry:
$ ivaldi submodule init libs/fail1
```

### Submodule Directories Don't Exist

After partial migration:

```bash
$ ivaldi forge

# Will detect and clone missing submodules
```

### Incorrect Commit References

If submodule points to wrong commit:

```bash
# Check .ivaldimodules
$ cat .ivaldimodules

# Manually update (future command):
$ ivaldi submodule update --remote libs/lib
```

## Comparison: Git vs Ivaldi

| Operation | Git | Ivaldi |
|-----------|-----|--------|
| **Initialize submodules** | `git submodule update --init` | Automatic during `forge` |
| **Clone with submodules** | `git clone --recurse-submodules` | `ivaldi download` (automatic) |
| **Update submodules** | `git submodule update --remote` | Future: `ivaldi submodule update` |
| **Switch branch** | Manual stash/restore | Auto-shelving |
| **Check status** | `git submodule status` | Future: `ivaldi submodule status` |

## Best Practices

1. **Let Ivaldi handle conversion automatically**
   - Don't manually edit `.ivaldimodules`
   - Trust the dual-hash mapping

2. **Verify after migration**
   ```bash
   $ cat .ivaldimodules  # Check configuration
   $ ls -la libs/        # Verify directories exist
   ```

3. **Test GitHub sync**
   ```bash
   $ ivaldi upload       # Verify push works
   $ ivaldi sync         # Verify pull works
   ```

4. **Keep both .gitmodules and .ivaldimodules**
   - `.gitmodules` for Git compatibility
   - `.ivaldimodules` for Ivaldi operations

## See Also

- [Submodule Commands](../commands/submodule.md)
- [GitHub Integration](github-integration.md)
- [Timeline Management](branching.md)
