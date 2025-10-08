---
layout: default
title: Team Collaboration
---

# Team Collaboration

Learn how to work effectively with your team using Ivaldi.

## Setting Up for Collaboration

### Initial Setup

```bash
# Clone team repository
ivaldi download team/project
cd project

# Configure your identity
ivaldi config --set user.name "Your Name"
ivaldi config --set user.email "you@company.com"
```

### GitHub Authentication

```bash
# Option 1: GitHub token
export GITHUB_TOKEN="your_token"

# Option 2: GitHub CLI
gh auth login
```

## Daily Team Workflow

### Morning Sync

Start your day by syncing with the team:

```bash
# See what's new on remote
ivaldi scout

# Update main timeline
ivaldi timeline switch main
ivaldi harvest --update

# Check status
ivaldi log --limit 5
```

### Work on Feature

```bash
# Create your feature timeline
ivaldi timeline create feature-user-dashboard

# Make changes
vim src/dashboard.js
ivaldi gather .
ivaldi seal "Add dashboard layout"

# Push for team visibility
ivaldi upload
```

### End of Day

```bash
# Push your progress
ivaldi gather .
ivaldi seal "WIP: Dashboard - completed layout, need to add data"
ivaldi upload

# Teammates can now see your work!
```

## Reviewing Teammate's Work

### Discover and Download

```bash
# See what timelines are available
$ ivaldi scout

Remote timelines available:
  main
  feature-authentication (Alice)
  feature-payment (Bob)
  bugfix-validation (Carol)

# Download Alice's branch
ivaldi harvest feature-authentication
```

### Review Changes

```bash
# Switch to teammate's timeline
ivaldi timeline switch feature-authentication

# Review commits
ivaldi log

# See what changed
ivaldi diff main

# Test locally
npm test
```

### Provide Feedback

```bash
# Add improvements
vim src/auth.js
ivaldi gather .
ivaldi seal "Add error handling to login flow"

# Push back
ivaldi upload

# Alice will see your contribution!
```

## Contributing to Shared Timeline

### Pull Latest Changes

```bash
# Switch to shared timeline
ivaldi timeline switch feature-payment

# Get latest from remote
ivaldi harvest feature-payment --update
```

### Add Your Changes

```bash
# Make improvements
vim src/payment/stripe.js
ivaldi gather .
ivaldi seal "Add Stripe webhook handling"

# Push
ivaldi upload
```

### Sync Regularly

```bash
# Before starting work
ivaldi harvest feature-shared --update

# After making changes
ivaldi upload

# This minimizes conflicts
```

## Handling Team Changes

### Merge Main into Feature

Keep your feature up-to-date with main:

```bash
# Switch to your feature
ivaldi timeline switch feature-dashboard

# Get latest main
ivaldi harvest main --update

# Merge main into feature
ivaldi fuse main to feature-dashboard

# Resolve any conflicts
ivaldi fuse --strategy=theirs main to feature-dashboard
# or manually resolve

# Push updated feature
ivaldi upload
```

### Update Multiple Features

```bash
# Get all updates
ivaldi scout
ivaldi harvest --update

# Update each feature timeline
for timeline in feature-a feature-b feature-c; do
    ivaldi timeline switch $timeline
    ivaldi fuse main to $timeline
done
```

## Code Review Workflow

### Prepare for Review

```bash
# Ensure clean history
ivaldi log

# Push final version
ivaldi gather .
ivaldi seal "Final: User dashboard with all features"
ivaldi upload
```

### Review Process

Reviewer:
```bash
# Get feature branch
ivaldi scout
ivaldi harvest feature-user-dashboard

# Switch and review
ivaldi timeline switch feature-user-dashboard
ivaldi log
ivaldi diff main

# Test
npm test

# Approve by merging
ivaldi timeline switch main
ivaldi fuse feature-user-dashboard to main
ivaldi upload
```

### Request Changes

Reviewer:
```bash
# Add comments as commits
ivaldi gather .
ivaldi seal "Review: Add input validation on line 42"
ivaldi upload
```

Author responds:
```bash
# Get reviewer's comments
ivaldi harvest feature-user-dashboard --update

# Make changes
vim src/dashboard.js
ivaldi seal "Address review: Add input validation"
ivaldi upload
```

## Selective Sync

Ivaldi's killer feature for teams: download only what you need.

### Download Specific Features

```bash
# See all branches
$ ivaldi scout

Remote timelines available:
  main
  feature-auth
  feature-payment
  feature-ui
  feature-mobile
  feature-analytics
  experimental-redesign

# Only download what you need
ivaldi harvest feature-auth feature-payment

# Skip the rest!
# Saves bandwidth and disk space
```

### Team Benefits

```bash
# Frontend developer: Only get UI branches
ivaldi harvest feature-ui feature-design

# Backend developer: Only get API branches
ivaldi harvest feature-api feature-database

# Full-stack: Get what you need when you need it
ivaldi scout
ivaldi harvest feature-auth
# Work on auth...
ivaldi harvest feature-payment
# Now work on payment...
```

## Release Coordination

### Preparing Release

Release manager:
```bash
# Create release timeline
ivaldi timeline create release-v1.2

# Merge approved features
ivaldi fuse feature-auth to release-v1.2
ivaldi fuse feature-payment to release-v1.2
ivaldi fuse feature-ui to release-v1.2

# Final testing
ivaldi seal "Update version to 1.2.0"
ivaldi seal "Update changelog"

# Merge to main
ivaldi timeline switch main
ivaldi fuse release-v1.2 to main
ivaldi upload
```

