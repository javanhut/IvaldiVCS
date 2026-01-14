package cli

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/seals"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
	"github.com/spf13/cobra"
)

var fuseCmd = &cobra.Command{
	Use:   "fuse <source-timeline> [to <target-timeline>]",
	Short: "Merge two timelines together",
	Long: `Fuse (merge) changes from one timeline into another.

If target timeline is not specified, the current timeline is used.

Examples:
  ivaldi fuse main                          # Fuse main into current timeline (auto strategy)
  ivaldi fuse main to new_tl                # Fuse main into new_tl
  ivaldi fuse feature-x                     # Fuse feature-x into current timeline
  ivaldi fuse --strategy=theirs feature     # Accept all source changes
  ivaldi fuse --strategy=ours feature       # Keep all target changes
  ivaldi fuse --continue                    # Continue merge after resolving conflicts
  ivaldi fuse --abort                       # Abort current merge

Strategies:
  auto    - Intelligent chunk-level merge (default)
  ours    - Keep target timeline version
  theirs  - Accept source timeline version
  union   - Combine both versions
  base    - Revert to common ancestor`,
	RunE: runFuse,
}

var (
	fuseContinue bool
	fuseAbort    bool
	fuseStrategy string
)

func init() {
	fuseCmd.Flags().BoolVar(&fuseContinue, "continue", false, "Continue merge after resolving conflicts")
	fuseCmd.Flags().BoolVar(&fuseAbort, "abort", false, "Abort current merge")
	fuseCmd.Flags().StringVar(&fuseStrategy, "strategy", "auto", "Merge strategy (auto, ours, theirs, union, base)")
}

func runFuse(cmd *cobra.Command, args []string) error {
	// Check if we're in an Ivaldi repository
	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Handle --abort flag
	if fuseAbort {
		return abortMerge(ivaldiDir)
	}

	// Handle --continue flag
	if fuseContinue {
		return continueMerge(ivaldiDir, workDir)
	}

	// Check if merge is already in progress
	if isMergeInProgress(ivaldiDir) {
		return fmt.Errorf("merge already in progress. Use 'ivaldi fuse --continue' or 'ivaldi fuse --abort'")
	}

	// Parse arguments
	if len(args) < 1 {
		return fmt.Errorf("source timeline required. Use: ivaldi fuse <source> [to <target>]")
	}

	sourceTimeline := args[0]
	var targetTimeline string

	// Check for "to" keyword
	if len(args) >= 3 && args[1] == "to" {
		targetTimeline = args[2]
	} else if len(args) == 1 {
		// Use current timeline as target
		refsManager, err := refs.NewRefsManager(ivaldiDir)
		if err != nil {
			return fmt.Errorf("failed to initialize refs: %w", err)
		}
		defer refsManager.Close()

		targetTimeline, err = refsManager.GetCurrentTimeline()
		if err != nil {
			return fmt.Errorf("failed to get current timeline: %w", err)
		}
	} else {
		return fmt.Errorf("invalid syntax. Use: ivaldi fuse <source> [to <target>]")
	}

	// Cannot fuse timeline into itself
	if sourceTimeline == targetTimeline {
		return fmt.Errorf("cannot fuse timeline '%s' into itself", sourceTimeline)
	}

	fmt.Printf("%s Fusing %s into %s...\n\n",
		colors.Cyan(">>"),
		colors.Bold(sourceTimeline),
		colors.Bold(targetTimeline))

	// Perform the fuse
	return performFuse(ivaldiDir, workDir, sourceTimeline, targetTimeline)
}

