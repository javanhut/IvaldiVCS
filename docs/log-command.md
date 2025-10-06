# Log Command

The `ivaldi log` command displays the commit history of your repository, showing seals (commits) in reverse chronological order with human-friendly seal names.

## Overview

The log command provides:
- **Commit history**: View all seals in the current timeline
- **Seal names**: Human-readable commit identifiers like "swift-eagle-flies-high-447abe9b"
- **Author information**: See who created each seal
- **Timestamps**: When each seal was created
- **Multiple formats**: Detailed or concise output
- **All timelines**: View history across all timelines

## Basic Usage

### View Commit History

Show the full commit history:

```bash
ivaldi log
```

**Example output:**
```
Seal: swift-eagle-flies-high-447abe9b (447abe9b)
Timeline: main
Author: Jane Smith <jane@example.com>
Date: 2025-10-05 14:30:22 -0700

    Add new authentication feature

    Implemented JWT-based authentication with refresh tokens

Seal: calm-river-flows-deep-2a1b3c4d (2a1b3c4d)
Timeline: main
Author: Jane Smith <jane@example.com>
Date: 2025-10-04 09:15:33 -0700

    Initial commit

    Set up project structure
```

### Concise Format

Show one line per commit:

```bash
ivaldi log --oneline
```

**Example output:**
```
447abe9b swift-eagle-flies-high-447abe9b Add new authentication feature
2a1b3c4d calm-river-flows-deep-2a1b3c4d Initial commit
```

### Limit Results

Show only the last N commits:

```bash
ivaldi log --limit 5
```

**Example output:**
```
Seal: swift-eagle-flies-high-447abe9b (447abe9b)
...
(Shows only the 5 most recent seals)
```

### All Timelines

View commits from all timelines:

```bash
ivaldi log --all
```

**Example output:**
```
Seal: feature-commit-abc123 (abc123)
Timeline: feature-auth
Author: Jane Smith <jane@example.com>
Date: 2025-10-05 16:45:12 -0700

    Add login endpoint

Seal: swift-eagle-flies-high-447abe9b (447abe9b)
Timeline: main
Author: Jane Smith <jane@example.com>
Date: 2025-10-05 14:30:22 -0700

    Add new authentication feature
```

## Command Options

### `--oneline`

Display each seal on a single line with short hash and message.

```bash
ivaldi log --oneline
```

**Format:** `<short-hash> <seal-name> <message>`

**Use case:** Quick overview of recent changes, scripting

### `--limit <number>`

Limit output to the specified number of most recent seals.

```bash
ivaldi log --limit 10
```

**Use case:** Viewing recent history without overwhelming output

### `--all`

Show commits from all timelines, not just the current one.

```bash
ivaldi log --all
```

**Use case:** Understanding project history across all branches

## Output Format

### Detailed Format (Default)

```
Seal: <seal-name> (<short-hash>)
Timeline: <timeline-name>
Author: <name> <email>
Date: <timestamp>

    <commit message>
    <additional lines>
```

### Oneline Format

```
<short-hash> <seal-name> <first-line-of-message>
```

## Common Workflows

### Review Recent Changes

Check what's been done recently:

```bash
ivaldi log --limit 5
```

### Quick History Scan

Get a concise overview:

```bash
ivaldi log --oneline --limit 20
```

### Find Specific Commit

Search for a commit by seal name or message:

```bash
ivaldi log | grep "authentication"
ivaldi log | grep "swift-eagle"
```

### Compare Timeline Histories

View history of different timelines:

```bash
# Current timeline
ivaldi log --limit 5

# Switch and view another timeline
ivaldi timeline switch feature-auth
ivaldi log --limit 5

# Or view all at once
ivaldi timeline switch main
ivaldi log --all
```

### Generate Release Notes

Use log output for release documentation:

```bash
# Get commits since last release
ivaldi log --oneline --limit 10 > release-notes.txt
```

### Audit Trail

Review who made changes and when:

```bash
ivaldi log | grep "Author:"
```

## Understanding Seal Names

Each commit gets a unique, memorable seal name:

```
swift-eagle-flies-high-447abe9b
│     │     │    │    │
│     │     │    │    └─── Short hash (8 chars)
│     │     │    └─────── Adjective
│     │     └────────────── Verb
│     └────────────────────── Adjective
└──────────────────────────── Noun
```

**Benefits:**
- Easier to remember than hashes
- Unique identifier for each commit
- Can use either seal name or hash to reference commits

## Practical Examples

### Example 1: Daily Standup

Review your commits from today:

```bash
ivaldi log --oneline --limit 10
```

### Example 2: Code Review

See what changed in a feature timeline:

```bash
ivaldi timeline switch feature-payment
ivaldi log
```

### Example 3: Debugging

