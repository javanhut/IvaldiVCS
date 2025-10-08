# Ivaldi Travel Command

The `ivaldi travel` command provides interactive time-travel functionality, allowing you to browse through your commit history and either diverge into a new timeline or overwrite your current timeline from any previous seal.

## Overview

Time travel in Ivaldi lets you:
- **Browse commit history** interactively
- **Diverge into new timelines** - Create a new branch from any past seal (non-destructive)
- **Overwrite timeline history** - Reset current timeline to a past seal (destructive)

This is similar to Git's `git log` combined with `git checkout` and `git reset --hard`, but with an interactive interface.

## Basic Usage

```bash
# Start interactive time travel (shows 20 most recent seals)
ivaldi travel

# Show all seals without pagination
ivaldi travel --all

# Show only 10 most recent seals
ivaldi travel --limit 10

# Search for seals containing "bug fix"
ivaldi travel --search "bug fix"
```

This opens an interactive browser showing seals in the current timeline.

## Interactive Seal Browser

When you run `ivaldi travel`, you'll see a list of seals:

### Interactive Navigation with Arrow Keys

The travel command features a modern, cursor-based interface:

```
Seals in timeline 'main':

> 1. fierce-gate-builds-quick-b6f8f458 (b6f8f458)
     should have major improvements
     John Doe <john@example.com> • 2025-10-07 14:32:10

  2. empty-phoenix-attacks-fresh-7bb05886 (7bb05886)
     pushing with repo
     John Doe <john@example.com> • 2025-10-07 14:30:45

  3. swift-eagle-flies-high-447abe9b (447abe9b)
     Add new feature
     John Doe <john@example.com> • 2025-10-07 14:28:22

Up/Down arrows navigate • Enter to select • q to quit
```

**The currently selected seal is highlighted with bold text** and a green arrow (>).

### With Pagination (>20 seals)

When there are many commits, pagination automatically activates:

```
Seals in timeline 'main' (showing 1-20 of 156):

> 1. fierce-gate-builds-quick-b6f8f458 (b6f8f458)
     should have major improvements
     John Doe <john@example.com> • 2025-10-07 14:32:10

  2. empty-phoenix-attacks-fresh-7bb05886 (7bb05886)
     pushing with repo
     John Doe <john@example.com> • 2025-10-07 14:30:45

  ...

  20. ancient-seal-name-12345678 (12345678)
      Initial commit
      Jane Doe <jane@example.com> • 2025-09-01 10:00:00

Page 1 of 8
Up/Down arrows navigate • Enter to select • n/p page • q to quit
```

**Navigation:**
- **Up/Down Arrow Keys** - Move cursor up/down through seals
- **Enter** - Select the highlighted seal
- **n/p** - Jump to next/previous page (pagination mode)
- **Numbers** - Type a number to jump directly to that seal
- **q** or **ESC** - Cancel and exit

**Advanced:**
- Arrow keys automatically handle page boundaries (moving down on last item goes to next page)
- The screen clears and refreshes as you navigate for a clean experience
- The current HEAD seal is marked with a dimmed > when not selected

## Using Arrow Keys

The travel command provides an intuitive cursor-based interface:

1. **Launch** - Run `ivaldi travel`
2. **Navigate** - Use Up/Down arrow keys to move through seals
3. **Select** - Press Enter when you find the seal you want
4. **Choose Action** - Decide to diverge or overwrite

The selected seal is highlighted in bold with a green arrow, making it easy to see where you are in history.

## Diverging into a New Timeline

When you select a past seal, you can create a new timeline branching from that point:

```bash
ivaldi travel
> 3  # Select seal #3

# Travel shows you what you selected:
Selected seal: swift-eagle-flies-high-447abe9b
  Position: 2 commits behind current HEAD
  Message: Add new feature

# Choose action:
? What would you like to do?

  1. Diverge - Create new timeline from this seal (keeps current timeline intact)
  2. Overwrite - Overwrite current timeline (removes all commits after this seal)
  3. Cancel

Choice (1/2/3): 1

Enter new timeline name: feature-experiment

Created new timeline 'feature-experiment' from seal swift-eagle-flies-high-447abe9b
Switched to timeline 'feature-experiment'
Workspace materialized to seal: swift-eagle-flies-high-447abe9b
```

### What Happens During Divergence

1. Creates a new timeline pointing to the selected seal
2. Switches to the new timeline
3. Materializes workspace to match the selected seal's state
4. Preserves the original timeline unchanged

