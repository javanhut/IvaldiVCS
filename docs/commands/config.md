---
layout: default
title: ivaldi config
---

# ivaldi config

View and modify configuration settings.

## Synopsis

```bash
ivaldi config
ivaldi config --list
ivaldi config --set <key> <value>
ivaldi config --get <key>
```

## Description

Manage user and repository configuration settings.

## Options

- (no args) - Interactive configuration
- `--list` - Show all configuration
- `--set <key> <value>` - Set a value
- `--get <key>` - Get a value

## Examples

### Interactive Configuration

```bash
ivaldi config
```

Prompts for:
- User name
- User email
- Other settings

### List Configuration

```bash
ivaldi config --list
```

Output:
```
user.name=Jane Doe
user.email=jane@example.com
color.ui=true
```

### Set Value

```bash
ivaldi config --set user.name "Jane Doe"
ivaldi config --set user.email "jane@example.com"
```

### Get Value

```bash
ivaldi config --get user.name
```

## Configuration Keys

### User Settings

- `user.name` - Your name for commits
- `user.email` - Your email for commits

### UI Settings

- `color.ui` - Enable colored output (true/false)

## Configuration Locations

### User Configuration

`~/.ivaldi/config` - Global settings for all repositories

### Repository Configuration

`.ivaldi/config` - Settings for current repository

Repository settings override user settings.

## First-Time Setup

After installing Ivaldi:

```bash
ivaldi config --set user.name "Your Name"
ivaldi config --set user.email "your.email@example.com"
```

Or use interactive mode:
```bash
ivaldi config
```

## Common Workflows

### Initial Setup

```bash
ivaldi forge
ivaldi config
# Enter name and email
```

### Change Email

```bash
ivaldi config --set user.email "new@example.com"
```

### View Settings

```bash
ivaldi config --list
```

## Related Commands

- [forge](forge.md) - Initialize repository
- [seal](seal.md) - Create commits (uses config)

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git config --global user.name` | `ivaldi config --set user.name` |
| `git config --list` | `ivaldi config --list` |
| `git config user.email` | `ivaldi config --get user.email` |

## Required Settings

Before creating seals, configure:
- `user.name`
- `user.email`

These appear in seal metadata.
