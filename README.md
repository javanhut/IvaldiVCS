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

# Create commit
ivaldi seal "Add new features"

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
Ivaldi uses intuitive command names:
- `gather`: Stage files (like `git add`)
- `seal`: Create commit (like `git commit`)

### Scout and Harvest
Remote operations are designed for selective collaboration:
- `scout`: Discover available remote branches
- `harvest`: Download only the branches you need

## Documentation

- **[Usage Guide](docs/usage-guide.md)**: Comprehensive guide to using Ivaldi VCS
- **[Architecture](docs/architecture.md)**: Technical details about how Ivaldi works
- **[Scout & Harvest](docs/scout-harvest.md)**: Remote timeline operations
- **[Timeline Branching](docs/timeline-branching.md)**: Advanced branching features
- **[GitHub Integration](docs/github-integration.md)**: GitHub synchronization
- **[Portal Commands](docs/portal-commands.md)**: Repository connection management

## Command Reference

### Repository Management
```bash
ivaldi forge                    # Initialize repository
ivaldi status                   # Show working directory status
```

### File Operations
```bash
ivaldi gather [files...]        # Stage files for commit
ivaldi seal <message>           # Create commit with staged files
```

### Timeline Management
```bash
ivaldi timeline create <name>   # Create new timeline
ivaldi timeline switch <name>   # Switch to timeline
ivaldi timeline list           # List all timelines
ivaldi timeline remove <name>   # Delete timeline
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

## Comparison with Git

| Feature | Git | Ivaldi |
|---------|-----|--------|
| Initialize | `git init` | `ivaldi forge` |
| Stage files | `git add` | `ivaldi gather` |
| Commit | `git commit` | `ivaldi seal` |
| Branch | `git branch` | `ivaldi timeline create` |
| Switch branch | `git checkout` | `ivaldi timeline switch` |
| Clone | `git clone` | `ivaldi download` |
| Push | `git push` | `ivaldi upload` |
| Fetch | `git fetch` | `ivaldi harvest` |

## Advantages over Git

1. **Intuitive Commands**: Clear, descriptive command names
2. **Auto-Shelving**: Never lose work when switching branches
3. **Selective Sync**: Download only branches you need
4. **Modern Hashing**: BLAKE3 for better security and performance
5. **Clean Storage**: Content-addressable storage with automatic deduplication
6. **GitHub Integration**: First-class GitHub support built-in

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