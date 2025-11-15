// Package gitlab provides GitLab API integration for Ivaldi VCS
// It operates independently from Git but can use Git credentials
package gitlab

import (
	"bytes"
	"context"
	"encoding/base64"
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

	"github.com/javanhut/Ivaldi-vcs/internal/auth"
)

const (
	GitLabAPIURL = "https://gitlab.com/api/v4"
)

// Client represents a GitLab API client
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

// Project represents a GitLab project (repository)
type Project struct {
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	PathWithNamespace string    `json:"path_with_namespace"`
	Description       string    `json:"description"`
	Visibility        string    `json:"visibility"`
	DefaultBranch     string    `json:"default_branch"`
	HTTPURLToRepo     string    `json:"http_url_to_repo"`
	CreatedAt         time.Time `json:"created_at"`
	LastActivityAt    time.Time `json:"last_activity_at"`
}

// Branch represents a GitLab branch
type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Commit    struct {
		ID string `json:"id"`
	} `json:"commit"`
}

// Commit represents a GitLab commit
type Commit struct {
	ID             string    `json:"id"`
	ShortID        string    `json:"short_id"`
	Message        string    `json:"message"`
	Title          string    `json:"title"`
	AuthorName     string    `json:"author_name"`
	AuthorEmail    string    `json:"author_email"`
	AuthoredDate   time.Time `json:"authored_date"`
	CommitterName  string    `json:"committer_name"`
	CommitterEmail string    `json:"committer_email"`
	CommittedDate  time.Time `json:"committed_date"`
	CreatedAt      time.Time `json:"created_at"`
	ParentIDs      []string  `json:"parent_ids"`
}

// Tag represents a GitLab tag
type Tag struct {
	Name      string `json:"name"`
	Message   string `json:"message"`
	Target    string `json:"target"`
	Commit    Commit `json:"commit"`
	Release   *Release `json:"release,omitempty"`
	Protected bool   `json:"protected"`
}

// Release represents a GitLab release
type Release struct {
	TagName     string `json:"tag_name"`
	Description string `json:"description"`
}

// TreeEntry represents an entry in a Git tree
type TreeEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Mode string `json:"mode"`
}

// FileContent represents a file's content from GitLab
type FileContent struct {
	FileName     string `json:"file_name"`
	FilePath     string `json:"file_path"`
	Size         int    `json:"size"`
	Encoding     string `json:"encoding"`
	Content      string `json:"content"`
	ContentSHA256 string `json:"content_sha256"`
	Ref          string `json:"ref"`
	BlobID       string `json:"blob_id"`
}

// Blob represents a GitLab blob
type Blob struct {
	Content   string `json:"content"`
	Encoding  string `json:"encoding"`
	SHA       string `json:"sha"`
	Size      int    `json:"size"`
}

// Tree represents a GitLab repository tree
type Tree struct {
	ID   string       `json:"id"`
	Name string       `json:"name"`
	Type string       `json:"type"`
	Path string       `json:"path"`
	Mode string       `json:"mode"`
}

// NewClient creates a new GitLab API client
func NewClient(owner, repo, token string) (*Client, error) {
	return NewClientWithURL(owner, repo, token, GitLabAPIURL)
}

// NewClientWithURL creates a new GitLab API client with a custom base URL
func NewClientWithURL(owner, repo, token, baseURL string) (*Client, error) {
	// Ensure base URL ends with /api/v4
	if !strings.HasSuffix(baseURL, "/api/v4") {
		// Remove trailing slash if present
		baseURL = strings.TrimSuffix(baseURL, "/")
		// Add https:// if no protocol specified
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "https://" + baseURL
		}
		baseURL = baseURL + "/api/v4"
	}

	// Extract host from baseURL for authentication
	host := "gitlab.com" // default
	if parsedURL, err := url.Parse(baseURL); err == nil && parsedURL.Host != "" {
		host = parsedURL.Host
	}

	// If no token provided, try to get one
	if token == "" {
		token = getAuthToken(host)
	}

	username := owner

	return &Client{
		httpClient:  &http.Client{Timeout: 60 * time.Second},
		baseURL:     baseURL,
		token:       token,
		username:    username,
		rateLimiter: &RateLimiter{},
	}, nil
}

