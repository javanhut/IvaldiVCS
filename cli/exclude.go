package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var excludeCommand = &cobra.Command{
	Use:   "exclude",
	Args:  cobra.MinimumNArgs(1),
	Short: "Excludes a file from gather",
	Long:  `Create a ivaldiignore file if it does exist and otherwise adds file to existing ignore file.`,
	RunE:  createOrAddExclude,
}

func createOrAddExclude(cmd *cobra.Command, args []string) error {
	ignoreFile := ".ivaldiignore"
	f, err := os.OpenFile(ignoreFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, pattern := range args {
		if _, err := f.WriteString(pattern + "\n"); err != nil {
			return fmt.Errorf("failed to write pattern '%s': %w", pattern, err)
		}
		fmt.Printf("Added '%s' to .ivaldiignore\n", pattern)
	}

	fmt.Printf("Successfully added %d patterns to .ivaldiignore\n", len(args))
	return nil
}
