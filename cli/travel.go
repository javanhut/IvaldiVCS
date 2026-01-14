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
	travelCmd.Flags().IntP("window-size", "w", 0, "Number of seals to show in viewport (0 for auto-detect)")
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
	windowSize, _ := cmd.Flags().GetInt("window-size")
	searchTerm, _ := cmd.Flags().GetString("search")

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

	// Display seals and let user select with fixed window scrolling
	selectedSeal, err := selectSealWithScrollWindow(seals, currentTimeline, windowSize)
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

// selectSealWithScrollWindow displays seals with a fixed scrolling window
func selectSealWithScrollWindow(seals []SealInfo, timelineName string, windowSize int) (*SealInfo, error) {
	totalSeals := len(seals)

	// Auto-detect window size from terminal height if not specified
	if windowSize <= 0 {
		width, height, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil || height < 10 {
			// Fallback to default
			windowSize = 10
		} else {
			// Reserve 6 lines for header/footer/spacing
			windowSize = height - 6
			// Each seal takes 4 lines (number+name, message, author, blank)
			windowSize = windowSize / 4
			if windowSize < 5 {
				windowSize = 5
			}
			if windowSize > 20 {
				windowSize = 20
			}
		}
		_ = width // Suppress unused variable warning
	}

	// If all seals fit in window, adjust window size
	if totalSeals < windowSize {
		windowSize = totalSeals
	}

	windowStart := 0 // First seal shown in window
	cursorPos := 0   // Cursor position within window (0 to windowSize-1)
	needsFullRedraw := true

	for {
		// Calculate window bounds
		windowEnd := windowStart + windowSize
		if windowEnd > totalSeals {
			windowEnd = totalSeals
		}
		actualWindowSize := windowEnd - windowStart

		// Display window
		if needsFullRedraw {
			displayFixedWindow(seals, timelineName, windowStart, windowEnd, cursorPos, totalSeals)
			needsFullRedraw = false
		} else {
			updateCursor(seals, timelineName, windowStart, windowEnd, cursorPos, totalSeals)
		}

		// Read key input
		key, err := readKey()
		if err != nil {
			return nil, fmt.Errorf("failed to read key: %w", err)
		}

		switch key {
		case "up":
			if cursorPos > 0 {
				// Move cursor up within window
				cursorPos--
			} else if windowStart > 0 {
				// Scroll window up
				windowStart--
				needsFullRedraw = true
			}

		case "down":
			if cursorPos < actualWindowSize-1 {
				// Move cursor down within window
				cursorPos++
			} else if windowEnd < totalSeals {
				// Scroll window down
				windowStart++
				needsFullRedraw = true
			}

		case "enter":
			absoluteIdx := windowStart + cursorPos
			// Restore terminal before returning
			fmt.Print("\033[?25h") // Show cursor
			return &seals[absoluteIdx], nil

		case "q":
			// Restore terminal before returning
			fmt.Print("\033[?25h") // Show cursor
			return nil, nil

		case "home":
			// Jump to top
			windowStart = 0
			cursorPos = 0
			needsFullRedraw = true

		case "end":
			// Jump to bottom
			windowStart = totalSeals - windowSize
			if windowStart < 0 {
				windowStart = 0
			}
			cursorPos = totalSeals - windowStart - 1
			needsFullRedraw = true

		default:
			// Try to parse as number
			if num, err := strconv.Atoi(key); err == nil {
				if num >= 1 && num <= totalSeals {
					// Jump to specific seal
					absoluteIdx := num - 1
					// Calculate window position to show selected seal
					if absoluteIdx < windowStart || absoluteIdx >= windowEnd {
						// Recenter window on selected item
						windowStart = absoluteIdx - windowSize/2
						if windowStart < 0 {
							windowStart = 0
						}
						if windowStart+windowSize > totalSeals {
							windowStart = totalSeals - windowSize
							if windowStart < 0 {
								windowStart = 0
							}
						}
						cursorPos = absoluteIdx - windowStart
						needsFullRedraw = true
					} else {
						// Item already in window, just move cursor
						cursorPos = absoluteIdx - windowStart
					}
				}
			}
		}
	}
}

