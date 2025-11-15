package gitclone

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// AuthMethod represents an authentication method for Git operations
type AuthMethod interface {
	toGoGitAuth() transport.AuthMethod
}

// BasicAuth represents HTTP basic authentication
type BasicAuth struct {
	Username string
	Password string
}

func (a *BasicAuth) toGoGitAuth() transport.AuthMethod {
	return &http.BasicAuth{
		Username: a.Username,
		Password: a.Password,
	}
}

// TokenAuth represents token-based authentication (GitHub, GitLab, etc.)
type TokenAuth struct {
	Token string
}

func (a *TokenAuth) toGoGitAuth() transport.AuthMethod {
	// For token auth, username can be anything (commonly "token" or "oauth2")
	return &http.BasicAuth{
		Username: "token",
		Password: a.Token,
	}
}

// SSHKeyAuth represents SSH key authentication
type SSHKeyAuth struct {
	User           string
	PrivateKeyPath string
	Password       string // For encrypted keys
}

func (a *SSHKeyAuth) toGoGitAuth() transport.AuthMethod {
	auth, err := ssh.NewPublicKeysFromFile(a.User, a.PrivateKeyPath, a.Password)
	if err != nil {
		return nil
	}
	return auth
}

// DetectAuth attempts to auto-detect authentication method from URL and environment
func DetectAuth(url string, username, password, token, sshKey string) (AuthMethod, error) {
	// Priority 1: Explicit CLI flags
	if token != "" {
		return &TokenAuth{Token: token}, nil
	}

	if username != "" {
		return &BasicAuth{
			Username: username,
			Password: password,
		}, nil
	}

	if sshKey != "" {
		return &SSHKeyAuth{
			User:           "git",
			PrivateKeyPath: sshKey,
		}, nil
	}

	// Priority 2: Check for SSH URLs
	if strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://") {
		// Try default SSH key
		homeDir, err := os.UserHomeDir()
		if err == nil {
			keyPath := filepath.Join(homeDir, ".ssh", "id_rsa")

			if _, err := os.Stat(keyPath); err == nil {
				return &SSHKeyAuth{
					User:           "git",
					PrivateKeyPath: keyPath,
				}, nil
			}

			// Try id_ed25519
			keyPath = filepath.Join(homeDir, ".ssh", "id_ed25519")
			if _, err := os.Stat(keyPath); err == nil {
				return &SSHKeyAuth{
					User:           "git",
					PrivateKeyPath: keyPath,
				}, nil
			}
		}

		return nil, fmt.Errorf("SSH key not found - use --ssh-key to specify path")
	}

	// Priority 3: Check environment variables
	if envToken := os.Getenv("GIT_TOKEN"); envToken != "" {
		return &TokenAuth{Token: envToken}, nil
	}

	if envUsername := os.Getenv("GIT_USERNAME"); envUsername != "" {
		envPassword := os.Getenv("GIT_PASSWORD")
		return &BasicAuth{
			Username: envUsername,
			Password: envPassword,
		}, nil
	}

	// Priority 4: For HTTPS URLs to public repos, try without auth
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return nil, nil // No auth needed for public repos
	}

	return nil, nil
}