func performFuse(ivaldiDir, workDir, sourceTimeline, targetTimeline string) error {
	// Initialize storage
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs: %w", err)
	}
	defer refsManager.Close()

	// Get source timeline
	sourceRef, err := refsManager.GetTimeline(sourceTimeline, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("source timeline '%s' not found: %w", sourceTimeline, err)
	}

	// Get target timeline
	targetRef, err := refsManager.GetTimeline(targetTimeline, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("target timeline '%s' not found: %w", targetTimeline, err)
	}

	// Convert hashes
	var sourceHash, targetHash cas.Hash
	copy(sourceHash[:], sourceRef.Blake3Hash[:])
	copy(targetHash[:], targetRef.Blake3Hash[:])

	// Read commits
	commitReader := commit.NewCommitReader(casStore)
	sourceCommit, err := commitReader.ReadCommit(sourceHash)
	if err != nil {
		return fmt.Errorf("failed to read source commit: %w", err)
	}

	targetCommit, err := commitReader.ReadCommit(targetHash)
	if err != nil {
		return fmt.Errorf("failed to read target commit: %w", err)
	}

	// Check for fast-forward possibility
	canFastForward := checkFastForward(casStore, targetHash, sourceHash)

	if canFastForward {
		return handleFastForward(ivaldiDir, refsManager, sourceTimeline, targetTimeline, sourceHash)
	}

	// Need to perform actual merge
	return handleMerge(ivaldiDir, workDir, casStore, refsManager, sourceTimeline, targetTimeline, sourceCommit, targetCommit, sourceHash, targetHash)
}

func checkFastForward(casStore cas.CAS, targetHash, sourceHash cas.Hash) bool {
	// Fast-forward is possible if target is an ancestor of source
	commitReader := commit.NewCommitReader(casStore)

	isAncestor, err := commitReader.IsAncestor(targetHash, sourceHash)
	if err != nil {
		return false
	}

	return isAncestor
}

func handleFastForward(ivaldiDir string, refsManager *refs.RefsManager, sourceTimeline, targetTimeline string, sourceHash cas.Hash) error {
	fmt.Println(colors.Green("[OK] Fast-forward merge possible"))
	fmt.Println()

	// Ask for confirmation
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Fast-forward %s to match %s? (y/N)> ", colors.Bold(targetTimeline), colors.Bold(sourceTimeline))

	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Println("Fuse cancelled.")
		return nil
	}

	// Update target timeline to point to source commit
	var hashArray [32]byte
	copy(hashArray[:], sourceHash[:])

	err = refsManager.UpdateTimeline(targetTimeline, refs.LocalTimeline, hashArray, [32]byte{}, "")
	if err != nil {
		return fmt.Errorf("failed to update timeline: %w", err)
	}

	fmt.Println()
	fmt.Printf("%s Fast-forwarded %s to %s\n",
		colors.SuccessText("[OK]"),
		colors.Bold(targetTimeline),
		colors.Bold(sourceTimeline))

	return nil
}

