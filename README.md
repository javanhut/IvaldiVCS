# Ivaldi VCS

A modern version control system designed as a Git alternative with enhanced features like timeline-based branching, content-addressable storage, and seamless GitHub integration.

## Features

- **Timeline-Based Branching**: Intuitive branch management with auto-shelving
- **Modern Cryptography**: BLAKE3 hashing for security and performance
- **Content-Addressable Storage**: Efficient deduplication and storage
- **GitHub Integration**: Seamless clone, push, and pull operations
- **Auto-Shelving**: Never lose work when switching branches
- **Selective Sync**: Download only the branches you need
- **Merkle Mountain Range**: Append-only commit history with cryptographic proofs

## Quick Start

### Installation

```bash
# Clone and build
git clone https://github.com/javanhut/IvaldiVCS
cd IvaldiVCS
go build -o ivaldi .

# Add to PATH or use ./ivaldi directly
```

### Basic Usage

```bash
# Initialize repository
ivaldi forge

# Check status
ivaldi status

# Stage files
ivaldi gather file1.txt file2.js

# Create commit (generates memorable seal name)
ivaldi seal "Add new features"
# Output: Created seal: swift-eagle-flies-high-447abe9b (447abe9b)

# Check current position
ivaldi whereami
# Output: Last Seal: swift-eagle-flies-high-447abe9b (just now)

# List all seals
ivaldi seals list

# Create and switch timeline (branch)
ivaldi timeline create feature-auth
ivaldi timeline switch main
```

### GitHub Integration

```bash
# Connect to GitHub repository
ivaldi portal add owner/repo

# Clone from GitHub
ivaldi download owner/awesome-project

# Discover remote branches
ivaldi scout

# Download specific branches
ivaldi harvest feature-auth bugfix-db

# Push changes
ivaldi upload
```

## Core Concepts

### Timelines
Timelines are Ivaldi's equivalent to Git branches, but with enhanced features:
- **Auto-shelving**: Uncommitted changes are automatically preserved when switching
- **Workspace isolation**: Each timeline maintains its own workspace state
- **Efficient storage**: Shared content between timelines via content-addressable storage

### Gather and Seal
Ivaldi uses intuitive command names with enhanced user experience:
- `gather`: Stage files (like `git add`)
- `seal`: Create commit with auto-generated human-friendly names (like `git commit`)
- `seals`: Manage seals with memorable names like "swift-eagle-flies-high-447abe9b"

### Scout and Harvest
Remote operations are designed for selective collaboration:
- `scout`: Discover available remote branches
- `harvest`: Download only the branches you need

## Documentation

