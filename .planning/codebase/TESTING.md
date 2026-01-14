# Testing Patterns

**Analysis Date:** 2026-01-13

## Test Framework

**Runner:**
- Go standard `testing` package
- No external test framework

**Assertion Library:**
- Go standard comparisons
- Matchers: `!=`, `==`, manual comparisons
- `t.Errorf()`, `t.Fatalf()` for failures

**Run Commands:**
```bash
make test                            # Run all tests with verbose output
go test ./...                        # Run all tests
go test -v ./internal/butterfly/     # Single package
go test -run TestSpecific ./...      # Single test
go test -bench=. ./internal/...      # Run benchmarks
```

## Test File Organization

**Location:**
- Colocated with source files
- Pattern: `{package}_test.go` or `{file}_test.go`

**Naming:**
- Unit tests: `*_test.go`
- Benchmark tests: `bench_test.go`
- No integration/e2e naming convention

**Structure:**
```
internal/
  butterfly/
    manager.go
    manager_test.go     # Tests for manager.go
    sync.go
    sync_test.go        # Tests for sync.go
  cas/
    cas.go
    cas_test.go
  history/
    mmr.go
    bench_test.go       # Benchmarks
```

## Test Structure

**Suite Organization:**
```go
func TestNewManager(t *testing.T) {
    ivaldiDir, manager, cleanup := setupTestEnv(t)
    defer cleanup()

    // Test assertions
    if manager == nil {
        t.Fatal("manager should not be nil")
    }
}
```

**Patterns:**
- Setup helper: `setupTestEnv(t *testing.T)` returns (dir, object, cleanup)
- Cleanup via defer
- Use `t.Fatal()` for setup failures
- Use `t.Errorf()` for assertion failures

## Mocking

**Framework:**
- No mocking framework
- Interface-based mocking (CAS interface)

**Patterns:**
```go
// In-memory CAS for testing
casStore := cas.NewMemoryCAS()

// Use interfaces to inject test implementations
func NewManager(..., casStore cas.CAS, ...) *Manager
```

**What to Mock:**
- File system operations (use temp directories)
- External APIs (not currently mocked)
- Database (use in-memory or temp file)

**What NOT to Mock:**
- Internal pure functions
- Data structures (HAMT, MMR)

## Fixtures and Factories

**Test Data:**
```go
// Factory functions in test file
func setupTestEnv(t *testing.T) (string, *Manager, func()) {
    tmpDir, err := os.MkdirTemp("", "butterfly-test-*")
    if err != nil {
        t.Fatalf("failed to create temp dir: %v", err)
    }
    // ... setup ...
    cleanup := func() {
        manager.Close()
        os.RemoveAll(tmpDir)
    }
    return ivaldiDir, manager, cleanup
}
```

**Location:**
- Factory functions: inline in test files
- No shared fixtures directory
- Temp directories for file-based tests

## Coverage

**Requirements:**
- No enforced coverage target
- Coverage tracked for awareness

**Configuration:**
- Go built-in coverage
- No exclusions configured

**View Coverage:**
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Test Types

**Unit Tests:**
- Test single function/method in isolation
- Mock external dependencies via interfaces
- Fast execution
- Examples: `internal/cas/cas_test.go`, `internal/hamtdir/hamtdir_test.go`

**Integration Tests:**
- Test multiple modules together
- Use temp directories for file system
- Examples: `internal/butterfly/manager_test.go` (tests manager + cas + refs + mmr)

**Benchmark Tests:**
- Measure performance
- Use `testing.B`
- Examples: `internal/fsmerkle/bench_test.go`, `internal/history/bench_test.go`

**E2E Tests:**
- Not currently implemented
- CLI tested manually

## Common Patterns

**Async Testing:**
```go
func TestConcurrency(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            // concurrent operations
        }()
    }
    wg.Wait()
}
```

**Error Testing:**
```go
if err == nil {
    t.Fatal("expected error, got nil")
}
```

**Table-Driven Tests:**
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"empty", "", ""},
    {"simple", "hello", "hello"},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test logic
    })
}
```

**Benchmark Pattern:**
```go
func BenchmarkBuildTreeFromMap(b *testing.B) {
    sizes := []int{10, 100, 1000}
    for _, size := range sizes {
        b.Run(fmt.Sprintf("N=%d", size), func(b *testing.B) {
            // setup
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                // operation to benchmark
            }
        })
    }
}
```

**Snapshot Testing:**
- Not used

## Test Files Summary

| Package | Test File | Description |
|---------|-----------|-------------|
| butterfly | `manager_test.go` | Manager lifecycle tests |
| cas | `cas_test.go` | CAS operations, concurrency |
| commit | `commit_test.go` | Commit object tests |
| diffmerge | `diffmerge_test.go` | Merge algorithm tests |
| filechunk | `filechunk_test.go` | File chunking tests |
| fsmerkle | `fsmerkle_test.go`, `bench_test.go` | Tree operations, benchmarks |
| hamtdir | `hamtdir_test.go` | HAMT directory tests |
| history | `bench_test.go` | MMR benchmarks |
| shift | `squasher_test.go` | Squashing logic tests |
| workspace | `workspace_test.go` | Workspace materialization |
| wsindex | `wsindex_test.go` | Workspace index tests |

---

*Testing analysis: 2026-01-13*
*Update when test patterns change*
