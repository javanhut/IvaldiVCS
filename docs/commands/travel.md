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

The `travel` command provides an interactive fixed-window interface to browse commit history and either:
- **Diverge**: Create a new timeline from any past seal (non-destructive)
- **Overwrite**: Reset current timeline to a past seal (destructive)

Features:
- **Fixed scrolling window** - No screen flashing, smooth navigation
- **Auto-sized viewport** - Adapts to terminal height
- **Visual scroll indicator** - Shows position in commit list
- **Arrow key navigation** - Smooth scrolling with cursor
- **Jump to anywhere** - Home/End keys and number shortcuts
- **Search functionality** - Filter by message, author, or name

## Options

- `--window-size <n>`, `-w <n>` - Number of seals in viewport (0 for auto-detect, default: auto)
- `--search <term>`, `-s <term>` - Filter seals by message, author, or name

## Examples

### Basic Time Travel

```bash
ivaldi travel
```

Shows interactive fixed-window seal browser:
```
╔═══════════════════════════════════════════════════════════════════════╗
║ ⏱ Seals in timeline 'main'                                           ║
║ Showing 1-10 of 38 [■■·········]                                     ║
╚═══════════════════════════════════════════════════════════════════════╝

→ 1. fierce-gate-builds-quick (b6f8)           [HEAD]
     Add authentication feature
     Jane Doe • 2025-10-07 14:32:10

  2. empty-phoenix-attacks-fresh (7bb0)
     Fix payment bug
     John Smith • 2025-10-07 14:30:45

  3. swift-eagle-flies-high (447a)
     Refactor database layer
     Alice Johnson • 2025-10-07 14:28:12

  ... (7 more seals in window)

╔═══════════════════════════════════════════════════════════════════════╗
║ ↑/↓ navigate • Enter select • Home/End jump • 1-9 goto • q quit      ║
╚═══════════════════════════════════════════════════════════════════════╝
```

**Features:**
- **Fixed viewport**: Window size adapts to terminal height
- **Scroll indicator**: `[■■·········]` shows position in list
- **Smooth scrolling**: Window scrolls when cursor reaches edges
- **No flashing**: Only updates cursor position, not entire screen

### Custom Window Size

```bash
# Show 15 seals at a time
ivaldi travel --window-size 15
ivaldi travel -w 15

# Auto-detect based on terminal height (default)
ivaldi travel
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
| ↑ (Up Arrow) | Move cursor up (scrolls window at top) |
| ↓ (Down Arrow) | Move cursor down (scrolls window at bottom) |
| Enter | Select highlighted seal |
| Home / H | Jump to first seal |
| End / E | Jump to last seal |
| 1-9 | Jump to specific seal number |
| q or ESC | Quit/cancel |

**Smooth Scrolling Behavior:**
- Cursor moves within visible window
- Window automatically scrolls when cursor reaches edge
- No screen clearing or flashing
- Visual scroll indicator shows position: `[■■■·······]`

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

## Fixed-Window Scrolling

For any size history, the fixed-window interface provides smooth navigation:

```
╔═══════════════════════════════════════════════════════════════════════╗
║ ⏱ Seals in timeline 'main'                                           ║
║ Showing 51-60 of 156 [········■■]  ← Scroll indicator                ║
╚═══════════════════════════════════════════════════════════════════════╝

  51. seal-name-51 (hash)
      Message...

→ 52. seal-name-52 (hash)  ← Current cursor position
      Message...

  ... (8 more seals)

  60. seal-name-60 (hash)
      Message...

╔═══════════════════════════════════════════════════════════════════════╗
║ ↑/↓ navigate • Enter select • Home/End jump • 1-9 goto • q quit      ║
╚═══════════════════════════════════════════════════════════════════════╝
```

**How It Works:**
- Fixed viewport shows 5-20 seals (based on terminal height)
- Arrow keys move cursor within window
- Window scrolls smoothly when cursor reaches edge
- Scroll indicator `[········■■]` shows relative position
- Press Home/End to jump to start/end
- Type seal number to jump directly

**Benefits:**
- No screen flashing or clearing
- Always know where you are
- Mouse-free navigation
- Works with any history size

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
