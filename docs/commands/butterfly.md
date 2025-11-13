# Butterfly Command

## Overview

The `butterfly` command (alias: `bf`) creates and manages **butterfly timelines** - experimental sandbox timelines that branch from a parent timeline. Butterflies enable safe experimentation without polluting the parent timeline's history.

## What is a Butterfly Timeline?

A butterfly timeline is a lightweight, experimental branch that:
- Branches from a parent timeline at a specific point
- Tracks its parent and divergence commit
- Can sync changes bidirectionally (up/down)
- Supports nested butterflies (butterfly → butterfly → butterfly)
- Uses automatic conflict resolution via fast-forwarding
- Never blocks or creates conflicts with the parent timeline

## Commands

### Create Butterfly

```bash
ivaldi timeline butterfly <name>
ivaldi tl bf <name>
```

Creates a new butterfly timeline from the current timeline.

**Behavior**:
- Creates butterfly with same workspace state as parent
- Records parent name and divergence point
- Automatically switches to the new butterfly
- Parent timeline remains unchanged

**Example**:
```bash
$ ivaldi tl bf experiment
Creating butterfly timeline 'experiment' from 'main'
Divergence point: swift-eagle-flies
✓ Created butterfly 'experiment'
✓ Switched to butterfly timeline
```

### Sync Up (Merge to Parent)

```bash
ivaldi timeline butterfly up
ivaldi tl bf up
```

Syncs the current butterfly timeline's changes up to its parent. This rebases butterfly commits onto the parent's latest state and merges them back.

**Behavior**:
- Must be run from within a butterfly timeline
- Rebases butterfly's commits on top of parent's latest
- Merges rebased commits to parent timeline
- Updates divergence point to parent's latest commit
- Uses automatic fast-forward conflict resolution

**Example**:
```bash
$ ivaldi tl bf up
Syncing butterfly 'experiment' up to parent 'main'...
✓ Parent 'main' now at: bold-hawk-soars
✓ Butterfly synchronized
```

### Sync Down (Merge from Parent)

```bash
ivaldi timeline butterfly down
ivaldi tl bf down
```

Syncs the parent timeline's changes down into the current butterfly. This merges parent's new commits into the butterfly while keeping butterfly's commits on top.

**Behavior**:
- Must be run from within a butterfly timeline
- Merges parent's new commits into butterfly
- Keeps butterfly's commits layered on top
- Updates divergence point
- Uses automatic fast-forward conflict resolution

**Example**:
```bash
$ ivaldi tl bf down
Syncing butterfly 'experiment' down from parent 'main'...
✓ Merged successfully
✓ Butterfly now includes parent's latest changes
```

### Remove Butterfly

```bash
ivaldi timeline butterfly rm <name> [--cascade]
ivaldi tl bf rm <name> [--cascade]
```

Removes a butterfly timeline.

**Flags**:
- `--cascade`: Delete all nested butterflies recursively

**Behavior (without `--cascade`)**:
- Deletes the butterfly timeline
- Child butterflies become orphaned
- Orphaned butterflies can still be used

**Behavior (with `--cascade`)**:
- Deletes butterfly and all nested butterflies
- Recursively removes entire butterfly tree

**Example**:
```bash
$ ivaldi tl bf rm experiment
Removing butterfly 'experiment'...
Warning: This butterfly has 2 nested butterflies:
  - feature-a
  - feature-b
These will become orphaned. Use --cascade to delete them.
✓ Removed butterfly 'experiment'

$ ivaldi tl bf rm experiment --cascade
✓ Removed butterfly 'experiment' and 2 nested butterflies
```

## Butterfly Information

### Check if Timeline is a Butterfly

Use `ivaldi whereami` (or `ivaldi wai`) to see if the current timeline is a butterfly:

**Standard Timeline**:
```
Timeline: main
Type: Standard
Last Seal: swift-eagle-flies (2 hours ago)
```

**Butterfly Timeline**:
```
Timeline: experiment
Type: Butterfly 🦋
Parent: main
Divergence: swift-eagle-flies
Nested butterflies: 2 (feature-a, feature-b)
Last Seal: bold-hawk-soars (5 minutes ago)
```

**Orphaned Butterfly**:
```
Timeline: experiment
Type: Butterfly 🦋 (Orphaned)
Original parent: main (deleted)
Divergence: swift-eagle-flies
```

### List Butterflies

