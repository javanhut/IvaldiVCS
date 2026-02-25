package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/config"
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/seals"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// FuseResultType indicates the outcome of a fuse operation
type FuseResultType int

const (
	FuseFastForward    FuseResultType = iota // Target was ancestor of source
	FuseMergeSuccess                         // Three-way merge succeeded
	FuseMergeConflicts                       // Three-way merge has conflicts
)

// FuseResult holds the result of a fuse operation
type FuseResult struct {
	Type      FuseResultType
	Source    string
	Target    string
	SealName  string
	Conflicts []string
	Added     int
	Modified  int
	Removed   int
}

// FuseStatus holds information about a merge in progress
type FuseStatus struct {
	InProgress     bool
	SourceTimeline string
	TargetTimeline string
	Conflicts      []string
}

// GetFuseStatus checks if a merge is in progress and returns its state
func GetFuseStatus(ivaldiDir string) (*FuseStatus, error) {
	mergeHeadPath := filepath.Join(ivaldiDir, "MERGE_HEAD")
	if _, err := os.Stat(mergeHeadPath); os.IsNotExist(err) {
		return &FuseStatus{InProgress: false}, nil
	}

	status := &FuseStatus{InProgress: true}

	// Read merge info
	mergeInfoPath := filepath.Join(ivaldiDir, "MERGE_INFO")
	data, err := os.ReadFile(mergeInfoPath)
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) >= 2 {
			status.SourceTimeline = lines[0]
			status.TargetTimeline = lines[1]
		}
	}

	// Read conflicts
	conflictPath := filepath.Join(ivaldiDir, "MERGE_CONFLICTS")
	if conflictData, err := os.ReadFile(conflictPath); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(conflictData)), "\n") {
			if line != "" {
				status.Conflicts = append(status.Conflicts, line)
			}
		}
	}

	return status, nil
}

// AbortFuse aborts the current merge in progress
func AbortFuse(ivaldiDir string) error {
	status, err := GetFuseStatus(ivaldiDir)
	if err != nil {
		return err
	}
	if !status.InProgress {
		return fmt.Errorf("no merge in progress")
	}

	os.Remove(filepath.Join(ivaldiDir, "MERGE_HEAD"))
	os.Remove(filepath.Join(ivaldiDir, "MERGE_INFO"))
	os.Remove(filepath.Join(ivaldiDir, "MERGE_CONFLICTS"))

	resStorage := diffmerge.NewResolutionStorage(ivaldiDir)
	resStorage.Delete()

	return nil
}