This is **non-destructive** - your original timeline remains intact with all its commits.

## Overwriting Timeline History

If you want to permanently remove commits from your current timeline:

```bash
ivaldi travel
> 3  # Select seal #3

? What would you like to do?

  1. Diverge - Create new timeline from this seal (keeps current timeline intact)
  2. Overwrite - Overwrite current timeline (removes all commits after this seal)
  3. Cancel

Choice (1/2/3): 2

WARNING: This will permanently remove 2 commit(s) from 'main'.
Are you sure? Type 'yes' to confirm: yes

Timeline 'main' reset to seal swift-eagle-flies-high-447abe9b
2 commit(s) removed from timeline
Workspace materialized to seal: swift-eagle-flies-high-447abe9b
```

### What Happens During Overwrite

1. Updates the current timeline to point to the selected seal
2. Removes all commits after that seal (they become orphaned)
3. Materializes workspace to match the selected seal's state
4. **Destructive** - commits after the selected seal are lost

**WARNING**: This is a destructive operation! Commits removed this way are not easily recoverable unless they've been pushed to a remote or are referenced by another timeline.

## Command Flags

### --limit, -n

Control how many seals to display:

```bash
# Show only 10 most recent seals
ivaldi travel --limit 10
ivaldi travel -n 10

# Show 50 seals
ivaldi travel -n 50
```

**Default**: 20 seals per page

### --all, -a

Show all seals without pagination:

```bash
# Display all seals (no pagination)
ivaldi travel --all
ivaldi travel -a
```

**Warning**: Use with caution on repositories with hundreds of commits.

### --search, -s

Search for seals by message, author, or seal name:

```bash
# Find seals containing "bug" in message
ivaldi travel --search "bug"
ivaldi travel -s "bug"

# Find seals by author
ivaldi travel --search "john@example.com"

# Find specific seal name
ivaldi travel --search "swift-eagle"
```

**Search is case-insensitive** and matches partial text.

### Combining Flags

```bash
# Search and show only 5 results
ivaldi travel --search "feature" --limit 5

# Search and show all results
ivaldi travel --search "refactor" --all
```

## Workflow Examples

### Example 1: Experimenting with Different Approaches

You want to try a different implementation approach without losing your current work:

```bash
# Current state: main timeline with 5 commits
ivaldi whereami
# Timeline: main
# Last Seal: newest-commit-12345678

# Travel to commit 3 to try different approach
ivaldi travel
> 3

# Diverge into experimental timeline
Choice: 1
Enter new timeline name: experimental-approach

# Now you have two timelines:
# - main (with all 5 commits)
# - experimental-approach (starting from commit 3)

# Work on experimental approach
ivaldi gather src/new-implementation.js
ivaldi seal "Try different algorithm"

# If experiment works, you can merge or upload it
ivaldi upload  # Creates separate branch on GitHub

# If experiment fails, switch back to main
ivaldi timeline switch main
# Your original work is untouched!
```

### Example 2: Fixing a Bug in Older Code

You discover a bug was introduced in commit 4, and you want to fix it:

```bash
# Travel to commit 3 (before the bug)
ivaldi travel
> 3

# Create fix timeline
Choice: 1
Enter new timeline name: bugfix-issue-42

# Fix the bug
ivaldi gather src/buggy-file.js
ivaldi seal "Fix issue #42"

# Upload as separate branch
ivaldi upload
```

### Example 3: Undoing Bad Commits

You made some commits that you want to completely undo:

```bash
# You have 3 bad commits you want to remove
ivaldi travel
> 4  # Go back before the bad commits

# Overwrite timeline to remove bad commits
Choice: 2
Are you sure? Type 'yes' to confirm: yes

# Timeline reset - bad commits are gone
# Continue working from this point
ivaldi gather src/correct-changes.js
ivaldi seal "Correct implementation"
```

### Example 4: Creating Multiple Feature Branches from One Point

You want to try multiple different features from the same starting point:

```bash
# Travel to stable commit
ivaldi travel
> 5

# Create first feature branch
Choice: 1
Enter new timeline name: feature-a
ivaldi gather src/feature-a.js
ivaldi seal "Implement feature A"

# Switch back to original timeline
ivaldi timeline switch main

# Travel again to same point
ivaldi travel
> 5

# Create second feature branch
Choice: 1
Enter new timeline name: feature-b
ivaldi gather src/feature-b.js
ivaldi seal "Implement feature B"

# Now you have:
# - main (original timeline)
# - feature-a (diverged from commit 5)
# - feature-b (diverged from commit 5)
```

