package cli

import (
	"bufio"
	"fmt"
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
	canFastForward := checkFastForward(targetCommit, sourceCommit)

	if canFastForward {
		return handleFastForward(ivaldiDir, refsManager, sourceTimeline, targetTimeline, sourceHash)
	}

	// Need to perform actual merge
	return handleMerge(ivaldiDir, workDir, casStore, refsManager, sourceTimeline, targetTimeline, sourceCommit, targetCommit, sourceHash, targetHash)
}

func checkFastForward(targetCommit, sourceCommit *commit.CommitObject) bool {
	// Fast-forward is possible if target is an ancestor of source
	// For now, simple check: target has no commits after source's parent
	// A proper implementation would walk the commit graph

	// If target is in source's parent chain, we can fast-forward
	for _, parent := range sourceCommit.Parents {
		// Simple check - if target is direct parent
		// TODO: Walk full parent chain
		if len(targetCommit.Parents) > 0 && parent == targetCommit.Parents[0] {
			return true
		}
	}

	return false
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
			baseIndex, _ = getCommitWorkspaceIndex(casStore, baseCommit)
		}
	}

	// If no base, use empty workspace
	if baseIndex.Count == 0 {
		wsBuilder := wsindex.NewBuilder(casStore)
		baseIndex, _ = wsBuilder.Build(nil)
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
	_ = refsManager.StoreSealName(sealName, mergeHashArray, fmt.Sprintf("Fuse %s into %s", sourceTimeline, targetTimeline))

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

	// If resolution exists and has conflicts, use interactive resolver
	if resolution != nil && !resolution.IsFullyResolved() {
		fmt.Println(colors.Cyan("Using interactive conflict resolver..."))
		fmt.Println()

		// TODO: Implement interactive resolution using the ConflictResolver
		// For now, we'll require the user to use a strategy
		fmt.Println(colors.Yellow("Interactive resolution not yet complete."))
		fmt.Println("Please rerun fuse with a strategy:")
		fmt.Printf("  %s - Accept source changes\n", colors.Blue("ivaldi fuse --strategy=theirs "+state.SourceTimeline))
		fmt.Printf("  %s - Keep target changes\n", colors.Green("ivaldi fuse --strategy=ours "+state.SourceTimeline))
		return fmt.Errorf("conflicts not resolved - use a strategy")
	}

	// Create merge commit
	fmt.Println(colors.Cyan("Creating merge commit..."))

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
	_ = refsManager.StoreSealName(sealName, mergeHashArray, fmt.Sprintf("Fuse %s into %s", state.SourceTimeline, state.TargetTimeline))

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

// These functions are no longer needed - Ivaldi uses intelligent conflict resolution
// without writing conflict markers to workspace files
