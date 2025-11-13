package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/seals"
	"github.com/javanhut/Ivaldi-vcs/internal/shift"
	"github.com/spf13/cobra"
)

var shiftCmd = &cobra.Command{
	Use:   "shift",
	Short: "Interactively squash multiple commits into one",
	Long: `Browse commit history and select a range of commits to squash into a single commit.
This is useful for cleaning up history before pushing to a remote repository.

Usage:
  ivaldi shift                 # Interactive mode - select start and end commits
  ivaldi shift --last N        # Squash last N commits
  ivaldi shift <start> <end>   # Squash specific range by seal name or hash

Examples:
  ivaldi shift                           # Interactive selection
  ivaldi shift --last 3                  # Squash last 3 commits
  ivaldi shift swift-eagle bold-hawk     # Squash from swift-eagle to bold-hawk

WARNING: This rewrites commit history. After using shift, you'll need to force push:
  ivaldi upload --force`,
	RunE: runShift,
}

var lastN int

func init() {
	shiftCmd.Flags().IntVar(&lastN, "last", 0, "Squash last N commits")
}

func runShift(cmd *cobra.Command, args []string) error {
	// Check if we're in an Ivaldi repository
	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	// Initialize refs manager
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	// Get current timeline
	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return fmt.Errorf("failed to get current timeline: %w", err)
	}

	// Get timeline info
	timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get timeline info: %w", err)
	}

	if timeline.Blake3Hash == [32]byte{} {
		return fmt.Errorf("timeline has no commits yet")
	}

	// Initialize CAS
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize squasher
	mmr := history.NewMMR()
	commitBuilder := commit.NewCommitBuilder(casStore, mmr)
	squasher := shift.NewCommitSquasher(casStore, commitBuilder)

	var startHash, endHash cas.Hash

	// Determine commit range based on arguments
	if lastN > 0 {
		// Use --last N mode
		if lastN < 2 {
			return fmt.Errorf("--last must be at least 2 to squash commits")
		}

		// Get commit history
		allSeals, err := getCommitHistory(casStore, refsManager, timeline.Blake3Hash)
		if err != nil {
			return fmt.Errorf("failed to get commit history: %w", err)
		}

		if lastN > len(allSeals) {
			return fmt.Errorf("only %d commits available, cannot squash last %d", len(allSeals), lastN)
		}

		// Start is the Nth commit from the end, end is HEAD
		copy(endHash[:], allSeals[0].Hash[:])
		copy(startHash[:], allSeals[lastN-1].Hash[:])

	} else if len(args) >= 2 {
		// Use specified start and end
		_, startSealHash, _, _, err := resolveSealReference(refsManager, args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve start commit '%s': %w", args[0], err)
		}
		startHash = startSealHash

		_, endSealHash, _, _, err := resolveSealReference(refsManager, args[1])
		if err != nil {
			return fmt.Errorf("failed to resolve end commit '%s': %w", args[1], err)
		}
		endHash = endSealHash

	} else {
		// Interactive mode
		start, end, err := selectCommitRangeForShift(casStore, refsManager, timeline.Blake3Hash, currentTimeline)
		if err != nil {
			return err
		}

		if start == nil || end == nil {
			fmt.Println("Shift cancelled.")
			return nil
		}

		copy(startHash[:], start.Hash[:])
		copy(endHash[:], end.Hash[:])
	}

	// Validate the range
	if err := squasher.ValidateRange(startHash, endHash); err != nil {
		return fmt.Errorf("invalid commit range: %w", err)
	}

	// Get commits in range
	commits, err := squasher.GetCommitRange(startHash, endHash)
	if err != nil {
		return fmt.Errorf("failed to get commit range: %w", err)
	}

	// Show review and get confirmation
	fmt.Printf("\n%s Range selected: %d commits will be squashed\n\n",
		colors.Green("✓"), len(commits))

	fmt.Printf("%s Review commits to squash:\n\n", colors.Bold("📋"))
	for i, c := range commits {
		shortHash := hex.EncodeToString(c.Hash[:4])
		firstLine := strings.Split(c.Message, "\n")[0]
		fmt.Printf("[%s] %d. %s - %s\n",
			colors.Green("✓"), i+1, colors.Gray(shortHash), firstLine)
	}

	// Get combined message suggestion
	suggestedMessage := squasher.GetCombinedMessage(commits)

	fmt.Printf("\n%s\n", colors.Bold("Suggested commit message:"))
	fmt.Printf("%s\n\n", colors.Dim(suggestedMessage))

	// Prompt for commit message
	fmt.Print("Enter new commit message (or press Enter to use suggested): ")
	reader := bufio.NewReader(os.Stdin)
	userMessage, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	userMessage = strings.TrimSpace(userMessage)
	finalMessage := suggestedMessage
	if userMessage != "" {
		finalMessage = userMessage
	}

	// Warning about history rewriting
	fmt.Printf("\n%s This will rewrite commit history!\n",
		colors.Yellow("⚠ WARNING:"))
	fmt.Printf("  • %d commits will be replaced with 1 commit\n", len(commits))
	fmt.Printf("  • You will need to force push: %s\n",
		colors.Bold("ivaldi upload --force"))
	fmt.Print("\nConfirm squash? (yes/no): ")

	confirmInput, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	confirm := strings.TrimSpace(strings.ToLower(confirmInput))
	if confirm != "yes" {
		fmt.Println("Shift cancelled.")
		return nil
	}

	// Perform the squash
	fmt.Printf("\n%s Squashing commits...\n", colors.Bold("🔨"))

	// Extract final state from end commit
	files, err := squasher.ExtractFinalState(endHash)
	if err != nil {
		return fmt.Errorf("failed to extract final state: %w", err)
	}

	// Get author from config
	author, err := getAuthorFromConfig()
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}

	// Get parent of start commit (will be parent of squashed commit)
	parentHash, err := squasher.GetParentOfStart(startHash)
	if err != nil {
		return fmt.Errorf("failed to get parent: %w", err)
	}

	// Create squashed commit
	_, squashedHash, err := squasher.CreateSquashedCommit(files, parentHash, author, finalMessage)
	if err != nil {
		return fmt.Errorf("failed to create squashed commit: %w", err)
	}

	// Update timeline to point to new commit
	var squashedHashArray [32]byte
	copy(squashedHashArray[:], squashedHash[:])

	// Generate and store seal name
	sealName := seals.GenerateSealName(squashedHashArray)
	err = refsManager.StoreSealName(sealName, squashedHashArray, finalMessage)
	if err != nil {
		fmt.Printf("Warning: Failed to store seal name: %v\n", err)
	}

	// Update timeline
	err = refsManager.UpdateTimeline(
		currentTimeline,
		refs.LocalTimeline,
		squashedHashArray,
		[32]byte{},
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to update timeline: %w", err)
	}

	// Update workspace to reflect the squashed commit
	fmt.Printf("%s Created squashed commit: %s (%s)\n",
		colors.Green("✓"),
		colors.Cyan(sealName),
		colors.Gray(hex.EncodeToString(squashedHashArray[:4])))
	fmt.Printf("%s Timeline updated\n", colors.Green("✓"))
	fmt.Printf("%s %d commits squashed into 1\n\n",
		colors.Green("✓"), len(commits))

	fmt.Printf("%s Remote history differs. Push with --force to update:\n",
		colors.Yellow("⚠"))
	fmt.Printf("  %s\n", colors.Bold("ivaldi upload --force"))

	return nil
}