- **[Documentation Wiki](https://javanhut.github.io/IvaldiVCS)**

### Getting Started
- **[Usage Guide](docs/usage-guide.md)**: Comprehensive guide to using Ivaldi VCS
- **[Configuration System](docs/config-system.md)**: User and repository configuration

### Core Features
- **[Architecture](docs/architecture.md)**: Technical details about how Ivaldi works
- **[Seal Names](docs/seal-names.md)**: Human-friendly commit naming system
- **[Timeline Branching](docs/timeline-branching.md)**: Advanced branching features

### Commands
- **[Config Command](docs/config-system.md)**: Configure user settings and preferences
- **[Log Command](docs/log-command.md)**: View commit history
- **[Diff Command](docs/diff-command.md)**: Compare changes between commits
- **[Reset Command](docs/reset-command.md)**: Unstage files and reset changes
- **[Fuse Command](docs/fuse-command.md)**: Merge timelines together
- **[Whereami Command](docs/whereami-command.md)**: Current timeline information

### Remote Operations
- **[Scout & Harvest](docs/scout-harvest.md)**: Remote timeline operations
- **[GitHub Integration](docs/github-integration.md)**: GitHub synchronization
- **[Portal Commands](docs/portal-commands.md)**: Repository connection management

## Command Reference

### Repository Management
```bash
ivaldi forge                    # Initialize repository
ivaldi status                   # Show working directory status
ivaldi whereami                 # Show current timeline details (alias: wai)
ivaldi config                   # Configure user settings (interactive)
ivaldi config --list            # List all configuration
```

### File Operations
```bash
ivaldi gather [files...]        # Stage files for commit
ivaldi seal <message>           # Create commit with staged files (generates unique seal name)
ivaldi seals list               # List all seals with their generated names
ivaldi seals show <name|hash>   # Show detailed information about a seal
ivaldi reset [files...]         # Unstage files
ivaldi reset --hard             # Discard all uncommitted changes
```

### History and Comparison
```bash
ivaldi log                      # Show commit history
ivaldi log --oneline            # Concise one-line format
ivaldi log --limit 10           # Show only last 10 commits
ivaldi diff                     # Show working directory changes
ivaldi diff --staged            # Show staged changes
ivaldi diff <seal>              # Compare with specific commit
ivaldi diff --stat              # Show summary statistics
```

### Timeline Management
```bash
ivaldi timeline create <name>   # Create new timeline
ivaldi timeline switch <name>   # Switch to timeline
ivaldi timeline list           # List all timelines
ivaldi timeline remove <name>   # Delete timeline
ivaldi fuse <source> to <target> # Merge timelines
ivaldi fuse --continue          # Continue merge after resolving conflicts
ivaldi fuse --abort             # Abort current merge
```

### Remote Operations
```bash
ivaldi portal add <owner/repo>  # Add GitHub connection
ivaldi portal list             # List connections
ivaldi download <url> [dir]    # Clone repository
ivaldi scout                   # Discover remote timelines
ivaldi harvest [names...]      # Download timelines
ivaldi upload                  # Push to GitHub
```

## Architecture Highlights

### Content-Addressable Storage (CAS)
- All content stored using BLAKE3 hashing
- Automatic deduplication across the entire repository
- Efficient storage of large files through chunking

### Merkle Mountain Range (MMR)
- Append-only commit history with cryptographic proofs
- Efficient verification of commit inclusion
- Persistent storage using BoltDB

### HAMT Directory Trees
- Hash Array Mapped Trie for efficient directory management
- Structural sharing between timelines
- O(log n) updates and lookups

### Workspace Management
- Intelligent file materialization when switching timelines
- Auto-shelving preserves uncommitted changes
- Minimal file system operations during switches

## Migrating from Git

Here's how to translate a common Git workflow to Ivaldi:

```bash
# Git commands → Ivaldi commands

git init                                                             → ivaldi forge
git add README.md                                                    → ivaldi gather README.md
git commit -m "first commit"                                         → ivaldi seal -m "first commit"
git branch -M main                                                   → (not needed - main is default)
git remote add origin https://github.com/javanhut/TestRepoIvaldi.git → ivaldi portal add origin https://github.com/javanhut/TestRepoIvaldi.git
git push -u origin main                                              → ivaldi upload
```

```bash
#Initialize empty repository

ivaldi forge
ivaldi gather README.md
ivaldi seal "first commit"
ivaldi portal add javanhut/TestRepoIvaldi
ivaldi upload
```

#### Differences Ivaldi has: 
Ivaldi sets main an default branch on initialization
Gather can be use specifically or empty for all file added
Sealing is like sealing a letter you have the message no need to specific it is.
Portals don't need a https://github.com/owner/Repo.git the owner and repo are enough.
You upload the code to github with upload keyword not a push.

## Comparison with Git

| Feature | Git | Ivaldi |
|---------|-----|--------|
| Initialize | `git init` | `ivaldi forge` |
| Configure | `git config` | `ivaldi config` |
| Stage files | `git add` | `ivaldi gather` |
| Unstage | `git reset` | `ivaldi reset` |
| Commit | `git commit` | `ivaldi seal` |
| Log | `git log` | `ivaldi log` |
| Diff | `git diff` | `ivaldi diff` |
| Branch | `git branch` | `ivaldi timeline create` |
| Switch branch | `git checkout` | `ivaldi timeline switch` |
| Merge | `git merge` | `ivaldi fuse` |
| Clone | `git clone` | `ivaldi download` |
| Push | `git push` | `ivaldi upload` |
| Fetch | `git fetch` | `ivaldi harvest` |

## Advantages over Git

1. **Intuitive Commands**: Clear, descriptive command names
2. **Human-Friendly Seals**: Commits get memorable names like "swift-eagle-flies-high-447abe9b"
3. **Auto-Shelving**: Never lose work when switching branches
4. **Selective Sync**: Download only branches you need
5. **Modern Hashing**: BLAKE3 for better security and performance
6. **Clean Storage**: Content-addressable storage with automatic deduplication
7. **GitHub Integration**: First-class GitHub support built-in

## Development

### Building

```bash
# Build the project
make build

# Run tests
make test

# Clean build artifacts
make clean
```

### Project Structure

```
├── cli/                    # Command-line interface
├── internal/              # Internal packages
│   ├── cas/              # Content-addressable storage
│   ├── commit/           # Commit management
│   ├── filechunk/        # File chunking system
│   ├── github/           # GitHub integration
│   ├── hamtdir/          # HAMT directory trees
│   ├── history/          # MMR and timeline history
│   ├── refs/             # Reference management
│   ├── workspace/        # Workspace materialization
│   └── wsindex/          # Workspace indexing
├── docs/                  # Documentation
└── main.go               # Entry point
```

### Contributing

1. Fork the repository
2. Create a feature timeline: `ivaldi timeline create feature-name`
3. Make your changes and commit: `ivaldi gather . && ivaldi seal "Description"`
4. Push to your fork: `ivaldi upload`
5. Create a Pull Request

## License

[License details to be added]

## Acknowledgments

Ivaldi VCS builds upon research in:
- Content-addressable storage systems
- Merkle data structures
- Modern cryptographic hashing
- Version control system design

---

*Ivaldi VCS - Modern version control for the modern developer*
