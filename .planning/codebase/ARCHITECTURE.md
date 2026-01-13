# Architecture

**Analysis Date:** 2026-01-13

## Pattern Overview

**Overall:** Layered Monolithic CLI with Service-Oriented Design

**Key Characteristics:**
- Single executable with 20+ subcommands
- Clean separation between CLI, services, and storage
- Content-addressable storage with BLAKE3 hashing
- Merkle Mountain Range for append-only history
- HAMT directories for scalable tree structures

## Layers

**Command Layer (CLI):**
- Purpose: Parse user input and route to handlers
- Contains: Command definitions, argument parsing, user interaction
- Location: `cli/*.go`
- Depends on: Service layer packages in `internal/`
- Used by: User via terminal

**Service Layer (Core Logic):**
- Purpose: Business logic for version control operations
- Contains: Commit management, timeline handling, workspace operations
- Location: `internal/commit/`, `internal/refs/`, `internal/workspace/`, `internal/butterfly/`
- Depends on: Storage layer, utility packages
- Used by: CLI commands

**Storage Layer:**
- Purpose: Persistent data management
- Contains: CAS, BoltDB wrapper, workspace index
- Location: `internal/cas/`, `internal/store/`, `internal/wsindex/`
- Depends on: File system, BoltDB
- Used by: Service layer

**Data Structures Layer:**
- Purpose: Core data types and algorithms
- Contains: HAMT, MMR, file chunking, diff/merge
- Location: `internal/hamtdir/`, `internal/history/`, `internal/filechunk/`, `internal/diffmerge/`
- Depends on: Storage layer
- Used by: Service layer

## Data Flow

**Commit Creation (ivaldi gather + seal):**

1. User runs: `ivaldi gather` then `ivaldi seal`
2. CLI parses command (`cli/management.go`)
3. Workspace scanner detects file changes (`internal/workspace/`)
4. Files chunked and hashed (`internal/filechunk/`)
5. Content stored by BLAKE3 hash (`internal/cas/file_cas.go`)
6. HAMT directory tree built (`internal/hamtdir/`)
7. Commit object created with tree hash + parents (`internal/commit/`)
8. Commit appended to MMR history (`internal/history/persistent_mmr.go`)
9. Timeline reference updated (`internal/refs/`)

**Timeline Switch (ivaldi timeline switch):**

1. User runs: `ivaldi timeline switch feature`
2. CLI routes to switch handler (`cli/timeline.go`)
3. Current changes auto-shelved if dirty (`internal/shelf/`)
4. Target commit loaded from refs (`internal/refs/`)
5. Tree structure loaded from CAS (`internal/hamtdir/`)
6. Workspace materialized from tree (`internal/workspace/`)
7. Files reconstructed from CAS (`internal/cas/`)

**State Management:**
- File-based: All state in `.ivaldi/` directory
- BoltDB for MMR and hash mappings
- No in-memory persistent state between commands

## Key Abstractions

**CAS (Content-Addressable Storage):**
- Purpose: Store and retrieve content by hash
- Examples: `internal/cas/file_cas.go` (file-based), `internal/cas/cas.go` (in-memory)
- Pattern: Interface with `Put()`, `Get()`, `Has()` methods

**MMR (Merkle Mountain Range):**
- Purpose: Append-only commit history with cryptographic proofs
- Examples: `internal/history/mmr.go`, `internal/history/persistent_mmr.go`
- Pattern: Persistent tree structure with efficient peak management

**HAMT (Hash Array Mapped Trie):**
- Purpose: Scalable directory representation
- Examples: `internal/hamtdir/hamtdir.go`
- Pattern: 32-way branching trie with canonical encoding

**RefsManager:**
- Purpose: Timeline/branch reference management
- Examples: `internal/refs/refs.go`
- Pattern: Singleton manager with file-based persistence

**Butterfly:**
- Purpose: Experimental sandbox branches with bidirectional sync
- Examples: `internal/butterfly/manager.go`, `internal/butterfly/sync.go`
- Pattern: State machine with metadata tracking

## Entry Points

**CLI Entry:**
- Location: `main.go`
- Triggers: User runs `ivaldi <command>`
- Responsibilities: Initialize Cobra, call `cli.Execute()`

**Command Dispatcher:**
- Location: `cli/cli.go`
- Triggers: Subcommand matched
- Responsibilities: Register commands, parse flags, delegate to handlers

## Error Handling

**Strategy:** Return errors up the call stack, handle at CLI boundary

**Patterns:**
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Early return on error
- Defer for cleanup (Close, RemoveAll)
- User-facing errors printed to stderr

## Cross-Cutting Concerns

**Logging:**
- Console output via fmt.Println/Printf
- Colored output via `internal/colors/`
- No structured logging framework

**Validation:**
- Input validation in CLI handlers
- Path validation before file operations
- Hash verification on content retrieval

**Concurrency:**
- Worker pools for parallel operations (8-16 workers)
- sync.Mutex/RWMutex for shared state
- WaitGroups + semaphores for coordination
- Reference-counted database connections (`internal/store/manager.go`)

**Authentication:**
- OAuth device flow for GitHub/GitLab
- Multiple fallback methods (env vars, git config, credential helpers)
- Token storage with restricted permissions

---

*Architecture analysis: 2026-01-13*
*Update when major patterns change*
