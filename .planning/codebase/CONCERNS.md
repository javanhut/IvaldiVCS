# Codebase Concerns

**Analysis Date:** 2026-01-13

## Tech Debt

**Incomplete TODO implementations:**
- Issue: Multiple TODO comments indicate unfinished features
- Files:
  - `cli/diff.go:420` - "TODO: Convert tree to FileMetadata"
  - `cli/diff.go:437` - "TODO: implement hash prefix resolution"
  - `cli/fuse.go:189` - "TODO: Walk full parent chain"
  - `cli/fuse.go:628` - "TODO: Implement interactive resolution using the ConflictResolver"
  - `cli/management.go:789` - "TODO: Implement actual download/clone functionality for standard Ivaldi remotes"
- Why: Features added incrementally during development
- Impact: Some user workflows incomplete (hash prefix, interactive merge)
- Fix approach: Prioritize and implement or remove if not needed

**Large file complexity:**
- Issue: Several files exceed 800 lines with complex logic
- Files:
  - `internal/github/sync.go` (1847 lines)
  - `cli/management.go` (1459 lines)
  - `internal/workspace/workspace.go` (827 lines)
  - `cli/fuse.go` (758 lines)
  - `internal/diffmerge/diffmerge.go` (757 lines)
- Why: Features added incrementally without refactoring
- Impact: Harder to maintain, test, and understand
- Fix approach: Extract cohesive modules (e.g., split sync.go into tree fetching, object conversion, etc.)

**Console output in internal packages:**
- Issue: ~146 instances of fmt.Println/Printf in internal packages
- Files: Throughout `internal/`
- Why: Quick development without logging abstraction
- Impact: Cannot suppress or redirect output, mixing concerns
- Fix approach: Introduce proper logging package, migrate to structured logging

## Known Bugs

**No critical bugs identified through static analysis.**

## Security Considerations

**Hardcoded OAuth credentials:**
- Risk: GitHub public app credentials embedded in code
- Files: `internal/auth/oauth.go:30` - `GitHubClientID = "178c6fc778ccc68e1d6a"`
- Current mitigation: Documented as intentional (public app), users can override via env vars
- Recommendations: Consider documenting security implications in README

**Missing input validation on downloaded paths:**
- Risk: Path traversal attacks possible from malicious remote data
- Files: `internal/github/sync.go`, `internal/gitlab/sync.go`
- Current mitigation: None explicit
- Recommendations: Validate paths from API responses before file operations

**Token in memory:**
- Risk: OAuth tokens held in memory during operations
- Files: `internal/auth/oauth.go`
- Current mitigation: Restricted file permissions (0600)
- Recommendations: Consider secure memory handling for long-running sessions

## Performance Bottlenecks

**No measured performance issues identified.**

The codebase uses efficient patterns:
- Worker pools for concurrent operations (8-16 workers)
- Content-addressable storage for deduplication
- BoltDB for efficient key-value operations
- HAMT for scalable directory structures

## Fragile Areas

**Error handling in cleanup paths:**
- Why fragile: `os.Chdir(originalDir)` errors silently ignored
- Files: `cli/management.go:148, 366, 493`
- Common failures: Directory deleted during operation
- Safe modification: Add error logging in cleanup
- Test coverage: Not tested

**Ignored error returns:**
- Why fragile: Multiple places discard errors with `_`
- Files:
  - `cli/timeline.go:78, 82, 195` - Build() and NewManager() errors discarded
  - `cli/fuse.go:259, 266, 407, 733` - Errors silently ignored
  - `internal/keys/keys.go:24` - rand.Read() error ignored
- Common failures: Silent failures lead to confusing behavior
- Safe modification: Handle or log all errors
- Test coverage: Not specifically tested

**Panic in production code:**
- Why fragile: Panic instead of error return
- Files: `internal/fsmerkle/types.go:143` - Panic on content size mismatch
- Common failures: Data corruption or API issues cause crash
- Safe modification: Convert to error return
- Test coverage: Has test for panic behavior

## Scaling Limits

**Not applicable for CLI tool.**

The tool operates on local repositories with practical limits based on:
- Filesystem capacity
- Available memory for large operations
- API rate limits (handled with backoff)

## Dependencies at Risk

**No critical dependency risks identified.**

Dependencies are from reputable sources:
- go.etcd.io/bbolt - CNCF project
- github.com/spf13/cobra - Widely used CLI framework
- github.com/go-git/go-git - Active Git implementation

## Missing Critical Features

**Interactive merge conflict resolution:**
- Problem: CLI prompts for strategy but no interactive file-by-file resolution
- Files: `cli/fuse.go:628` - TODO comment
- Current workaround: Users must manually edit files, use strategy flags
- Blocks: User-friendly merge workflow
- Implementation complexity: Medium (UI + diffmerge integration)

**Hash prefix resolution:**
- Problem: Cannot use short commit hashes
- Files: `cli/diff.go:437` - TODO comment
- Current workaround: Use full hashes
- Blocks: Convenient commit references
- Implementation complexity: Low (prefix matching in refs)

**GitLab OAuth registration:**
- Problem: GitLab OAuth ClientID is empty
- Files: `internal/auth/oauth.go:38` - Empty string
- Current workaround: Use personal access tokens or git credentials
- Blocks: Seamless GitLab authentication
- Implementation complexity: Low (register app, add credentials)

## Test Coverage Gaps

**CLI commands:**
- What's not tested: Most CLI command logic
- Risk: Regressions in user-facing features
- Priority: Medium
- Difficulty to test: Requires integration testing setup

**Sync operations:**
- What's not tested: GitHub/GitLab sync logic (1800+ lines)
- Files: `internal/github/sync.go`, `internal/gitlab/sync.go`
- Risk: API changes could break sync silently
- Priority: High
- Difficulty to test: Requires API mocking

**Error paths:**
- What's not tested: Many error handling branches
- Risk: Error recovery may not work as expected
- Priority: Medium
- Difficulty to test: Need to simulate failures

---

*Concerns audit: 2026-01-13*
*Update as issues are fixed or new ones discovered*