// FuseTimelines merges the source timeline into the target timeline
func FuseTimelines(ivaldiDir, workDir, source, target, strategy string) (*FuseResult, error) {
	// Check for merge already in progress
	fuseStatus, _ := GetFuseStatus(ivaldiDir)
	if fuseStatus != nil && fuseStatus.InProgress {
		return nil, fmt.Errorf("merge already in progress. Abort first")
	}

	if source == target {
		return nil, fmt.Errorf("cannot fuse timeline '%s' into itself", source)
	}

	// Initialize storage
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize refs: %w", err)
	}
	defer refsManager.Close()

	// Get timeline refs
	sourceRef, err := refsManager.GetTimeline(source, refs.LocalTimeline)
	if err != nil {
		return nil, fmt.Errorf("source timeline '%s' not found: %w", source, err)
	}

	targetRef, err := refsManager.GetTimeline(target, refs.LocalTimeline)
	if err != nil {
		return nil, fmt.Errorf("target timeline '%s' not found: %w", target, err)
	}

	var sourceHash, targetHash cas.Hash
	copy(sourceHash[:], sourceRef.Blake3Hash[:])
	copy(targetHash[:], targetRef.Blake3Hash[:])

	commitReader := commit.NewCommitReader(casStore)

	// Check fast-forward
	isAncestor, err := commitReader.IsAncestor(targetHash, sourceHash)
	if err == nil && isAncestor {
		var hashArray [32]byte
		copy(hashArray[:], sourceHash[:])
		err = refsManager.UpdateTimeline(target, refs.LocalTimeline, hashArray, [32]byte{}, "")
		if err != nil {
			return nil, fmt.Errorf("failed to fast-forward: %w", err)
		}
		return &FuseResult{
			Type:   FuseFastForward,
			Source: source,
			Target: target,
		}, nil
	}

	// Three-way merge
	sourceCommit, err := commitReader.ReadCommit(sourceHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read source commit: %w", err)
	}

	targetCommit, err := commitReader.ReadCommit(targetHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read target commit: %w", err)
	}

	// Get workspace indexes
	sourceIndex, err := fuseGetWorkspaceIndex(casStore, sourceCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get source workspace: %w", err)
	}

	targetIndex, err := fuseGetWorkspaceIndex(casStore, targetCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get target workspace: %w", err)
	}

	// Find base (common ancestor)
	var baseIndex wsindex.IndexRef
	if len(targetCommit.Parents) > 0 {
		baseCommit, bErr := commitReader.ReadCommit(targetCommit.Parents[0])
		if bErr == nil {
			baseIndex, _ = fuseGetWorkspaceIndex(casStore, baseCommit)
		}
	}
	if baseIndex.Count == 0 {
		wsBuilder := wsindex.NewBuilder(casStore)
		baseIndex, _ = wsBuilder.Build(nil)
	}

	// Merge with strategy
	strategyType := diffmerge.StrategyType(strategy)
	merger := diffmerge.NewMerger(casStore)
	mergeResult, err := merger.MergeWorkspacesWithStrategy(baseIndex, targetIndex, sourceIndex, strategyType)
	if err != nil {
		return nil, fmt.Errorf("merge failed: %w", err)
	}

	if !mergeResult.Success {
		// Save merge state
		saveFuseState(ivaldiDir, source, target, sourceHash, targetHash, mergeResult.Conflicts)

		// Save resolution metadata
		resStorage := diffmerge.NewResolutionStorage(ivaldiDir)
		resolution := diffmerge.CreateResolution(source, target, sourceHash, targetHash, strategyType)
		resStorage.Save(resolution)

		var conflictPaths []string
		for _, c := range mergeResult.Conflicts {
			conflictPaths = append(conflictPaths, c.Path)
		}
		return &FuseResult{
			Type:      FuseMergeConflicts,
			Source:    source,
			Target:    target,
			Conflicts: conflictPaths,
		}, nil
	}

	// Compute diff for stats
	var added, modified, removed int
	differ := diffmerge.NewDiffer(casStore)
	diff, dErr := differ.DiffWorkspaces(targetIndex, *mergeResult.MergedIndex)
	if dErr == nil && diff != nil {
		for _, change := range diff.FileChanges {
			switch change.Type {
			case diffmerge.Added:
				added++
			case diffmerge.Modified:
				modified++
			case diffmerge.Removed:
				removed++
			}
		}
	}

	// Create merge commit
	author, err := config.GetAuthor()
	if err != nil {
		return nil, fmt.Errorf("failed to get author: %w", err)
	}

	wsLoader := wsindex.NewLoader(casStore)
	mergedFiles, err := wsLoader.ListAll(*mergeResult.MergedIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to list merged files: %w", err)
	}

	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		mmr = &history.PersistentMMR{MMR: history.NewMMR()}
	}
	defer mmr.Close()

	commitBuilder := commit.NewCommitBuilder(casStore, mmr.MMR)
	mergeCommit, err := commitBuilder.CreateCommit(
		mergedFiles,
		[]cas.Hash{targetHash, sourceHash},
		author,
		author,
		fmt.Sprintf("Fuse %s into %s", source, target),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create merge commit: %w", err)
	}

	mergeHash := commitBuilder.GetCommitHash(mergeCommit)
	var mergeHashArray [32]byte
	copy(mergeHashArray[:], mergeHash[:])

	err = refsManager.UpdateTimeline(target, refs.LocalTimeline, mergeHashArray, [32]byte{}, "")
	if err != nil {
		return nil, fmt.Errorf("failed to update timeline: %w", err)
	}

	sealName := seals.GenerateSealName(mergeHashArray)
	refsManager.StoreSealName(sealName, mergeHashArray, fmt.Sprintf("Fuse %s into %s", source, target))

	// Clean up resolution storage
	resStorage := diffmerge.NewResolutionStorage(ivaldiDir)
	if res, _ := resStorage.Load(); res != nil {
		res.MarkCompleted()
		resStorage.SaveHistory(res)
	}
	resStorage.Delete()

	return &FuseResult{
		Type:     FuseMergeSuccess,
		Source:   source,
		Target:   target,
		SealName: sealName,
		Added:    added,
		Modified: modified,
		Removed:  removed,
	}, nil
}

// fuseGetWorkspaceIndex extracts a workspace index from a commit
func fuseGetWorkspaceIndex(casStore cas.CAS, commitObj *commit.CommitObject) (wsindex.IndexRef, error) {
	commitReader := commit.NewCommitReader(casStore)
	tree, err := commitReader.ReadTree(commitObj)
	if err != nil {
		return wsindex.IndexRef{}, err
	}
	// TODO: Properly convert tree to workspace index
	_ = tree
	wsBuilder := wsindex.NewBuilder(casStore)
	return wsBuilder.Build(nil)
}

// saveFuseState persists merge state to disk so it can be aborted or continued
func saveFuseState(ivaldiDir, source, target string, sourceHash, targetHash cas.Hash, conflicts []diffmerge.Conflict) {
	mergeHeadPath := filepath.Join(ivaldiDir, "MERGE_HEAD")
	os.WriteFile(mergeHeadPath, []byte(sourceHash.String()), 0644)

	mergeInfoPath := filepath.Join(ivaldiDir, "MERGE_INFO")
	info := fmt.Sprintf("%s\n%s\n%s\n%s\n", source, target, sourceHash.String(), targetHash.String())
	os.WriteFile(mergeInfoPath, []byte(info), 0644)

	if len(conflicts) > 0 {
		conflictListPath := filepath.Join(ivaldiDir, "MERGE_CONFLICTS")
		var paths []string
		for _, c := range conflicts {
			paths = append(paths, c.Path)
		}
		os.WriteFile(conflictListPath, []byte(strings.Join(paths, "\n")), 0644)
	}
}