// selectCommitRangeForShift provides interactive selection of commit range
func selectCommitRangeForShift(casStore cas.CAS, refsManager *refs.RefsManager, headHash [32]byte, timelineName string) (*SealInfo, *SealInfo, error) {
	// Get commit history
	allSeals, err := getCommitHistory(casStore, refsManager, headHash)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get commit history: %w", err)
	}

	if len(allSeals) < 2 {
		return nil, nil, fmt.Errorf("need at least 2 commits to squash")
	}

	// Phase 1: Select START commit (oldest in range)
	fmt.Printf("\n%s Select START of commit range (oldest):\n\n",
		colors.Bold("⏱"))

	startSeal, err := selectSealWithArrowKeys(allSeals, timelineName, 0, len(allSeals))
	if err != nil {
		return nil, nil, err
	}

	if startSeal == nil {
		return nil, nil, nil
	}

	// Phase 2: Select END commit (newest in range)
	// Filter to only show commits from HEAD to start
	var filteredSeals []SealInfo
	for _, seal := range allSeals {
		filteredSeals = append(filteredSeals, seal)
		if seal.Hash == startSeal.Hash {
			break
		}
	}

	fmt.Printf("\n%s Select END of commit range (newest):\n\n",
		colors.Bold("⏱"))

	// Mark the start position
	displaySealsWithMarker(filteredSeals, timelineName, startSeal.Hash)

	endSeal, err := selectSealWithArrowKeys(filteredSeals, timelineName, 0, len(filteredSeals))
	if err != nil {
		return nil, nil, err
	}

	if endSeal == nil {
		return nil, nil, nil
	}

	// Validate that end is after or equal to start
	if endSeal.Position > startSeal.Position {
		return nil, nil, fmt.Errorf("end commit must be newer than or equal to start commit")
	}

	return startSeal, endSeal, nil
}

// displaySealsWithMarker displays seals with a marker for the start commit
func displaySealsWithMarker(seals []SealInfo, timelineName string, startHash [32]byte) {
	fmt.Printf("\n%s Seals in timeline '%s':\n\n", colors.Bold("⏱"), colors.Bold(timelineName))

	for i, seal := range seals {
		var prefix string
		if seal.Hash == startHash {
			prefix = colors.Yellow("  [START] ")
		} else if i == 0 {
			prefix = colors.Dim("→ ")
		} else {
			prefix = "  "
		}

		sealHash := hex.EncodeToString(seal.Hash[:4])
		fmt.Printf("%s%d. %s (%s)\n", prefix, i+1, colors.Cyan(seal.SealName), colors.Gray(sealHash))
		fmt.Printf("     %s\n", seal.Message)
		fmt.Printf("     %s • %s\n\n", seal.Author, seal.Timestamp)
	}
}