// displayFixedWindow displays the entire fixed-size scrolling window
func displayFixedWindow(seals []SealInfo, timelineName string, windowStart, windowEnd, cursorPos, totalSeals int) {
	// Clear screen and hide cursor
	fmt.Print("\033[2J\033[H\033[?25l")

	// Header with scroll indicator
	scrollIndicator := buildScrollIndicator(windowStart, windowEnd, totalSeals)
	fmt.Printf("╔═══════════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ %s Seals in timeline '%s' %s ║\n",
		colors.Bold("⏱"),
		colors.Bold(timelineName),
		strings.Repeat(" ", 60-len(timelineName)-20))
	fmt.Printf("║ Showing %d-%d of %d %s%s ║\n",
		windowStart+1, windowEnd, totalSeals,
		scrollIndicator,
		strings.Repeat(" ", 60-len(scrollIndicator)-len(fmt.Sprintf("Showing %d-%d of %d ", windowStart+1, windowEnd, totalSeals))))
	fmt.Printf("╚═══════════════════════════════════════════════════════════════════════╝\n\n")

	// Display seals in window
	for i := windowStart; i < windowEnd; i++ {
		seal := seals[i]
		relativePos := i - windowStart
		isSelected := (relativePos == cursorPos)
		isHead := (seal.Position == 0)

		displaySealLine(seal, i+1, isSelected, isHead)
	}

	// Footer with help
	fmt.Println()
	fmt.Printf("╔═══════════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ %s ║\n",
		colors.Dim("↑/↓ navigate • Enter select • Home/End jump • 1-9 goto • q quit")[:71])
	fmt.Printf("╚═══════════════════════════════════════════════════════════════════════╝\n")
}

// updateCursor efficiently updates just the cursor position (no full redraw)
func updateCursor(seals []SealInfo, timelineName string, windowStart, windowEnd, cursorPos, totalSeals int) {
	// This is a simplified version - for now just do full redraw
	// In a more advanced version, we would use ANSI escape codes to update specific lines
	displayFixedWindow(seals, timelineName, windowStart, windowEnd, cursorPos, totalSeals)
}

// buildScrollIndicator creates a visual scroll position indicator
func buildScrollIndicator(windowStart, windowEnd, totalSeals int) string {
	if totalSeals <= windowEnd-windowStart {
		return "[■■■■■■■■■■]" // All items visible
	}

	barLength := 10
	position := float64(windowStart) / float64(totalSeals)
	windowRatio := float64(windowEnd-windowStart) / float64(totalSeals)

	filledStart := int(position * float64(barLength))
	filledLength := int(windowRatio * float64(barLength))
	if filledLength < 1 {
		filledLength = 1
	}

	bar := "["
	for i := 0; i < barLength; i++ {
		if i >= filledStart && i < filledStart+filledLength {
			bar += "■"
		} else {
			bar += "·"
		}
	}
	bar += "]"

	return bar
}

// displaySealLine displays a single seal with appropriate highlighting
func displaySealLine(seal SealInfo, number int, isSelected, isHead bool) {
	sealName := seal.SealName
	sealHash := hex.EncodeToString(seal.Hash[:4])
	message := seal.Message
	if len(message) > 60 {
		message = message[:57] + "..."
	}
	authorTime := fmt.Sprintf("%s • %s", seal.Author, seal.Timestamp)

	var prefix string
	if isSelected {
		if isHead {
			prefix = colors.Green("→ ") + colors.Bold("[HEAD] ")
		} else {
			prefix = colors.Green("→ ")
		}
	} else if isHead {
		prefix = "  " + colors.Dim("[HEAD] ")
	} else {
		prefix = "  "
	}

	if isSelected {
		// Highlighted line with background
		fmt.Printf("%s%s%d. %s (%s)%s\n",
			prefix,
			colors.Bold(colors.Green("")),
			number,
			colors.Bold(colors.Cyan(sealName)),
			colors.Bold(colors.Gray(sealHash)),
			colors.Bold(""))
		fmt.Printf("     %s\n", colors.Bold(message))
		fmt.Printf("     %s\n\n", colors.Bold(colors.Gray(authorTime)))
	} else {
		// Normal line
		fmt.Printf("%s%d. %s (%s)\n", prefix, number, colors.Cyan(sealName), colors.Gray(sealHash))
		fmt.Printf("     %s\n", message)
		fmt.Printf("     %s\n\n", colors.Gray(authorTime))
	}
}

// readKey reads a single key press (including arrow keys and special keys)
func readKey() (string, error) {
	// Save old terminal state
	oldState, err := term.MakeRaw(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(syscall.Stdin), oldState)

	buf := make([]byte, 6)
	n, err := os.Stdin.Read(buf)
	if err != nil {
		return "", err
	}

	// Handle escape sequences (arrow keys and special keys)
	if n >= 3 && buf[0] == 27 && buf[1] == 91 {
		switch buf[2] {
		case 65: // Up arrow
			return "up", nil
		case 66: // Down arrow
			return "down", nil
		case 67: // Right arrow
			return "right", nil
		case 68: // Left arrow
			return "left", nil
		case 72: // Home key
			return "home", nil
		case 70: // End key
			return "end", nil
		case 49: // Extended escape sequences
			if n >= 4 && buf[3] == 126 {
				return "home", nil // Home on some terminals
			}
		case 52: // Extended escape sequences
			if n >= 4 && buf[3] == 126 {
				return "end", nil // End on some terminals
			}
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
		case 'h', 'H':
			return "home", nil
		case 'e', 'E':
			return "end", nil
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			// For number input, accumulate digits
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