// NewClientFromURL creates a GitLab client from a GitLab URL
func NewClientFromURL(urlStr string) (*Client, error) {
	owner, repo, err := ParseGitLabURL(urlStr)
	if err != nil {
		return nil, err
	}
	return NewClient(owner, repo, "")
}

// getAuthToken attempts to get GitLab auth token from various sources
func getAuthToken(host string) string {
	// 1. Check Ivaldi OAuth token (highest priority)
	if token, err := auth.GetToken(auth.PlatformGitLab); err == nil && token != "" {
		return token
	}

	// 2. Check environment variable
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		return token
	}

	// 3. Check git config for gitlab token
	if token := getGitConfig("gitlab.token"); token != "" {
		return token
	}

	// 4. Try to read from git credential helper
	if token := getGitCredential(host); token != "" {
		return token
	}

	// 5. Check .netrc file
	if token := getNetrcToken(host); token != "" {
		return token
	}

	// 6. Check glab CLI config
	if token := getGLabCLIToken(); token != "" {
		return token
	}

	return ""
}

// Helper functions for authentication
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

	// Disable interactive prompts to prevent user from being prompted
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

// ParseGitLabURL extracts owner, repo, and host from a GitLab URL
func ParseGitLabURL(urlStr string) (owner, repo string, err error) {
	owner, repo, _, err = ParseGitLabURLWithHost(urlStr)
	return owner, repo, err
}

// ParseGitLabURLWithHost extracts owner, repo, and host from a GitLab URL
func ParseGitLabURLWithHost(urlStr string) (owner, repo, host string, err error) {
	urlStr = strings.TrimSpace(urlStr)

	// Remove .git suffix if present
	urlStr = strings.TrimSuffix(urlStr, ".git")

	// Default to gitlab.com
	host = "gitlab.com"

	// Check if it's a full URL with protocol
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		parsedURL, parseErr := url.Parse(urlStr)
		if parseErr != nil {
			return "", "", "", fmt.Errorf("invalid URL: %s", urlStr)
		}
		host = parsedURL.Host
		urlStr = strings.TrimPrefix(parsedURL.Path, "/")
	} else {
		// Check for host/owner/repo format
		parts := strings.Split(urlStr, "/")
		if len(parts) >= 3 {
			// First part might be the host
			if strings.Contains(parts[0], ".") {
				host = parts[0]
				urlStr = strings.Join(parts[1:], "/")
			}
		}
		// Remove gitlab.com prefix if present
		urlStr = strings.TrimPrefix(urlStr, "gitlab.com/")
	}

	// Split into parts
	parts := strings.Split(urlStr, "/")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid GitLab URL: %s (expected format: owner/repo)", urlStr)
	}

	// Last two parts should be owner/repo
	owner = parts[len(parts)-2]
	repo = parts[len(parts)-1]

	return owner, repo, host, nil
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Update rate limit info from headers
	c.updateRateLimitFromResponse(resp)

	// Check for rate limit error
	if resp.StatusCode == http.StatusTooManyRequests {
		return resp, fmt.Errorf("API rate limit exceeded. Please authenticate to increase rate limits. Run: ivaldi auth login --gitlab")
	}

	return resp, nil
}

// updateRateLimitFromResponse updates rate limit info from response headers
func (c *Client) updateRateLimitFromResponse(resp *http.Response) {
	// GitLab uses different rate limit headers than GitHub
	// RateLimit-Remaining, RateLimit-Limit, RateLimit-Reset
	if remaining := resp.Header.Get("RateLimit-Remaining"); remaining != "" {
		fmt.Sscanf(remaining, "%d", &c.rateLimiter.Remaining)
	}
	if limit := resp.Header.Get("RateLimit-Limit"); limit != "" {
		fmt.Sscanf(limit, "%d", &c.rateLimiter.Limit)
	}
}

