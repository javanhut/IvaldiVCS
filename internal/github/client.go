// Package github provides GitHub API integration for Ivaldi VCS
// It operates independently from Git but can use Git credentials
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	GitHubAPIURL = "https://api.github.com"
	AcceptHeader = "application/vnd.github.v3+json"
)

// Client represents a GitHub API client
type Client struct {
	httpClient  *http.Client
	baseURL     string
	token       string
	username    string
	rateLimiter *RateLimiter
}

// RateLimiter tracks API rate limits
type RateLimiter struct {
	Remaining int
	Limit     int
	Reset     time.Time
}

// Repository represents a GitHub repository
type Repository struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	Private       bool      `json:"private"`
	DefaultBranch string    `json:"default_branch"`
	CloneURL      string    `json:"clone_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Size          int       `json:"size"`
}

// Branch represents a GitHub branch
type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Commit    struct {
		SHA string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
}

// FileContent represents a file's content from GitHub
type FileContent struct {
	Type        string `json:"type"`
	Encoding    string `json:"encoding"`
	Size        int    `json:"size"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	SHA         string `json:"sha"`
	URL         string `json:"url"`
	GitURL      string `json:"git_url"`
	HTMLURL     string `json:"html_url"`
	DownloadURL string `json:"download_url"`
}

// TreeEntry represents an entry in a Git tree
type TreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int    `json:"size,omitempty"`
	SHA  string `json:"sha"`
	URL  string `json:"url,omitempty"`
}

// Tree represents a Git tree structure
type Tree struct {
	SHA       string      `json:"sha"`
	URL       string      `json:"url"`
	Tree      []TreeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// NewClient creates a new GitHub API client
func NewClient() (*Client, error) {
	// Try to get authentication from various sources
	token := getAuthToken()
	username := getUsername()

	if token == "" {
		return nil, fmt.Errorf("no GitHub authentication found. Please set GITHUB_TOKEN environment variable or configure git credentials")
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:     GitHubAPIURL,
		token:       token,
		username:    username,
		rateLimiter: &RateLimiter{},
	}, nil
}

// getAuthToken attempts to get GitHub auth token from various sources
func getAuthToken() string {
	// 1. Check environment variable (highest priority)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	// 2. Check git config for github token
	if token := getGitConfig("github.token"); token != "" {
		return token
	}

	// 3. Try to read from git credential helper
	if token := getGitCredential("github.com"); token != "" {
		return token
	}

	// 4. Check .netrc file
	if token := getNetrcToken("github.com"); token != "" {
		return token
	}

	// 5. Check gh CLI config
	if token := getGHCLIToken(); token != "" {
		return token
	}

	return ""
}

// getUsername attempts to get GitHub username
func getUsername() string {
	// 1. From environment
	if user := os.Getenv("GITHUB_USER"); user != "" {
		return user
	}

	// 2. From git config
	if user := getGitConfig("github.user"); user != "" {
		return user
	}

	// 3. From global git config
	if user := getGitConfig("user.name"); user != "" {
		return user
	}

	return ""
}

// getGitConfig reads a git config value
func getGitConfig(key string) string {
	cmd := exec.Command("git", "config", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getGitCredential uses git credential helper to get credentials
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

// getNetrcToken reads token from .netrc file
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

// getGHCLIToken reads token from GitHub CLI config
func getGHCLIToken() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check gh cli config file
	ghConfigPath := filepath.Join(home, ".config", "gh", "hosts.yml")
	content, err := os.ReadFile(ghConfigPath)
	if err != nil {
		return ""
	}

	// Simple extraction - proper implementation would use YAML parser
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

// doRequest performs an authenticated API request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", AcceptHeader)
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Update rate limit info
	c.updateRateLimits(resp)

	// Check for API errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// updateRateLimits updates rate limit information from response headers
func (c *Client) updateRateLimits(resp *http.Response) {
	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		fmt.Sscanf(remaining, "%d", &c.rateLimiter.Remaining)
	}
	if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
		fmt.Sscanf(limit, "%d", &c.rateLimiter.Limit)
	}
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		var timestamp int64
		fmt.Sscanf(reset, "%d", &timestamp)
		c.rateLimiter.Reset = time.Unix(timestamp, 0)
	}
}

// GetRepository fetches repository information
func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var repository Repository
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return nil, fmt.Errorf("failed to decode repository: %w", err)
	}

	return &repository, nil
}

// GetBranch fetches branch information
func (c *Client) GetBranch(ctx context.Context, owner, repo, branch string) (*Branch, error) {
	path := fmt.Sprintf("/repos/%s/%s/branches/%s", owner, repo, branch)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var b Branch
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return nil, fmt.Errorf("failed to decode branch: %w", err)
	}

	return &b, nil
}

// GetFileContent fetches a file's content from a repository
func (c *Client) GetFileContent(ctx context.Context, owner, repo, path, ref string) (*FileContent, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	if ref != "" {
		apiPath += fmt.Sprintf("?ref=%s", ref)
	}

	resp, err := c.doRequest(ctx, "GET", apiPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var content FileContent
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	return &content, nil
}

// GetTree fetches the tree structure of a repository
func (c *Client) GetTree(ctx context.Context, owner, repo, sha string, recursive bool) (*Tree, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/trees/%s", owner, repo, sha)
	if recursive {
		path += "?recursive=1"
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tree Tree
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("failed to decode tree: %w", err)
	}

	return &tree, nil
}

// DownloadFile downloads raw file content
func (c *Client) DownloadFile(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	// Use raw content endpoint
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	if ref != "" {
		apiPath += fmt.Sprintf("?ref=%s", ref)
	}

	resp, err := c.doRequest(ctx, "GET", apiPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var content FileContent
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	// Decode base64 content
	if content.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(content.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 content: %w", err)
		}
		return decoded, nil
	}

	return []byte(content.Content), nil
}

// GetRateLimit returns current rate limit status
func (c *Client) GetRateLimit() *RateLimiter {
	return c.rateLimiter
}

// IsRateLimited checks if we're currently rate limited
func (c *Client) IsRateLimited() bool {
	if c.rateLimiter.Remaining == 0 && time.Now().Before(c.rateLimiter.Reset) {
		return true
	}
	return false
}

// WaitForRateLimit waits if rate limited
func (c *Client) WaitForRateLimit() {
	if c.IsRateLimited() {
		waitTime := time.Until(c.rateLimiter.Reset)
		fmt.Printf("Rate limited. Waiting %v until reset...\n", waitTime)
		time.Sleep(waitTime)
	}
}

// FileUploadRequest represents a request to upload/update a file
type FileUploadRequest struct {
	Message string `json:"message"`
	Content string `json:"content"`
	SHA     string `json:"sha,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

// UploadFile uploads or updates a file in a repository
func (c *Client) UploadFile(ctx context.Context, owner, repo, path string, req FileUploadRequest) error {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)

	method := "PUT"
	resp, err := c.doRequest(ctx, method, apiPath, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// TestAuth tests if authentication is working
func (c *Client) TestAuth(ctx context.Context) error {
	resp, err := c.doRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