func handleMerge(ivaldiDir, workDir string, casStore cas.CAS, refsManager *refs.RefsManager,
	sourceTimeline, targetTimeline string, sourceCommit, targetCommit *commit.CommitObject,
	sourceHash, targetHash cas.Hash) error {

	fmt.Println(colors.Yellow("[MERGE] Three-way merge required"))
	fmt.Println()

	// Get workspace indexes for both commits
	sourceIndex, err := getCommitWorkspaceIndex(casStore, sourceCommit)
	if err != nil {
		return fmt.Errorf("failed to get source workspace: %w", err)
	}

	targetIndex, err := getCommitWorkspaceIndex(casStore, targetCommit)
	if err != nil {
		return fmt.Errorf("failed to get target workspace: %w", err)
	}

	// Find common ancestor (base)
	// For now, use target's parent as base (simplified)
	var baseIndex wsindex.IndexRef
	if len(targetCommit.Parents) > 0 {
		baseCommit, err := commit.NewCommitReader(casStore).ReadCommit(targetCommit.Parents[0])
		if err == nil {
			baseIndex, err = getCommitWorkspaceIndex(casStore, baseCommit)
			if err != nil {
				log.Printf("Warning: could not get base workspace index: %v", err)
			}
		}
	}

	// If no base, use empty workspace
	if baseIndex.Count == 0 {
		wsBuilder := wsindex.NewBuilder(casStore)
		var err error
		baseIndex, err = wsBuilder.Build(nil)
		if err != nil {
			log.Printf("Warning: could not build empty workspace index: %v", err)
		}
	}

	// Parse merge strategy
	strategy := diffmerge.StrategyType(fuseStrategy)

	// Perform three-way merge with intelligent strategy
	merger := diffmerge.NewMerger(casStore)
	mergeResult, err := merger.MergeWorkspacesWithStrategy(baseIndex, targetIndex, sourceIndex, strategy)
	if err != nil {
		return fmt.Errorf("failed to merge: %w", err)
	}

	// Check for conflicts
	if !mergeResult.Success {
		fmt.Printf("%s Merge conflicts detected:\n\n", colors.Yellow("[CONFLICTS]"))

		for _, conflict := range mergeResult.Conflicts {
			fmt.Printf("  %s %s\n", colors.Red("CONFLICT:"), colors.Bold(conflict.Path))
		}

		fmt.Println()
		fmt.Printf("%s %d file(s) with conflicts\n", colors.Yellow(">>"), len(mergeResult.Conflicts))
		fmt.Println()

		// With intelligent conflict resolution, we DON'T write markers to files
		// Instead, we save the merge state and offer resolution options

		// Save merge state
		mergeState := &MergeState{
			SourceTimeline: sourceTimeline,
			TargetTimeline: targetTimeline,
			SourceHash:     sourceHash,
			TargetHash:     targetHash,
			Conflicts:      mergeResult.Conflicts,
		}

		if err := saveMergeState(ivaldiDir, mergeState); err != nil {
			return fmt.Errorf("failed to save merge state: %w", err)
		}

		// Save resolution metadata
		resStorage := diffmerge.NewResolutionStorage(ivaldiDir)
		resolution := diffmerge.CreateResolution(sourceTimeline, targetTimeline, sourceHash, targetHash, strategy)
		if err := resStorage.Save(resolution); err != nil {
			return fmt.Errorf("failed to save resolution: %w", err)
		}

		fmt.Println(colors.Bold("Resolution options:"))
		fmt.Printf("  %s - Use interactive resolver\n", colors.Cyan("ivaldi fuse --continue"))
		fmt.Printf("  %s - Accept all source changes\n", colors.Blue("ivaldi fuse --strategy=theirs "+sourceTimeline))
		fmt.Printf("  %s - Keep all target changes\n", colors.Green("ivaldi fuse --strategy=ours "+sourceTimeline))
		fmt.Printf("  %s - Abort merge\n", colors.Red("ivaldi fuse --abort"))
		fmt.Println()
		fmt.Println(colors.Yellow("Note: Workspace files are NOT modified - conflicts are resolved separately"))

		return nil // Don't return error - merge is paused
	}

	// Show diff of changes
	fmt.Println(colors.SectionHeader("Changes to be merged:"))
	fmt.Println()

	differ := diffmerge.NewDiffer(casStore)
	diff, err := differ.DiffWorkspaces(targetIndex, *mergeResult.MergedIndex)
	if err != nil {
		return fmt.Errorf("failed to compute diff: %w", err)
	}

	if len(diff.FileChanges) == 0 {
		fmt.Println(colors.Gray("No changes (already up to date)"))
	} else {
		showMergeDiffSummary(diff)
	}

	fmt.Println()

	// Ask for confirmation
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Apply merge from %s to %s? (y/N)> ", colors.Bold(sourceTimeline), colors.Bold(targetTimeline))

	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Println("Fuse cancelled.")
		return nil
	}

	// Create merge commit
	fmt.Println()
	fmt.Println(colors.Cyan("Creating merge commit..."))

	author, err := getAuthorFromConfig()
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}

	// Get merged files
	wsLoader := wsindex.NewLoader(casStore)
	mergedFiles, err := wsLoader.ListAll(*mergeResult.MergedIndex)
	if err != nil {
		return fmt.Errorf("failed to list merged files: %w", err)
	}

	// Initialize MMR
	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		mmr = &history.PersistentMMR{MMR: history.NewMMR()}
	}
	defer mmr.Close()

	// Create merge commit with both parents
	commitBuilder := commit.NewCommitBuilder(casStore, mmr.MMR)
	mergeCommit, err := commitBuilder.CreateCommit(
		mergedFiles,
		[]cas.Hash{targetHash, sourceHash}, // Both parents
		author,
		author,
		fmt.Sprintf("Fuse %s into %s", sourceTimeline, targetTimeline),
	)
	if err != nil {
		return fmt.Errorf("failed to create merge commit: %w", err)
	}

	// Get merge commit hash
	mergeHash := commitBuilder.GetCommitHash(mergeCommit)
	var mergeHashArray [32]byte
	copy(mergeHashArray[:], mergeHash[:])

	// Update target timeline
	err = refsManager.UpdateTimeline(targetTimeline, refs.LocalTimeline, mergeHashArray, [32]byte{}, "")
	if err != nil {
		return fmt.Errorf("failed to update timeline: %w", err)
	}

	// Generate seal name
	sealName := seals.GenerateSealName(mergeHashArray)
	if err := refsManager.StoreSealName(sealName, mergeHashArray, fmt.Sprintf("Fuse %s into %s", sourceTimeline, targetTimeline)); err != nil {
		log.Printf("Warning: failed to store seal name: %v", err)
	}

	// Clean up resolution storage (merge succeeded)
	resStorage := diffmerge.NewResolutionStorage(ivaldiDir)
	if res, _ := resStorage.Load(); res != nil {
		res.MarkCompleted()
		resStorage.SaveHistory(res) // Archive for reference
	}
	resStorage.Delete()

	fmt.Println()
	fmt.Printf("%s Changes from %s fused into %s!\n",
		colors.SuccessText("[OK]"),
		colors.Bold(sourceTimeline),
		colors.Bold(targetTimeline))
	fmt.Printf("  Merge seal: %s\n", colors.Cyan(sealName))

	// Show detailed diff
	if len(diff.FileChanges) > 0 {
		fmt.Println()
		fmt.Println(colors.SectionHeader("Diff summary:"))
		showMergeChangesDetail(diff)
	}

	return nil
}

