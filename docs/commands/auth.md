# Auth Command

Manage GitHub authentication for Ivaldi VCS using OAuth.

## Overview

The `auth` command provides a secure way to authenticate with GitHub using OAuth device flow. This works exactly like `gh auth login` from the GitHub CLI, providing seamless access to all your repositories without needing to manually create and manage personal access tokens.

**Ivaldi uses the same OAuth App as GitHub CLI**, so authentication works immediately without any setup required.

## Quick Start

Simply run:
```bash
ivaldi auth login
```

That's it! No configuration needed.

## Subcommands

### auth login

Authenticate with GitHub using OAuth device flow.

```bash
ivaldi auth login
```

This command will:
1. Generate a unique user code
2. Display the GitHub verification URL
3. Wait for you to authorize the application in your browser
4. Store the OAuth token securely in `~/.config/ivaldi/auth.json`

**Example:**

```bash
$ ivaldi auth login
Initiating GitHub authentication...

First, copy your one-time code: ABCD-1234
Then visit: https://github.com/login/device

Waiting for authentication...

Authentication successful!
```

Once authenticated, you can immediately use GitHub-related commands like `download`, `upload`, `scout`, and `harvest` without any additional configuration.

### auth status

Check your current authentication status and view user information.

```bash
ivaldi auth status
```

This command will:
- Verify if you're authenticated
- Show which authentication method is being used
- Display your GitHub username
- Show your account information
- Validate that the token is still valid

**Example when authenticated via Ivaldi OAuth:**

```bash
$ ivaldi auth status
Authenticated via 'ivaldi auth login'

Logged in to GitHub as: javanhut
Name: John Doe
Email: john@example.com
Account type: User
```

**Example when authenticated via GitHub CLI:**

```bash
$ ivaldi auth status
Authenticated via 'gh auth login' (GitHub CLI)

Logged in to GitHub as: javanhut
Name: John Doe
Email: john@example.com
Account type: User

Note: You're using an external authentication method.
To use Ivaldi's built-in OAuth, run:
  ivaldi auth login
```

**Example when authenticated via environment variable:**

```bash
$ ivaldi auth status
Authenticated via GITHUB_TOKEN environment variable

Logged in to GitHub as: javanhut
Name: John Doe
Email: john@example.com
Account type: User

Note: You're using an external authentication method.
To use Ivaldi's built-in OAuth, run:
  ivaldi auth login
```

**Example when not authenticated:**

```bash
$ ivaldi auth status
Not authenticated with GitHub

To authenticate, run:
  ivaldi auth login

Alternatively, you can:
  - Set GITHUB_TOKEN environment variable
  - Use 'gh auth login' (GitHub CLI)
  - Configure git credentials
```

### auth logout

Remove stored authentication credentials.

```bash
ivaldi auth logout
```

This command will:
- Delete the OAuth token from `~/.config/ivaldi/auth.json`
- Require re-authentication for future GitHub operations

**Example:**

```bash
$ ivaldi auth logout
Logged out successfully
```

## Authentication Priority

Ivaldi checks for GitHub credentials in the following order:

1. **Ivaldi OAuth token** (from `ivaldi auth login`) - **Highest priority**
2. `GITHUB_TOKEN` environment variable
3. Git config (`github.token`)
4. Git credential helper
5. `.netrc` file
6. GitHub CLI (`gh`) config

This means if you authenticate using `ivaldi auth login`, that token will be used even if other methods are configured.

### Checking Your Authentication Source

The `ivaldi auth status` command will tell you exactly which authentication method is currently active:

| Auth Method | Status Message |
|-------------|----------------|
| Ivaldi OAuth | `Authenticated via 'ivaldi auth login'` |
| GitHub CLI | `Authenticated via 'gh auth login' (GitHub CLI)` |
| Environment Variable | `Authenticated via GITHUB_TOKEN environment variable` |
| Git Config | `Authenticated via git config (github.token)` |
| Git Credential Helper | `Authenticated via git credential helper` |
| .netrc File | `Authenticated via .netrc file` |

This helps you understand which credentials Ivaldi is using and troubleshoot authentication issues.

## Security

- OAuth tokens are stored with restricted permissions (0600) in `~/.config/ivaldi/auth.json`
- Only you (the file owner) can read the token file
- Tokens are requested with minimal required scopes: `repo`, `read:user`, `user:email`
- You can revoke access at any time through:
  - GitHub settings: https://github.com/settings/applications (look for "GitHub CLI")
  - Running `ivaldi auth logout`
- You can override scopes via `IVALDI_GITHUB_SCOPES` environment variable (default is sufficient for most users)
- Ivaldi uses GitHub CLI's public OAuth App, which is trusted and widely used
- OAuth App Client IDs are public (not secret) and safe to share
- Your personal access token is unique to you and stored securely

## Token Scopes

The OAuth token requests the following scopes:

- **repo**: Full control of private repositories (required for clone, push, pull operations)
- **read:user**: Read user profile information
- **user:email**: Read user email addresses

## Advanced: Using Your Own OAuth App

By default, Ivaldi uses GitHub CLI's public OAuth App. If you prefer to use your own:

1. Create a GitHub OAuth App at: https://github.com/settings/developers
2. Click "New OAuth App" (NOT "New GitHub App")
3. Enable "Device Flow" in the OAuth App settings
4. Copy the Client ID
5. Set environment variable: `export IVALDI_GITHUB_CLIENT_ID=your_client_id_here`

**Note:** GitHub Apps (Client IDs starting with `Iv1.`) will NOT work. You must use an OAuth App.

## Troubleshooting

### Token expired or invalid

If you see authentication errors:

```bash
ivaldi auth status
```

If the token is invalid, re-authenticate:

```bash
ivaldi auth logout
ivaldi auth login
```

### Permission denied errors

If you get permission errors when accessing repositories:
1. Ensure the repository exists and you have access to it on GitHub
2. Check your authentication: `ivaldi auth status`
3. Try re-authenticating: `ivaldi auth login`
4. If you've overridden the OAuth App with `IVALDI_GITHUB_CLIENT_ID`, make sure it's an OAuth App (not a GitHub App)

### Browser not available

The OAuth device flow works well for:
- Headless servers
- Remote SSH sessions
- Containerized environments

You can copy the verification URL and code to any device with a browser, authenticate there, and the CLI will automatically receive the token.

## Comparison with Other Methods

### vs. Personal Access Token (PAT)

**OAuth (ivaldi auth login):**
- Automatic token management
- No manual token creation
- Easy revocation through logout
- Better security (shorter-lived tokens)

**Personal Access Token:**
- Manual creation through GitHub settings
- Must be copied and stored manually
- Requires setting environment variable or git config
- Tokens don't expire automatically

### vs. GitHub CLI (gh)

Ivaldi's OAuth implementation is nearly identical to GitHub CLI's authentication:
- Both use the **same OAuth App** by default (GitHub CLI's public OAuth App)
- Both use OAuth device flow
- Both store tokens securely
- Both provide easy login/logout
- Both provide full user-level access to repositories

**Differences:**
- Ivaldi stores tokens in `~/.config/ivaldi/auth.json`
- GitHub CLI stores tokens in `~/.config/gh/hosts.yml`
- Tokens are separate - logging into one doesn't log you into the other
- Ivaldi can read from GitHub CLI config as a fallback if you haven't run `ivaldi auth login`

## See Also

- [Portal Command](portal.md) - Managing repository connections
- [Download Command](download.md) - Cloning repositories
- [Upload Command](upload.md) - Pushing to GitHub
- [GitHub Integration Guide](../guides/github-integration.md)