### Example 5: Finding a Specific Commit in Large History

You have 200 commits and need to find where a feature was added:

```bash
# Search for commits mentioning "authentication"
ivaldi travel --search "authentication"

# Shows only matching seals:
Seals in timeline 'main' (3 results):

  1. seal-name-123 (12345678)
     Add authentication system
     ...

  2. seal-name-456 (45678901)
     Fix authentication bug
     ...

  3. seal-name-789 (78901234)
     Refactor authentication
     ...

# Select the one you want
> 1
```

### Example 6: Navigating Large History with Pagination

You have 150 commits and want to go back far in history:

```bash
# Start travel (shows page 1: commits 1-20)
ivaldi travel

Page 1 of 8
> n  # Next page

# Now showing commits 21-40
Page 2 of 8
> n  # Next page

# Now showing commits 41-60
Page 3 of 8
> 50  # Select commit 50

# Travel to that commit
```

## Safety Features

### 1. Confirmation for Destructive Operations

Overwriting requires explicit confirmation:
```
WARNING: This will permanently remove 2 commit(s) from 'main'.
Are you sure? Type 'yes' to confirm:
```

Only typing exactly `yes` will proceed with the overwrite.

### 2. Clear Visual Feedback

The command shows:
- How many commits will be affected
- Which seal you're traveling to
- Whether the operation is destructive or non-destructive

### 3. Timeline Preservation

When diverging, the original timeline is completely preserved, allowing you to return if needed.

## Integration with Other Commands

### After Traveling

Once you've traveled to a seal, you can use all normal Ivaldi commands:

```bash
# Check where you are
ivaldi whereami

# View files at this point in history
ivaldi status

# Make changes
ivaldi gather newfile.txt
ivaldi seal "New work from old seal"

# Upload to GitHub
ivaldi upload
```

### Combining with Timeline Commands

```bash
# Travel and diverge
ivaldi travel
# ... select seal and diverge to 'experiment'

# List all timelines
ivaldi timeline list
# Shows: main, experiment

# Switch between them
ivaldi timeline switch main
ivaldi timeline switch experiment
```

## Best Practices

### 1. **Diverge for Experiments**
When trying something new, always diverge rather than overwrite:
```bash
# Good: Non-destructive
ivaldi travel -> diverge -> new timeline

# Risky: Destructive
ivaldi travel -> overwrite
```

### 2. **Commit Before Traveling**
Commit or stash your work before traveling:
```bash
# Before traveling
ivaldi gather .
ivaldi seal "WIP before time travel"

# Then travel
ivaldi travel
```

### 3. **Push Before Overwriting**
If you have commits you might want later, push them first:
```bash
# Push current state
ivaldi upload

# Then safe to overwrite locally
ivaldi travel -> overwrite
```

### 4. **Use Descriptive Timeline Names**
When diverging, use clear names:
```bash
# Good
experiment-new-algorithm
bugfix-memory-leak
feature-user-authentication

# Bad
test
new
temp
```

### 5. **Clean Up Old Timelines**
Remove timelines you no longer need:
```bash
ivaldi timeline remove old-experiment
```

### 6. **Use Search for Large Histories**
When you have many commits, use search to find what you need:
```bash
# Instead of scrolling through 100+ commits
ivaldi travel --search "the thing I'm looking for"
```

### 7. **Adjust Limit for Your Needs**
Set the display limit based on your workflow:
```bash
# Quick navigation in active projects
ivaldi travel -n 5

# Thorough review
ivaldi travel -n 50

# See everything
ivaldi travel --all
```

## Comparison with Git

| Operation | Git | Ivaldi |
|-----------|-----|--------|
| View history | `git log` | `ivaldi travel` (interactive) |
| Create branch from past | `git checkout -b new-branch <commit>` | `ivaldi travel` -> diverge |
| Reset to past commit | `git reset --hard <commit>` | `ivaldi travel` -> overwrite |
| Interactive mode | `git log` (view only) | `ivaldi travel` (interactive actions) |
| Navigation | Scroll terminal output | Arrow keys with cursor |

## Advantages Over Git

1. **Interactive Interface**: Browse and select commits visually with arrow keys
2. **Real-time Cursor**: See exactly which commit you're selecting
3. **Clear Options**: Explicit choice between diverge and overwrite
4. **Safety Prompts**: Confirmation required for destructive operations
5. **Human-Friendly Names**: Seals have memorable names instead of just hashes
6. **Integrated Workspace**: Automatically materializes workspace to selected state
7. **Smart Pagination**: Handles large histories gracefully with automatic page navigation

