# Codebase Structure

**Analysis Date:** 2026-01-13

## Directory Layout

```
IvaldiVCS/
├── main.go              # Entry point (calls cli.Execute())
├── go.mod               # Go module definition (v1.24.5)
├── go.sum               # Dependency checksums
├── Makefile             # Build automation
├── README.md            # Project documentation
├── setup.sh             # Automated installation
├── uninstall.sh         # Removal script
├── .golangci.yml        # Linter configuration
├── cli/                 # Command-line interface
├── internal/            # Core packages (27 subdirectories)
├── docs/                # Documentation
└── .github/             # GitHub Actions workflows
```

## Directory Purposes

**cli/**
- Purpose: CLI command implementations
- Contains: 22 Go files for commands and utilities
- Key files:
  - `cli.go` - Root command setup, subcommand registration
  - `management.go` - gather, seal, download commands
  - `timeline.go` - Timeline/branch management
  - `butterfly.go` - Experimental sandbox timelines
  - `fuse.go` - Merge operations
  - `shift.go` - Interactive commit squashing
  - `travel.go` - Time travel to previous commits
  - `portal.go` - Remote repository management
  - `harvest.go` - Download from remote
  - `scout.go` - Discover remote branches

**internal/**
- Purpose: Core functionality packages
- Contains: 27 subdirectories with focused responsibilities
- Subdirectories:
  - `auth/` - OAuth and authentication
  - `butterfly/` - Butterfly timeline system
  - `cas/` - Content-addressable storage
  - `commit/` - Commit object management
  - `colors/` - Terminal color utilities
  - `config/` - Repository configuration
  - `converter/` - Git format conversion
  - `diffmerge/` - Diff and merge operations
  - `filechunk/` - Large file chunking
  - `fsmerkle/` - Filesystem Merkle tree
  - `gitclone/` - Git cloning utilities
  - `github/` - GitHub API client
  - `gitlab/` - GitLab API client
  - `hamtdir/` - Hash Array Mapped Trie for directories
  - `history/` - Merkle Mountain Range history
  - `keys/` - Human-readable key generation
  - `objects/` - Git object utilities
  - `pack/` - Object packing/compression
  - `progress/` - Terminal progress bars
  - `proto/` - Protocol negotiation
  - `refs/` - Timeline reference management
  - `seals/` - Commit naming
  - `shelf/` - Auto-shelving system
  - `shift/` - Commit squashing logic
  - `store/` - BoltDB wrapper
  - `submodule/` - Git submodule support
  - `workspace/` - Workspace materialization
  - `wsindex/` - Workspace file tracking

**docs/**
- Purpose: User and developer documentation
- Contains: Architecture docs, command references, guides
- Key files:
  - `architecture.md` - System design overview
  - `SHIFT_FEATURE.md` - Squashing feature guide
  - `commands/` - Individual command documentation

## Key File Locations

**Entry Points:**
- `main.go` - CLI entry point
- `cli/cli.go` - Command dispatcher

**Configuration:**
- `go.mod` - Go module definition
- `.golangci.yml` - Linter configuration
- `Makefile` - Build targets

**Core Logic:**
- `internal/commit/commit.go` - Commit object creation
- `internal/refs/refs.go` - Timeline reference management
- `internal/cas/file_cas.go` - Content storage
- `internal/history/persistent_mmr.go` - Append-only history
- `internal/hamtdir/hamtdir.go` - Directory HAMT

**Testing:**
- `internal/*/` - Test files colocated (15 test files)
- Pattern: `*_test.go` alongside source

**Documentation:**
- `README.md` - User-facing documentation
- `docs/` - Extended documentation

## Naming Conventions

**Files:**
- snake_case for source: `file_cas.go`, `persistent_mmr.go`
- snake_case for tests: `*_test.go`
- Concurrent variants: `*_concurrent.go`

**Directories:**
- lowercase single-word: `cas`, `refs`, `commit`
- Domain-focused naming: `butterfly`, `github`, `gitlab`

**Special Patterns:**
- No index.go barrel files
- Package name matches directory name
- One main type per package typically

## Where to Add New Code

**New CLI Command:**
- Primary code: `cli/{command-name}.go`
- Register in: `cli/cli.go` init()
- Documentation: `docs/commands/{command-name}.md`

**New Internal Package:**
- Implementation: `internal/{name}/`
- Main file: `internal/{name}/{name}.go`
- Tests: `internal/{name}/{name}_test.go`

**New API Integration:**
- Implementation: `internal/{platform}/client.go`
- Sync logic: `internal/{platform}/sync.go`

**Utilities:**
- Shared helpers: appropriate `internal/` package
- CLI utilities: `cli/utils.go`

## Special Directories

**.ivaldi/ (per-repository)**
- Purpose: Repository data storage
- Source: Created by `ivaldi forge`
- Contains:
  - `objects/` - Content-addressable storage
  - `refs/heads/` - Timeline references
  - `objects.db` - BoltDB database
  - `HEAD` - Current timeline pointer
  - `index` - Workspace index
  - `shelves/` - Auto-shelved changes
- Committed: No (local repository state)

**build/**
- Purpose: Build output
- Source: Created by `make build`
- Committed: No (gitignored)

---

*Structure analysis: 2026-01-13*
*Update when directory structure changes*