func getCommitWorkspaceIndex(casStore cas.CAS, commitObj *commit.CommitObject) (wsindex.IndexRef, error) {
	// Read tree and convert to workspace index
	// This is simplified - in production you'd fully materialize the tree
	commitReader := commit.NewCommitReader(casStore)
	tree, err := commitReader.ReadTree(commitObj)
	if err != nil {
		return wsindex.IndexRef{}, err
	}

	// For now, return empty index
	// TODO: Properly convert tree to workspace index
	_ = tree
	wsBuilder := wsindex.NewBuilder(casStore)
	return wsBuilder.Build(nil)
}

func showMergeDiffSummary(diff *diffmerge.WorkspaceDiff) {
	added := 0
	modified := 0
	removed := 0

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

	if added > 0 {
		fmt.Printf("  %s %d files\n", colors.Green("+"), added)
	}
	if modified > 0 {
		fmt.Printf("  %s %d files\n", colors.Blue("~"), modified)
	}
	if removed > 0 {
		fmt.Printf("  %s %d files\n", colors.Red("-"), removed)
	}
}

func showMergeChangesDetail(diff *diffmerge.WorkspaceDiff) {
	maxShow := 10
	shown := 0

	for _, change := range diff.FileChanges {
		if shown >= maxShow {
			remaining := len(diff.FileChanges) - shown
			fmt.Printf("  %s\n", colors.Gray(fmt.Sprintf("... and %d more changes", remaining)))
			break
		}

		switch change.Type {
		case diffmerge.Added:
			fmt.Printf("  %s %s\n", colors.Green("+"), change.Path)
		case diffmerge.Modified:
			fmt.Printf("  %s %s\n", colors.Blue("~"), change.Path)
		case diffmerge.Removed:
			fmt.Printf("  %s %s\n", colors.Red("-"), change.Path)
		}
		shown++
	}
}

// MergeState stores information about an in-progress merge
type MergeState struct {
	SourceTimeline string
	TargetTimeline string
	SourceHash     cas.Hash
	TargetHash     cas.Hash
	Conflicts      []diffmerge.Conflict
}

// saveMergeState saves merge state to disk
func saveMergeState(ivaldiDir string, state *MergeState) error {
	// Save merge head (source commit)
	mergeHeadPath := filepath.Join(ivaldiDir, "MERGE_HEAD")
	if err := os.WriteFile(mergeHeadPath, []byte(state.SourceHash.String()), 0644); err != nil {
		return err
	}

	// Save merge info
	mergeInfoPath := filepath.Join(ivaldiDir, "MERGE_INFO")
	info := fmt.Sprintf("%s\n%s\n%s\n%s\n",
		state.SourceTimeline,
		state.TargetTimeline,
		state.SourceHash.String(),
		state.TargetHash.String())
	if err := os.WriteFile(mergeInfoPath, []byte(info), 0644); err != nil {
		return err
	}

	// Save conflict list
	if len(state.Conflicts) > 0 {
		conflictListPath := filepath.Join(ivaldiDir, "MERGE_CONFLICTS")
		var conflictPaths []string
		for _, c := range state.Conflicts {
			conflictPaths = append(conflictPaths, c.Path)
		}
		if err := os.WriteFile(conflictListPath, []byte(strings.Join(conflictPaths, "\n")), 0644); err != nil {
			return err
		}
	}

	return nil
}

