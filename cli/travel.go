package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var travelCmd = &cobra.Command{
	Use:   "travel",
	Short: "Interactively browse and travel to previous seals",
	Long: `Browse previous seals in the current timeline and travel to a specific point in history.
From there, you can either:
- Create a new timeline branching from that point (non-destructive)
- Overwrite all changes after that point (destructive)

Flags:
  --limit N     Show only the N most recent seals (default: 20)
  --all         Show all seals (no pagination)
  --search TEXT Search for seals containing TEXT in message`,
	RunE: runTravel,
}

func init() {
	travelCmd.Flags().IntP("limit", "n", 20, "Number of recent seals to show (0 for all)")
	travelCmd.Flags().BoolP("all", "a", false, "Show all seals without pagination")
	travelCmd.Flags().StringP("search", "s", "", "Search for seals by message content")
}

// SealInfo holds information about a seal for display
type SealInfo struct {
	Hash      [32]byte
	SealName  string
	Message   string
	Author    string
	Timestamp string
	Position  int // Position in history (0 = current, 1 = previous, etc.)
}

func runTravel(cmd *cobra.Command, args []string) error {
	// Check if we're in an Ivaldi repository
	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Get flags
	limit, _ := cmd.Flags().GetInt("limit")
	showAll, _ := cmd.Flags().GetBool("all")
	searchTerm, _ := cmd.Flags().GetString("search")

	if showAll {
		limit = 0 // 0 means no limit
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

	// Get commit history
	allSeals, err := getCommitHistory(casStore, refsManager, timeline.Blake3Hash)
	if err != nil {
		return fmt.Errorf("failed to get commit history: %w", err)
	}

	if len(allSeals) == 0 {
		return fmt.Errorf("no seals found in timeline")
	}

	// Filter seals if search term provided
	var seals []SealInfo
	if searchTerm != "" {
		seals = filterSeals(allSeals, searchTerm)
		if len(seals) == 0 {
			return fmt.Errorf("no seals found matching '%s'", searchTerm)
		}
	} else {
		seals = allSeals
	}

	// Display seals and let user select (with pagination if needed)
	selectedSeal, err := selectSealWithPagination(seals, currentTimeline, limit)
	if err != nil {
		return err
	}

	if selectedSeal == nil {
		fmt.Println("Travel cancelled.")
		return nil
	}

	// Check if selected seal is current
	if selectedSeal.Position == 0 {
		fmt.Printf("%s Already at this seal.\n", colors.InfoText("ℹ"))
		return nil
	}

	// Show what will happen
	fmt.Printf("\n%s Selected seal: %s\n", colors.Cyan("→"), colors.Bold(selectedSeal.SealName))
	fmt.Printf("  Position: %d commits behind current HEAD\n", selectedSeal.Position)
	fmt.Printf("  Message: %s\n", colors.Gray(selectedSeal.Message))

	// Ask user what to do
	action, newTimelineName, err := promptForAction(currentTimeline, selectedSeal)
	if err != nil {
		return err
	}

	switch action {
	case "diverge":
		return createDivergentTimeline(casStore, refsManager, ivaldiDir, workDir, currentTimeline, newTimelineName, selectedSeal)
	case "overwrite":
		return overwriteTimeline(casStore, refsManager, ivaldiDir, workDir, currentTimeline, selectedSeal)
	case "cancel":
		fmt.Println("Travel cancelled.")
		return nil
	}

	return nil
}

// getCommitHistory retrieves the full commit history
func getCommitHistory(casStore cas.CAS, refsManager *refs.RefsManager, headHash [32]byte) ([]SealInfo, error) {
	var seals []SealInfo
	commitReader := commit.NewCommitReader(casStore)

	var currentHash cas.Hash
	copy(currentHash[:], headHash[:])
	position := 0

	visited := make(map[cas.Hash]bool)

	for {
		// Check for cycles
		if visited[currentHash] {
			break
		}
		visited[currentHash] = true

		// Read commit
		commitObj, err := commitReader.ReadCommit(currentHash)
		if err != nil {
			break
		}

		// Get seal name
		var hashArray [32]byte
		copy(hashArray[:], currentHash[:])
		sealName, err := refsManager.GetSealNameByHash(hashArray)
		if err != nil || sealName == "" {
			sealName = hex.EncodeToString(currentHash[:4])
		}

		// Create seal info
		seal := SealInfo{
			Hash:      hashArray,
			SealName:  sealName,
			Message:   commitObj.Message,
			Author:    commitObj.Author,
			Timestamp: commitObj.CommitTime.Format("2006-01-02 15:04:05"),
			Position:  position,
		}
		seals = append(seals, seal)

		// Move to parent
		if len(commitObj.Parents) == 0 {
			break
		}

		currentHash = commitObj.Parents[0]
		position++
	}

	return seals, nil
}

// selectSealWithPagination displays seals with pagination and lets user select one
func selectSealWithPagination(seals []SealInfo, timelineName string, pageSize int) (*SealInfo, error) {
	totalSeals := len(seals)

	// If no limit or seals fit on one page, use arrow key navigation
	if pageSize == 0 || totalSeals <= pageSize {
		return selectSealWithArrowKeys(seals, timelineName, 0, totalSeals)
	}

	// Paginated display with arrow key navigation
	currentPage := 0
	totalPages := (totalSeals + pageSize - 1) / pageSize
	cursorPos := 0 // Cursor position within current page

	for {
		startIdx := currentPage * pageSize
		endIdx := startIdx + pageSize
		if endIdx > totalSeals {
			endIdx = totalSeals
		}

		// Display current page with cursor
		displaySealsWithCursor(seals, timelineName, startIdx, endIdx, startIdx+cursorPos, totalSeals, currentPage, totalPages)

		// Read key input
		key, err := readKey()
		if err != nil {
			return nil, fmt.Errorf("failed to read key: %w", err)
		}

		pageSize := endIdx - startIdx

		switch key {
		case "up":
			if cursorPos > 0 {
				cursorPos--
			} else if currentPage > 0 {
				// Move to previous page, last item
				currentPage--
				newStart := currentPage * pageSize
				newEnd := newStart + pageSize
				if newEnd > totalSeals {
					newEnd = totalSeals
				}
				cursorPos = (newEnd - newStart) - 1
			}

		case "down":
			if cursorPos < pageSize-1 {
				cursorPos++
			} else if currentPage < totalPages-1 {
				// Move to next page, first item
				currentPage++
				cursorPos = 0
			}

		case "enter":
			selectedIdx := startIdx + cursorPos
			return &seals[selectedIdx], nil

		case "q":
			return nil, nil

		case "n":
			if currentPage < totalPages-1 {
				currentPage++
				cursorPos = 0
			}

		case "p":
			if currentPage > 0 {
				currentPage--
				cursorPos = 0
			}

		default:
			// Try to parse as number
			if num, err := strconv.Atoi(key); err == nil {
				if num >= 1 && num <= totalSeals {
					return &seals[num-1], nil
				}
			}
		}
	}
}

// displaySealsWithCursor displays seals with a cursor highlight
func displaySealsWithCursor(seals []SealInfo, timelineName string, startIdx, endIdx, cursorIdx, totalSeals, currentPage, totalPages int) {
	// Clear screen
	fmt.Print("\033[2J\033[H")

	if totalPages > 1 {
		fmt.Printf("\n%s Seals in timeline '%s' (showing %d-%d of %d):\n\n",
			colors.Bold("⏱"), colors.Bold(timelineName),
			startIdx+1, endIdx, totalSeals)
	} else {
		fmt.Printf("\n%s Seals in timeline '%s':\n\n", colors.Bold("⏱"), colors.Bold(timelineName))
	}

	for i := startIdx; i < endIdx; i++ {
		seal := seals[i]

		// Determine prefix (cursor or HEAD marker)
		var prefix string
		if i == cursorIdx {
			// Current cursor position - highlighted
			prefix = colors.Green("→ ")
		} else if i == 0 {
			// HEAD but not selected
			prefix = colors.Dim("→ ")
		} else {
			prefix = "  "
		}

		// Highlight entire line if cursor is on it
		sealName := seal.SealName
		sealHash := hex.EncodeToString(seal.Hash[:4])
		message := seal.Message
		authorTime := fmt.Sprintf("%s • %s", seal.Author, seal.Timestamp)

		if i == cursorIdx {
			// Highlighted/selected line
			fmt.Printf("%s%d. %s (%s)\n", prefix, i+1, colors.Bold(colors.Cyan(sealName)), colors.Bold(colors.Gray(sealHash)))
			fmt.Printf("     %s\n", colors.Bold(message))
			fmt.Printf("     %s\n", colors.Bold(colors.Gray(authorTime)))
		} else {
			// Normal line
			fmt.Printf("%s%d. %s (%s)\n", prefix, i+1, colors.Cyan(sealName), colors.Gray(sealHash))
			fmt.Printf("     %s\n", message)
			fmt.Printf("     %s\n", colors.Gray(authorTime))
		}

		if i < endIdx-1 {
			fmt.Println()
		}
	}

	// Show navigation help
	fmt.Println()
	if totalPages > 1 {
		fmt.Printf("%s\n", colors.Dim(fmt.Sprintf("Page %d of %d", currentPage+1, totalPages)))
	}

	var helpItems []string
	helpItems = append(helpItems, "↑/↓ navigate")
	helpItems = append(helpItems, "Enter to select")
	if totalPages > 1 {
		helpItems = append(helpItems, "n/p page")
	}
	helpItems = append(helpItems, "q to quit")

	fmt.Printf("%s\n", colors.Dim(strings.Join(helpItems, " • ")))
}

// selectSealWithArrowKeys displays seals and lets user select with arrow keys
func selectSealWithArrowKeys(seals []SealInfo, timelineName string, startIdx, endIdx int) (*SealInfo, error) {
	cursorPos := 0 // Start at first seal

	for {
		// Display seals with cursor
		displaySealsWithCursor(seals, timelineName, startIdx, endIdx, cursorPos, endIdx-startIdx, 0, 1)

		// Read key input
		key, err := readKey()
		if err != nil {
			return nil, fmt.Errorf("failed to read key: %w", err)
		}

		switch key {
		case "up":
			if cursorPos > 0 {
				cursorPos--
			}

		case "down":
			if cursorPos < endIdx-startIdx-1 {
				cursorPos++
			}

		case "enter":
			return &seals[cursorPos], nil

		case "q":
			return nil, nil

		default:
			// Try to parse as number
			if num, err := strconv.Atoi(key); err == nil {
				if num >= 1 && num <= endIdx {
					return &seals[num-1], nil
				}
			}
		}
	}
}

// readKey reads a single key press (including arrow keys)
func readKey() (string, error) {
	// Save old terminal state
	oldState, err := term.MakeRaw(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(syscall.Stdin), oldState)

	buf := make([]byte, 3)
	n, err := os.Stdin.Read(buf)
	if err != nil {
		return "", err
	}

	// Handle escape sequences (arrow keys)
	if n == 3 && buf[0] == 27 && buf[1] == 91 {
		switch buf[2] {
		case 65: // Up arrow
			return "up", nil
		case 66: // Down arrow
			return "down", nil
		case 67: // Right arrow
			return "right", nil
		case 68: // Left arrow
			return "left", nil
		}
	}

	// Handle single characters
	if n == 1 {
		switch buf[0] {
		case 10, 13: // Enter
			return "enter", nil
		case 27: // ESC
			return "q", nil
		case 'q', 'Q':
			return "q", nil
		case 'n', 'N':
			return "n", nil
		case 'p', 'P':
			return "p", nil
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			// For number input, we need to read more characters
			return string(buf[0]), nil
		}
	}

	return "", nil
}

// filterSeals filters seals by search term in message, author, or seal name
func filterSeals(seals []SealInfo, searchTerm string) []SealInfo {
	searchLower := strings.ToLower(searchTerm)
	var filtered []SealInfo

	for _, seal := range seals {
		if strings.Contains(strings.ToLower(seal.Message), searchLower) ||
			strings.Contains(strings.ToLower(seal.Author), searchLower) ||
			strings.Contains(strings.ToLower(seal.SealName), searchLower) {
			filtered = append(filtered, seal)
		}
	}

	return filtered
}

// promptForAction asks user what they want to do at the selected seal
func promptForAction(currentTimeline string, seal *SealInfo) (action string, newTimelineName string, err error) {
	fmt.Printf("\n%s What would you like to do?\n\n", colors.Bold("?"))
	fmt.Printf("  1. %s - Create new timeline from this seal (keeps current timeline intact)\n", colors.Green("Diverge"))
	fmt.Printf("  2. %s - Overwrite current timeline (removes all commits after this seal)\n", colors.Yellow("Overwrite"))
	fmt.Printf("  3. %s - Cancel\n", colors.Gray("Cancel"))

	fmt.Print("\nChoice (1/2/3): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)

	switch input {
	case "1":
		// Ask for new timeline name
		fmt.Printf("\nEnter new timeline name: ")
		nameInput, err := reader.ReadString('\n')
		if err != nil {
			return "", "", fmt.Errorf("failed to read timeline name: %w", err)
		}
		newTimelineName = strings.TrimSpace(nameInput)
		if newTimelineName == "" {
			return "", "", fmt.Errorf("timeline name cannot be empty")
		}
		return "diverge", newTimelineName, nil

	case "2":
		// Confirm overwrite
		fmt.Printf("\n%s This will permanently remove %d commit(s) from '%s'.\n",
			colors.Yellow("⚠ WARNING:"), seal.Position, currentTimeline)
		fmt.Print("Are you sure? Type 'yes' to confirm: ")
		confirmInput, err := reader.ReadString('\n')
		if err != nil {
			return "", "", fmt.Errorf("failed to read confirmation: %w", err)
		}
		confirm := strings.TrimSpace(confirmInput)
		if confirm != "yes" {
			return "cancel", "", nil
		}
		return "overwrite", "", nil

	case "3", "":
		return "cancel", "", nil

	default:
		return "", "", fmt.Errorf("invalid choice")
	}
}

// createDivergentTimeline creates a new timeline branching from the selected seal
func createDivergentTimeline(casStore cas.CAS, refsManager *refs.RefsManager, ivaldiDir, workDir, currentTimeline, newTimelineName string, seal *SealInfo) error {
	// Check if timeline already exists
	existing, _ := refsManager.GetTimeline(newTimelineName, refs.LocalTimeline)
	if existing != nil {
		return fmt.Errorf("timeline '%s' already exists", newTimelineName)
	}

	// Create new timeline pointing to the selected seal
	err := refsManager.CreateTimeline(
		newTimelineName,
		refs.LocalTimeline,
		seal.Hash,
		[32]byte{},
		"",
		fmt.Sprintf("Diverged from '%s' at seal %s", currentTimeline, seal.SealName),
	)
	if err != nil {
		return fmt.Errorf("failed to create new timeline: %w", err)
	}

	fmt.Printf("%s Created new timeline '%s' from seal %s\n",
		colors.Green("✓"), colors.Bold(newTimelineName), colors.Cyan(seal.SealName))

	// Switch to new timeline
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
	err = materializer.MaterializeTimeline(newTimelineName)
	if err != nil {
		return fmt.Errorf("failed to switch to new timeline: %w", err)
	}

	fmt.Printf("%s Switched to timeline '%s'\n", colors.Green("✓"), colors.Bold(newTimelineName))
	fmt.Printf("%s Workspace materialized to seal: %s\n", colors.InfoText("ℹ"), seal.SealName)

	return nil
}

// overwriteTimeline overwrites the current timeline to the selected seal
func overwriteTimeline(casStore cas.CAS, refsManager *refs.RefsManager, ivaldiDir, workDir, currentTimeline string, seal *SealInfo) error {
	// Update timeline to point to the selected seal
	err := refsManager.UpdateTimeline(
		currentTimeline,
		refs.LocalTimeline,
		seal.Hash,
		[32]byte{},
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to update timeline: %w", err)
	}

	// Materialize workspace to this seal
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)

	// Get timeline with updated hash
	timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get updated timeline: %w", err)
	}

	// Create target index from the seal
	targetIndex, err := materializer.CreateTargetIndex(*timeline)
	if err != nil {
		return fmt.Errorf("failed to create target index: %w", err)
	}

	// Get current state
	currentState, err := materializer.GetCurrentState()
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	// Apply changes
	differ := diffmerge.NewDiffer(casStore)
	diff, err := differ.DiffWorkspaces(currentState.Index, targetIndex)
	if err != nil {
		return fmt.Errorf("failed to compute diff: %w", err)
	}

	err = applyWorkspaceChanges(materializer, diff)
	if err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	fmt.Printf("%s Timeline '%s' reset to seal %s\n",
		colors.Yellow("⚠"), colors.Bold(currentTimeline), colors.Cyan(seal.SealName))
	fmt.Printf("%s %d commit(s) removed from timeline\n",
		colors.InfoText("ℹ"), seal.Position)
	fmt.Printf("%s Workspace materialized to seal: %s\n", colors.InfoText("ℹ"), seal.SealName)

	return nil
}

// applyWorkspaceChanges is a helper to apply workspace changes
func applyWorkspaceChanges(m *workspace.Materializer, diff *diffmerge.WorkspaceDiff) error {
	return m.ApplyChangesToWorkspace(diff)
}
