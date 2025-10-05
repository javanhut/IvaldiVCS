package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff [options] [<commit>] [<commit>]",
	Short: "Show differences between commits or working directory",
	Long: `Show changes between commits, commit and working directory, or staging area.

Examples:
  ivaldi diff                     # Working directory vs staged
  ivaldi diff --staged            # Staged vs HEAD
  ivaldi diff <seal>              # Working directory vs commit
  ivaldi diff <seal1> <seal2>     # Between two commits
  ivaldi diff --stat              # Show summary statistics only`,
	RunE: runDiff,
}

var (
	diffStaged bool
	diffStat   bool
)

func init() {
	diffCmd.Flags().BoolVar(&diffStaged, "staged", false, "Show diff of staged changes")
	diffCmd.Flags().BoolVar(&diffStat, "stat", false, "Show only statistics")
}

func runDiff(cmd *cobra.Command, args []string) error {
	// Check if we're in an Ivaldi repository
	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Initialize CAS
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Determine what to compare based on arguments
	switch len(args) {
	case 0:
		// No args: working directory vs staged (or HEAD if --staged)
		return diffWorkingOrStaged(casStore, ivaldiDir, workDir)
	case 1:
		// One arg: working directory vs specified commit
		return diffWorkingVsCommit(casStore, ivaldiDir, workDir, args[0])
	case 2:
		// Two args: compare two commits
		return diffCommitVsCommit(casStore, ivaldiDir, args[0], args[1])
	default:
		return fmt.Errorf("too many arguments. See: ivaldi diff --help")
	}
}

// diffWorkingOrStaged shows diff of working directory vs staged, or staged vs HEAD
func diffWorkingOrStaged(casStore cas.CAS, ivaldiDir, workDir string) error {
	if diffStaged {
		// Show staged vs HEAD
		return diffStagedVsHead(casStore, ivaldiDir, workDir)
	}

	// Show working directory vs staged (or HEAD if nothing staged)
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
	currentIndex, err := materializer.ScanWorkspace()
	if err != nil {
		return fmt.Errorf("failed to scan workspace: %w", err)
	}

	// Get staged files
	stagedFiles, err := getStagedFilesList(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to get staged files: %w", err)
	}

	if len(stagedFiles) == 0 {
		// No staged files, compare with HEAD
		return diffWorkingVsHead(casStore, ivaldiDir, currentIndex)
	}

	// Build index from staged files
	wsLoader := wsindex.NewLoader(casStore)
	allFiles, err := wsLoader.ListAll(currentIndex)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	// Filter to staged files
	var stagedMetadata []wsindex.FileMetadata
	stagedMap := make(map[string]bool)
	for _, f := range stagedFiles {
		stagedMap[f] = true
	}

	for _, file := range allFiles {
		if stagedMap[file.Path] {
			stagedMetadata = append(stagedMetadata, file)
		}
	}

	// Build staged index
	wsBuilder := wsindex.NewBuilder(casStore)
	stagedIndex, err := wsBuilder.Build(stagedMetadata)
	if err != nil {
		return fmt.Errorf("failed to build staged index: %w", err)
	}

	// Show diff
	return showDiff(casStore, stagedIndex, currentIndex, "staged", "working directory")
}

// diffStagedVsHead shows diff of staged changes vs HEAD
func diffStagedVsHead(casStore cas.CAS, ivaldiDir, workDir string) error {
	// Get HEAD commit
	headIndex, err := getHeadIndex(casStore, ivaldiDir)
	if err != nil {
		return err
	}

	// Get staged files and build index
	stagedFiles, err := getStagedFilesList(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to get staged files: %w", err)
	}

	if len(stagedFiles) == 0 {
		fmt.Println("No staged files.")
		return nil
	}

	// Scan workspace to get current file data
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
	currentIndex, err := materializer.ScanWorkspace()
	if err != nil {
		return fmt.Errorf("failed to scan workspace: %w", err)
	}

	wsLoader := wsindex.NewLoader(casStore)
	allFiles, err := wsLoader.ListAll(currentIndex)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	// Filter to staged files
	var stagedMetadata []wsindex.FileMetadata
	stagedMap := make(map[string]bool)
	for _, f := range stagedFiles {
		stagedMap[f] = true
	}

	for _, file := range allFiles {
		if stagedMap[file.Path] {
			stagedMetadata = append(stagedMetadata, file)
		}
	}

	// Build staged index
	wsBuilder := wsindex.NewBuilder(casStore)
	stagedIndex, err := wsBuilder.Build(stagedMetadata)
	if err != nil {
		return fmt.Errorf("failed to build staged index: %w", err)
	}

	return showDiff(casStore, headIndex, stagedIndex, "HEAD", "staged")
}

