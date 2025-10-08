---
layout: default
title: ivaldi travel
---

# ivaldi travel

Interactive time travel through commit history with arrow key navigation.

## Synopsis

```bash
ivaldi travel
ivaldi travel [options]
```

## Description

The `travel` command provides an interactive interface to browse commit history and either:
- **Diverge**: Create a new timeline from any past seal (non-destructive)
- **Overwrite**: Reset current timeline to a past seal (destructive)

Features:
- Arrow key navigation
- Cursor-based selection
- Automatic pagination for large histories
- Search functionality

## Options

- `--limit <n>`, `-n <n>` - Show only N most recent seals (default: 20)
- `--all`, `-a` - Show all seals without pagination
- `--search <term>`, `-s <term>` - Filter seals by message, author, or name

## Examples

### Basic Time Travel

```bash
ivaldi travel
```

Shows interactive seal browser:
```
Seals in timeline 'main':

> 1. fierce-gate-builds-quick-b6f8f458 (b6f8f458)
     Add authentication feature
     Jane Doe <jane@example.com> • 2025-10-07 14:32:10

  2. empty-phoenix-attacks-fresh-7bb05886 (7bb05886)
     Fix payment bug
     John Smith <john@example.com> • 2025-10-07 14:30:45

Up/Down arrows navigate • Enter to select • q to quit
```

### Limit Results

```bash
ivaldi travel --limit 10
ivaldi travel -n 5
```

### Show All Seals

```bash
ivaldi travel --all
```

### Search Seals

```bash
ivaldi travel --search "authentication"
ivaldi travel -s "bug fix"
ivaldi travel --search "john@example.com"
```

## Interactive Navigation

### Keyboard Controls

| Key | Action |
|-----|--------|
| Up Arrow | Move cursor up |
| Down Arrow | Move cursor down |
| Enter | Select highlighted seal |
| n | Next page (when paginated) |
| p | Previous page (when paginated) |
| 1-9 | Jump to seal number |
| q or ESC | Quit/cancel |

### Using Arrow Keys

1. **Launch**: Run `ivaldi travel`
2. **Navigate**: Use Up/Down arrows to move through seals
3. **Select**: Press Enter on the seal you want
4. **Choose**: Decide to diverge or overwrite

The selected seal is highlighted in bold with a green arrow (>).

## Diverging (Non-Destructive)

Create a new timeline from a past seal:

```bash
$ ivaldi travel
# Navigate to desired seal, press Enter

Selected seal: swift-eagle-flies-high-447abe9b
  Position: 2 commits behind current HEAD
  Message: Add authentication

? What would you like to do?
  1. Diverge - Create new timeline from this seal
  2. Overwrite - Reset current timeline
  3. Cancel

Choice: 1
Enter new timeline name: experiment-auth

Created new timeline 'experiment-auth' from seal swift-eagle-flies-high
Switched to timeline 'experiment-auth'
Workspace materialized to seal: swift-eagle-flies-high-447abe9b
```

Your original timeline remains unchanged!

## Overwriting (Destructive)

Reset timeline to a past seal:

```bash
$ ivaldi travel
# Navigate to desired seal, press Enter

Choice: 2

WARNING: This will permanently remove 2 commit(s) from 'main'.
Are you sure? Type 'yes' to confirm: yes

Timeline 'main' reset to seal swift-eagle-flies-high-447abe9b
2 commit(s) removed from timeline
Workspace materialized to seal: swift-eagle-flies-high-447abe9b
```

## Use Cases

### Experiment with Different Approaches

Try a different implementation:
```bash
ivaldi travel
# Select commit 3, diverge to 'experimental-approach'
# Original work preserved in 'main'
```

### Fix Bug in Older Code

Branch from before the bug:
```bash
ivaldi travel
# Select commit before bug, diverge to 'bugfix-issue-42'
# Fix and create separate branch
```

### Undo Bad Commits

Remove unwanted commits:
```bash
ivaldi travel
# Select good commit, overwrite to remove bad commits
# Warning: Destructive!
```

### Create Multiple Features from One Point

Branch multiple times from stable commit:
```bash
ivaldi travel
# Diverge to 'feature-a'
ivaldi timeline switch main

ivaldi travel
# Diverge to 'feature-b' from same point
```

## Pagination

For large histories (>20 commits), pagination activates:

```
Seals in timeline 'main' (showing 1-20 of 156):

> 1. seal-name-1 (hash1)
     Message...

  ...

  20. seal-name-20 (hash20)
      Message...

Page 1 of 8
Up/Down arrows navigate • Enter to select • n/p page • q to quit
```

Navigation:
- Arrow keys auto-handle page boundaries
- Type 'n' for next page, 'p' for previous
- Jump to any seal number directly

## Best Practices

### Diverge for Experiments

Always diverge rather than overwrite when experimenting:
```bash
# Good: Non-destructive
ivaldi travel -> diverge -> new timeline

# Risky: Destructive
ivaldi travel -> overwrite
```

### Commit Before Traveling

Save work before time traveling:
```bash
ivaldi gather .
ivaldi seal "WIP before time travel"
ivaldi travel
```

### Push Before Overwriting

Back up commits you might want later:
```bash
ivaldi upload  # Push to GitHub
ivaldi travel  # Now safe to overwrite locally
```

### Use Search for Large Histories

Don't scroll through hundreds of commits:
```bash
ivaldi travel --search "keyword"
```

## Related Commands

- [timeline](timeline.md) - Create timelines from current HEAD
- [log](log.md) - View commit history (non-interactive)
- [whereami](whereami.md) - Show current position
- [fuse](fuse.md) - Merge timelines

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git log` | `ivaldi travel` (interactive) |
| `git checkout -b new <commit>` | Diverge option |
| `git reset --hard <commit>` | Overwrite option |
| Scroll terminal output | Arrow key navigation |
| View only | Interactive actions |

## Troubleshooting

### Timeline Has No Commits

```
Error: timeline has no commits yet
```

Solution:
```bash
ivaldi seal "Initial commit"
ivaldi travel
```

### No Seals Found

No commits in timeline.

Solution:
```bash
ivaldi whereami  # Check current timeline
ivaldi timeline list  # See all timelines
```

### Search Returns Nothing

```
No seals found matching 'search-term'
```

Try:
- Broader search term
- Different keywords
- Check you're on right timeline
