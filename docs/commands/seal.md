---
layout: default
title: ivaldi seal
---

# ivaldi seal

Create a commit (seal) with staged files.

## Synopsis

```bash
ivaldi seal <message>
ivaldi seal -m <message>
```

## Description

The `seal` command creates a commit from the currently staged files. Each seal receives:
- **Unique BLAKE3 hash** - Content identifier
- **Memorable name** - Human-friendly identifier like "swift-eagle-flies-high-447abe9b"
- **Message** - Your commit description
- **Author** - From configuration
- **Timestamp** - When created
- **Parent(s)** - Links to previous seal(s)

## Arguments

- `<message>` - Commit message describing the changes

## Options

- `-m <message>` - Specify message (alternative syntax)

## Examples

### Basic Seal

```bash
ivaldi seal "Add login feature"
```

Output:
```
Created seal: swift-eagle-flies-high-447abe9b (447abe9b)
```

### With -m Flag

```bash
ivaldi seal -m "Fix authentication bug"
```

### Multi-line Message

```bash
ivaldi seal "Add user authentication

Implemented JWT-based authentication with:
- Login endpoint
- Token refresh
- Session management"
```

## Seal Names

Every seal gets a unique memorable name:

```
swift-eagle-flies-high-447abe9b
│     │     │    │    │
│     │     │    │    └─── Short hash (8 chars)
│     │     │    └─────── Adjective
│     │     └────────────── Verb
│     └────────────────────── Adjective
└──────────────────────────── Noun
```

Benefits:
- Easier to remember than hashes
- Unique identifier for each commit
- Can reference by full name, partial name, or hash

## Workflow

Complete seal workflow:

```bash
# 1. Make changes
echo "new code" >> src/main.go

# 2. Check status
ivaldi status

# 3. Stage files
ivaldi gather src/main.go

# 4. Create seal
ivaldi seal "Add new feature"

# 5. Verify
ivaldi log --limit 1
```

## Viewing Seals

### List All Seals

```bash
ivaldi seals list
```

### Show Seal Details

```bash
ivaldi seals show swift-eagle-flies-high-447abe9b
```

### View History

```bash
ivaldi log
ivaldi log --oneline
```

## Common Workflows

### Quick Commit

```bash
ivaldi gather .
ivaldi seal "Quick fix"
```

### Feature Commit

```bash
# Stage related files
ivaldi gather src/auth/ tests/auth/

# Create descriptive seal
ivaldi seal "Add OAuth2 authentication

- Implemented OAuth2 flow
- Added Google and GitHub providers
- Updated user model
- Added integration tests"
```

### WIP Commit

```bash
ivaldi gather src/
ivaldi seal "WIP: Refactoring authentication"
```

## Best Practices

### Write Clear Messages

Good:
```bash
ivaldi seal "Fix null pointer in user login handler"
```

Bad:
```bash
ivaldi seal "fix"
```

### Commit Often

```bash
# Small, focused seals are better
ivaldi seal "Add login endpoint"
ivaldi seal "Add logout endpoint"
ivaldi seal "Add session management"
```

### Use Meaningful Names

The first line should be a concise summary:
```bash
ivaldi seal "Add user authentication feature

Detailed explanation here...
"
```

## Related Commands

- [gather](gather.md) - Stage files before sealing
- [status](status.md) - Check what's staged
- [log](log.md) - View seal history
- [reset](reset.md) - Undo changes before sealing

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git commit -m "msg"` | `ivaldi seal "msg"` |
| Uses SHA-1 hash | Uses BLAKE3 hash |
| Hash only | Hash + memorable name |
| `git commit --amend` | (use `travel` to modify history) |

## Troubleshooting

### Nothing to Seal

```
Error: no changes staged for seal
```

Solution:
```bash
ivaldi gather .
ivaldi seal "Your message"
```

### No Message Provided

```
Error: seal message required
```

Solution:
```bash
ivaldi seal "Add your message here"
```

### Author Not Configured

```
Error: user.name not configured
```

Solution:
```bash
ivaldi config --set user.name "Your Name"
ivaldi config --set user.email "you@example.com"
```
