package cli

import (
	"fmt"
	"os"

	"github.com/javanhut/Ivaldi-vcs/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive TUI dashboard",
	Long:  `Opens an interactive terminal UI for managing your Ivaldi repository — staging, status, logs, and more.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ivaldiDir := ".ivaldi"
		if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
			return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
		}

		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		return tui.Run(workDir, ivaldiDir)
	},
}
