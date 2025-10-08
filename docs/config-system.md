# Configuration System

Ivaldi VCS provides a flexible configuration system similar to Git, allowing you to customize your version control experience at both global and repository levels.

## Overview

The configuration system supports:
- **Two-level hierarchy**: Global (user-wide) and local (repository-specific) settings
- **Interactive mode**: Easy setup with guided prompts
- **Command-line mode**: Direct configuration for scripting
- **Precedence**: Local settings override global settings

## Configuration Files

### Global Configuration
- **Location**: `~/.ivaldiconfig`
- **Scope**: Applies to all Ivaldi repositories on your system
- **Use case**: User identity, editor preferences, default behaviors

### Local Configuration
- **Location**: `.ivaldi/config` (in repository root)
- **Scope**: Applies only to the current repository
- **Use case**: Repository-specific settings, project overrides

## Configuration Options

### User Settings

#### user.name
Your name for commit authorship.

```bash
ivaldi config user.name "Jane Smith"
```

#### user.email
Your email address for commit authorship.

```bash
ivaldi config user.email "jane@example.com"
```

### Core Settings

#### core.editor
Default text editor for commit messages and conflict resolution.

```bash
ivaldi config core.editor "vim"
ivaldi config core.editor "code --wait"
ivaldi config core.editor "nano"
```

#### core.pager
Program to use for paginating long output.

```bash
ivaldi config core.pager "less"
ivaldi config core.pager "more"
```

#### core.autoshelf
Automatically preserve uncommitted changes when switching timelines.

```bash
ivaldi config core.autoshelf true    # Default
ivaldi config core.autoshelf false
```

### Color Settings

#### color.ui
Enable or disable colored output globally.

```bash
ivaldi config color.ui true     # Default
ivaldi config color.ui false
```

#### color.status
Enable colored output for status command.

```bash
ivaldi config color.status true    # Default
```

#### color.diff
Enable colored output for diff command.

```bash
ivaldi config color.diff true      # Default
```

## Usage

### Interactive Configuration

Run without arguments for guided setup:

```bash
ivaldi config
```

**Example session:**
```
Interactive Configuration

Username (johnsmith)> Jane Smith
Email (john@example.com)> jane@example.com
Scope (global/local) [global]> global

Config saved!

  Scope: global
  Username: Jane Smith
  Email: jane@example.com
```

**Features:**
- Shows current values as defaults
- Press Enter to keep existing value
- Choose global or local scope
- Validates input before saving

### Setting Values

Set a configuration value:

```bash
# Local (repository-specific)
ivaldi config user.name "Jane Smith"

# Global (all repositories)
ivaldi config --global user.name "Jane Smith"
```

### Getting Values

Retrieve a single configuration value:

```bash
ivaldi config user.name
```

**Output:**
```
Jane Smith
```

### Listing All Configuration

View all current settings:

```bash
ivaldi config --list
```

**Example output:**
```
User Configuration:
  user.name = Jane Smith
  user.email = jane@example.com

Core Configuration:
  core.editor = vim
  core.pager = less
  core.autoshelf = true

Color Configuration:
  color.ui = true
  color.status = true
  color.diff = true
```

## Command Reference

```bash
# Interactive mode
ivaldi config

# Set local value
ivaldi config <key> <value>

# Set global value
ivaldi config --global <key> <value>

# Get value
ivaldi config <key>

# List all settings
ivaldi config --list
```

## Common Workflows

### First-Time Setup

When using Ivaldi for the first time:

```bash
# Configure your identity
ivaldi config --global user.name "Your Name"
ivaldi config --global user.email "you@example.com"

# Set your preferred editor
ivaldi config --global core.editor "vim"
```

### Repository-Specific Identity

Use a different identity for a specific project:

```bash
cd work-project
ivaldi config user.name "Jane Smith (Work)"
ivaldi config user.email "jane.smith@company.com"
```

### Team Configuration

Ensure consistent settings across a team:

```bash
# Each team member runs:
ivaldi config --global core.autoshelf true
ivaldi config --global color.ui true
```

### Troubleshooting Setup

Check if configuration is properly set:

```bash
ivaldi config --list

# If user settings are missing:
ivaldi config user.name "Your Name"
ivaldi config user.email "you@example.com"
```

## Configuration Precedence

When both global and local configurations exist, Ivaldi follows this order:

1. **Local** (`.ivaldi/config`) - highest priority
2. **Global** (`~/.ivaldiconfig`) - fallback
3. **Defaults** - built-in defaults

**Example:**
```bash
# Global setting
ivaldi config --global user.name "Jane Smith"

# Local override
cd my-project
ivaldi config user.name "Jane S. (Project)"

# This repository uses: Jane S. (Project)
# Other repositories use: Jane Smith
```

## Required Configuration

Before creating commits, you must configure:

```bash
ivaldi config user.name "Your Name"
ivaldi config user.email "you@example.com"
```

**Without configuration, commits will fail:**
```
Error: user.name and user.email not configured
Please set user.name and user.email: ivaldi config user.name "Your Name"
```

## File Format

Configuration files use a simple key-value format:

```ini
[user]
name = Jane Smith
email = jane@example.com

[core]
editor = vim
pager = less
autoshelf = true

[color]
ui = true
status = true
diff = true
```

## Best Practices

### 1. Set Global Defaults
Configure your identity globally for all repositories:
```bash
ivaldi config --global user.name "Your Name"
ivaldi config --global user.email "you@example.com"
```

### 2. Use Local Overrides Sparingly
Only override locally when necessary for specific projects.

### 3. Keep It Simple
Start with minimal configuration and add settings as needed.

### 4. Document Team Settings
If working in a team, document required configuration in your project README.

### 5. Validate Configuration
Before important operations, verify your settings:
```bash
ivaldi config --list
```

## Comparison with Git

| Feature | Git | Ivaldi |
|---------|-----|--------|
| Global config | `git config --global` | `ivaldi config --global` |
| Local config | `git config --local` | `ivaldi config` (default) |
| List settings | `git config --list` | `ivaldi config --list` |
| Interactive | Not available | `ivaldi config` (no args) |
| Get value | `git config user.name` | `ivaldi config user.name` |
| Set value | `git config user.name "Name"` | `ivaldi config user.name "Name"` |

## Troubleshooting

### Configuration Not Taking Effect

Check configuration precedence:
```bash
ivaldi config --list
```

Verify which file is being used (local overrides global).

### Cannot Create Commits

Ensure user identity is configured:
```bash
ivaldi config user.name
ivaldi config user.email

# If empty, set them:
ivaldi config user.name "Your Name"
ivaldi config user.email "you@example.com"
```

### Invalid Configuration File

If the configuration file is corrupted:
```bash
# Remove and recreate
rm ~/.ivaldiconfig
ivaldi config --global user.name "Your Name"
ivaldi config --global user.email "you@example.com"
```

### Global vs Local Confusion

To see where a value comes from, check both:
```bash
cat ~/.ivaldiconfig        # Global
cat .ivaldi/config         # Local
```

## Advanced Topics

### Environment Variables

Some settings can be overridden with environment variables:
```bash
export IVALDI_EDITOR=vim
ivaldi seal  # Uses vim regardless of config
```

### Scripting with Config

Use in scripts to validate setup:
```bash
#!/bin/bash
if ! ivaldi config user.name > /dev/null 2>&1; then
    echo "Please configure user.name"
    exit 1
fi
```

### Multiple Identities

Manage multiple identities with shell aliases:
```bash
# In ~/.bashrc or ~/.zshrc
alias ivaldi-work='ivaldi config user.email "work@company.com"'
alias ivaldi-personal='ivaldi config user.email "personal@example.com"'
```

## Related Commands

- `ivaldi forge` - Initialize repository (creates local config)
- `ivaldi seal` - Create commit (uses configured author)
- `ivaldi whereami` - Show current status (displays configured user)