Team members:
```bash
# Get release timeline for testing
ivaldi harvest release-v1.2
ivaldi timeline switch release-v1.2

# Test and report
```

## Conflict Resolution

### Preventing Conflicts

```bash
# Sync frequently
ivaldi harvest main --update
ivaldi fuse main to feature-branch

# Small, focused changes
# Communicate with team
```

### Resolving Conflicts

```bash
$ ivaldi fuse feature-a to main

[CONFLICTS] Merge conflicts detected:
  CONFLICT: src/shared.go

# Strategy 1: Accept their changes
ivaldi fuse --strategy=theirs feature-a to main

# Strategy 2: Keep our changes
ivaldi fuse --strategy=ours feature-a to main

# Strategy 3: Manual resolution
vim src/shared.go  # Edit file
ivaldi gather src/shared.go
ivaldi fuse --continue
```

### Communication

When conflicts occur:
1. Talk to teammate who worked on conflicted file
2. Decide on resolution together
3. Apply agreed-upon changes
4. Test thoroughly

## Team Best Practices

### Naming Conventions

Agree on timeline naming:
```bash
# By person
alice/feature-auth
bob/feature-payment

# By type
feature/authentication
bugfix/memory-leak
hotfix/security-patch

# By ticket
JIRA-123-add-login
JIRA-456-fix-validation
```

### Commit Messages

Use consistent format:
```bash
# Good team messages
ivaldi seal "feat: Add OAuth2 authentication"
ivaldi seal "fix: Resolve memory leak in parser"
ivaldi seal "docs: Update API documentation"

# Include ticket numbers
ivaldi seal "JIRA-123: Implement user login endpoint"
```

### Regular Syncing

```bash
# Morning routine
ivaldi scout
ivaldi harvest --update

# Before starting work
ivaldi harvest main --update
ivaldi fuse main to feature-branch

# Before ending day
ivaldi upload
```

### Feature Timeline Lifecycle

1. **Create**: `ivaldi timeline create feature-name`
2. **Push early**: `ivaldi upload` (even WIP)
3. **Sync daily**: `ivaldi fuse main to feature-name`
4. **Communicate**: Let team know about significant changes
5. **Review**: Request code review
6. **Merge**: Integrate into main
7. **Clean**: Remove merged timeline

## Team Communication Patterns

### Feature Handoff

Developer A:
```bash
ivaldi seal "WIP: Authentication - completed login, need logout"
ivaldi upload
# Message team: "Login complete, can someone add logout?"
```

Developer B:
```bash
ivaldi harvest feature-auth
ivaldi timeline switch feature-auth
ivaldi seal "Add logout endpoint"
ivaldi upload
# Message: "Logout added!"
```

### Pair Programming

```bash
# Developer A drives
ivaldi gather .
ivaldi seal "Add validation logic"
ivaldi upload

# Developer B takes over
ivaldi harvest feature-shared --update
ivaldi gather .
ivaldi seal "Add error handling"
ivaldi upload

# Developer A continues
ivaldi harvest feature-shared --update
# ...
```

### Code Review Comments

```bash
# Reviewer
ivaldi timeline switch feature-to-review
ivaldi seal "Review: Consider using factory pattern here"
ivaldi seal "Review: Need unit tests for this function"
ivaldi upload

# Author sees and addresses
ivaldi harvest feature-to-review --update
ivaldi log
# Make changes based on feedback
```

## Large Team Strategies

### Component Teams

```bash
# Frontend team
ivaldi harvest feature-ui-*

# Backend team
ivaldi harvest feature-api-*

# Mobile team
ivaldi harvest feature-mobile-*
```

### Integration Timeline

```bash
# Create integration timeline
ivaldi timeline create integration-sprint-42

# Each team merges their work
ivaldi fuse feature-ui-dashboard to integration-sprint-42
ivaldi fuse feature-api-users to integration-sprint-42

# Test integration
# Fix issues

# Merge to main when ready
ivaldi fuse integration-sprint-42 to main
```

## Troubleshooting

### Someone Pushed Breaking Changes

```bash
# Use time travel to before breakage
ivaldi travel
# Select good commit, diverge
# Continue work on stable base
```

### Accidentally Pushed to Wrong Branch

```bash
# Download correct branch
ivaldi harvest correct-branch

# Cherry-pick changes
ivaldi timeline switch correct-branch
# Manually apply changes
ivaldi gather .
ivaldi seal "Moved changes to correct branch"
ivaldi upload
```

### Lost Local Work

```bash
# Check all timelines
ivaldi timeline list

# Check each timeline
ivaldi timeline switch feature-x
ivaldi status

# If pushed, re-download
ivaldi harvest feature-x --update
```

## Summary

Team collaboration essentials:
- **Scout & Harvest**: Discover and selectively download branches
- **Regular Syncing**: Stay up-to-date with team changes
- **Clear Communication**: Use descriptive names and messages
- **Code Review**: Collaborate on timelines
- **Conflict Resolution**: Use strategies or communicate

Key commands:
- `ivaldi scout` - See remote branches
- `ivaldi harvest` - Download branches selectively
- `ivaldi upload` - Share your work
- `ivaldi fuse` - Integrate changes

## Next Steps

- Master [GitHub Integration](github-integration.md)
- Learn [Timeline Branching](branching.md)
- Review [Basic Workflow](basic-workflow.md)
