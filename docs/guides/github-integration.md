---
layout: default
title: GitHub Integration
---

# GitHub Integration

Master Ivaldi's seamless GitHub integration.

## Overview

Ivaldi provides first-class GitHub support:
- Clone repositories
- Push and pull changes
- Selective branch downloading
- Automatic Git compatibility
- Portal-based connection management

## Initial Setup

### GitHub Authentication

#### Option 1: Personal Access Token

1. Create token on GitHub (Settings → Developer settings → Personal access tokens)
2. Required scopes: `repo`, `workflow`
3. Set environment variable:

```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

Add to `.bashrc` or `.zshrc` for persistence:
```bash
echo 'export GITHUB_TOKEN="ghp_your_token"' >> ~/.bashrc
```

#### Option 2: GitHub CLI

```bash
gh auth login
```

This automatically configures authentication for Ivaldi.

### Verify Authentication

```bash
# Try listing a repo
ivaldi scout
# Should work without errors
```

## Connecting to Repository

### Add Portal

```bash
ivaldi portal add owner/repository-name
```

Example:
```bash
ivaldi portal add javanhut/IvaldiVCS
```

### List Portals

```bash
$ ivaldi portal list

Configured portals:
  javanhut/IvaldiVCS
  myusername/my-project
```

### Remove Portal

```bash
ivaldi portal remove owner/repository
```

## Cloning Repositories

### Basic Clone

```bash
ivaldi download owner/repository
```

Example:
```bash
ivaldi download javanhut/IvaldiVCS
cd IvaldiVCS
```

### Clone to Specific Directory

```bash
ivaldi download owner/repository my-directory
cd my-directory
```

### After Cloning

```bash
# Check status
ivaldi whereami

# See history
ivaldi log --limit 10

# Discover other branches
ivaldi scout
```

## Pushing Changes

### Upload Timeline

```bash
# Make changes
ivaldi gather .
ivaldi seal "Add feature"

# Push to GitHub
ivaldi upload
```

### What Happens

1. Ivaldi converts seals to Git commits
2. Creates/updates branch on GitHub
3. Preserves commit metadata
4. Compatible with Git clients

### First Push

```bash
# Create repository on GitHub first
# Then:
ivaldi forge
ivaldi gather .
ivaldi seal "Initial commit"
ivaldi portal add username/new-repo
ivaldi upload
```

## Fetching Changes

### Discover Remote Branches

```bash
$ ivaldi scout

Remote timelines available:
  main
  feature-authentication
  feature-payment
  develop
```

### Download Specific Branches

```bash
ivaldi harvest feature-authentication
```

### Download Multiple Branches

```bash
ivaldi harvest feature-auth feature-payment bugfix-security
```

### Update All Timelines

```bash
ivaldi harvest --update
```

## Selective Sync

Ivaldi's advantage: download only what you need.

### Traditional Git

```bash
git clone url
# Downloads ALL branches
# Large repos = long wait
# Lots of disk space used
```

### Ivaldi Way

```bash
ivaldi download owner/repo
# Downloads main branch only

ivaldi scout
# See what's available

ivaldi harvest feature-auth
# Download only this branch
# Fast and efficient!
```

### Real-World Example

```bash
# Large repo with 50 branches
$ ivaldi scout
Remote timelines available:
  main
  feature-1
  feature-2
  ...
  feature-50

# Only download what you need
$ ivaldi harvest feature-auth feature-payment
# Downloaded 2 branches, skipped 48
# Saved bandwidth and disk space!
```

## Complete GitHub Workflows

### Contributing to Open Source

```bash
# Fork on GitHub
# Then clone your fork
ivaldi download yourusername/project
cd project

# Add upstream
ivaldi portal add upstream/project

# Create feature branch
ivaldi timeline create feature-new-feature

# Make changes
ivaldi gather .
ivaldi seal "Add awesome feature"

# Push to your fork
ivaldi upload

# Create PR on GitHub web interface
```

### Team Development

```bash
# Clone team repo
ivaldi download team/project
cd project

# See what team is working on
ivaldi scout

# Download specific features
ivaldi harvest feature-alice feature-bob

# Create your feature
ivaldi timeline create feature-yourname
ivaldi gather .
ivaldi seal "Your feature"
ivaldi upload

# Team can now harvest your branch
```

### Hotfix Workflow

```bash
# Clone production repo
ivaldi download company/production
cd production

# Create hotfix
ivaldi timeline create hotfix-security
ivaldi gather .
ivaldi seal "Fix security vulnerability CVE-2025-1234"

# Push hotfix
ivaldi upload

# Deploy (GitHub Actions can trigger on branch)
```

### Release Management

```bash
# Prepare release
ivaldi timeline create release-v1.2.0

# Merge features
ivaldi fuse feature-a to release-v1.2.0
ivaldi fuse feature-b to release-v1.2.0

# Final adjustments
ivaldi seal "Update version to 1.2.0"
ivaldi seal "Update changelog"

