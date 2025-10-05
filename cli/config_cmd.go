package cli

import (
	"fmt"

	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config [key] [value]",
	Short: "Get and set configuration options",
	Long: `Get and set Ivaldi configuration options.

Configuration can be set at two levels:
- Global (~/.ivaldiconfig) - applies to all repositories
- Repository (.ivaldi/config) - applies to current repository only

Examples:
  ivaldi config user.name "Your Name"
  ivaldi config user.email "you@example.com"
  ivaldi config --global user.name "Your Name"
  ivaldi config --list
  ivaldi config user.name`,
	RunE: runConfig,
}

var (
	configGlobal bool
	configList   bool
)

func init() {
	configCmd.Flags().BoolVar(&configGlobal, "global", false, "Use global config file")
	configCmd.Flags().BoolVar(&configList, "list", false, "List all configuration")
}

func runConfig(cmd *cobra.Command, args []string) error {
	// Handle --list flag
	if configList {
		return listConfig()
	}

	// Handle get value (1 arg)
	if len(args) == 1 {
		return getConfigValue(args[0])
	}

	// Handle set value (2 args)
	if len(args) == 2 {
		return setConfigValue(args[0], args[1], configGlobal)
	}

	// Invalid usage
	return fmt.Errorf("invalid usage. See: ivaldi config --help")
}

func listConfig() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println(colors.SectionHeader("User Configuration:"))
	if cfg.User.Name != "" {
		fmt.Printf("  user.name = %s\n", colors.InfoText(cfg.User.Name))
	} else {
		fmt.Printf("  user.name = %s\n", colors.Gray("(not set)"))
	}
	if cfg.User.Email != "" {
		fmt.Printf("  user.email = %s\n", colors.InfoText(cfg.User.Email))
	} else {
		fmt.Printf("  user.email = %s\n", colors.Gray("(not set)"))
	}

	fmt.Println()
	fmt.Println(colors.SectionHeader("Core Configuration:"))
	if cfg.Core.Editor != "" {
		fmt.Printf("  core.editor = %s\n", colors.InfoText(cfg.Core.Editor))
	} else {
		fmt.Printf("  core.editor = %s\n", colors.Gray("(not set)"))
	}
	if cfg.Core.Pager != "" {
		fmt.Printf("  core.pager = %s\n", colors.InfoText(cfg.Core.Pager))
	} else {
		fmt.Printf("  core.pager = %s\n", colors.Gray("(not set)"))
	}
	fmt.Printf("  core.autoshelf = %s\n", colors.InfoText(fmt.Sprintf("%t", cfg.Core.AutoShelf)))

	fmt.Println()
	fmt.Println(colors.SectionHeader("Color Configuration:"))
	fmt.Printf("  color.ui = %s\n", colors.InfoText(fmt.Sprintf("%t", cfg.Color.UI)))
	fmt.Printf("  color.status = %s\n", colors.InfoText(fmt.Sprintf("%t", cfg.Color.Status)))
	fmt.Printf("  color.diff = %s\n", colors.InfoText(fmt.Sprintf("%t", cfg.Color.Diff)))

	return nil
}

func getConfigValue(key string) error {
	value, err := config.GetValue(key)
	if err != nil {
		return err
	}

	if value == "" {
		fmt.Printf("%s is %s\n", key, colors.Gray("(not set)"))
	} else {
		fmt.Println(value)
	}

	return nil
}

func setConfigValue(key, value string, global bool) error {
	err := config.SetValue(key, value, global)
	if err != nil {
		return err
	}

	scope := "repository"
	if global {
		scope = "global"
	}

	fmt.Printf("%s %s config: %s = %s\n",
		colors.SuccessText("Set"),
		scope,
		colors.Bold(key),
		colors.InfoText(value))

	// Show hint if setting user config for the first time
	if key == "user.name" || key == "user.email" {
		cfg, _ := config.LoadConfig()
		if cfg.User.Name == "" || cfg.User.Email == "" {
			fmt.Println()
			fmt.Println(colors.Dim("Hint: Make sure to also set:"))
			if cfg.User.Name == "" {
				fmt.Printf("  %s\n", colors.InfoText("ivaldi config user.name \"Your Name\""))
			}
			if cfg.User.Email == "" {
				fmt.Printf("  %s\n", colors.InfoText("ivaldi config user.email \"you@example.com\""))
			}
		}
	}

	return nil
}
