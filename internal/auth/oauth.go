package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Platform represents a Git hosting platform
type Platform string

const (
	PlatformGitHub Platform = "github"
	PlatformGitLab Platform = "gitlab"
)

// GitHub OAuth constants
const (
	// GitHubClientID - Using GitHub CLI's public OAuth App by default
	// This allows Ivaldi to work exactly like 'gh auth login' without requiring users to create their own OAuth App
	// Users can override with their own OAuth App via IVALDI_GITHUB_CLIENT_ID environment variable
	GitHubClientID       = "178c6fc778ccc68e1d6a" // GitHub CLI's public OAuth App
	GitHubDeviceCodeURL  = "https://github.com/login/device/code"
	GitHubAccessTokenURL = "https://github.com/login/oauth/access_token"
	GitHubScopes         = "repo,read:user,user:email"
)

// GitLab OAuth constants
const (
	GitLabClientID      = "" // Placeholder - needs to be registered
	GitLabDeviceCodeURL = "https://gitlab.com/oauth/authorize_device"
	GitLabAccessTokenURL = "https://gitlab.com/oauth/token"
	GitLabScopes        = "read_api,write_repository,read_user"
)

// TokenStore manages OAuth tokens
type TokenStore struct {
	configPath string
}

// Token represents an OAuth token for a specific platform
type Token struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
	CreatedAt   time.Time `json:"created_at"`
}

// TokenStorage stores tokens for multiple platforms
type TokenStorage struct {
	GitHub *Token `json:"github,omitempty"`
	GitLab *Token `json:"gitlab,omitempty"`
}

// DeviceCodeResponse represents the response from device code request
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// AccessTokenResponse represents the response from access token request
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// NewTokenStore creates a new token store
func NewTokenStore() (*TokenStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".config", "ivaldi")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	return &TokenStore{
		configPath: filepath.Join(configDir, "auth.json"),
	}, nil
}

// LoadToken loads the stored token for a specific platform
func (ts *TokenStore) LoadToken(platform Platform) (*Token, error) {
	data, err := os.ReadFile(ts.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read token: %w", err)
	}

	// Try to parse as new multi-platform format first
	var storage TokenStorage
	if err := json.Unmarshal(data, &storage); err == nil {
		switch platform {
		case PlatformGitHub:
			return storage.GitHub, nil
		case PlatformGitLab:
			return storage.GitLab, nil
		default:
			return nil, fmt.Errorf("unknown platform: %s", platform)
		}
	}

	// Fall back to old single-token format for backward compatibility
	// This assumes old tokens are GitHub tokens
	if platform == PlatformGitHub {
		var token Token
		if err := json.Unmarshal(data, &token); err != nil {
			return nil, fmt.Errorf("failed to parse token: %w", err)
		}
		return &token, nil
	}

	return nil, nil
}

// LoadAllTokens loads all stored tokens
func (ts *TokenStore) LoadAllTokens() (*TokenStorage, error) {
	data, err := os.ReadFile(ts.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &TokenStorage{}, nil
		}
		return nil, fmt.Errorf("failed to read tokens: %w", err)
	}

	// Try to parse as new multi-platform format
	var storage TokenStorage
	if err := json.Unmarshal(data, &storage); err == nil {
		return &storage, nil
	}

	// Fall back to old single-token format
	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Migrate old format to new format
	return &TokenStorage{GitHub: &token}, nil
}

// SaveToken saves the token for a specific platform to disk
func (ts *TokenStore) SaveToken(platform Platform, token *Token) error {
	token.CreatedAt = time.Now()

	// Load existing tokens
	storage, err := ts.LoadAllTokens()
	if err != nil {
		return err
	}

	// Update the token for the specified platform
	switch platform {
	case PlatformGitHub:
		storage.GitHub = token
	case PlatformGitLab:
		storage.GitLab = token
	default:
		return fmt.Errorf("unknown platform: %s", platform)
	}

	// Save updated storage
	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	if err := os.WriteFile(ts.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write tokens: %w", err)
	}

	return nil
}

// DeleteToken removes the stored token for a specific platform
func (ts *TokenStore) DeleteToken(platform Platform) error {
	storage, err := ts.LoadAllTokens()
	if err != nil {
		return err
	}

	// Remove the token for the specified platform
	switch platform {
	case PlatformGitHub:
		storage.GitHub = nil
	case PlatformGitLab:
		storage.GitLab = nil
	default:
		return fmt.Errorf("unknown platform: %s", platform)
	}

	// If no tokens remain, delete the file
	if storage.GitHub == nil && storage.GitLab == nil {
		if err := os.Remove(ts.configPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete token file: %w", err)
		}
		return nil
	}

	// Otherwise, save updated storage
	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	if err := os.WriteFile(ts.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write tokens: %w", err)
	}

	return nil
}

