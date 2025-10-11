package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
)

// ConflictResolver provides an interactive UI for resolving merge conflicts.
type ConflictResolver struct {
	casStore cas.CAS
	reader   *bufio.Reader
}

// NewConflictResolver creates a new ConflictResolver.
func NewConflictResolver(casStore cas.CAS) *ConflictResolver {
	return &ConflictResolver{
		casStore: casStore,
		reader:   bufio.NewReader(os.Stdin),
	}
}

// ResolveConflicts interactively resolves conflicts for a merge result.
// Returns the resolved chunks or error.
func (cr *ConflictResolver) ResolveConflicts(result *diffmerge.ChunkMergeResult) ([]cas.Hash, error) {
	if result.Success {
		return result.MergedChunks, nil
	}

	fmt.Println()
	fmt.Printf("%s Conflicts detected in %s\n", colors.Yellow("[CONFLICT]"), colors.Bold(result.Path))
	fmt.Printf("  %d chunk conflict(s) need resolution\n\n", len(result.Conflicts))

	resolvedChunks := make([]cas.Hash, 0)

	for i, conflict := range result.Conflicts {
		fmt.Printf("%s Conflict %d/%d (chunk %d):\n", colors.Cyan(">>"), i+1, len(result.Conflicts), conflict.ChunkIndex)
		fmt.Println()

		choice, err := cr.showConflictAndGetChoice(conflict)
		if err != nil {
			return nil, err
		}

		resolvedChunk, err := cr.applyChoice(conflict, choice)
		if err != nil {
			return nil, err
		}

		if resolvedChunk != nil {
			resolvedChunks = append(resolvedChunks, *resolvedChunk)
		}
	}

	return resolvedChunks, nil
}

// showConflictAndGetChoice displays a conflict and gets user's resolution choice.
func (cr *ConflictResolver) showConflictAndGetChoice(conflict diffmerge.ChunkConflict) (diffmerge.ResolutionChoice, error) {
	// Show conflict details
	hasBase := conflict.BaseChunk != nil
	hasLeft := conflict.LeftChunk != nil
	hasRight := conflict.RightChunk != nil

	if hasBase {
		fmt.Println(colors.Gray("--- BASE (common ancestor) ---"))
		cr.showChunkPreview(conflict.BaseData)
		fmt.Println()
	}

	if hasLeft {
		fmt.Println(colors.Green("--- OURS (target timeline) ---"))
		cr.showChunkPreview(conflict.LeftData)
		fmt.Println()
	}

	if hasRight {
		fmt.Println(colors.Blue("--- THEIRS (source timeline) ---"))
		cr.showChunkPreview(conflict.RightData)
		fmt.Println()
	}

	// Show resolution options
	fmt.Println(colors.Bold("Resolution options:"))
	if hasLeft {
		fmt.Printf("  %s - Keep OURS (target timeline version)\n", colors.Green("[o]"))
	}
	if hasRight {
		fmt.Printf("  %s - Accept THEIRS (source timeline version)\n", colors.Blue("[t]"))
	}
	if hasBase {
		fmt.Printf("  %s - Revert to BASE (common ancestor)\n", colors.Gray("[b]"))
	}
	if hasLeft && hasRight {
		fmt.Printf("  %s - Use UNION (combine both)\n", colors.Yellow("[u]"))
	}
	fmt.Printf("  %s - Skip this file\n", colors.Dim("[s]"))
	fmt.Println()

	// Get user choice
	for {
		fmt.Print(colors.Cyan("Your choice> "))
		input, err := cr.reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "o", "ours":
			if hasLeft {
				return diffmerge.ChoiceOurs, nil
			}
			fmt.Println(colors.Red("Invalid choice: no target version available"))

		case "t", "theirs":
			if hasRight {
				return diffmerge.ChoiceTheirs, nil
			}
			fmt.Println(colors.Red("Invalid choice: no source version available"))

		case "b", "base":
			if hasBase {
				return diffmerge.ChoiceBase, nil
			}
			fmt.Println(colors.Red("Invalid choice: no base version available"))

		case "u", "union":
			if hasLeft && hasRight {
				return diffmerge.ChoiceUnion, nil
			}
			fmt.Println(colors.Red("Invalid choice: need both versions for union"))

		case "s", "skip":
			return "", fmt.Errorf("user skipped conflict resolution")

		default:
			fmt.Println(colors.Red("Invalid choice. Please try again."))
		}
	}
}

