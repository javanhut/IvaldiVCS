---
layout: default
title: Basic Workflow
---

# Basic Workflow

Learn the day-to-day workflow for using Ivaldi effectively.

## Daily Development Cycle

### 1. Check Status

Start your day by checking where you are:

```bash
ivaldi whereami
# or shorter:
ivaldi wai
```

See what files have changed:

```bash
ivaldi status
```

### 2. Make Changes

Work on your project normally. Edit files, add features, fix bugs.

### 3. Review Changes

Before committing, review what changed:

```bash
# See which files changed
ivaldi status

# See specific changes
ivaldi diff

# See staged changes
ivaldi diff --staged
```

### 4. Stage Files

Add files to the next commit:

```bash
# Stage specific files
ivaldi gather src/main.go README.md

# Stage directory
ivaldi gather src/

# Stage everything
ivaldi gather .
```

### 5. Create Seal (Commit)

Create a commit with your changes:

```bash
ivaldi seal "Add user authentication feature"
```

Output:
```
Created seal: swift-eagle-flies-high-447abe9b (447abe9b)
```

### 6. Push to GitHub

Upload your changes:

```bash
ivaldi upload
```

## Complete Example

Here's a complete workflow from start to finish:

```bash
# Morning: Check where you are
$ ivaldi wai
Timeline: feature-auth
Last Seal: calm-river-flows-deep (2 hours ago)
Workspace: Clean

# Make changes
$ echo "new feature" >> src/auth.go
$ vim src/login.go

# Check what changed
$ ivaldi status
Timeline: feature-auth
Unstaged changes:
  modified: src/auth.go
  modified: src/login.go

# Review specific changes
$ ivaldi diff

# Stage changes
$ ivaldi gather src/

# Verify staging
$ ivaldi status
Timeline: feature-auth
Staged changes:
  modified: src/auth.go
  modified: src/login.go

# Create seal
$ ivaldi seal "Implement OAuth2 login flow"
Created seal: swift-eagle-flies-high-447abe9b (447abe9b)

# Push to GitHub
$ ivaldi upload
Uploading to javanhut/my-project...
Success!
```

## Working on Features

### Start a Feature

```bash
# Create feature timeline
ivaldi timeline create feature-payment

# Verify switch
ivaldi wai
```

### Work on Feature

```bash
# Make changes
vim src/payment.go

# Stage and seal frequently
ivaldi gather src/payment.go
ivaldi seal "Add payment validation"

# Continue working
vim src/checkout.go
ivaldi gather src/checkout.go
ivaldi seal "Add checkout flow"
```

### Complete Feature

```bash
# Switch to main
ivaldi timeline switch main

# Merge feature
ivaldi fuse feature-payment to main

# Push to GitHub
ivaldi upload

# Clean up (optional)
ivaldi timeline remove feature-payment
```

## Quick Fixes

### Hotfix Workflow

```bash
# On main, need quick fix
ivaldi wai  # Confirm on main

# Make fix
vim src/security.go

# Quick commit
ivaldi gather .
ivaldi seal "Fix security vulnerability CVE-2025-1234"

# Push immediately
ivaldi upload
```

### WIP Commits

Save work in progress:

```bash
# End of day, feature not complete
ivaldi gather .
ivaldi seal "WIP: User authentication - need to add tests"

# Next morning, continue where you left off
ivaldi wai
```

## Switching Contexts

Ivaldi's auto-shelving makes context switching easy:

```bash
# Working on feature-auth
$ ivaldi wai
Timeline: feature-auth
Workspace: 3 files modified

# Urgent: need to fix bug in main
$ ivaldi timeline switch main
# Your feature-auth changes are auto-shelved

# Fix bug
$ vim src/bug.go
$ ivaldi gather .
$ ivaldi seal "Fix critical bug"
$ ivaldi upload

# Return to feature
$ ivaldi timeline switch feature-auth
# Your changes are automatically restored!

$ ivaldi status
Unstaged changes:
  modified: src/auth.go
  modified: src/login.go
  modified: src/oauth.go
# All your work is back!
```

## Collaboration

### Reviewing Teammate's Work

```bash
# See what's available
ivaldi scout

# Download their branch
ivaldi harvest feature-payments

# Switch to it
ivaldi timeline switch feature-payments

# Review
ivaldi log
ivaldi diff

# Switch back
ivaldi timeline switch main
```

### Contributing to Feature

```bash
# Get teammate's branch
ivaldi harvest feature-ui

# Switch to it
ivaldi timeline switch feature-ui

# Make improvements
vim src/components/button.js
ivaldi gather .
ivaldi seal "Improve button accessibility"

# Push changes
ivaldi upload
```

## Best Practices

### Commit Often

Small, focused commits are better:

```bash
# Good: Small focused commits
ivaldi seal "Add login endpoint"
ivaldi seal "Add logout endpoint"
ivaldi seal "Add session validation"

# Bad: One huge commit
ivaldi seal "Add authentication"  # (100 files changed)
```

### Write Clear Messages

```bash
# Good: Descriptive
ivaldi seal "Fix null pointer exception in user login handler"

# Bad: Vague
ivaldi seal "fix"
```

### Review Before Committing

```bash
# Always check before sealing
ivaldi status
ivaldi diff
ivaldi gather .
ivaldi seal "Your message"
```

### Keep Main Stable

```bash
# Use feature timelines for development
ivaldi timeline create feature-x
# Work on feature-x
# Test thoroughly
ivaldi timeline switch main
ivaldi fuse feature-x to main
```

### Sync Regularly

```bash
# Daily sync
ivaldi scout
ivaldi harvest --update
ivaldi timeline switch main
```

## Common Patterns

### Feature Branch Pattern

```bash
1. ivaldi timeline create feature-name
2. Make changes
3. ivaldi gather . && ivaldi seal "Description"
4. Repeat step 2-3
5. ivaldi timeline switch main
6. ivaldi fuse feature-name to main
7. ivaldi upload
```

### Hotfix Pattern

```bash
1. ivaldi timeline switch main
2. Fix issue
3. ivaldi gather . && ivaldi seal "Fix: description"
4. ivaldi upload
```

### Experimentation Pattern

```bash
1. ivaldi travel
2. Select stable commit, diverge
3. Try experimental approach
4. If successful: merge
5. If not: abandon timeline
```

## Troubleshooting Common Issues

### Forgot to Create Feature Timeline

```bash
# Already made changes on main
ivaldi status

# Create timeline from current state
ivaldi timeline create feature-x
# Changes move with you!
```

### Committed to Wrong Timeline

```bash
# On main, should have been on feature
ivaldi timeline create feature-x
ivaldi wai
# Last commit is now on feature-x!
```

### Need to Undo Last Commit

```bash
# Use time travel
ivaldi travel
# Select commit before mistake
# Choose: Overwrite (or Diverge to be safe)
```

## Summary

Basic daily workflow:
1. `ivaldi wai` - Check where you are
2. Make changes
3. `ivaldi status` - See what changed
4. `ivaldi gather .` - Stage changes
5. `ivaldi seal "message"` - Commit
6. `ivaldi upload` - Push to GitHub

That's it! This covers 90% of daily Ivaldi usage.

## Next Steps

- Learn about [Timeline Branching](branching.md)
- Explore [Team Collaboration](collaboration.md)
- Master [GitHub Integration](github-integration.md)
