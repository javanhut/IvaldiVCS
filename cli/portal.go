package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

var portalCmd = &cobra.Command{
	Use:   "portal",
	Short: "Manage repository connections",
	Long:  `Manage GitHub repository connections for this Ivaldi repository. Portal commands allow you to see, add, or remove repository connections.`,
}

var portalAddCmd = &cobra.Command{
	Use:   "add <github:owner/repo>",
	Short: "Add a GitHub repository connection",
	Long: `Add or update the GitHub repository connection for this Ivaldi repository.
Examples:
  ivaldi portal add github:myuser/myproject
  ivaldi portal add myuser/myproject              # github: prefix is optional`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we're in an Ivaldi repository
		ivaldiDir := ".ivaldi"
		if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
			return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
		}

		// Parse repository argument
		repoArg := args[0]

		// Remove github: prefix if present
		repoArg, _ = strings.CutPrefix(repoArg, "github:")

		// Validate format
		parts := strings.Split(repoArg, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid repository format. Use: owner/repo")
		}

		owner, repo := parts[0], parts[1]

		// Initialize refs manager
		refsManager, err := refs.NewRefsManager(ivaldiDir)
		if err != nil {
			return fmt.Errorf("failed to initialize refs manager: %w", err)
		}
		defer refsManager.Close()

		// Set GitHub repository
		if err := refsManager.SetGitHubRepository(owner, repo); err != nil {
			return fmt.Errorf("failed to set GitHub repository: %w", err)
		}

		fmt.Printf("[OK] Added GitHub repository connection: %s/%s\n", owner, repo)
		return nil
	},
}

var portalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List repository connections",
	Long:  `List the current GitHub repository connections for this Ivaldi repository.`,
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

		// Get GitHub repository
		owner, repo, err := refsManager.GetGitHubRepository()
		if err != nil {
			fmt.Println("No GitHub repository connections configured.")
			fmt.Println("Use 'ivaldi portal add owner/repo' to add one.")
			return nil
		}

		// Get current timeline for additional context
		currentTimeline, err := refsManager.GetCurrentTimeline()
		if err != nil {
			currentTimeline = "unknown"
		}

		fmt.Println("Repository Connections:")
		fmt.Printf("  GitHub: %s/%s\n", owner, repo)
		fmt.Printf("  Current Timeline: %s\n", currentTimeline)
		fmt.Printf("  Upload Command: ivaldi upload\n")

		return nil
	},
}

var portalRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove repository connection",
	Long:  `Remove the current GitHub repository connection for this Ivaldi repository.`,
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

		// Check if there's a configuration to remove
		owner, repo, err := refsManager.GetGitHubRepository()
		if err != nil {
			fmt.Println("No GitHub repository connection configured.")
			return nil
		}

		// Remove GitHub repository configuration
		if err := refsManager.RemoveGitHubRepository(); err != nil {
			return fmt.Errorf("failed to remove GitHub repository: %w", err)
		}

		fmt.Printf("[OK] Removed GitHub repository connection: %s/%s\n", owner, repo)
		fmt.Println("You can now use 'ivaldi portal add owner/repo' to configure a new connection.")

		return nil
	},
}
