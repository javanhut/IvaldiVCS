# Technology Stack

**Analysis Date:** 2026-01-13

## Languages

**Primary:**
- Go 1.24.5 - All application code (`go.mod`)

**Secondary:**
- Shell scripts - Installation and setup (`setup.sh`, `uninstall.sh`)

## Runtime

**Environment:**
- Go runtime 1.24.5
- Linux/macOS (Windows not officially supported)

**Package Manager:**
- Go Modules
- Lockfile: `go.sum` present

## Frameworks

**Core:**
- Cobra v1.10.1 - CLI command framework (`go.mod`)

**Testing:**
- Go standard `testing` package - Unit and benchmark tests
- No external test framework

**Build/Dev:**
- Make - Build automation (`Makefile`)
- golangci-lint - Linting (`.golangci.yml`)
- go fmt - Code formatting

## Key Dependencies

**Critical:**
- github.com/spf13/cobra v1.10.1 - CLI structure and subcommands
- lukechampine.com/blake3 v1.4.1 - BLAKE3 hashing for content addressing (`internal/cas/cas.go`)
- go.etcd.io/bbolt v1.4.3 - Embedded key-value database for MMR and metadata (`internal/store/kv.go`)

**Infrastructure:**
- github.com/go-git/go-git/v5 v5.16.3 - Pure Go Git implementation for cloning/objects (indirect)
- github.com/ProtonMail/go-crypto v1.1.6 - Cryptographic primitives for SSH
- github.com/klauspost/compress v1.18.0 - High-performance compression
- github.com/sergi/go-diff - Diff algorithm implementation (`internal/diffmerge`)
- github.com/schollz/progressbar/v3 - Terminal progress bars (`internal/progress`)

## Configuration

**Environment:**
- No required environment variables for basic operation
- Optional: `IVALDI_GITHUB_CLIENT_ID`, `IVALDI_GITHUB_CLIENT_SECRET` for custom OAuth
- Optional: `GITHUB_TOKEN`, `GITLAB_TOKEN` for authentication fallback

**Build:**
- `Makefile` - Build targets (build, install, clean, test, lint)
- `.golangci.yml` - Linter configuration with 12+ enabled linters
- Build flags: `-ldflags "-s -w"` for stripped production binaries

## Platform Requirements

**Development:**
- Any platform with Go 1.19+ (1.24.5 recommended)
- Git for version control
- No external database required (BoltDB embedded)

**Production:**
- Distributed as single binary
- Installed via `make install` to `/usr/local/bin`
- Runs on user's system without dependencies
- Storage: `.ivaldi/` directory in each repository

---

*Stack analysis: 2026-01-13*
*Update after major dependency changes*
