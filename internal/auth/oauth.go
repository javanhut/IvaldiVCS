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

const (
	// GitHub OAuth App credentials for Ivaldi VCS
	// Note: These would need to be registered with GitHub
	ClientID     = "Iv1.b507a08c87ecfe98" // This is a placeholder - you'll need to register your app
	DeviceCodeURL = "https://github.com/login/device/code"
	AccessTokenURL = "https://github.com/login/oauth/access_token"
	Scopes        = "repo,read:user,user:email"
)

// TokenStore manages OAuth tokens
type TokenStore struct {
	configPath string
}

// Token represents an OAuth token
type Token struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
	CreatedAt   time.Time `json:"created_at"`
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

// LoadToken loads the stored token
func (ts *TokenStore) LoadToken() (*Token, error) {
	data, err := os.ReadFile(ts.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read token: %w", err)
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	return &token, nil
}

// SaveToken saves the token to disk
func (ts *TokenStore) SaveToken(token *Token) error {
	token.CreatedAt = time.Now()

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(ts.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}

	return nil
}

// DeleteToken removes the stored token
func (ts *TokenStore) DeleteToken() error {
	if err := os.Remove(ts.configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	return nil
}

// RequestDeviceCode initiates the OAuth device flow
func RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", ClientID)
	data.Set("scope", Scopes)

	req, err := http.NewRequestWithContext(ctx, "POST", DeviceCodeURL, strings.NewReader(data.Encode()))
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

// PollForAccessToken polls GitHub for the access token
func PollForAccessToken(ctx context.Context, deviceCode string, interval int) (*Token, error) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := checkAccessToken(ctx, deviceCode)
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
func checkAccessToken(ctx context.Context, deviceCode string) (*Token, error) {
	data := url.Values{}
	data.Set("client_id", ClientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequestWithContext(ctx, "POST", AccessTokenURL, strings.NewReader(data.Encode()))
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

// GetToken returns the current token if available
func GetToken() (string, error) {
	store, err := NewTokenStore()
	if err != nil {
		return "", err
	}

	token, err := store.LoadToken()
	if err != nil {
		return "", err
	}

	if token == nil {
		return "", nil
	}

	return token.AccessToken, nil
}

// IsAuthenticated checks if the user is authenticated
func IsAuthenticated() bool {
	token, err := GetToken()
	return err == nil && token != ""
}

// AuthMethod represents different authentication methods
type AuthMethod struct {
	Name        string
	Description string
	Token       string
}

// GetAuthMethod returns the active authentication method
func GetAuthMethod() *AuthMethod {
	// 1. Check Ivaldi OAuth token
	if token, err := GetToken(); err == nil && token != "" {
		return &AuthMethod{
			Name:        "ivaldi",
			Description: "Authenticated via 'ivaldi auth login'",
			Token:       token,
		}
	}

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
	cmd := exec.Command("git", "credential", "fill")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("protocol=https\nhost=%s\n\n", host))

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

// Login performs the OAuth device flow login
func Login(ctx context.Context) error {
	fmt.Println("Initiating GitHub authentication...")

	deviceCode, err := RequestDeviceCode(ctx)
	if err != nil {
		return fmt.Errorf("failed to start authentication: %w", err)
	}

	fmt.Printf("\nFirst, copy your one-time code: %s\n", deviceCode.UserCode)
	fmt.Printf("Then visit: %s\n", deviceCode.VerificationURI)
	fmt.Println("\nWaiting for authentication...")

	token, err := PollForAccessToken(ctx, deviceCode.DeviceCode, deviceCode.Interval)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	store, err := NewTokenStore()
	if err != nil {
		return err
	}

	if err := store.SaveToken(token); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println("\nAuthentication successful!")
	return nil
}

// Logout removes the stored authentication token
func Logout() error {
	store, err := NewTokenStore()
	if err != nil {
		return err
	}

	if err := store.DeleteToken(); err != nil {
		return err
	}

	fmt.Println("Logged out successfully")
	return nil
}