// loadMergeState loads merge state from disk
func loadMergeState(ivaldiDir string) (*MergeState, error) {
	mergeInfoPath := filepath.Join(ivaldiDir, "MERGE_INFO")
	data, err := os.ReadFile(mergeInfoPath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 4 {
		return nil, fmt.Errorf("invalid merge info file")
	}

	state := &MergeState{
		SourceTimeline: lines[0],
		TargetTimeline: lines[1],
	}

	// Parse hashes (simplified - assumes hex encoding)
	// In production, use proper hash parsing
	copy(state.SourceHash[:], []byte(lines[2])[:32])
	copy(state.TargetHash[:], []byte(lines[3])[:32])

	return state, nil
}

// isMergeInProgress checks if a merge is currently in progress
func isMergeInProgress(ivaldiDir string) bool {
	mergeHeadPath := filepath.Join(ivaldiDir, "MERGE_HEAD")
	_, err := os.Stat(mergeHeadPath)
	return err == nil
}

// abortMerge aborts the current merge
func abortMerge(ivaldiDir string) error {
	if !isMergeInProgress(ivaldiDir) {
		return fmt.Errorf("no merge in progress")
	}

	fmt.Println(colors.Yellow("Aborting merge..."))

	// Remove merge state files
	os.Remove(filepath.Join(ivaldiDir, "MERGE_HEAD"))
	os.Remove(filepath.Join(ivaldiDir, "MERGE_INFO"))
	os.Remove(filepath.Join(ivaldiDir, "MERGE_CONFLICTS"))

	// Remove resolution storage
	resStorage := diffmerge.NewResolutionStorage(ivaldiDir)
	resStorage.Delete()

	fmt.Println(colors.SuccessText("[OK] Merge aborted"))
	fmt.Println(colors.Dim("Workspace remains clean - no files were modified during merge attempt."))

	return nil
}

// continueMerge continues a merge after conflicts are resolved
func continueMerge(ivaldiDir, workDir string) error {
	if !isMergeInProgress(ivaldiDir) {
		return fmt.Errorf("no merge in progress")
	}

	// Load merge state
	state, err := loadMergeState(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to load merge state: %w", err)
	}

	fmt.Printf("%s Continuing merge of %s into %s...\n\n",
		colors.Cyan(">>"),
		colors.Bold(state.SourceTimeline),
		colors.Bold(state.TargetTimeline))

	// Load resolution storage to check if merge was already resolved
	resStorage := diffmerge.NewResolutionStorage(ivaldiDir)
	resolution, err := resStorage.Load()
	if err != nil {
		return fmt.Errorf("failed to load resolution: %w", err)
	}

	// Initialize CAS store (needed for interactive resolution)
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// If resolution exists and has conflicts, use interactive resolver
	if resolution != nil && !resolution.IsFullyResolved() {
		fmt.Println(colors.Cyan("Using interactive conflict resolver..."))
		fmt.Println()

		// Get conflict file paths
		conflictListPath := filepath.Join(ivaldiDir, "MERGE_CONFLICTS")
		conflictData, err := os.ReadFile(conflictListPath)
		if err != nil {
			return fmt.Errorf("failed to read conflict list: %w", err)
		}
		conflictPaths := strings.Split(strings.TrimSpace(string(conflictData)), "\n")

		// Load commits for three-way merge data
		commitReader := commit.NewCommitReader(casStore)
		targetCommit, err := commitReader.ReadCommit(state.TargetHash)
		if err != nil {
			return fmt.Errorf("failed to read target commit: %w", err)
		}
		sourceCommit, err := commitReader.ReadCommit(state.SourceHash)
		if err != nil {
			return fmt.Errorf("failed to read source commit: %w", err)
		}
		var baseCommit *commit.CommitObject
		if len(targetCommit.Parents) > 0 {
			var err error
			baseCommit, err = commitReader.ReadCommit(targetCommit.Parents[0])
			if err != nil {
				log.Printf("Warning: could not read base commit: %v", err)
			}
		}

		// Resolve each conflicting file interactively
		resolvedFiles := []string{}
		for _, path := range conflictPaths {
			if path == "" {
				continue
			}

			result, err := resolveConflictedFile(casStore, ivaldiDir, workDir, path, baseCommit, targetCommit, sourceCommit)
			if err != nil {
				return fmt.Errorf("failed to resolve %s: %w", path, err)
			}

			// Update resolution status
			if resolution.Files[path] != nil {
				resolution.Files[path].Resolved = result.Success
			}
			resolvedFiles = append(resolvedFiles, path)
		}

		// Save updated resolution
		if err := resStorage.Save(resolution); err != nil {
			return fmt.Errorf("failed to save resolution: %w", err)
		}

		// Auto-stage resolved files
		stageDir := filepath.Join(ivaldiDir, "stage")
		if err := os.MkdirAll(stageDir, 0755); err != nil {
			return fmt.Errorf("failed to create stage directory: %w", err)
		}
		stageFilePath := filepath.Join(stageDir, "files")
		if err := os.WriteFile(stageFilePath, []byte(strings.Join(resolvedFiles, "\n")), 0644); err != nil {
			return fmt.Errorf("failed to stage files: %w", err)
		}

		fmt.Printf("\n%s All conflicts resolved!\n", colors.SuccessText("[OK]"))
		fmt.Printf("  %d file(s) resolved and staged\n", len(resolvedFiles))
		fmt.Println()
		fmt.Println("Run 'ivaldi fuse --continue' again to complete the merge.")
		return nil
	}

	// Create merge commit
	fmt.Println(colors.Cyan("Creating merge commit..."))

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs: %w", err)
	}
	defer refsManager.Close()

	author, err := getAuthorFromConfig()
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}

	// Get staged files (or all files if none staged)
	stageFile := filepath.Join(ivaldiDir, "stage", "files")
	var stagedFiles []string
	if data, err := os.ReadFile(stageFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				stagedFiles = append(stagedFiles, line)
			}
		}
	}

	if len(stagedFiles) == 0 {
		return fmt.Errorf("no files staged. Stage resolved files with 'ivaldi gather <file>...'")
	}

	// Scan workspace for staged files
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)

	wsIndex, err := materializer.ScanWorkspace()
	if err != nil {
		return fmt.Errorf("failed to scan workspace: %w", err)
	}

	wsLoader := wsindex.NewLoader(casStore)
	allFiles, err := wsLoader.ListAll(wsIndex)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	// Filter to staged files
	var mergedFiles []wsindex.FileMetadata
	stagedMap := make(map[string]bool)
	for _, f := range stagedFiles {
		stagedMap[f] = true
	}

	for _, file := range allFiles {
		if stagedMap[file.Path] {
			mergedFiles = append(mergedFiles, file)
		}
	}

	// Initialize MMR
	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		mmr = &history.PersistentMMR{MMR: history.NewMMR()}
	}
	defer mmr.Close()

	// Create merge commit
	commitBuilder := commit.NewCommitBuilder(casStore, mmr.MMR)
	mergeCommit, err := commitBuilder.CreateCommit(
		mergedFiles,
		[]cas.Hash{state.TargetHash, state.SourceHash},
		author,
		author,
		fmt.Sprintf("Fuse %s into %s", state.SourceTimeline, state.TargetTimeline),
	)
	if err != nil {
		return fmt.Errorf("failed to create merge commit: %w", err)
	}

	// Get merge commit hash
	mergeHash := commitBuilder.GetCommitHash(mergeCommit)
	var mergeHashArray [32]byte
	copy(mergeHashArray[:], mergeHash[:])

	// Update target timeline
	err = refsManager.UpdateTimeline(state.TargetTimeline, refs.LocalTimeline, mergeHashArray, [32]byte{}, "")
	if err != nil {
		return fmt.Errorf("failed to update timeline: %w", err)
	}

	// Generate seal name
	sealName := seals.GenerateSealName(mergeHashArray)
	if err := refsManager.StoreSealName(sealName, mergeHashArray, fmt.Sprintf("Fuse %s into %s", state.SourceTimeline, state.TargetTimeline)); err != nil {
		log.Printf("Warning: failed to store seal name: %v", err)
	}

	// Clean up merge state
	os.Remove(filepath.Join(ivaldiDir, "MERGE_HEAD"))
	os.Remove(filepath.Join(ivaldiDir, "MERGE_INFO"))
	os.Remove(filepath.Join(ivaldiDir, "MERGE_CONFLICTS"))
	os.Remove(stageFile)

	// Clean up and archive resolution
	resStorage = diffmerge.NewResolutionStorage(ivaldiDir)
	if resolution != nil {
		resolution.MarkCompleted()
		resStorage.SaveHistory(resolution) // Archive for reference
	}
	resStorage.Delete()

	fmt.Println()
	fmt.Printf("%s Merge completed successfully!\n", colors.SuccessText("[OK]"))
	fmt.Printf("  Merge seal: %s\n", colors.Cyan(sealName))
	fmt.Printf("  Timeline %s updated\n", colors.Bold(state.TargetTimeline))

	return nil
}