// GetProject fetches information about a project
func (c *Client) GetProject(ctx context.Context, owner, repo string) (*Project, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")
	endpoint := fmt.Sprintf("/projects/%s", encodedPath)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(body))
	}

	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to decode project: %w", err)
	}

	return &project, nil
}

// GetBranch fetches information about a specific branch
func (c *Client) GetBranch(ctx context.Context, owner, repo, branch string) (*Branch, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")
	encodedBranch := strings.ReplaceAll(branch, "/", "%2F")
	endpoint := fmt.Sprintf("/projects/%s/repository/branches/%s", encodedPath, encodedBranch)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(body))
	}

	var branchInfo Branch
	if err := json.NewDecoder(resp.Body).Decode(&branchInfo); err != nil {
		return nil, fmt.Errorf("failed to decode branch: %w", err)
	}

	return &branchInfo, nil
}

// ListBranches lists all branches in a repository
func (c *Client) ListBranches(ctx context.Context, owner, repo string) ([]Branch, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")
	endpoint := fmt.Sprintf("/projects/%s/repository/branches", encodedPath)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(body))
	}

	var branches []Branch
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return nil, fmt.Errorf("failed to decode branches: %w", err)
	}

	return branches, nil
}

// GetCommit fetches information about a specific commit
func (c *Client) GetCommit(ctx context.Context, owner, repo, sha string) (*Commit, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")
	endpoint := fmt.Sprintf("/projects/%s/repository/commits/%s", encodedPath, sha)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(body))
	}

	var commit Commit
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return nil, fmt.Errorf("failed to decode commit: %w", err)
	}

	return &commit, nil
}

// GetTree fetches the repository tree at a specific ref
func (c *Client) GetTree(ctx context.Context, owner, repo, ref string, recursive bool) ([]TreeEntry, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")
	endpoint := fmt.Sprintf("/projects/%s/repository/tree", encodedPath)

	// Add query parameters
	params := fmt.Sprintf("?ref=%s&per_page=100", ref)
	if recursive {
		params += "&recursive=true"
	}
	endpoint += params

	var allEntries []TreeEntry
	for {
		resp, err := c.doRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(body))
		}

		var entries []TreeEntry
		if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode tree: %w", err)
		}
		resp.Body.Close()

		allEntries = append(allEntries, entries...)

		// Check for next page
		nextLink := resp.Header.Get("Link")
		if nextLink == "" || !strings.Contains(nextLink, `rel="next"`) {
			break
		}

		// Parse next page URL from Link header
		for _, link := range strings.Split(nextLink, ",") {
			if strings.Contains(link, `rel="next"`) {
				start := strings.Index(link, "<") + 1
				end := strings.Index(link, ">")
				if start > 0 && end > start {
					endpoint = link[start:end]
					break
				}
			}
		}
	}

	return allEntries, nil
}

// GetFile fetches a file's content
func (c *Client) GetFile(ctx context.Context, owner, repo, path, ref string) (*FileContent, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")
	encodedFilePath := strings.ReplaceAll(path, "/", "%2F")
	endpoint := fmt.Sprintf("/projects/%s/repository/files/%s?ref=%s", encodedPath, encodedFilePath, ref)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(body))
	}

	var file FileContent
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, fmt.Errorf("failed to decode file: %w", err)
	}

	return &file, nil
}

// DownloadFile downloads a file's raw content
func (c *Client) DownloadFile(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	fileContent, err := c.GetFile(ctx, owner, repo, path, ref)
	if err != nil {
		return nil, err
	}

	// Decode base64 content
	if fileContent.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(fileContent.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode file content: %w", err)
		}
		return decoded, nil
	}

	return []byte(fileContent.Content), nil
}