// RequestDeviceCode initiates the OAuth device flow for a specific platform
func RequestDeviceCode(ctx context.Context, platform Platform) (*DeviceCodeResponse, error) {
	var clientID, deviceCodeURL, scopes string

	switch platform {
	case PlatformGitHub:
		clientID = getGitHubClientID()
		deviceCodeURL = GitHubDeviceCodeURL
		scopes = getGitHubScopes()
	case PlatformGitLab:
		clientID = GitLabClientID
		deviceCodeURL = GitLabDeviceCodeURL
		scopes = GitLabScopes
	default:
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", scopes)

	req, err := http.NewRequestWithContext(ctx, "POST", deviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var deviceCode DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &deviceCode, nil
}

// PollForAccessToken polls the platform for the access token
func PollForAccessToken(ctx context.Context, platform Platform, deviceCode string, interval int) (*Token, error) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := checkAccessToken(ctx, platform, deviceCode)
			if err != nil {
				// Check if it's a retriable error
				if strings.Contains(err.Error(), "authorization_pending") {
					continue
				}
				if strings.Contains(err.Error(), "slow_down") {
					// Increase interval
					ticker.Reset(time.Duration(interval+5) * time.Second)
					continue
				}
				return nil, err
			}
			return token, nil
		}
	}
}

// checkAccessToken checks if the access token is ready
func checkAccessToken(ctx context.Context, platform Platform, deviceCode string) (*Token, error) {
	var clientID, accessTokenURL string

	switch platform {
	case PlatformGitHub:
		clientID = getGitHubClientID()
		accessTokenURL = GitHubAccessTokenURL
	case PlatformGitLab:
		clientID = GitLabClientID
		accessTokenURL = GitLabAccessTokenURL
	default:
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequestWithContext(ctx, "POST", accessTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request access token: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp AccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDescription)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token received")
	}

	return &Token{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
		Scope:       tokenResp.Scope,
	}, nil
}

func getGitHubClientID() string {
	if v := strings.TrimSpace(os.Getenv("IVALDI_GITHUB_CLIENT_ID")); v != "" {
		return v
	}
	if GitHubClientID == "" {
		// Return empty string - this will be caught in RequestDeviceCode
		return ""
	}
	return GitHubClientID
}

func getGitHubScopes() string {
	if v := strings.TrimSpace(os.Getenv("IVALDI_GITHUB_SCOPES")); v != "" {
		return v
	}
	return GitHubScopes
}

// GetToken returns the current token for a platform if available
func GetToken(platform Platform) (string, error) {
	store, err := NewTokenStore()
	if err != nil {
		return "", err
	}

	token, err := store.LoadToken(platform)
	if err != nil {
		return "", err
	}

	if token == nil {
		return "", nil
	}

	return token.AccessToken, nil
}

// GetTokenType returns the stored token type for a platform if available.
func GetTokenType(platform Platform) (string, error) {
	store, err := NewTokenStore()
	if err != nil {
		return "", err
	}

	token, err := store.LoadToken(platform)
	if err != nil {
		return "", err
	}

	if token == nil {
		return "", nil
	}

	return token.TokenType, nil
}

// IsAuthenticated checks if the user is authenticated for a specific platform
func IsAuthenticated(platform Platform) bool {
	token, err := GetToken(platform)
	return err == nil && token != ""
}

// AuthMethod represents different authentication methods
type AuthMethod struct {
	Name        string
	Description string
	Token       string
}

// GetAuthMethod returns the active authentication method for a specific platform
func GetAuthMethod(platform Platform) *AuthMethod {
	// 1. Check Ivaldi OAuth token
	if token, err := GetToken(platform); err == nil && token != "" {
		platformName := string(platform)
		return &AuthMethod{
			Name:        "ivaldi",
			Description: fmt.Sprintf("Authenticated via 'ivaldi auth login --%s'", platformName),
			Token:       token,
		}
	}

	// Platform-specific checks
	switch platform {
	case PlatformGitHub:
		return getGitHubAuthMethod()
	case PlatformGitLab:
		return getGitLabAuthMethod()
	}

	return nil
}

