package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

func TestPushCommitCreatesCommitForSameTreeSeal(t *testing.T) {
	rs, workDir := setupSyncCommitTestRepo(t)

	filePath := filepath.Join(workDir, "notes.txt")
	if err := os.WriteFile(filePath, []byte("same-tree"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := rs.createIvaldiCommit("first seal", "main", ""); err != nil {
		t.Fatalf("failed to create first local commit: %v", err)
	}
	if err := rs.createIvaldiCommit("second seal", "main", ""); err != nil {
		t.Fatalf("failed to create second local commit: %v", err)
	}

	headHash := getMainHeadHash(t, rs.ivaldiDir)

	const (
		owner        = "octo"
		repo         = "nest"
		remoteParent = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		remoteTree   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		newCommitSHA = "cccccccccccccccccccccccccccccccccccccccc"
	)

	createCommitCalled := false
	updateRefCalled := false
	blobCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/repos/%s/%s/branches/main", owner, repo):
			writeJSON(t, w, http.StatusOK, map[string]any{
				"name": "main",
				"commit": map[string]any{
					"sha": remoteParent,
					"url": "http://example.test/commit",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/repos/%s/%s/git/commits/%s", owner, repo, remoteParent):
			writeJSON(t, w, http.StatusOK, map[string]any{
				"sha": remoteParent,
				"tree": map[string]any{
					"sha": remoteTree,
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/repos/%s/%s/git/commits", owner, repo):
			createCommitCalled = true
			var req CreateCommitRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode create commit request: %v", err)
			}
			if req.Tree != remoteTree {
				t.Fatalf("expected commit tree %s, got %s", remoteTree, req.Tree)
			}
			if len(req.Parents) != 1 || req.Parents[0] != remoteParent {
				t.Fatalf("expected single parent %s, got %#v", remoteParent, req.Parents)
			}
			if req.Message != "second seal" {
				t.Fatalf("expected message 'second seal', got %q", req.Message)
			}
			writeJSON(t, w, http.StatusCreated, map[string]any{
				"sha":     newCommitSHA,
				"url":     "http://example.test/new-commit",
				"message": req.Message,
			})
		case r.Method == http.MethodPatch && r.URL.Path == fmt.Sprintf("/repos/%s/%s/git/refs/heads/main", owner, repo):
			updateRefCalled = true
			var req UpdateRefRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode update ref request: %v", err)
			}
			if req.SHA != newCommitSHA {
				t.Fatalf("expected ref update SHA %s, got %s", newCommitSHA, req.SHA)
			}
			writeJSON(t, w, http.StatusOK, map[string]any{"ref": "refs/heads/main"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/git/blobs"):
			blobCalled = true
			writeJSON(t, w, http.StatusInternalServerError, map[string]any{"message": "unexpected blob upload"})
		default:
			writeJSON(t, w, http.StatusNotFound, map[string]any{
				"message": fmt.Sprintf("unexpected endpoint: %s %s", r.Method, r.URL.Path),
			})
		}
	}))
	defer server.Close()

	rs.client = &Client{
		httpClient:  server.Client(),
		baseURL:     server.URL,
		rateLimiter: &RateLimiter{},
	}

	err := rs.PushCommit(context.Background(), owner, repo, "main", headHash, false, "main")
	if err != nil {
		t.Fatalf("PushCommit failed: %v", err)
	}

	if !createCommitCalled {
		t.Fatal("expected create commit to be called for same-tree seal")
	}
	if !updateRefCalled {
		t.Fatal("expected branch ref update call")
	}
	if blobCalled {
		t.Fatal("did not expect blob uploads for same-tree seal commit")
	}

	if got := getMainTimelineGitSHA(t, rs.ivaldiDir); got != newCommitSHA {
		t.Fatalf("expected timeline git SHA %s, got %s", newCommitSHA, got)
	}
}

func TestPushCommitWaitsForGitDataAfterBootstrap(t *testing.T) {
	rs, workDir := setupSyncCommitTestRepo(t)

	filePath := filepath.Join(workDir, "notes.txt")
	if err := os.WriteFile(filePath, []byte("initial"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := rs.createIvaldiCommit("initial seal", "main", ""); err != nil {
		t.Fatalf("failed to create local commit: %v", err)
	}

	headHash := getMainHeadHash(t, rs.ivaldiDir)

	const (
		owner           = "octo"
		repo            = "nest"
		bootstrapSHA    = "dddddddddddddddddddddddddddddddddddddddd"
		bootstrapTree   = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		uploadedBlobSHA = "ffffffffffffffffffffffffffffffffffffffff"
		finalTreeSHA    = "1111111111111111111111111111111111111111"
		finalCommitSHA  = "2222222222222222222222222222222222222222"
	)

	gitDataReady := false
	readinessChecks := 0
	blobCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/repos/%s/%s/branches/main", owner, repo):
			writeJSON(t, w, http.StatusNotFound, map[string]any{"message": "branch not found"})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/repos/%s/%s", owner, repo):
			writeJSON(t, w, http.StatusOK, map[string]any{
				"name":           repo,
				"full_name":      owner + "/" + repo,
				"default_branch": "main",
			})
		case r.Method == http.MethodPut && r.URL.Path == fmt.Sprintf("/repos/%s/%s/contents/.ivaldi-bootstrap", owner, repo):
			writeJSON(t, w, http.StatusCreated, map[string]any{
				"commit": map[string]any{
					"sha": bootstrapSHA,
					"tree": map[string]any{
						"sha": bootstrapTree,
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/repos/%s/%s/git/commits/%s", owner, repo, bootstrapSHA):
			readinessChecks++
			if readinessChecks < 3 {
				writeJSON(t, w, http.StatusNotFound, map[string]any{"message": "commit not visible yet"})
				return
			}
			gitDataReady = true
			writeJSON(t, w, http.StatusOK, map[string]any{
				"sha": bootstrapSHA,
				"tree": map[string]any{
					"sha": bootstrapTree,
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/repos/%s/%s/git/blobs", owner, repo):
			blobCalls++
			if !gitDataReady {
				writeJSON(t, w, http.StatusConflict, map[string]any{"message": "Git Repository is empty."})
				return
			}
			writeJSON(t, w, http.StatusCreated, map[string]any{
				"sha": uploadedBlobSHA,
				"url": "http://example.test/blob",
			})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/repos/%s/%s/git/trees", owner, repo):
			writeJSON(t, w, http.StatusCreated, map[string]any{
				"sha": finalTreeSHA,
				"url": "http://example.test/tree",
			})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/repos/%s/%s/git/commits", owner, repo):
			writeJSON(t, w, http.StatusCreated, map[string]any{
				"sha": finalCommitSHA,
				"url": "http://example.test/commit",
			})
		case r.Method == http.MethodPatch && r.URL.Path == fmt.Sprintf("/repos/%s/%s/git/refs/heads/main", owner, repo):
			writeJSON(t, w, http.StatusOK, map[string]any{"ref": "refs/heads/main"})
		default:
			writeJSON(t, w, http.StatusNotFound, map[string]any{
				"message": fmt.Sprintf("unexpected endpoint: %s %s", r.Method, r.URL.Path),
			})
		}
	}))
	defer server.Close()

	rs.client = &Client{
		httpClient:  server.Client(),
		baseURL:     server.URL,
		rateLimiter: &RateLimiter{},
	}

	err := rs.PushCommit(context.Background(), owner, repo, "main", headHash, false, "main")
	if err != nil {
		t.Fatalf("PushCommit failed: %v", err)
	}

	if readinessChecks < 3 {
		t.Fatalf("expected readiness polling before blob upload, checks=%d", readinessChecks)
	}
	if blobCalls == 0 {
		t.Fatal("expected blob upload call after bootstrap")
	}
	if got := getMainTimelineGitSHA(t, rs.ivaldiDir); got != finalCommitSHA {
		t.Fatalf("expected timeline git SHA %s, got %s", finalCommitSHA, got)
	}
}

func TestPushCommitSkipsWhenAlreadyUpToDate(t *testing.T) {
	rs, workDir := setupSyncCommitTestRepo(t)

	filePath := filepath.Join(workDir, "notes.txt")
	if err := os.WriteFile(filePath, []byte("already-synced"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	const remoteSHA = "9999999999999999999999999999999999999999"
	if err := rs.createIvaldiCommit("synced seal", "main", remoteSHA); err != nil {
		t.Fatalf("failed to create local commit: %v", err)
	}
	headHash := getMainHeadHash(t, rs.ivaldiDir)

	branchCalls := 0
	unexpectedCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/nest/branches/main":
			branchCalls++
			writeJSON(t, w, http.StatusOK, map[string]any{
				"name": "main",
				"commit": map[string]any{
					"sha": remoteSHA,
					"url": "http://example.test/commit",
				},
			})
		default:
			unexpectedCalls++
			writeJSON(t, w, http.StatusNotFound, map[string]any{
				"message": fmt.Sprintf("unexpected endpoint: %s %s", r.Method, r.URL.Path),
			})
		}
	}))
	defer server.Close()

	rs.client = &Client{
		httpClient:  server.Client(),
		baseURL:     server.URL,
		rateLimiter: &RateLimiter{},
	}

	err := rs.PushCommit(context.Background(), "octo", "nest", "main", headHash, false, "main")
	if err != nil {
		t.Fatalf("PushCommit failed: %v", err)
	}

	if branchCalls != 1 {
		t.Fatalf("expected exactly one branch lookup, got %d", branchCalls)
	}
	if unexpectedCalls != 0 {
		t.Fatalf("expected no extra API calls, got %d", unexpectedCalls)
	}
}

func getMainHeadHash(t *testing.T, ivaldiDir string) cas.Hash {
	t.Helper()

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("failed to open refs manager: %v", err)
	}
	defer refsManager.Close()

	timeline, err := refsManager.GetTimeline("main", refs.LocalTimeline)
	if err != nil {
		t.Fatalf("failed to read main timeline: %v", err)
	}

	var hash cas.Hash
	copy(hash[:], timeline.Blake3Hash[:])
	return hash
}

func getMainTimelineGitSHA(t *testing.T, ivaldiDir string) string {
	t.Helper()

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		t.Fatalf("failed to open refs manager: %v", err)
	}
	defer refsManager.Close()

	timeline, err := refsManager.GetTimeline("main", refs.LocalTimeline)
	if err != nil {
		t.Fatalf("failed to read main timeline: %v", err)
	}
	return timeline.GitSHA1Hash
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("failed to encode test response: %v", err)
	}
}
