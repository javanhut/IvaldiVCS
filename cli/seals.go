package cli

import (
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

var sealsCmd = &cobra.Command{
	Use:   "seals",
	Short: "Manage seals (commits) with human-friendly names",
	Long: `The seals command provides functionality to list, show, and manage 
seals (commits) with their human-friendly generated names.

Seals use a 4-word naming pattern: adjective-noun-verb-adverb-hash8
Example: swift-eagle-flies-high-447abe9b`,
}

var sealsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all seals with their names",
	Long:    `List all seals in the repository with their generated names, timestamps, and messages.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Get all seal names
		sealNames, err := refsManager.ListSealNames()
		if err != nil {
			return fmt.Errorf("failed to list seals: %w", err)
		}

		if len(sealNames) == 0 {
			fmt.Println("No seals found.")
			fmt.Println("Create your first seal with: ivaldi gather <files> && ivaldi seal <message>")
			return nil
		}

		// Collect seal information for sorting
		type sealInfo struct {
			name      string
			hash      [32]byte
			timestamp time.Time
			message   string
		}

		var seals []sealInfo
		for _, sealName := range sealNames {
			hash, timestamp, message, err := refsManager.GetSealByName(sealName)
			if err != nil {
				fmt.Printf("Warning: Failed to get seal info for %s: %v\n", sealName, err)
				continue
			}

			seals = append(seals, sealInfo{
				name:      sealName,
				hash:      hash,
				timestamp: timestamp,
				message:   message,
			})
		}

		// Sort by timestamp (newest first)
		sort.Slice(seals, func(i, j int) bool {
			return seals[i].timestamp.After(seals[j].timestamp)
		})

		// Display seals
		fmt.Printf("Seals in repository (%d total):\n\n", len(seals))
		for _, seal := range seals {
			timeAgo := formatTimeAgo(seal.timestamp)
			shortHash := hex.EncodeToString(seal.hash[:4])

			// Truncate message if too long
			message := seal.message
			if len(message) > 50 {
				message = message[:47] + "..."
			}

			fmt.Printf("%-40s %s (%s)\n", seal.name, shortHash, timeAgo)
			fmt.Printf("  \"%s\"\n\n", message)
		}

		return nil
	},
}

var sealsShowCmd = &cobra.Command{
	Use:   "show <seal-name|hash>",
	Short: "Show detailed information about a seal",
	Args:  cobra.ExactArgs(1),
	Long: `Show detailed information about a specific seal, including its full hash,
timestamp, message, and other metadata. You can reference seals by their full name,
name prefix, or hash.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sealRef := args[0]

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

		// Try to resolve seal reference
		sealName, hash, timestamp, message, err := resolveSealReference(refsManager, sealRef)
		if err != nil {
			return fmt.Errorf("failed to find seal: %w", err)
		}

		// Display detailed seal information
		fmt.Printf("Seal: %s\n", sealName)
		fmt.Printf("Hash: %s\n", hex.EncodeToString(hash[:]))
		fmt.Printf("Short Hash: %s\n", hex.EncodeToString(hash[:4]))
		fmt.Printf("Created: %s (%s)\n", timestamp.Format("2006-01-02 15:04:05"), formatTimeAgo(timestamp))
		fmt.Printf("Message: %s\n", message)

		return nil
	},
}

// resolveSealReference resolves a seal reference (name, prefix, or hash) to full seal info
func resolveSealReference(refsManager *refs.RefsManager, sealRef string) (string, [32]byte, time.Time, string, error) {
	// First try exact name match
	if hash, timestamp, message, err := refsManager.GetSealByName(sealRef); err == nil {
		return sealRef, hash, timestamp, message, nil
	}

	// Get all seal names for prefix/partial matching
	sealNames, err := refsManager.ListSealNames()
	if err != nil {
		return "", [32]byte{}, time.Time{}, "", fmt.Errorf("failed to list seals: %w", err)
	}

	var matches []string

	// Check for prefix matches or hash matches
	for _, sealName := range sealNames {
		// Check if input matches the seal name prefix (without hash suffix)
		if sealName == sealRef {
			matches = append(matches, sealName)
			continue
		}

		// Check if input is a prefix of the seal name
		if len(sealRef) >= 4 && len(sealRef) <= len(sealName) && sealName[:len(sealRef)] == sealRef {
			matches = append(matches, sealName)
			continue
		}

		// Check if input matches the hash part
		hash, _, _, err := refsManager.GetSealByName(sealName)
		if err == nil {
			hashStr := hex.EncodeToString(hash[:])
			shortHashStr := hex.EncodeToString(hash[:4])

			if len(sealRef) <= len(hashStr) && hashStr[:len(sealRef)] == sealRef {
				matches = append(matches, sealName)
				continue
			}
			if len(sealRef) <= len(shortHashStr) && shortHashStr[:len(sealRef)] == sealRef {
				matches = append(matches, sealName)
				continue
			}
		}
	}

	if len(matches) == 0 {
		return "", [32]byte{}, time.Time{}, "", fmt.Errorf("no seal found matching '%s'", sealRef)
	}

	if len(matches) > 1 {
		fmt.Fprintf(os.Stderr, "Multiple seals match '%s':\n", sealRef)
		for _, match := range matches {
			fmt.Fprintf(os.Stderr, "  %s\n", match)
		}
		return "", [32]byte{}, time.Time{}, "", fmt.Errorf("ambiguous seal reference")
	}

	// Single match found
	hash, timestamp, message, err := refsManager.GetSealByName(matches[0])
	if err != nil {
		return "", [32]byte{}, time.Time{}, "", fmt.Errorf("failed to get seal info: %w", err)
	}

	return matches[0], hash, timestamp, message, nil
}

func init() {
	sealsCmd.AddCommand(sealsListCmd, sealsShowCmd)
}
