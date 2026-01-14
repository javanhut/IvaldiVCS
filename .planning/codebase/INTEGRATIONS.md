# External Integrations

**Analysis Date:** 2026-01-13

## APIs & External Services

**GitHub API:**
- GitHub REST API v3 - Repository operations, file content, branches, commits
  - SDK/Client: Custom HTTP client (`internal/github/client.go`)
  - Auth: OAuth token, environment variable, or Git credentials
  - Endpoints used: repos, contents, branches, commits, archive downloads
  - Rate limiting support with exponential backoff

**GitLab API:**
- GitLab API v4 - Repository operations similar to GitHub
  - SDK/Client: Custom HTTP client (`internal/gitlab/client.go`)
  - Auth: OAuth token, environment variable, or Git credentials
  - Endpoints used: projects, repository files, branches, commits

**External APIs:**
- Not applicable - No third-party APIs beyond GitHub/GitLab

## Data Storage

**Databases:**
- BoltDB (embedded) - Key-value store for Merkle Mountain Range and metadata
  - Connection: File-based at `.ivaldi/objects.db`
  - Client: go.etcd.io/bbolt v1.4.3 (`internal/store/kv.go`)
  - Migrations: Not applicable (schema embedded in code)

**File Storage:**
- Local filesystem - Content-addressable storage
  - Location: `.ivaldi/objects/` with sharded directories
  - Format: BLAKE3 hash-based content addressing (`internal/cas/file_cas.go`)
  - Organization: First 2 chars of hash as directory prefix

**Caching:**
- In-memory tree cache during sync operations (`internal/github/sync.go`)
- No persistent caching layer

## Authentication & Identity

**Auth Provider:**
- Custom OAuth implementation with device flow (`internal/auth/oauth.go`)
  - GitHub: Uses GitHub CLI public app (ClientID: `178c6fc778ccc68e1d6a`)
  - GitLab: Placeholder for registration (ClientID empty)
  - Token storage: `~/.config/ivaldi/auth.json` with 0600 permissions

**OAuth Integrations:**
- GitHub OAuth Device Flow - Social sign-in for repository access
  - Credentials: Built-in or via `IVALDI_GITHUB_CLIENT_ID` env var
  - Scopes: `repo,read:user,user:email`
- GitLab OAuth Device Flow - Repository access
  - Scopes: `read_api,write_repository,read_user`

**Fallback Authentication (in order):**
1. Ivaldi OAuth tokens (`~/.config/ivaldi/auth.json`)
2. Environment variables (`GITHUB_TOKEN`, `GITLAB_TOKEN`)
3. Git config (`git config github.token`)
4. Git credential helper (`git credential fill`)
5. `.netrc` file
6. GitHub CLI (`gh auth login`)
7. GitLab CLI (`glab auth login`)

## Monitoring & Observability

**Error Tracking:**
- None (stdout/stderr only)

**Analytics:**
- None

**Logs:**
- Console output only (fmt.Println throughout)
- No structured logging framework

## CI/CD & Deployment

**Hosting:**
- Self-hosted CLI tool
- Distributed as source with build instructions
- Binary installation to `/usr/local/bin`

**CI Pipeline:**
- GitHub Actions (`.github/workflows/`)
  - `ci.yml` - Continuous integration
  - `release.yml` - Release automation
  - Secrets: None required for tests

## Environment Configuration

**Development:**
- Required env vars: None (all optional)
- Secrets location: `~/.config/ivaldi/auth.json` (created on first auth)
- Mock/stub services: Local Git repositories

**Staging:**
- Not applicable (CLI tool)

**Production:**
- Secrets management: User's home directory with restricted permissions
- No server-side component

## Webhooks & Callbacks

**Incoming:**
- None (CLI tool, no server)

**Outgoing:**
- None

## External Tool Dependencies

| Tool | Purpose | Required | File |
|------|---------|----------|------|
| git | Credential helper, config lookup | Optional | `internal/auth/oauth.go` |
| gh | GitHub CLI token fallback | Optional | `internal/auth/oauth.go` |
| glab | GitLab CLI token fallback | Optional | `internal/auth/oauth.go` |

## Rate Limiting

- GitHub and GitLab clients track rate limits from response headers
- `RateLimiter` struct stores remaining requests and reset time
- Support for `Retry-After` headers
- Raw content endpoints used when possible to avoid API rate limits

---

*Integration audit: 2026-01-13*
*Update when adding/removing external services*
