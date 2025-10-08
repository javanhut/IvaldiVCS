---
layout: default
title: Timeline Branching
---

# Timeline Branching

Master Ivaldi's enhanced branching system with timelines and auto-shelving.

## Understanding Timelines

Timelines are Ivaldi's version of branches with powerful enhancements:
- **Auto-shelving**: Changes preserved automatically
- **Workspace isolation**: Each timeline has its own state
- **Efficient storage**: Shared content deduplicated

## Creating Timelines

### From Current Timeline

```bash
# Create from where you are now
ivaldi timeline create feature-authentication
```

### From Specific Timeline

```bash
# Create from main
ivaldi timeline create hotfix main
```

### From History

Use time travel to create from any past seal:

```bash
ivaldi travel
# Navigate to desired seal
# Select "Diverge"
# Enter timeline name
```

## Switching Timelines

### Basic Switch

```bash
ivaldi timeline switch main
```

### With Auto-Shelving

Auto-shelving in action:

```bash
# Working on feature with uncommitted changes
$ ivaldi wai
Timeline: feature-auth
Workspace: 3 files modified

# Switch to main
$ ivaldi timeline switch main
# Changes automatically shelved

$ ivaldi status
Working directory: clean
# Main is clean!

# Switch back to feature
$ ivaldi timeline switch feature-auth

$ ivaldi status
Unstaged changes:
  modified: src/auth.go
  modified: src/login.go
  modified: src/oauth.go
# All changes restored!
```

## Timeline Patterns

### Feature Timeline Pattern

Long-running feature development:

```bash
# Create feature timeline
ivaldi timeline create feature-payment-gateway

# Work on feature
ivaldi gather src/payment/
ivaldi seal "Add Stripe integration"

ivaldi gather src/checkout/
ivaldi seal "Update checkout flow"

# Regularly sync with main
ivaldi timeline switch main
ivaldi harvest --update
ivaldi timeline switch feature-payment-gateway
ivaldi fuse main to feature-payment-gateway

# When complete, merge to main
ivaldi timeline switch main
ivaldi fuse feature-payment-gateway to main
ivaldi upload
```

### Topic Timeline Pattern

Small, focused changes:

```bash
# Quick topic timeline
ivaldi timeline create fix-validation-bug

# Make focused change
ivaldi gather src/validation.go
ivaldi seal "Fix email validation regex"

# Merge immediately
ivaldi timeline switch main
ivaldi fuse fix-validation-bug to main
ivaldi upload

# Clean up
ivaldi timeline remove fix-validation-bug
```

### Experiment Timeline Pattern

Try new ideas safely:

```bash
# Create experiment timeline
ivaldi timeline create experiment-caching

# Try experimental approach
ivaldi gather src/cache.go
ivaldi seal "Experiment: Redis caching layer"

# Test it out...

# If successful:
ivaldi timeline switch main
ivaldi fuse experiment-caching to main

# If unsuccessful:
ivaldi timeline switch main
ivaldi timeline remove experiment-caching
# No harm done!
```

### Release Timeline Pattern

Prepare releases:

```bash
# Create release timeline
ivaldi timeline create release-v1.2

# Merge features
ivaldi fuse feature-a to release-v1.2
ivaldi fuse feature-b to release-v1.2

# Final testing and fixes
ivaldi seal "Update version to 1.2.0"
ivaldi seal "Update changelog"

# Merge to main
ivaldi timeline switch main
ivaldi fuse release-v1.2 to main
ivaldi upload
```

## Advanced Timeline Operations

### Parallel Development

Work on multiple features simultaneously:

```bash
# Start multiple features
ivaldi timeline create feature-auth
ivaldi timeline create feature-payments
ivaldi timeline create feature-notifications

# Switch between them freely
ivaldi timeline switch feature-auth
# ... work ...

ivaldi timeline switch feature-payments
# ... work ...

ivaldi timeline switch feature-notifications
# ... work ...

# Auto-shelving preserves all work!
```

### Stacked Timelines

Build features on top of each other:

```bash
# Base feature
ivaldi timeline create feature-user-model
ivaldi gather src/models/user.go
ivaldi seal "Add user model"

# Feature that depends on it
ivaldi timeline create feature-authentication
# (branched from feature-user-model)
ivaldi gather src/auth/
ivaldi seal "Add authentication using user model"

# When base is ready, merge both
ivaldi timeline switch main
ivaldi fuse feature-user-model to main
ivaldi timeline switch feature-authentication
ivaldi fuse main to feature-authentication  # Update base
ivaldi timeline switch main
ivaldi fuse feature-authentication to main
```

### Timeline Renaming Strategy

Use prefixes for organization:

```bash
# Feature timelines
ivaldi timeline create feature/authentication
ivaldi timeline create feature/payment-gateway

# Bugfix timelines
ivaldi timeline create bugfix/memory-leak
ivaldi timeline create bugfix/null-pointer

# Experimental timelines
ivaldi timeline create experiment/new-algorithm
ivaldi timeline create experiment/performance-opt
```

