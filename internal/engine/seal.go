package engine

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/config"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/seals"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// SealResult holds the result of a seal (commit) operation
type SealResult struct {
	SealName string
	Hash     string // short hex hash
	Timeline string
	Message  string
	Files    int
}

// CreateSeal creates a sealed commit from staged files
func CreateSeal(ivaldiDir, workDir, message string) (*SealResult, error) {
	// Check for staged files
	stageFile := filepath.Join(ivaldiDir, "stage", "files")
	if _, err := os.Stat(stageFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no files staged for commit. Use gather to stage files first")
	}

	stageData, err := os.ReadFile(stageFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read staged files: %w", err)
	}

	stagedFiles := strings.Fields(string(stageData))
	if len(stagedFiles) == 0 {
		return nil, fmt.Errorf("no files staged for commit")
	}

	// Initialize refs manager
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	// Get current timeline
	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return nil, fmt.Errorf("failed to get current timeline: %w", err)
	}

	// Initialize CAS
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	mmr := history.NewMMR()
	commitBuilder := commit.NewCommitBuilder(casStore, mmr)

	// Scan staged files
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
	wsIndex, err := materializer.ScanSpecificFiles(stagedFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to scan staged files: %w", err)
	}

	wsLoader := wsindex.NewLoader(casStore)
	workspaceFiles, err := wsLoader.ListAll(wsIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace files: %w", err)
	}

	// Get author
	author, err := config.GetAuthor()
	if err != nil {
		return nil, fmt.Errorf("author not configured: %w", err)
	}

	// Get parent commit
	var parents []cas.Hash
	timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err == nil && timeline.Blake3Hash != [32]byte{} {
		var parentHash cas.Hash
		copy(parentHash[:], timeline.Blake3Hash[:])
		parents = append(parents, parentHash)
	}

	// Create commit
	commitObj, err := commitBuilder.CreateCommit(
		workspaceFiles,
		parents,
		author,
		author,
		message,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit: %w", err)
	}

	commitHash := commitBuilder.GetCommitHash(commitObj)

	var commitHashArray [32]byte
	copy(commitHashArray[:], commitHash[:])

	// Generate and store seal name
	sealName := seals.GenerateSealName(commitHashArray)
	err = refsManager.StoreSealName(sealName, commitHashArray, message)
	if err != nil {
		log.Printf("Warning: Failed to store seal name: %v", err)
	}

	// Update timeline reference
	err = refsManager.CreateTimeline(
		currentTimeline,
		refs.LocalTimeline,
		commitHashArray,
		[32]byte{},
		"",
		fmt.Sprintf("Commit: %s", message),
	)
	if err != nil {
		log.Printf("Note: Timeline update: %v", err)
	}

	// Clean up staging area
	if err := os.Remove(stageFile); err != nil {
		log.Printf("Warning: Failed to clean up staging area: %v", err)
	}

	return &SealResult{
		SealName: sealName,
		Hash:     shortHash(commitHashArray),
		Timeline: currentTimeline,
		Message:  message,
		Files:    len(stagedFiles),
	}, nil
}
