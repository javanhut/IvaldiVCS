package shift

import (
	"testing"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

func TestValidateRange(t *testing.T) {
	// Create in-memory CAS
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)

	// Create a chain of commits: C1 -> C2 -> C3
	files1 := []wsindex.FileMetadata{
		{
			Path: "file1.txt",
			FileRef: filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content1")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
			ModTime:  time.Now(),
			Mode:     0644,
			Size:     8,
			Checksum: cas.SumB3([]byte("content1")),
		},
	}

	commit1, err := builder.CreateCommit(files1, nil, "author", "author", "First commit")
	if err != nil {
		t.Fatalf("Failed to create commit 1: %v", err)
	}
	hash1 := builder.GetCommitHash(commit1)

	// Create second commit
	files2 := append(files1, wsindex.FileMetadata{
		Path: "file2.txt",
		FileRef: filechunk.NodeRef{
			Hash: cas.SumB3([]byte("content2")),
			Kind: filechunk.Leaf,
			Size: 8,
		},
		ModTime:  time.Now(),
		Mode:     0644,
		Size:     8,
		Checksum: cas.SumB3([]byte("content2")),
	})

	commit2, err := builder.CreateCommit(files2, []cas.Hash{hash1}, "author", "author", "Second commit")
	if err != nil {
		t.Fatalf("Failed to create commit 2: %v", err)
	}
	hash2 := builder.GetCommitHash(commit2)

	// Create third commit
	files3 := append(files2, wsindex.FileMetadata{
		Path: "file3.txt",
		FileRef: filechunk.NodeRef{
			Hash: cas.SumB3([]byte("content3")),
			Kind: filechunk.Leaf,
			Size: 8,
		},
		ModTime:  time.Now(),
		Mode:     0644,
		Size:     8,
		Checksum: cas.SumB3([]byte("content3")),
	})

	commit3, err := builder.CreateCommit(files3, []cas.Hash{hash2}, "author", "author", "Third commit")
	if err != nil {
		t.Fatalf("Failed to create commit 3: %v", err)
	}
	hash3 := builder.GetCommitHash(commit3)

	// Test valid range: hash1 to hash3
	err = squasher.ValidateRange(hash1, hash3)
	if err != nil {
		t.Errorf("Expected valid range hash1->hash3, got error: %v", err)
	}

	// Test valid range: hash2 to hash3
	err = squasher.ValidateRange(hash2, hash3)
	if err != nil {
		t.Errorf("Expected valid range hash2->hash3, got error: %v", err)
	}

	// Test invalid range: hash3 to hash1 (reversed)
	err = squasher.ValidateRange(hash3, hash1)
	if err == nil {
		t.Error("Expected error for reversed range hash3->hash1, got nil")
	}

	// Test invalid range: non-existent commit
	fakeHash := cas.SumB3([]byte("fake"))
	err = squasher.ValidateRange(hash1, fakeHash)
	if err == nil {
		t.Error("Expected error for non-existent end commit, got nil")
	}
}

func TestGetCommitRange(t *testing.T) {
	// Create in-memory CAS
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)

	// Create commits
	files := []wsindex.FileMetadata{
		{
			Path: "test.txt",
			FileRef: filechunk.NodeRef{
				Hash: cas.SumB3([]byte("test")),
				Kind: filechunk.Leaf,
				Size: 4,
			},
			ModTime:  time.Now(),
			Mode:     0644,
			Size:     4,
			Checksum: cas.SumB3([]byte("test")),
		},
	}

	// Commit 1
	commit1, _ := builder.CreateCommit(files, nil, "author", "author", "Commit 1")
	hash1 := builder.GetCommitHash(commit1)

	// Commit 2
	commit2, _ := builder.CreateCommit(files, []cas.Hash{hash1}, "author", "author", "Commit 2")
	hash2 := builder.GetCommitHash(commit2)

	// Commit 3
	commit3, _ := builder.CreateCommit(files, []cas.Hash{hash2}, "author", "author", "Commit 3")
	hash3 := builder.GetCommitHash(commit3)

	// Get range from hash1 to hash3
	commits, err := squasher.GetCommitRange(hash1, hash3)
	if err != nil {
		t.Fatalf("Failed to get commit range: %v", err)
	}

	// Verify we got all 3 commits in chronological order
	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}

	if commits[0].Message != "Commit 1" {
		t.Errorf("Expected first commit message 'Commit 1', got '%s'", commits[0].Message)
	}

	if commits[2].Message != "Commit 3" {
		t.Errorf("Expected third commit message 'Commit 3', got '%s'", commits[2].Message)
	}
}

func TestGetCombinedMessage(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)

	// Test empty commits
	commits := []CommitInfo{}
	msg := squasher.GetCombinedMessage(commits)
	if msg != "Empty squash" {
		t.Errorf("Expected 'Empty squash' for empty commits, got '%s'", msg)
	}

	// Test single commit
	commits = []CommitInfo{
		{Message: "Single commit message"},
	}
	msg = squasher.GetCombinedMessage(commits)
	if msg != "Single commit message" {
		t.Errorf("Expected single message to be preserved, got '%s'", msg)
	}

	// Test multiple commits
	commits = []CommitInfo{
		{Message: "First commit"},
		{Message: "Second commit"},
		{Message: "Third commit"},
	}
	msg = squasher.GetCombinedMessage(commits)
	expected := "Squashed 3 commits:\n\nFirst commit\nSecond commit\nThird commit"
	if msg != expected {
		t.Errorf("Expected combined message:\n%s\n\nGot:\n%s", expected, msg)
	}

	// Test multi-line commits (should only use first line)
	commits = []CommitInfo{
		{Message: "First line\nSecond line\nThird line"},
		{Message: "Another commit\nWith details"},
	}
	msg = squasher.GetCombinedMessage(commits)
	expected = "Squashed 2 commits:\n\nFirst line\nAnother commit"
	if msg != expected {
		t.Errorf("Expected first lines only:\n%s\n\nGot:\n%s", expected, msg)
	}
}

func TestGetParentOfStart(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	mmr := history.NewMMR()
	builder := commit.NewCommitBuilder(casStore, mmr)
	squasher := NewCommitSquasher(casStore, builder)

	files := []wsindex.FileMetadata{
		{
			Path: "test.txt",
			FileRef: filechunk.NodeRef{
				Hash: cas.SumB3([]byte("test")),
				Kind: filechunk.Leaf,
				Size: 4,
			},
			ModTime:  time.Now(),
			Mode:     0644,
			Size:     4,
			Checksum: cas.SumB3([]byte("test")),
		},
	}

	// Create root commit (no parent)
	commit1, _ := builder.CreateCommit(files, nil, "author", "author", "Root commit")
	hash1 := builder.GetCommitHash(commit1)

	// Create child commit
	commit2, _ := builder.CreateCommit(files, []cas.Hash{hash1}, "author", "author", "Child commit")
	hash2 := builder.GetCommitHash(commit2)

	// Test root commit (should return zero hash)
	parent, err := squasher.GetParentOfStart(hash1)
	if err != nil {
		t.Fatalf("Failed to get parent of root: %v", err)
	}
	if parent != (cas.Hash{}) {
		t.Error("Expected zero hash for root commit parent")
	}

	// Test child commit (should return hash1)
	parent, err = squasher.GetParentOfStart(hash2)
	if err != nil {
		t.Fatalf("Failed to get parent of child: %v", err)
	}
	if parent != hash1 {
		t.Error("Expected parent to be hash1")
	}
}