// CreateBlob creates a new blob in the repository
func (c *Client) CreateBlob(ctx context.Context, owner, repo string, content []byte) (string, error) {
	// GitLab doesn't have a direct blob creation API like GitHub
	// This would be used as part of a commit creation process
	// For now, return an error indicating it's not directly supported
	return "", fmt.Errorf("GitLab does not support direct blob creation; use CreateCommit instead")
}

// CreateTree creates a new tree in the repository
func (c *Client) CreateTree(ctx context.Context, owner, repo string, baseTree string, entries []TreeEntry) (string, error) {
	// GitLab doesn't have a direct tree creation API like GitHub
	// Trees are created as part of commits
	return "", fmt.Errorf("GitLab does not support direct tree creation; use CreateCommit instead")
}

// CreateCommit creates a new commit in the repository
func (c *Client) CreateCommit(ctx context.Context, owner, repo, branch, message string, actions []CommitAction) (*Commit, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")
	endpoint := fmt.Sprintf("/projects/%s/repository/commits", encodedPath)

	commitData := map[string]interface{}{
		"branch":         branch,
		"commit_message": message,
		"actions":        actions,
	}

	jsonData, err := json.Marshal(commitData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal commit data: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(body))
	}

	var commit Commit
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return nil, fmt.Errorf("failed to decode commit: %w", err)
	}

	return &commit, nil
}

// CommitAction represents an action in a commit
type CommitAction struct {
	Action          string `json:"action"` // create, delete, move, update, chmod
	FilePath        string `json:"file_path"`
	PreviousPath    string `json:"previous_path,omitempty"`
	Content         string `json:"content,omitempty"`
	Encoding        string `json:"encoding,omitempty"` // text or base64
	LastCommitID    string `json:"last_commit_id,omitempty"`
	ExecuteFilemode bool   `json:"execute_filemode,omitempty"`
}

// UpdateRef updates a branch reference
func (c *Client) UpdateRef(ctx context.Context, owner, repo, branch, sha string) error {
	// In GitLab, updating a ref is done through protected branches or by creating commits
	// This is typically handled through the commit creation process
	return fmt.Errorf("GitLab does not support direct ref updates; use CreateCommit instead")
}

// ListCommits fetches commits from a repository branch with optional depth limit
func (c *Client) ListCommits(ctx context.Context, owner, repo, branch string, depth int) ([]*Commit, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")

	commits := make([]*Commit, 0)
	page := 1
	perPage := 100

	for {
		endpoint := fmt.Sprintf("/projects/%s/repository/commits?ref_name=%s&per_page=%d&page=%d", encodedPath, branch, perPage, page)
		resp, err := c.doRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(body))
		}

		var pageCommits []*Commit
		if err := json.NewDecoder(resp.Body).Decode(&pageCommits); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode commits: %w", err)
		}
		resp.Body.Close()

		for _, commit := range pageCommits {
			commits = append(commits, commit)

			if depth > 0 && len(commits) >= depth {
				return commits, nil
			}
		}

		if len(pageCommits) < perPage {
			break
		}

		page++
	}

	return commits, nil
}

// ListTags fetches all tags from a repository
func (c *Client) ListTags(ctx context.Context, owner, repo string) ([]*Tag, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")

	tags := make([]*Tag, 0)
	page := 1
	perPage := 100

	for {
		endpoint := fmt.Sprintf("/projects/%s/repository/tags?per_page=%d&page=%d", encodedPath, perPage, page)
		resp, err := c.doRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(body))
		}

		var pageTags []*Tag
		if err := json.NewDecoder(resp.Body).Decode(&pageTags); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode tags: %w", err)
		}
		resp.Body.Close()

		for _, tag := range pageTags {
			tags = append(tags, tag)
		}

		if len(pageTags) < perPage {
			break
		}

		page++
	}

	return tags, nil
}