# Push release branch
ivaldi upload

# Merge to main and tag (on GitHub)
ivaldi timeline switch main
ivaldi fuse release-v1.2.0 to main
ivaldi upload
```

## Git Interoperability

### Ivaldi and Git Side-by-Side

Ivaldi repositories are Git-compatible:

```bash
# Clone with Ivaldi
ivaldi download owner/repo
cd repo

# Use Git commands
git status
git log

# Use Ivaldi commands
ivaldi status
ivaldi log

# Both work!
```

### Migrating from Git

```bash
# Existing Git repo
cd my-git-project

# Initialize Ivaldi
ivaldi forge
# Automatically imports Git history!

# Continue with Ivaldi
ivaldi status
ivaldi gather .
ivaldi seal "First Ivaldi commit"
ivaldi portal add owner/repo
ivaldi upload

# Git history preserved
ivaldi log  # Shows all commits, including Git ones
```

### Pushing to Git Remote

```bash
# Ivaldi repo can push to Git remote
ivaldi upload
# Creates standard Git commits
# Other developers can use Git to pull
```

## Working with Pull Requests

### Creating Pull Request

```bash
# Create feature branch
ivaldi timeline create feature-new-api
ivaldi gather .
ivaldi seal "Add new API endpoints"
ivaldi upload

# Create PR on GitHub web interface
# Base: main
# Compare: feature-new-api
```

### Reviewing Pull Request

```bash
# Someone opened PR #42
# Branch: feature-authentication

# Download the branch
ivaldi harvest feature-authentication
ivaldi timeline switch feature-authentication

# Review
ivaldi log
ivaldi diff main

# Test locally
npm test

# Add review comments as commits
ivaldi seal "Review: Add error handling on line 42"
ivaldi upload

# Or approve and merge on GitHub
```

### Updating Pull Request

```bash
# PR feedback received
# Update your branch
ivaldi timeline switch feature-new-api

# Make changes
ivaldi gather .
ivaldi seal "Address review feedback"
ivaldi upload

# PR automatically updates!
```

## Advanced GitHub Integration

### Multiple Remotes

```bash
# Add multiple portals
ivaldi portal add upstream/original
ivaldi portal add myusername/fork

# Upload pushes to first portal
# Or specify which one
# (currently uses first portal)
```

### GitHub Actions Integration

Ivaldi works with GitHub Actions:

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [ main, feature-* ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      # Standard Git checkout works!

      - name: Run tests
        run: npm test
```

### Branch Protection

Protect important branches on GitHub:
- Settings → Branches → Branch protection rules
- Protect `main` branch
- Require reviews
- Require status checks

Ivaldi respects these rules.

## Best Practices

### Regular Syncing

```bash
# Morning routine
ivaldi scout
ivaldi harvest --update

# Before starting work
ivaldi harvest main --update
ivaldi fuse main to feature-branch
```

### Descriptive Branch Names

```bash
# GitHub shows branch names
# Make them descriptive
ivaldi timeline create feature-user-authentication
# Not: ivaldi timeline create feature1
```

### Commit Messages

```bash
# PR reviewers see commit messages
# Make them clear
ivaldi seal "Add OAuth2 authentication with Google provider"
# Not: ivaldi seal "stuff"
```

### Clean History

```bash
# Before uploading, ensure clean history
ivaldi log
# Reorder/squash commits if needed using travel
ivaldi upload
```

## Troubleshooting

### Authentication Failed

```
Error: GitHub authentication failed
```

Solutions:
```bash
# Refresh token
export GITHUB_TOKEN="new_token"

# Or re-authenticate with CLI
gh auth login
gh auth status
```

### Repository Not Found

```
Error: repository not found: owner/repo
```

Solutions:
- Check repository name spelling
- Verify you have access
- For private repos, ensure token has `repo` scope

### Upload Failed

```
Error: failed to upload
```

Solutions:
```bash
# Check portal configuration
ivaldi portal list

# Verify authentication
gh auth status

# Check network connection
ping github.com
```

### Conflicts on Upload

```
Error: remote has changes
```

Solutions:
```bash
# Fetch and merge remote changes
ivaldi harvest timeline-name --update
ivaldi fuse main to timeline-name
ivaldi upload
```

## Summary

GitHub integration essentials:
- **Portal**: Connection to GitHub repo
- **Download**: Clone repository
- **Upload**: Push changes
- **Scout**: Discover remote branches
- **Harvest**: Selectively fetch branches

Key commands:
- `ivaldi portal add owner/repo` - Connect to GitHub
- `ivaldi download owner/repo` - Clone repository
- `ivaldi scout` - See remote branches
- `ivaldi harvest branch` - Download branches
- `ivaldi upload` - Push changes

## Next Steps

- Learn [Team Collaboration](collaboration.md)
- Master [Timeline Branching](branching.md)
- Explore [Basic Workflow](basic-workflow.md)