// diffWorkingVsHead shows working directory vs HEAD
func diffWorkingVsHead(casStore cas.CAS, ivaldiDir string, currentIndex wsindex.IndexRef) error {
	headIndex, err := getHeadIndex(casStore, ivaldiDir)
	if err != nil {
		return err
	}

	return showDiff(casStore, headIndex, currentIndex, "HEAD", "working directory")
}

// diffWorkingVsCommit shows working directory vs specified commit
func diffWorkingVsCommit(casStore cas.CAS, ivaldiDir, workDir, commitRef string) error {
	// Get commit index
	commitIndex, err := getCommitIndexByRef(casStore, ivaldiDir, commitRef)
	if err != nil {
		return err
	}

	// Get working directory index
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
	workingIndex, err := materializer.ScanWorkspace()
	if err != nil {
		return fmt.Errorf("failed to scan workspace: %w", err)
	}

	return showDiff(casStore, commitIndex, workingIndex, commitRef, "working directory")
}

// diffCommitVsCommit shows diff between two commits
func diffCommitVsCommit(casStore cas.CAS, ivaldiDir, ref1, ref2 string) error {
	index1, err := getCommitIndexByRef(casStore, ivaldiDir, ref1)
	if err != nil {
		return err
	}

	index2, err := getCommitIndexByRef(casStore, ivaldiDir, ref2)
	if err != nil {
		return err
	}

	return showDiff(casStore, index1, index2, ref1, ref2)
}

// showDiff displays the diff between two workspace indexes
func showDiff(casStore cas.CAS, oldIndex, newIndex wsindex.IndexRef, oldName, newName string) error {
	differ := diffmerge.NewDiffer(casStore)
	diff, err := differ.DiffWorkspaces(oldIndex, newIndex)
	if err != nil {
		return fmt.Errorf("failed to compute diff: %w", err)
	}

	if len(diff.FileChanges) == 0 {
		fmt.Println("No differences.")
		return nil
	}

	// Show statistics if requested
	if diffStat {
		return showDiffStats(diff, oldName, newName)
	}

	// Show full diff
	fmt.Printf("Diff between %s and %s:\n\n", colors.Cyan(oldName), colors.Cyan(newName))

	for _, change := range diff.FileChanges {
		switch change.Type {
		case diffmerge.Added:
			fmt.Printf("%s %s\n", colors.Green("+++"), colors.Bold(change.Path))
			if change.NewFile != nil {
				showFileContent(casStore, change.NewFile, true)
			}
		case diffmerge.Removed:
			fmt.Printf("%s %s\n", colors.Red("---"), colors.Bold(change.Path))
			if change.OldFile != nil {
				showFileContent(casStore, change.OldFile, false)
			}
		case diffmerge.Modified:
			fmt.Printf("%s %s\n", colors.Blue("M  "), colors.Bold(change.Path))
			if change.OldFile != nil && change.NewFile != nil {
				showFileDiff(casStore, change.OldFile, change.NewFile)
			}
		}
		fmt.Println()
	}

	return nil
}

// showDiffStats shows summary statistics of changes
func showDiffStats(diff *diffmerge.WorkspaceDiff, oldName, newName string) error {
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

	total := added + modified + removed

	fmt.Printf("Diff between %s and %s:\n\n", colors.Cyan(oldName), colors.Cyan(newName))
	fmt.Printf("  %s changed: %s added, %s modified, %s removed\n",
		colors.Bold(fmt.Sprintf("%d files", total)),
		colors.Green(fmt.Sprintf("%d", added)),
		colors.Blue(fmt.Sprintf("%d", modified)),
		colors.Red(fmt.Sprintf("%d", removed)))

	return nil
}

