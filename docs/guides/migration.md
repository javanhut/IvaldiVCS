---
layout: default
title: Git to Ivaldi Migration
---

# Git to Ivaldi Migration Guide

Transition from Git to Ivaldi with confidence.

## Quick Translation

### Common Commands

| Git | Ivaldi |
|-----|--------|
| `git init` | `ivaldi forge` |
| `git add file.txt` | `ivaldi gather file.txt` |
| `git add .` | `ivaldi gather .` |
| `git commit -m "msg"` | `ivaldi seal "msg"` |
| `git status` | `ivaldi status` |
| `git log` | `ivaldi log` |
| `git log --oneline` | `ivaldi log --oneline` |
| `git diff` | `ivaldi diff` |
| `git diff --staged` | `ivaldi diff --staged` |
| `git reset file` | `ivaldi reset file` |
| `git reset --hard` | `ivaldi reset --hard` |
| `git branch new` | `ivaldi timeline create new` |
| `git checkout branch` | `ivaldi timeline switch branch` |
| `git branch` | `ivaldi timeline list` |
| `git branch -d branch` | `ivaldi timeline remove branch` |
| `git merge branch` | `ivaldi fuse branch to main` |
| `git clone url` | `ivaldi download owner/repo` |
| `git push` | `ivaldi upload` |
| `git fetch` | `ivaldi scout` + `ivaldi harvest` |
| `git pull` | `ivaldi harvest --update` |
| `git remote add` | `ivaldi portal add owner/repo` |
| `git config` | `ivaldi config` |

## Workflow Translation

### Git Workflow

```bash
# Git workflow
git clone https://github.com/user/repo.git
cd repo
git checkout -b feature
vim file.txt
git add file.txt
git commit -m "Add feature"
git push -u origin feature
git checkout main
git merge feature
git push
```

### Ivaldi Equivalent

```bash
# Ivaldi workflow
ivaldi download user/repo
cd repo
ivaldi timeline create feature
vim file.txt
ivaldi gather file.txt
ivaldi seal "Add feature"
ivaldi upload
ivaldi timeline switch main
ivaldi fuse feature to main
ivaldi upload
```

## Key Differences

### 1. Branches vs Timelines

**Git:**
- Branches share working directory
- Must manually stash changes when switching

**Ivaldi:**
- Timelines have isolated workspaces
- Auto-shelving preserves changes automatically

Example:
```bash
# Git
git checkout feature
# Work...
git stash  # Manual stash required
git checkout main

# Ivaldi
ivaldi timeline switch feature
# Work...
ivaldi timeline switch main  # Auto-shelved!
```

### 2. Commits vs Seals

**Git:**
- Commits identified by SHA-1 hash only
- `a1b2c3d4567890...`

**Ivaldi:**
- Seals get memorable names + BLAKE3 hash
- `swift-eagle-flies-high-447abe9b`

Example:
```bash
# Git
git log --oneline
a1b2c3d Add feature

# Ivaldi
ivaldi log --oneline
447abe9b swift-eagle-flies-high-447abe9b Add feature
```

### 3. Remote Operations

**Git:**
- Downloads all branches
- `git fetch` gets everything

**Ivaldi:**
- Selective downloading
- Choose specific branches

Example:
```bash
# Git
git fetch origin  # Downloads all branches

# Ivaldi
ivaldi scout  # See what's available
ivaldi harvest feature-auth  # Download only this
```

### 4. Merging

**Git:**
- Conflict markers written to files
- Manual editing required

**Ivaldi:**
- Multiple merge strategies
- Workspace stays clean
- No conflict markers in files

Example:
```bash
# Git
git merge feature
# Conflict! Now file.txt has <<<<<<< markers

# Ivaldi
ivaldi fuse feature to main
# Conflict! Workspace stays clean
ivaldi fuse --strategy=theirs feature to main
# Or manually resolve without markers
```

## Migrating Existing Repository

### Option 1: Automatic Import

```bash
cd existing-git-repo
ivaldi forge
# Automatically imports Git history!

ivaldi log  # See all commits
ivaldi timeline list  # See all branches as timelines
```

### Option 2: Fresh Start

```bash
# Create new Ivaldi repo
mkdir new-project
cd new-project
ivaldi forge

# Copy files from Git repo
cp -r ../old-git-repo/* .

# Start fresh
ivaldi gather .
ivaldi seal "Import from Git"
```

## Side-by-Side Usage

You can use Git and Ivaldi together:

```bash
cd my-repo

# Git commands work
git status
git log

# Ivaldi commands work
ivaldi status
ivaldi log

# Both manage the same repository!
```

## Concept Mapping

### Git Concepts → Ivaldi Concepts

| Git Concept | Ivaldi Concept |
|-------------|----------------|
| Repository | Repository |
| Branch | Timeline |
| Commit | Seal |
| SHA-1 hash | BLAKE3 hash + memorable name |
| Staging area | Staging area (same) |
| Working directory | Workspace |
| Remote | Portal |
| Clone | Download |
| Push | Upload |
| Fetch | Scout + Harvest |
| Merge | Fuse |
| Stash | Auto-shelving |
| HEAD | Current timeline pointer |
| Index | Workspace index |

### Git Workflows → Ivaldi Workflows