// getGitHubAuthMethod checks GitHub-specific authentication methods
func getGitHubAuthMethod() *AuthMethod {
	// 2. Check environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return &AuthMethod{
			Name:        "env",
			Description: "Authenticated via GITHUB_TOKEN environment variable",
			Token:       token,
		}
	}

	// 3. Check git config for github token
	if token := getGitConfig("github.token"); token != "" {
		return &AuthMethod{
			Name:        "git-config",
			Description: "Authenticated via git config (github.token)",
			Token:       token,
		}
	}

	// 4. Try to read from git credential helper
	if token := getGitCredential("github.com"); token != "" {
		return &AuthMethod{
			Name:        "git-credential",
			Description: "Authenticated via git credential helper",
			Token:       token,
		}
	}

	// 5. Check .netrc file
	if token := getNetrcToken("github.com"); token != "" {
		return &AuthMethod{
			Name:        "netrc",
			Description: "Authenticated via .netrc file",
			Token:       token,
		}
	}

	// 6. Check gh CLI config
	if token := getGHCLIToken(); token != "" {
		return &AuthMethod{
			Name:        "gh-cli",
			Description: "Authenticated via 'gh auth login' (GitHub CLI)",
			Token:       token,
		}
	}

	return nil
}

// getGitLabAuthMethod checks GitLab-specific authentication methods
func getGitLabAuthMethod() *AuthMethod {
	// 2. Check environment variable
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		return &AuthMethod{
			Name:        "env",
			Description: "Authenticated via GITLAB_TOKEN environment variable",
			Token:       token,
		}
	}

	// 3. Check git config for gitlab token
	if token := getGitConfig("gitlab.token"); token != "" {
		return &AuthMethod{
			Name:        "git-config",
			Description: "Authenticated via git config (gitlab.token)",
			Token:       token,
		}
	}

	// 4. Try to read from git credential helper
	if token := getGitCredential("gitlab.com"); token != "" {
		return &AuthMethod{
			Name:        "git-credential",
			Description: "Authenticated via git credential helper",
			Token:       token,
		}
	}

	// 5. Check .netrc file
	if token := getNetrcToken("gitlab.com"); token != "" {
		return &AuthMethod{
			Name:        "netrc",
			Description: "Authenticated via .netrc file",
			Token:       token,
		}
	}

	// 6. Check glab CLI config (GitLab CLI)
	if token := getGLabCLIToken(); token != "" {
		return &AuthMethod{
			Name:        "glab-cli",
			Description: "Authenticated via 'glab auth login' (GitLab CLI)",
			Token:       token,
		}
	}

	return nil
}

// Helper functions for GetAuthMethod
func getGitConfig(key string) string {
	cmd := exec.Command("git", "config", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func getGitCredential(host string) string {
	// Use a context with timeout to prevent hanging on interactive prompts
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "credential", "fill")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("protocol=https\nhost=%s\n\n", host))

	// Prevent any terminal interaction by setting these
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "password=") {
			return strings.TrimPrefix(line, "password=")
		}
	}

	return ""
}

func getNetrcToken(machine string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	netrcPath := filepath.Join(home, ".netrc")
	content, err := os.ReadFile(netrcPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	inMachine := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "machine ") && strings.Contains(line, machine) {
			inMachine = true
		} else if inMachine && strings.HasPrefix(line, "password ") {
			return strings.TrimPrefix(line, "password ")
		} else if strings.HasPrefix(line, "machine ") {
			inMachine = false
		}
	}

	return ""
}

func getGHCLIToken() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	ghConfigPath := filepath.Join(home, ".config", "gh", "hosts.yml")
	content, err := os.ReadFile(ghConfigPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, "oauth_token:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "token:") && i > 0 && strings.Contains(lines[i-1], "github.com") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

func getGLabCLIToken() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	glabConfigPath := filepath.Join(home, ".config", "glab-cli", "config.yml")
	content, err := os.ReadFile(glabConfigPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, "token:") && i > 0 && strings.Contains(lines[i-1], "gitlab.com") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

// Login performs the OAuth device flow login for a specific platform
func Login(ctx context.Context, platform Platform) error {
	platformName := string(platform)
	fmt.Printf("Initiating %s authentication...\n", platformName)

	deviceCode, err := RequestDeviceCode(ctx, platform)
	if err != nil {
		return fmt.Errorf("failed to start authentication: %w", err)
	}

	fmt.Printf("\nFirst, copy your one-time code: %s\n", deviceCode.UserCode)
	fmt.Printf("Then visit: %s\n", deviceCode.VerificationURI)
	fmt.Println("\nWaiting for authentication...")

	token, err := PollForAccessToken(ctx, platform, deviceCode.DeviceCode, deviceCode.Interval)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	store, err := NewTokenStore()
	if err != nil {
		return err
	}

	if err := store.SaveToken(platform, token); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Printf("\n%s authentication successful!\n", platformName)
	return nil
}

// Logout removes the stored authentication token for a specific platform
func Logout(platform Platform) error {
	store, err := NewTokenStore()
	if err != nil {
		return err
	}

	if err := store.DeleteToken(platform); err != nil {
		return err
	}

	platformName := string(platform)
	fmt.Printf("Logged out from %s successfully\n", platformName)
	return nil
}