// getFileFromCommit retrieves file metadata from a commit's tree.
// Returns nil if the file doesn't exist in the commit.
func getFileFromCommit(casStore cas.CAS, commitObj *commit.CommitObject, path string) *wsindex.FileMetadata {
	if commitObj == nil {
		return nil
	}

	commitReader := commit.NewCommitReader(casStore)
	tree, err := commitReader.ReadTree(commitObj)
	if err != nil {
		return nil
	}

	// Convert tree to file metadata and search for the path
	files, err := commitReader.TreeToFileMetadata(tree)
	if err != nil {
		return nil
	}

	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}

	return nil
}

// resolveConflictedFile performs interactive resolution for a single conflicting file.
// It recreates the chunk-level merge data and uses ConflictResolver to get user choices.
func resolveConflictedFile(
	casStore cas.CAS,
	ivaldiDir string,
	workDir string,
	path string,
	baseCommit, targetCommit, sourceCommit *commit.CommitObject,
) (*diffmerge.ChunkMergeResult, error) {
	// Get file metadata from each commit
	baseFile := getFileFromCommit(casStore, baseCommit, path)
	targetFile := getFileFromCommit(casStore, targetCommit, path)
	sourceFile := getFileFromCommit(casStore, sourceCommit, path)

	// Use ChunkMerger to recreate the conflict data
	chunkMerger := diffmerge.NewChunkMerger(casStore)
	result, err := chunkMerger.MergeFile(path, baseFile, targetFile, sourceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to merge file %s: %w", path, err)
	}

	// If no conflicts, file is already resolved
	if result.Success {
		// Write the merged file to workspace
		if len(result.MergedChunks) > 0 {
			err = writeMergedFile(casStore, workDir, path, result.MergedChunks)
			if err != nil {
				return nil, fmt.Errorf("failed to write merged file: %w", err)
			}
		}
		return result, nil
	}

	// Use ConflictResolver for interactive resolution
	resolver := NewConflictResolver(casStore)
	resolvedChunks, err := resolver.ResolveConflicts(result)
	if err != nil {
		return nil, fmt.Errorf("conflict resolution failed: %w", err)
	}

	// Update result with resolved chunks
	result.MergedChunks = resolvedChunks
	result.Success = true
	result.Conflicts = nil

	// Write the resolved file to workspace
	if len(resolvedChunks) > 0 {
		err = writeMergedFile(casStore, workDir, path, resolvedChunks)
		if err != nil {
			return nil, fmt.Errorf("failed to write resolved file: %w", err)
		}
	}

	return result, nil
}

// writeMergedFile writes merged chunks to the workspace.
func writeMergedFile(casStore cas.CAS, workDir string, path string, chunks []cas.Hash) error {
	filePath := filepath.Join(workDir, path)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write each chunk
	for _, chunkHash := range chunks {
		data, err := casStore.Get(chunkHash)
		if err != nil {
			return fmt.Errorf("failed to read chunk %s: %w", chunkHash.String()[:8], err)
		}
		if _, err := file.Write(data); err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}
	}

	return nil
}

// These functions are no longer needed - Ivaldi uses intelligent conflict resolution
// without writing conflict markers to workspace files
