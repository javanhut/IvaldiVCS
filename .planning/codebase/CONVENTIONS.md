# Coding Conventions

**Analysis Date:** 2026-01-13

## Naming Patterns

**Files:**
- snake_case for all source files: `file_cas.go`, `persistent_mmr.go`
- snake_case for test files: `manager_test.go`, `sync_test.go`
- Benchmark files: `bench_test.go`

**Functions:**
- PascalCase for exported: `NewManager()`, `CreateButterfly()`, `Build()`
- camelCase for unexported: `setupTestEnv()`, `createInitialCommit()`
- Constructor pattern: `NewXxx()` prefix

**Variables:**
- camelCase for variables: `tmpDir`, `ivaldiDir`, `casStore`
- Single-letter for loops: `i`, `k`, `v`
- Descriptive for business logic: `divergenceHash`, `timelineID`

**Types:**
- PascalCase for interfaces and structs: `Manager`, `CAS`, `CommitObject`
- No `I` prefix for interfaces
- `Xxx` suffix for options: `Config`, `Options`

## Code Style

**Formatting:**
- gofmt with simplify option
- Tabs for indentation
- No hard line length limit
- Configuration: `.golangci.yml`

**Linting:**
- golangci-lint with 12+ linters enabled
- Key linters: errcheck, gosec, govet, staticcheck, revive
- Run: `make lint`
- Excluded in tests: errcheck, gosec, unparam

## Import Organization

**Order:**
1. Standard library imports
2. External imports (blank line separator)
3. Internal imports (blank line separator)

**Grouping:**
- Blank line between each group
- Alphabetical within groups

**Example:**
```go
import (
    "fmt"
    "os"
    "path/filepath"

    "go.etcd.io/bbolt"

    "github.com/javanhut/Ivaldi-vcs/internal/cas"
    "github.com/javanhut/Ivaldi-vcs/internal/refs"
)
```

**Path Aliases:**
- None used - full import paths

## Error Handling

**Patterns:**
- Return errors up the call stack
- Wrap with context: `fmt.Errorf("message: %w", err)`
- Early return on error

**Error Types:**
- Use standard errors with wrapping
- No custom error types currently
- Include operation context in messages

**Example:**
```go
if err != nil {
    return nil, fmt.Errorf("open shared ivaldi store: %w", err)
}
```

## Logging

**Framework:**
- fmt.Println/Printf for output
- No structured logging

**Patterns:**
- User-facing messages to stdout
- Errors to stderr (via log.Fatal or fmt.Fprintf)
- Progress indicators via progressbar library

**Where:**
- CLI layer for user messages
- Internal packages use return errors

## Comments

**When to Comment:**
- Package-level documentation required
- Exported types and functions
- Complex algorithms and business logic

**Package Comments:**
```go
// Package hamtdir implements Hash Array Mapped Trie for scalable directory storage.
//
// Directories are represented as HAMTs where:
// - Keys are file/subdirectory names (strings)
// - Values are either file references (NodeRef) or subdirectory references (DirRef)
```

**Function Comments:**
```go
// NewBuilder creates a new Builder with the given CAS.
func NewBuilder(casStore cas.CAS) *Builder {
```

**TODO Comments:**
- Format: `// TODO: description`
- Link to issue if exists: `// TODO: description (issue #123)`

## Function Design

**Size:**
- No strict limit, but prefer smaller functions
- Large functions exist in sync and merge logic

**Parameters:**
- Use pointer receivers for methods
- Prefer structs for multiple related parameters
- Context as first parameter when needed

**Return Values:**
- Return error as last value
- Use named returns sparingly
- Prefer explicit returns

## Module Design

**Exports:**
- Named exports only (Go convention)
- Public API via PascalCase functions/types
- Internal helpers via camelCase

**Package Organization:**
- One main type per package typically
- Related types grouped in same package
- Test files colocated

**Dependencies:**
- CLI imports internal packages
- Internal packages minimize cross-dependencies
- Storage layer has no business logic dependencies

---

*Convention analysis: 2026-01-13*
*Update when patterns change*