Use `ivaldi timeline list` (or `ivaldi tl ls`) to see all timelines with butterfly indicators:

```
Local Timelines:
* main              Created timeline 'main'
  experiment 🦋      Butterfly from 'main'
  feature-a 🦋       Butterfly from 'experiment'
  production        Created timeline 'production'
```

The 🦋 indicator shows which timelines are butterflies.

## Workflow Examples

### Basic Experimentation

```bash
# Create butterfly for experimentation
$ ivaldi tl bf test-feature
✓ Created butterfly 'test-feature'

# Make experimental changes
$ echo "new code" > feature.go
$ ivaldi gather feature.go
$ ivaldi seal "Experimental feature implementation"

# Test it out...
# If it works, merge back to parent
$ ivaldi tl bf up
✓ Parent 'main' now has your changes

# Clean up
$ ivaldi tl sw main
$ ivaldi tl bf rm test-feature
```

### Nested Butterflies

```bash
# Main development
$ ivaldi tl bf develop
✓ Created butterfly 'develop' from 'main'

# Create feature butterfly off develop
$ ivaldi tl bf feature-login
✓ Created butterfly 'feature-login' from 'develop'

# Work on feature...
$ ivaldi seal "Add login form"

# Merge feature to develop
$ ivaldi tl bf up
✓ Parent 'develop' updated

# Switch to develop and test
$ ivaldi tl sw develop

# Merge develop to main
$ ivaldi tl bf up
✓ Parent 'main' updated
```

### Sync with Parent Updates

```bash
# You're working on a butterfly
$ ivaldi tl bf experiment

# Meanwhile, main gets important updates
# Pull parent changes down
$ ivaldi tl bf down
✓ Merged parent changes

# Continue working with latest parent code
$ ivaldi seal "Updated with main changes"

# When ready, push changes up
$ ivaldi tl bf up
```

## Automatic Conflict Resolution

Butterfly uses **fast-forward merge strategy** for automatic conflict resolution:

### Resolution Rules

| Scenario | Resolution |
|----------|-----------|
| Both added same file | Layer: Keep theirs on top of ours |
| Both modified same file | Layer: Apply changes on top of each other |
| Deleted vs Modified | Keep modified version |
| Added in one | Keep added file |

### Example

**Base**:
```
file.txt:
line 1
line 2
line 3
```

**Butterfly changes**:
```
line 1 modified by butterfly
line 2
line 3
```

**Parent changes**:
```
line 1
line 2
line 3 modified by parent
```

**Merged result** (layered):
```
line 1 modified by butterfly
line 2
line 3 modified by parent
```

## Best Practices

1. **Create butterflies for experiments**: Don't pollute main timelines with experimental code

2. **Sync regularly**: Use `tl bf down` to keep your butterfly updated with parent changes

3. **Clean up finished butterflies**: Remove butterflies after merging to avoid clutter

4. **Use nested butterflies for sub-features**: Create butterfly chains for complex development

5. **Check `whereami`before syncing**: Make sure you're on the right butterfly

6. **Name butterflies descriptively**: Use names like `test-feature`, `experiment-perf`, `try-new-api`

## Technical Details

### Storage

Butterflies are stored as:
- Regular timeline in `.ivaldi/refs/heads/`
- Metadata in `.ivaldi/butterflies/metadata.db` (BoltDB)
  - Parent name and divergence point
  - Orphaned status
  - Nested butterfly relationships

### Divergence Tracking

Each butterfly tracks:
- **Divergence Hash**: Commit where butterfly was created
- **Parent Name**: Timeline it branched from
- **Created At**: Timestamp of creation

### Orphaned Butterflies

When a parent butterfly is deleted:
- Child butterflies become "orphaned"
- They can still be used normally
- `whereami` shows orphaned status
- Original parent name is preserved

## Limitations

1. **No manual conflict resolution**: Conflicts are automatically resolved via fast-forward
2. **Parent must exist**: Cannot sync orphaned butterflies
3. **Single parent**: Each butterfly has exactly one parent
4. **No re-parenting**: Cannot change a butterfly's parent after creation (yet)

## See Also

- [Timeline Commands](timeline.md) - General timeline management
- [Seal Commands](seal.md) - Creating commits
- [Fuse Command](fuse.md) - Merging timelines (traditional merge)
- [Whereami Command](whereami.md) - Check current timeline status