// showChunkPreview shows a preview of chunk content.
func (cr *ConflictResolver) showChunkPreview(data []byte) {
	if len(data) == 0 {
		fmt.Println(colors.Dim("  (empty)"))
		return
	}

	// Show first N lines or bytes
	maxLines := 20
	maxBytes := 1024

	content := data
	if len(content) > maxBytes {
		content = content[:maxBytes]
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	for _, line := range lines {
		fmt.Printf("  %s\n", line)
	}

	if len(data) > maxBytes {
		remaining := len(data) - maxBytes
		fmt.Println(colors.Dim(fmt.Sprintf("  ... (%d more bytes)", remaining)))
	}
}

// applyChoice applies the user's resolution choice to get the resolved chunk.
func (cr *ConflictResolver) applyChoice(conflict diffmerge.ChunkConflict, choice diffmerge.ResolutionChoice) (*cas.Hash, error) {
	switch choice {
	case diffmerge.ChoiceOurs:
		if conflict.LeftChunk != nil {
			return conflict.LeftChunk, nil
		}
		return nil, nil // Deleted in ours

	case diffmerge.ChoiceTheirs:
		if conflict.RightChunk != nil {
			return conflict.RightChunk, nil
		}
		return nil, nil // Deleted in theirs

	case diffmerge.ChoiceBase:
		if conflict.BaseChunk != nil {
			return conflict.BaseChunk, nil
		}
		return nil, nil // Didn't exist in base

	case diffmerge.ChoiceUnion:
		// For union, we need to combine the chunks
		// This is a simplified implementation
		if conflict.LeftChunk != nil && conflict.RightChunk != nil {
			// Combine both chunks
			combined := append(conflict.LeftData, conflict.RightData...)

			// Store the combined chunk
			hash := cas.SumB3(combined)
			err := cr.casStore.Put(hash, combined)
			if err != nil {
				return nil, fmt.Errorf("failed to store union chunk: %w", err)
			}
			return &hash, nil
		}
		// One side deleted - use whichever exists
		if conflict.LeftChunk != nil {
			return conflict.LeftChunk, nil
		}
		if conflict.RightChunk != nil {
			return conflict.RightChunk, nil
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown resolution choice: %s", choice)
	}
}

// ResolveAllConflicts resolves all conflicts in a merge using interactive resolution.
func (cr *ConflictResolver) ResolveAllConflicts(conflicts map[string]*diffmerge.ChunkMergeResult) (map[string][]cas.Hash, error) {
	resolved := make(map[string][]cas.Hash)

	for path, result := range conflicts {
		chunks, err := cr.ResolveConflicts(result)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", path, err)
		}
		resolved[path] = chunks
	}

	return resolved, nil
}

// ConfirmStrategy asks the user to confirm the merge strategy.
func (cr *ConflictResolver) ConfirmStrategy(strategy diffmerge.StrategyType, conflictCount int) (bool, error) {
	fmt.Println()
	fmt.Printf("%s Using merge strategy: %s\n", colors.Cyan(">>"), colors.Bold(string(strategy)))

	if conflictCount > 0 {
		fmt.Printf("  %s %d conflict(s) detected\n", colors.Yellow("!"), conflictCount)
	} else {
		fmt.Printf("  %s No conflicts - merge can proceed automatically\n", colors.Green("✓"))
	}
	fmt.Println()

	if strategy == diffmerge.StrategyAuto && conflictCount > 0 {
		fmt.Println(colors.Yellow("Auto-resolution found conflicts that need manual resolution."))
		fmt.Println("Options:")
		fmt.Printf("  %s - Resolve interactively\n", colors.Cyan("[i]"))
		fmt.Printf("  %s - Use %s strategy (keep target version)\n", colors.Green("[o]"), colors.Bold("ours"))
		fmt.Printf("  %s - Use %s strategy (accept source version)\n", colors.Blue("[t]"), colors.Bold("theirs"))
		fmt.Printf("  %s - Abort merge\n", colors.Red("[a]"))
		fmt.Println()

		for {
			fmt.Print(colors.Cyan("Your choice> "))
			input, err := cr.reader.ReadString('\n')
			if err != nil {
				return false, err
			}

			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "i", "interactive":
				return true, nil
			case "o", "ours":
				fmt.Println(colors.Green("Using 'ours' strategy - keeping target timeline version"))
				return true, nil
			case "t", "theirs":
				fmt.Println(colors.Blue("Using 'theirs' strategy - accepting source timeline version"))
				return true, nil
			case "a", "abort":
				return false, nil
			default:
				fmt.Println(colors.Red("Invalid choice. Please try again."))
			}
		}
	}

	// No conflicts or non-auto strategy - proceed
	return true, nil
}