## Merging Timelines

### Fast-Forward Merge

When timeline is ahead:

```bash
$ ivaldi fuse feature-auth to main

[MERGE] Fast-forward merge detected
>> Updating main timeline...
[OK] Merge completed successfully!
```

### Three-Way Merge

When both timelines have changes:

```bash
$ ivaldi fuse feature-payment to main

Analyzing timelines for merge...
Changes to be merged:
  5 files will be added
  3 files will be modified

Proceed with merge? (yes/no): yes

>> Creating merge commit...
[OK] Merge completed successfully!
```

### Conflict Resolution

When conflicts occur:

```bash
$ ivaldi fuse feature-auth to main

[CONFLICTS] Merge conflicts detected:
  CONFLICT: src/auth.go

Resolution options:
  ivaldi fuse --strategy=theirs feature-auth to main
  ivaldi fuse --strategy=ours feature-auth to main
  ivaldi fuse --abort

# Choose strategy or resolve manually
$ ivaldi fuse --strategy=theirs feature-auth to main
[OK] Merge completed successfully!
```

See [fuse command](../commands/fuse.md) for detailed merge strategies.

## Timeline Management

### List Timelines

```bash
$ ivaldi timeline list

* main
  feature-auth
  feature-payments
  bugfix-security
  experiment-caching
```

### Remove Timelines

```bash
# After merging, clean up
ivaldi timeline remove feature-auth
```

### Archive Pattern

Before removing, push to GitHub to archive:

```bash
# Push for archival
ivaldi timeline switch old-feature
ivaldi upload

# Now safe to remove locally
ivaldi timeline switch main
ivaldi timeline remove old-feature
```

## Best Practices

### Timeline Naming

Good names:
- `feature-user-authentication`
- `bugfix-memory-leak-in-parser`
- `refactor-database-layer`
- `experiment-new-caching-strategy`

Bad names:
- `test`
- `new`
- `branch1`
- `temp`

### Timeline Lifecycle

1. **Create**: `ivaldi timeline create feature-x`
2. **Develop**: Make commits
3. **Sync**: Regularly merge main into feature
4. **Review**: Test thoroughly
5. **Merge**: Merge feature into main
6. **Upload**: Push to GitHub
7. **Clean**: Remove local timeline

### Keep Timelines Focused

```bash
# Good: Focused timeline
ivaldi timeline create add-user-login
# Only login-related changes

# Bad: Unfocused timeline
ivaldi timeline create various-updates
# Unrelated changes mixed together
```

### Sync Regularly

```bash
# Daily: Update feature with main
ivaldi timeline switch feature-auth
ivaldi fuse main to feature-auth

# This prevents merge conflicts later
```

### Commit Before Switching

While auto-shelving protects you:

```bash
# Good practice
ivaldi gather .
ivaldi seal "WIP: Feature in progress"
ivaldi timeline switch main

# Auto-shelving works but commits are better
```

## Timeline Strategies

### GitFlow-Style

```bash
# Main timelines
main (production)
develop (integration)

# Feature timelines
feature/user-auth
feature/payments

# Release timelines
release/v1.2

# Hotfix timelines
hotfix/security-patch
```

### Trunk-Based Development

```bash
# Single main timeline
main (always deployable)

# Short-lived feature timelines
feature-login (1-2 days)
feature-logout (1-2 days)

# Merge quickly and frequently
```

### Custom Workflow

Design your own:

```bash
# Stability tiers
main (production)
staging (pre-production)
develop (active development)

# Feature branches from develop
ivaldi timeline switch develop
ivaldi timeline create feature-x
```

## Troubleshooting

### Lost Changes

Auto-shelving should prevent this, but if needed:

```bash
# List all timelines
ivaldi timeline list

# Switch to each and check
ivaldi timeline switch feature-x
ivaldi status
```

### Merge Conflicts

```bash
# Use merge strategies
ivaldi fuse --strategy=theirs feature to main
ivaldi fuse --strategy=ours feature to main

# Or resolve manually
ivaldi fuse feature to main
# Edit conflicted files
ivaldi gather .
ivaldi fuse --continue
```

### Timeline Cleanup

```bash
# Remove multiple timelines
for timeline in old-feature-1 old-feature-2 old-feature-3; do
    ivaldi timeline remove $timeline
done
```

## Summary

Timelines provide:
- Enhanced branching with auto-shelving
- Safe experimentation
- Parallel development
- Clean merge strategies

Key commands:
- `ivaldi timeline create <name>` - Create timeline
- `ivaldi timeline switch <name>` - Switch timeline
- `ivaldi fuse <source> to <target>` - Merge timelines
- `ivaldi timeline list` - List all timelines
- `ivaldi timeline remove <name>` - Remove timeline

## Next Steps

- Explore [Team Collaboration](collaboration.md)
- Learn [GitHub Integration](github-integration.md)
- Master [Time Travel](../commands/travel.md)