Find when a bug was introduced:

```bash
ivaldi log | grep -A 5 "payment"
# Shows commits related to payment feature
```

### Example 4: Statistics

Count commits by author:

```bash
ivaldi log | grep "Author:" | sort | uniq -c
```

### Example 5: Timeline Comparison

Compare commit counts across timelines:

```bash
# Main timeline
ivaldi timeline switch main
echo "Main: $(ivaldi log --oneline | wc -l) commits"

# Feature timeline
ivaldi timeline switch feature-auth
echo "Feature: $(ivaldi log --oneline | wc -l) commits"
```

## Integration with Other Commands

### With `ivaldi diff`

Compare specific commits found in log:

```bash
ivaldi log --oneline
# Note two seal hashes: abc123 and def456
ivaldi diff abc123 def456
```

### With `ivaldi seals`

Get detailed information about a seal:

```bash
ivaldi log --oneline
# Find interesting seal: swift-eagle-flies-high-447abe9b
ivaldi seals show swift-eagle-flies-high-447abe9b
```

### With `ivaldi fuse`

Understand timeline history before merging:

```bash
# Check what will be merged
ivaldi timeline switch feature-auth
ivaldi log --limit 5

# View target timeline
ivaldi timeline switch main
ivaldi log --limit 5

# Perform merge
ivaldi fuse feature-auth to main
```

## Comparison with Git

| Feature | Git | Ivaldi |
|---------|-----|--------|
| View history | `git log` | `ivaldi log` |
| Oneline format | `git log --oneline` | `ivaldi log --oneline` |
| Limit results | `git log -n 5` | `ivaldi log --limit 5` |
| All branches | `git log --all` | `ivaldi log --all` |
| Show author | `git log --author` | (built-in to default output) |
| Pretty format | `git log --pretty` | (default is pretty) |

## Tips and Tricks

### 1. Combine Flags

Use multiple options together:

```bash
ivaldi log --oneline --limit 20 --all
```

### 2. Pipe to Tools

Use standard Unix tools for filtering:

```bash
ivaldi log | grep "Jane"
ivaldi log --oneline | head -10
ivaldi log | less
```

### 3. Save History

Export log for documentation:

```bash
ivaldi log > project-history.txt
ivaldi log --oneline > commits-summary.txt
```

### 4. Search by Date

Find commits from a specific time:

```bash
ivaldi log | grep "2025-10-05"
```

### 5. Author Statistics

See contribution patterns:

```bash
ivaldi log | grep "Author:" | cut -d: -f2 | sort | uniq -c | sort -rn
```

## Troubleshooting

### No Output

If `ivaldi log` shows nothing:

```bash
# Check if there are any commits
ivaldi status

# Verify you're in an Ivaldi repository
ls -la .ivaldi/

# Check current timeline
ivaldi whereami
```

**Solution:** Create your first commit:
```bash
ivaldi gather .
ivaldi seal "Initial commit"
```

### Garbled Output

If output looks corrupted:

```bash
# Disable colors
ivaldi config color.ui false
ivaldi log
```

### Too Much Output

If history is overwhelming:

```bash
# Use pagination
ivaldi log | less

# Or limit results
ivaldi log --limit 10

# Or use oneline
ivaldi log --oneline
```

### Missing Commits

If expected commits don't appear:

```bash
# Check if you're on the right timeline
ivaldi whereami

# View all timelines
ivaldi log --all

# Or switch to the correct timeline
ivaldi timeline switch <name>
ivaldi log
```

## Advanced Usage

### Custom Formatting with Grep

Extract specific information:

```bash
# Just seal names
ivaldi log | grep "^Seal:" | cut -d' ' -f2

# Just authors
ivaldi log | grep "^Author:" | cut -d' ' -f2-

# Just dates
ivaldi log | grep "^Date:" | cut -d' ' -f2-
```

### Timeline Analysis

Compare activity across timelines:

```bash
#!/bin/bash
for timeline in $(ivaldi timeline list | tail -n +2 | awk '{print $1}'); do
    count=$(ivaldi timeline switch $timeline && ivaldi log --oneline 2>/dev/null | wc -l)
    echo "$timeline: $count commits"
done
```

### Generate Changelog

Create a changelog from commit messages:

```bash
echo "# Changelog" > CHANGELOG.md
echo "" >> CHANGELOG.md
ivaldi log --oneline | while read line; do
    echo "- $line" >> CHANGELOG.md
done
```

## Related Commands

- `ivaldi seals list` - List all seals with detailed information
- `ivaldi seals show <name>` - Show details of a specific seal
- `ivaldi whereami` - Show current position in history
- `ivaldi diff <seal>` - Compare working directory with a seal
- `ivaldi timeline list` - List all timelines
