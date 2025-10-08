---
layout: default
title: ivaldi timeline
---

# ivaldi timeline

Manage timelines (branches) in your repository.

## Synopsis

```bash
ivaldi timeline create <name> [from]
ivaldi timeline switch <name>
ivaldi timeline list
ivaldi timeline remove <name>
```

## Description

Timelines are Ivaldi's equivalent to Git branches, with enhanced features like auto-shelving and workspace isolation.

## Subcommands

### create

Create a new timeline.

```bash
ivaldi timeline create <name> [from]
```

Arguments:
- `<name>` - Name for the new timeline
- `[from]` - Optional source timeline (defaults to current)

Examples:
```bash
# Create from current timeline
ivaldi timeline create feature-auth

# Create from specific timeline
ivaldi timeline create hotfix main
```

### switch

Switch to a different timeline.

```bash
ivaldi timeline switch <name>
```

Arguments:
- `<name>` - Timeline to switch to

Features:
- **Auto-shelving**: Uncommitted changes automatically saved
- **Workspace materialization**: Files updated to match timeline
- **Change restoration**: Return to timeline, changes restored

Examples:
```bash
# Switch to main
ivaldi timeline switch main

# Switch to feature branch
ivaldi timeline switch feature-auth
```

### list

List all timelines in the repository.

```bash
ivaldi timeline list
```

Output:
```
* main
  feature-auth
  bugfix-payment
  experiment-new-algo
```

The `*` indicates the current timeline.

### remove

Delete a timeline.

```bash
ivaldi timeline remove <name>
```

Arguments:
- `<name>` - Timeline to delete

Example:
```bash
ivaldi timeline remove old-feature
```

Note: Cannot remove current timeline. Switch first.

## Auto-Shelving

When switching timelines, uncommitted changes are automatically preserved:

```bash
# Working on feature with changes
echo "WIP" >> feature.txt

# Switch to main
ivaldi timeline switch main
# Changes automatically shelved

# Switch back
ivaldi timeline switch feature-auth
# Changes automatically restored!
```

## Common Workflows

### Feature Development

```bash
# Create feature timeline
ivaldi timeline create feature-login

# Work on feature
ivaldi gather src/login.go
ivaldi seal "Add login page"

# Switch back to main
ivaldi timeline switch main

# Merge when ready
ivaldi fuse feature-login to main
```

### Hotfix

```bash
# Create hotfix from main
ivaldi timeline switch main
ivaldi timeline create hotfix-security

# Fix issue
ivaldi gather src/auth.go
ivaldi seal "Fix security vulnerability"

# Merge back
ivaldi timeline switch main
ivaldi fuse hotfix-security to main

# Clean up
ivaldi timeline remove hotfix-security
```

### Experiment

```bash
# Create experimental timeline
ivaldi timeline create experiment-new-algo

# Try new approach
# ... make changes ...

# If successful, merge
ivaldi timeline switch main
ivaldi fuse experiment-new-algo to main

# If unsuccessful, abandon
ivaldi timeline switch main
ivaldi timeline remove experiment-new-algo
```

### Parallel Development

```bash
# List all timelines
ivaldi timeline list

# Work on multiple features
ivaldi timeline switch feature-a
# ... work ...

ivaldi timeline switch feature-b
# ... work ...

ivaldi timeline switch main
# Merge them all
ivaldi fuse feature-a to main
ivaldi fuse feature-b to main
```

## Timeline Naming

Good timeline names:
- `feature-authentication`
- `bugfix-null-pointer`
- `refactor-database-layer`
- `experiment-caching`

Bad timeline names:
- `test`
- `temp`
- `new`
- `branch1`

## Best Practices

### Commit Before Switching

While auto-shelving protects you, commit when possible:
```bash
ivaldi gather .
ivaldi seal "WIP: Feature in progress"
ivaldi timeline switch main
```

### Clean Up Old Timelines

Remove timelines you no longer need:
```bash
ivaldi timeline list
ivaldi timeline remove old-feature
```

### Use Descriptive Names

Clear names help collaboration:
```bash
# Good
ivaldi timeline create feature-user-authentication

# Bad
ivaldi timeline create feature1
```

## Related Commands

- [fuse](fuse.md) - Merge timelines
- [travel](travel.md) - Time travel and create timelines from history
- [log](log.md) - View timeline history
- [whereami](whereami.md) - Show current timeline

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git branch new-branch` | `ivaldi timeline create new-branch` |
| `git checkout branch` | `ivaldi timeline switch branch` |
| `git branch` | `ivaldi timeline list` |
| `git branch -d branch` | `ivaldi timeline remove branch` |
| Manual stashing | Automatic shelving |
| Shared workspace | Isolated workspace |

## Troubleshooting

### Timeline Already Exists

```
Error: timeline 'feature-x' already exists
```

Solutions:
- Choose different name
- Remove existing: `ivaldi timeline remove feature-x`
- Switch to existing: `ivaldi timeline switch feature-x`

### Cannot Remove Current Timeline

```
Error: cannot remove current timeline
```

Solution:
```bash
ivaldi timeline switch main
ivaldi timeline remove old-timeline
```

### Timeline Not Found

```
Error: timeline 'feature-x' not found
```

Solution:
```bash
ivaldi timeline list
# Choose existing timeline
```