## Technical Details

### How Divergence Works

1. Creates new timeline reference pointing to selected seal's commit hash
2. Updates working directory to match that commit's tree
3. Switches current timeline to the new one
4. Original timeline remains at its HEAD

### How Overwrite Works

1. Updates current timeline reference to point to selected seal's commit hash
2. Orphans commits after the selected seal (they still exist in CAS but are unreferenced)
3. Updates working directory to match the selected seal's tree
4. Timeline history is truncated to the selected point

### Seal Traversal

The travel command walks backward through commits using parent references:
```
HEAD (seal 1) -> parent -> seal 2 -> parent -> seal 3 -> parent -> ...
```

Each seal in the list shows its position relative to HEAD (0 = current, 1 = one commit back, etc.).

## Troubleshooting

### "Timeline has no commits yet"

```bash
Error: timeline has no commits yet
```

**Solution**: Create at least one commit before using travel:
```bash
ivaldi gather .
ivaldi seal "Initial commit"
ivaldi travel
```

### "No seals found in timeline"

This means the timeline has no commit history.

**Solution**: Ensure you're on a timeline with commits:
```bash
ivaldi whereami  # Check current timeline
ivaldi timeline list  # See all timelines
```

### "No seals found matching 'search-term'"

Your search didn't find any matching seals.

**Solution**: Try a different search term or broaden your search:
```bash
# Try partial matches
ivaldi travel --search "bug"    # Instead of "bug fix issue #123"

# Search by author
ivaldi travel --search "john"

# Check if you're on the right timeline
ivaldi whereami
```

### "Timeline 'name' already exists"

When diverging, you tried to use a timeline name that already exists.

**Solution**: Choose a different name or remove the existing timeline:
```bash
ivaldi timeline remove existing-name
# Then try again with that name
```

### Selection Out of Range

```bash
Error: invalid selection. Please enter a number between 1 and 156
```

**Solution**: Enter a valid seal number from the total range, even if it's on a different page.

### Too Many Seals to Navigate

If you have hundreds of commits and pagination is cumbersome:

**Solution**: Use search to narrow down:
```bash
# Find what you're looking for
ivaldi travel --search "keyword"

# Or use ivaldi log to see history first
ivaldi log --limit 50
# Then use travel with specific limit
ivaldi travel -n 30
```

## Performance Notes

### Large Repositories

The travel command is optimized for repositories with many commits:

- **Lazy loading**: Only displays seals as needed
- **Pagination**: Default 20 seals per page prevents overwhelming output
- **Search indexing**: Filters seals efficiently without loading full content
- **Memory efficient**: Processes commit history in streaming fashion

### Recommendations by Repository Size

| Commits | Recommended Command |
|---------|-------------------|
| < 20 | `ivaldi travel` (default) |
| 20-100 | `ivaldi travel` (paginated) |
| 100-500 | `ivaldi travel --search <term>` or `ivaldi travel -n 10` |
| 500+ | `ivaldi travel --search <term>` (use search) |

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Up Arrow | Move cursor up |
| Down Arrow | Move cursor down |
| Enter | Select highlighted seal |
| n | Next page (when paginated) |
| p | Previous page (when paginated) |
| q | Quit/cancel |
| ESC | Quit/cancel |
| 1-9 | Jump directly to seal number |

**Pro Tips:**
- Hold Up/Down arrows to quickly scroll through history
- At page boundaries, Up/Down arrows automatically move to the next/previous page
- You can type numbers even while navigating with arrows to jump directly

## Future Enhancements

Potential improvements being considered:

- [DONE] Arrow key navigation for seal selection (Implemented!)
- Preview of changes when selecting a seal
- `--preview` flag to see what would change without applying
- `--force` flag to skip confirmations (for scripts)
- Time-based selection (e.g., "travel to 3 days ago")
- Regex search through commit messages
- Restore orphaned commits feature
- Date range filtering (`--since`, `--until`)
- Author filtering (`--author`)
- Interactive search (type to filter live)
- Mouse support for clicking seals

## Related Commands

- `ivaldi whereami` - Show current position
- `ivaldi seals list` - List all seals
- `ivaldi timeline create` - Create new timeline from current HEAD
- `ivaldi timeline switch` - Switch between timelines
- `ivaldi log` - View commit history (non-interactive)
