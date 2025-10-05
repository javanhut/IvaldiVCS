package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
  ivaldi config                            # Interactive mode
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

	// Handle interactive mode (no args and no flags)
	if len(args) == 0 {
		return interactiveConfig()
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

// interactiveConfig runs an interactive configuration session
func interactiveConfig() error {
	// Load existing config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println(colors.SectionHeader("Interactive Configuration"))
	fmt.Println()

	// Get username
	currentName := cfg.User.Name
	if currentName == "" {
		currentName = "not set"
	}
	fmt.Printf("Username (%s)> ", colors.Dim(currentName))
	userName, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read username: %w", err)
	}
	userName = strings.TrimSpace(userName)
	if userName != "" {
		cfg.User.Name = userName
	} else if cfg.User.Name == "" || cfg.User.Name == "not set" {
		// If no default and user pressed enter, keep prompting or use empty
		cfg.User.Name = ""
	}

	// Get email
	currentEmail := cfg.User.Email
	if currentEmail == "" {
		currentEmail = "not set"
	}
	fmt.Printf("Email (%s)> ", colors.Dim(currentEmail))
	userEmail, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read email: %w", err)
	}
	userEmail = strings.TrimSpace(userEmail)
	if userEmail != "" {
		cfg.User.Email = userEmail
	} else if cfg.User.Email == "" || cfg.User.Email == "not set" {
		cfg.User.Email = ""
	}

	// Get scope (global or local)
	fmt.Printf("Scope (global/local) [%s]> ", colors.Dim("global"))
	scopeInput, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read scope: %w", err)
	}
	scopeInput = strings.TrimSpace(strings.ToLower(scopeInput))

	isGlobal := true
	if scopeInput == "local" || scopeInput == "l" {
		isGlobal = false
	} else if scopeInput != "" && scopeInput != "global" && scopeInput != "g" {
		fmt.Printf("%s Invalid scope '%s', using global\n", colors.Yellow("Warning:"), scopeInput)
	}

	// Save config
	var saveErr error
	if isGlobal {
		saveErr = config.SaveGlobalConfig(cfg)
	} else {
		saveErr = config.SaveRepoConfig(cfg)
	}

	if saveErr != nil {
		return fmt.Errorf("failed to save config: %w", saveErr)
	}

	// Show summary
	fmt.Println()
	fmt.Println(colors.SuccessText("Config saved!"))
	fmt.Println()

	scope := "global"
	if !isGlobal {
		scope = "local"
	}

	fmt.Printf("  Scope: %s\n", colors.InfoText(scope))
	if cfg.User.Name != "" {
		fmt.Printf("  Username: %s\n", colors.InfoText(cfg.User.Name))
	}
	if cfg.User.Email != "" {
		fmt.Printf("  Email: %s\n", colors.InfoText(cfg.User.Email))
	}

	return nil
}