**Feature Branch (Git):**
```bash
git checkout -b feature
# work
git add .
git commit -m "msg"
git push
git checkout main
git merge feature
```

**Feature Timeline (Ivaldi):**
```bash
ivaldi timeline create feature
# work
ivaldi gather .
ivaldi seal "msg"
ivaldi upload
ivaldi timeline switch main
ivaldi fuse feature to main
```

**Hotfix (Git):**
```bash
git checkout main
git checkout -b hotfix
# fix
git add .
git commit -m "fix"
git checkout main
git merge hotfix
git push
```

**Hotfix (Ivaldi):**
```bash
ivaldi timeline switch main
ivaldi timeline create hotfix
# fix
ivaldi gather .
ivaldi seal "fix"
ivaldi timeline switch main
ivaldi fuse hotfix to main
ivaldi upload
```

## Advanced Features

### Git Rebase → Ivaldi Time Travel

**Git:**
```bash
git rebase -i HEAD~3
# Interactive rebase
```

**Ivaldi:**
```bash
ivaldi travel
# Navigate to desired commit
# Select "Diverge" to create new timeline
```

### Git Cherry-Pick → Ivaldi Diverge

**Git:**
```bash
git cherry-pick abc123
```

**Ivaldi:**
```bash
ivaldi travel
# Select commit to cherry-pick
# Diverge to new timeline
# Fuse into target
```

### Git Stash → Auto-Shelving

**Git:**
```bash
git stash
git checkout other-branch
git checkout original-branch
git stash pop
```

**Ivaldi:**
```bash
ivaldi timeline switch other-branch
ivaldi timeline switch original-branch
# Changes automatically restored!
```

## Common Gotchas

### 1. No `git pull`

Git:
```bash
git pull origin main
```

Ivaldi:
```bash
ivaldi harvest main --update
ivaldi fuse main to current-timeline
```

### 2. Different Merge Syntax

Git:
```bash
git merge feature-branch
```

Ivaldi:
```bash
ivaldi fuse feature-branch to main
# Note: Explicit target
```

### 3. No `git add -A`

Git:
```bash
git add -A
```

Ivaldi:
```bash
ivaldi gather .
# or
ivaldi gather
```

### 4. Branch vs Timeline

Git: "branch"
Ivaldi: "timeline"

Remember to use "timeline" in commands.

## Cheat Sheet

### Daily Development

| Task | Git | Ivaldi |
|------|-----|--------|
| Check status | `git status` | `ivaldi status` |
| Stage all | `git add .` | `ivaldi gather .` |
| Commit | `git commit -m "msg"` | `ivaldi seal "msg"` |
| Push | `git push` | `ivaldi upload` |
| View history | `git log` | `ivaldi log` |

### Branching

| Task | Git | Ivaldi |
|------|-----|--------|
| Create branch | `git checkout -b feature` | `ivaldi timeline create feature` |
| Switch branch | `git checkout main` | `ivaldi timeline switch main` |
| List branches | `git branch` | `ivaldi timeline list` |
| Delete branch | `git branch -d feature` | `ivaldi timeline remove feature` |
| Merge | `git merge feature` | `ivaldi fuse feature to main` |

### Remote

| Task | Git | Ivaldi |
|------|-----|--------|
| Clone | `git clone url` | `ivaldi download owner/repo` |
| Add remote | `git remote add origin url` | `ivaldi portal add owner/repo` |
| Push | `git push` | `ivaldi upload` |
| Fetch | `git fetch` | `ivaldi scout` then `ivaldi harvest` |
| Pull | `git pull` | `ivaldi harvest --update` + `ivaldi fuse` |

## Migration Checklist

- [ ] Install Ivaldi
- [ ] Configure user: `ivaldi config`
- [ ] Try on test repo first
- [ ] Import existing repo: `ivaldi forge` in Git repo
- [ ] Verify history: `ivaldi log`
- [ ] Practice basic workflow
- [ ] Learn timeline management
- [ ] Set up GitHub portal
- [ ] Test push/pull
- [ ] Migrate team if applicable

## Learning Path

1. **Start Small**: Use Ivaldi on a personal project
2. **Learn Basics**: Master `gather`, `seal`, `status`, `log`
3. **Explore Timelines**: Practice creating and switching
4. **Try Merging**: Learn `fuse` command
5. **GitHub Integration**: Set up portals, upload/download
6. **Advanced Features**: Time travel, auto-shelving
7. **Team Usage**: Collaborate with others

## Getting Help

### Command Help

```bash
ivaldi <command> --help
```

### Documentation

- [Getting Started](../getting-started.md)
- [Core Concepts](../core-concepts.md)
- [Command Reference](../commands/index.md)

### Community

- Report issues on [GitHub](https://github.com/javanhut/IvaldiVCS/issues)
- Check [README](https://github.com/javanhut/IvaldiVCS)

## Summary

**Key Differences:**
1. Timelines (not branches) with auto-shelving
2. Seals (not commits) with memorable names
3. Selective sync (not download everything)
4. Clean merge resolution (no conflict markers in files)

**Same Concepts:**
- Staging area
- Commit history
- Merging
- Remote repositories

**You'll Love:**
- Auto-shelving when switching
- Memorable seal names
- Selective branch downloading
- Interactive time travel
- Clean conflict resolution

Welcome to Ivaldi!
