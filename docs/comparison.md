---
layout: default
title: Comparison with Git
---

# Comparison with Git

Understanding how Ivaldi differs from and improves upon Git.

## Command Comparison

| Feature | Git | Ivaldi |
|---------|-----|--------|
| Initialize | `git init` | `ivaldi forge` |
| Configure | `git config` | `ivaldi config` |
| Stage files | `git add` | `ivaldi gather` |
| Unstage | `git reset` | `ivaldi reset` |
| Commit | `git commit` | `ivaldi seal` |
| Log | `git log` | `ivaldi log` |
| Diff | `git diff` | `ivaldi diff` |
| Status | `git status` | `ivaldi status` |
| Branch | `git branch` | `ivaldi timeline create` |
| Switch branch | `git checkout` / `git switch` | `ivaldi timeline switch` |
| Merge | `git merge` | `ivaldi fuse` |
| Clone | `git clone` | `ivaldi download` |
| Push | `git push` | `ivaldi upload` |
| Fetch | `git fetch` | `ivaldi scout` + `ivaldi harvest` |
| Remote | `git remote add` | `ivaldi portal add` |
| Stash | `git stash` | (automatic shelving) |
| Rebase | `git rebase -i` | `ivaldi travel` (diverge) |
| Ignore files | Edit `.gitignore` | `ivaldi exclude` |

## Conceptual Differences

### 1. Branches vs Timelines

| Aspect | Git Branches | Ivaldi Timelines |
|--------|--------------|------------------|
| Working directory | Shared | Isolated |
| Switching | Manual stash | Auto-shelving |
| State preservation | Manual | Automatic |
| Name | Branch | Timeline |

**Git:**
```bash
# Working on feature
git checkout feature
# Edit files...

# Need to switch
git stash  # Manual stash required
git checkout main
# Do work on main
git checkout feature
git stash pop  # Manual restore
```

**Ivaldi:**
```bash
# Working on feature
ivaldi timeline switch feature
# Edit files...

# Need to switch
ivaldi timeline switch main  # Auto-shelved!
# Do work on main
ivaldi timeline switch feature  # Auto-restored!
```

### 2. Commits vs Seals

| Aspect | Git Commits | Ivaldi Seals |
|--------|-------------|--------------|
| Identifier | SHA-1 hash | BLAKE3 hash + memorable name |
| Example | `a1b2c3d456789...` | `swift-eagle-flies-high-447abe9b` |
| Readability | Low | High |
| Uniqueness | Guaranteed | Guaranteed |
| Referencing | Hash only | Name, partial name, or hash |

**Git:**
```bash
git log --oneline
a1b2c3d Add authentication
9f8e7d6 Fix bug

git show a1b2c3d  # Must remember hash
```

**Ivaldi:**
```bash
ivaldi log --oneline
447abe9b swift-eagle-flies-high-447abe9b Add authentication
7bb0588 empty-phoenix-attacks-fresh-7bb05886 Fix bug

ivaldi seals show swift-eagle  # Use memorable name!
```

### 3. Remote Operations

| Aspect | Git | Ivaldi |
|--------|-----|--------|
| Download | All branches | Selective |
| Syntax | Full URL | `owner/repo` |
| Efficiency | Downloads everything | Downloads what you need |

**Git:**
```bash
git clone https://github.com/owner/repo.git
# Downloads ALL branches
# Large repos = long wait
```

**Ivaldi:**
```bash
ivaldi download owner/repo
# Downloads main only

ivaldi scout  # See what's available
ivaldi harvest feature-x  # Get only what you need
# Fast and efficient!
```

### 4. Merging

| Aspect | Git Merge | Ivaldi Fuse |
|--------|-----------|-------------|
| Conflict markers | Written to files | Never written to files |
| Resolution | Edit marked files | Strategies or interactive |
| Granularity | Line-based | Chunk-based (64KB) |
| Workspace | Polluted during conflict | Always clean |
| False conflicts | Common (whitespace) | Rare (hash-based) |

**Git:**
```bash
git merge feature

# Conflict!
<<<<<<< HEAD
our code
=======
their code
>>>>>>> feature

# File now has markers, workspace polluted
```

**Ivaldi:**
```bash
ivaldi fuse feature to main

# Conflict!
# Workspace stays clean - no markers!

# Choose strategy
ivaldi fuse --strategy=theirs feature to main
# Or resolve interactively
```

### 5. Hashing Algorithm

| Aspect | Git | Ivaldi |
|--------|-----|--------|
| Algorithm | SHA-1 | BLAKE3 |
| Speed | Slower | Faster (10x) |
| Security | Deprecated | State-of-the-art |
| Parallelization | No | Yes |
| Collision resistance | Weaker | Stronger |

## Feature Comparison

### Git Features → Ivaldi Equivalents

| Git Feature | Ivaldi Equivalent | Notes |
|-------------|------------------|-------|
| Branches | Timelines | Enhanced with auto-shelving |
| Commits | Seals | Added memorable names |
| Tags | Not yet implemented | Coming soon |
| Submodules | Not yet implemented | Coming soon |
| Hooks | Not yet implemented | Coming soon |
| LFS | Built-in chunking | No separate extension needed |
| Reflog | MMR history | Append-only, tamper-proof |
| Cherry-pick | Time travel diverge | Interactive |
| Rebase | Time travel | Non-destructive option |
| Bisect | Not yet implemented | Coming soon |

### Ivaldi-Only Features

Features Ivaldi has that Git doesn't:

1. **Auto-Shelving**: Automatic preservation of changes when switching
2. **Memorable Seal Names**: Human-friendly commit identifiers
3. **Interactive Time Travel**: Arrow-key navigation through history
4. **Selective Sync**: Download only specific branches
5. **Chunk-Level Merging**: 64KB chunks with BLAKE3 hashing
6. **Clean Conflict Resolution**: No markers in workspace files
7. **Content-Addressable Storage**: Automatic deduplication
8. **Merkle Mountain Range**: Cryptographic commit proofs

## Workflow Comparison

### Daily Development

**Git:**
```bash
git status
git add .
git commit -m "message"
git push
```

**Ivaldi:**
```bash
ivaldi status
ivaldi gather .
ivaldi seal "message"
ivaldi upload
```

### Feature Development

**Git:**
```bash
git checkout -b feature
# work
git add .
git commit -m "msg"
git push -u origin feature
git checkout main
git merge feature
git push
git branch -d feature
```

**Ivaldi:**
```bash
ivaldi timeline create feature
# work
ivaldi gather .
ivaldi seal "msg"
ivaldi upload
ivaldi timeline switch main
ivaldi fuse feature to main
ivaldi upload
ivaldi timeline remove feature
```

### Hotfix

**Git:**
```bash
git checkout main
git pull
git checkout -b hotfix
# fix
git add .
git commit -m "fix"
git checkout main
git merge hotfix
git push
```

**Ivaldi:**
```bash
ivaldi timeline switch main
ivaldi harvest main --update
ivaldi timeline create hotfix
# fix
ivaldi gather .
ivaldi seal "fix"
ivaldi timeline switch main
ivaldi fuse hotfix to main
ivaldi upload
```

## Performance Comparison

### Large Files

| Operation | Git | Ivaldi |
|-----------|-----|--------|
| Hashing | Slower (SHA-1) | Faster (BLAKE3) |
| Chunking | Git LFS extension | Built-in |
| Deduplication | Limited | Automatic |
| Storage | Delta compression | Content-addressable |

### Repository Size

| Aspect | Git | Ivaldi |
|--------|-----|--------|
| Deduplication | Delta-based | Hash-based |
| Shared content | Limited | Aggressive |
| Large repos | Slower over time | Consistently fast |

### Network

| Operation | Git | Ivaldi |
|-----------|-----|--------|
| Clone | All branches | Main only |
| Fetch | All or nothing | Selective |
| Bandwidth | Higher | Lower |

## Security Comparison

| Aspect | Git | Ivaldi |
|--------|-----|--------|
| Hash algorithm | SHA-1 (deprecated) | BLAKE3 (modern) |
| Collision resistance | Weak | Strong |
| Tampering detection | Basic | Advanced |
| Cryptographic proofs | Limited | MMR-based |

## Migration Ease

### From Git to Ivaldi

```bash
cd git-repo
ivaldi forge
# Automatic import!
```

- Preserves all commits
- Converts branches to timelines
- Maintains history
- Git repo still usable

### From Ivaldi to Git

Ivaldi repositories are Git-compatible:
- Can use `git` commands
- Can push to Git remotes
- Other developers can use Git

## Advantages of Ivaldi

### 1. User Experience

- **Intuitive commands**: `forge`, `gather`, `seal` vs `init`, `add`, `commit`
- **Memorable names**: `swift-eagle` vs `a1b2c3d`
- **Auto-shelving**: No manual stashing
- **Clean merges**: No conflict markers in files

### 2. Performance

- **BLAKE3**: Faster hashing
- **Built-in chunking**: No LFS needed
- **Selective sync**: Download only what you need
- **Content-addressable**: Better deduplication

### 3. Safety

- **Modern cryptography**: BLAKE3 vs SHA-1
- **MMR**: Tamper-proof history
- **Auto-shelving**: Never lose work
- **Non-destructive time travel**: Safe experimentation

### 4. Collaboration

- **Selective sync**: Team members download only relevant branches
- **Clean conflict resolution**: Easier collaboration
- **Clear seal names**: Better communication

## When to Use Each

### Use Git When

- Team requires Git (company policy)
- Existing large Git infrastructure
- Need specific Git tools/integrations
- Git expertise in team

### Use Ivaldi When

- Starting new project
- Want better UX
- Need selective sync
- Value auto-shelving
- Want modern cryptography
- Prefer human-readable identifiers

### Use Both When

- Transitioning from Git
- Team has mixed preferences
- Need compatibility

Ivaldi works alongside Git!

## Learning Curve

### Git Knowledge → Ivaldi

If you know Git:
- 90% of concepts transfer directly
- Main differences: timelines, seals, auto-shelving
- Can be productive in 30 minutes

### New to VCS

Ivaldi may be easier to learn:
- More intuitive command names
- Auto-shelving prevents common mistakes
- Memorable seal names easier than hashes

## Compatibility

### Ivaldi with Git Tools

| Tool | Compatible? |
|------|-------------|
| GitHub | Yes - full integration |
| GitLab | Via Git compatibility |
| Bitbucket | Via Git compatibility |
| GitHub Actions | Yes |
| Git GUIs | Via Git compatibility |

### File Formats

- `.ivaldi` and `.git` can coexist
- Ivaldi repos work with Git commands
- Can push to Git remotes

## Summary

**Ivaldi improves on Git with:**
- Auto-shelving timelines
- Memorable seal names
- Selective sync
- Clean conflict resolution
- Modern cryptography (BLAKE3)
- Interactive time travel

**Git advantages:**
- Ubiquity
- Tool ecosystem
- Community size
- Longer history

**Bottom line:**
Ivaldi offers a better user experience with modern features while maintaining Git compatibility for interoperability.

## Next Steps

- Try Ivaldi: [Getting Started](getting-started.md)
- Learn concepts: [Core Concepts](core-concepts.md)
- Migrate project: [Migration Guide](guides/migration.md)
