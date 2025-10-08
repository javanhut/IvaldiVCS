---
layout: default
title: ivaldi harvest
---

# ivaldi harvest

Download specific remote timelines (branches) from GitHub.

## Synopsis

```bash
ivaldi harvest [timeline-names...]
ivaldi harvest --update
```

## Description

Selectively download timelines from GitHub. Unlike Git which downloads everything, harvest lets you choose what you need.

## Options

- `[timeline-names...]` - Specific timelines to download
- `--update` - Update existing timelines and download new ones

## Examples

### Download Specific Timelines

```bash
ivaldi harvest feature-auth bugfix-payment
```

### Download All New Timelines

```bash
ivaldi harvest
```

### Update Existing Timelines

```bash
ivaldi harvest --update
```

## Selective Sync

Ivaldi's killer feature: download only what you need.

```bash
# See what's available
ivaldi scout

# Download only specific branches
ivaldi harvest feature-auth
```

This saves bandwidth and disk space!

## Common Workflows

### Review Teammate's Work

```bash
ivaldi scout
ivaldi harvest feature-auth
ivaldi timeline switch feature-auth
ivaldi log
```

### Sync Multiple Features

```bash
ivaldi scout
ivaldi harvest feature-a feature-b feature-c
```

### Update Everything

```bash
ivaldi harvest --update
```

## Use Cases

### Collaboration

```bash
# Download teammate's branch
ivaldi harvest feature-payment

# Review it
ivaldi timeline switch feature-payment
ivaldi log
ivaldi diff
```

### Selective Work

```bash
# Only download what you need
ivaldi scout
ivaldi harvest feature-auth  # Skip other branches
```

### Sync Repository

```bash
# Get all updates
ivaldi harvest --update
```

## Related Commands

- [scout](scout.md) - Discover remote timelines
- [upload](upload.md) - Push timelines
- [timeline](timeline.md) - Switch to harvested timeline

## Comparison with Git

| Git | Ivaldi |
|-----|--------|
| `git fetch` | `ivaldi harvest` (selective) |
| `git fetch --all` | `ivaldi harvest --update` |
| Downloads everything | Download only what you want |

## Advantages

- **Selective**: Choose specific branches
- **Efficient**: Don't download unnecessary data
- **Bandwidth**: Save network usage
- **Disk Space**: Save local storage

## Troubleshooting

### Timeline Not Found

```
Error: remote timeline 'feature-x' not found
```

Solution:
```bash
ivaldi scout  # Check available timelines
```

### No Portal Configured

```
Error: no GitHub repository configured
```

Solution:
```bash
ivaldi portal add owner/repo
```