// showFileContent shows the content of a file (for added/removed files)
func showFileContent(casStore cas.CAS, file *wsindex.FileMetadata, added bool) {
	// For simplicity, just show file size
	prefix := colors.Red("- ")
	if added {
		prefix = colors.Green("+ ")
	}
	fmt.Printf("%sFile size: %d bytes\n", prefix, file.FileRef.Size)
}

// showFileDiff shows line-by-line diff for modified files
func showFileDiff(casStore cas.CAS, oldFile, newFile *wsindex.FileMetadata) {
	// Read file contents
	oldContent, err := readFileContent(casStore, oldFile)
	if err != nil {
		fmt.Printf("  %s\n", colors.Gray("(binary file or read error)"))
		return
	}

	newContent, err := readFileContent(casStore, newFile)
	if err != nil {
		fmt.Printf("  %s\n", colors.Gray("(binary file or read error)"))
		return
	}

	// Simple line-by-line diff
	oldLines := strings.Split(string(oldContent), "\n")
	newLines := strings.Split(string(newContent), "\n")

	// Show up to 20 lines of diff
	maxLines := 20
	shown := 0

	for i := 0; i < len(oldLines) || i < len(newLines); i++ {
		if shown >= maxLines {
			fmt.Printf("  %s\n", colors.Gray("... (diff truncated)"))
			break
		}

		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if oldLine != "" && i < len(oldLines) {
				fmt.Printf("%s %s\n", colors.Red("-"), oldLine)
				shown++
			}
			if newLine != "" && i < len(newLines) {
				fmt.Printf("%s %s\n", colors.Green("+"), newLine)
				shown++
			}
		}
	}
}

// readFileContent reads the content of a file from its metadata
func readFileContent(casStore cas.CAS, file *wsindex.FileMetadata) ([]byte, error) {
	loader := filechunk.NewLoader(casStore)
	return loader.ReadAll(file.FileRef)
}

// getHeadIndex returns the workspace index for the HEAD commit
func getHeadIndex(casStore cas.CAS, ivaldiDir string) (wsindex.IndexRef, error) {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to initialize refs: %w", err)
	}
	defer refsManager.Close()

	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to get current timeline: %w", err)
	}

	timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to get timeline: %w", err)
	}

	if timeline.Blake3Hash == [32]byte{} {
		// No commits yet, return empty index
		wsBuilder := wsindex.NewBuilder(casStore)
		return wsBuilder.Build(nil)
	}

	return getCommitIndex(casStore, timeline.Blake3Hash)
}

// getCommitIndex returns the workspace index for a commit
func getCommitIndex(casStore cas.CAS, commitHash [32]byte) (wsindex.IndexRef, error) {
	var hash cas.Hash
	copy(hash[:], commitHash[:])

	commitReader := commit.NewCommitReader(casStore)
	commitObj, err := commitReader.ReadCommit(hash)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to read commit: %w", err)
	}

	_, err = commitReader.ReadTree(commitObj)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to read tree: %w", err)
	}

	// Build workspace index from tree files
	// This is a simplified version - in reality we'd need to properly convert tree entries to FileMetadata
	wsBuilder := wsindex.NewBuilder(casStore)
	return wsBuilder.Build(nil) // TODO: Convert tree to FileMetadata
}

// getCommitIndexByRef resolves a ref (seal name or hash) to a workspace index
func getCommitIndexByRef(casStore cas.CAS, ivaldiDir, ref string) (wsindex.IndexRef, error) {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return wsindex.IndexRef{}, fmt.Errorf("failed to initialize refs: %w", err)
	}
	defer refsManager.Close()

	// Try to resolve as seal name first
	commitHash, _, _, err := refsManager.GetSealByName(ref)
	if err == nil {
		return getCommitIndex(casStore, commitHash)
	}

	// Try as short hash (TODO: implement hash prefix resolution)
	return wsindex.IndexRef{}, fmt.Errorf("commit not found: %s", ref)
}

// getStagedFilesList returns the list of staged files
func getStagedFilesList(ivaldiDir string) ([]string, error) {
	stageFile := filepath.Join(ivaldiDir, "stage", "files")
	if _, err := os.Stat(stageFile); os.IsNotExist(err) {
		return []string{}, nil
	}

	data, err := os.ReadFile(stageFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
