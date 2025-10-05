package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config represents Ivaldi configuration
type Config struct {
	User  UserConfig  `json:"user"`
	Core  CoreConfig  `json:"core"`
	Color ColorConfig `json:"color"`
}

// UserConfig holds user identity information
type UserConfig struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CoreConfig holds core Ivaldi settings
type CoreConfig struct {
	Editor    string `json:"editor,omitempty"`
	Pager     string `json:"pager,omitempty"`
	AutoShelf bool   `json:"auto_shelf"`
}

// ColorConfig holds color settings
type ColorConfig struct {
	UI     bool `json:"ui"`
	Status bool `json:"status"`
	Diff   bool `json:"diff"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		User: UserConfig{
			Name:  "",
			Email: "",
		},
		Core: CoreConfig{
			Editor:    os.Getenv("EDITOR"),
			Pager:     os.Getenv("PAGER"),
			AutoShelf: true,
		},
		Color: ColorConfig{
			UI:     true,
			Status: true,
			Diff:   true,
		},
	}
}

// globalConfigPath returns the path to the global config file
func globalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".ivaldiconfig"), nil
}

// repoConfigPath returns the path to the repository config file
func repoConfigPath() string {
	return filepath.Join(".ivaldi", "config")
}

// LoadConfig loads configuration from both global and repository config files
// Repository config takes precedence over global config
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	// Load global config if it exists
	globalPath, err := globalConfigPath()
	if err == nil {
		if data, err := os.ReadFile(globalPath); err == nil {
			var globalCfg Config
			if err := json.Unmarshal(data, &globalCfg); err == nil {
				// Merge global config
				mergeConfig(cfg, &globalCfg)
			}
		}
	}

	// Load repository config if it exists
	repoPath := repoConfigPath()
	if data, err := os.ReadFile(repoPath); err == nil {
		var repoCfg Config
		if err := json.Unmarshal(data, &repoCfg); err == nil {
			// Merge repo config (overrides global)
			mergeConfig(cfg, &repoCfg)
		}
	}

	return cfg, nil
}

// SaveGlobalConfig saves configuration to the global config file
func SaveGlobalConfig(cfg *Config) error {
	globalPath, err := globalConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(globalPath, data, 0644)
}

// SaveRepoConfig saves configuration to the repository config file
func SaveRepoConfig(cfg *Config) error {
	repoPath := repoConfigPath()

	// Ensure .ivaldi directory exists
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return fmt.Errorf("failed to create .ivaldi directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(repoPath, data, 0644)
}

// GetValue retrieves a configuration value by key (e.g., "user.name")
func GetValue(key string) (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}

	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid config key: %s (expected format: section.key)", key)
	}

	section := parts[0]
	field := parts[1]

	switch section {
	case "user":
		switch field {
		case "name":
			return cfg.User.Name, nil
		case "email":
			return cfg.User.Email, nil
		default:
			return "", fmt.Errorf("unknown user config field: %s", field)
		}
	case "core":
		switch field {
		case "editor":
			return cfg.Core.Editor, nil
		case "pager":
			return cfg.Core.Pager, nil
		case "autoshelf":
			return fmt.Sprintf("%t", cfg.Core.AutoShelf), nil
		default:
			return "", fmt.Errorf("unknown core config field: %s", field)
		}
	case "color":
		switch field {
		case "ui":
			return fmt.Sprintf("%t", cfg.Color.UI), nil
		case "status":
			return fmt.Sprintf("%t", cfg.Color.Status), nil
		case "diff":
			return fmt.Sprintf("%t", cfg.Color.Diff), nil
		default:
			return "", fmt.Errorf("unknown color config field: %s", field)
		}
	default:
		return "", fmt.Errorf("unknown config section: %s", section)
	}
}

// SetValue sets a configuration value by key (e.g., "user.name", "Your Name")
func SetValue(key, value string, global bool) error {
	// Load existing config
	var cfg *Config
	var err error

	if global {
		// For global, load from global config or use default
		globalPath, _ := globalConfigPath()
		if data, err := os.ReadFile(globalPath); err == nil {
			cfg = &Config{}
			if err := json.Unmarshal(data, cfg); err != nil {
				cfg = DefaultConfig()
			}
		} else {
			cfg = DefaultConfig()
		}
	} else {
		// For repo, load from repo config or use default
		repoPath := repoConfigPath()
		if data, err := os.ReadFile(repoPath); err == nil {
			cfg = &Config{}
			if err := json.Unmarshal(data, cfg); err != nil {
				cfg = DefaultConfig()
			}
		} else {
			cfg = DefaultConfig()
		}
	}

	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid config key: %s (expected format: section.key)", key)
	}

	section := parts[0]
	field := parts[1]

	// Set the value
	switch section {
	case "user":
		switch field {
		case "name":
			cfg.User.Name = value
		case "email":
			cfg.User.Email = value
		default:
			return fmt.Errorf("unknown user config field: %s", field)
		}
	case "core":
		switch field {
		case "editor":
			cfg.Core.Editor = value
		case "pager":
			cfg.Core.Pager = value
		case "autoshelf":
			cfg.Core.AutoShelf = value == "true"
		default:
			return fmt.Errorf("unknown core config field: %s", field)
		}
	case "color":
		switch field {
		case "ui":
			cfg.Color.UI = value == "true"
		case "status":
			cfg.Color.Status = value == "true"
		case "diff":
			cfg.Color.Diff = value == "true"
		default:
			return fmt.Errorf("unknown color config field: %s", field)
		}
	default:
		return fmt.Errorf("unknown config section: %s", section)
	}

	// Save config
	if global {
		err = SaveGlobalConfig(cfg)
	} else {
		err = SaveRepoConfig(cfg)
	}

	return err
}

// GetAuthor returns the formatted author string "Name <email>"
func GetAuthor() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}

	if cfg.User.Name == "" || cfg.User.Email == "" {
		return "", fmt.Errorf("user.name and user.email not configured. Run: ivaldi config user.name \"Your Name\" && ivaldi config user.email \"you@example.com\"")
	}

	return fmt.Sprintf("%s <%s>", cfg.User.Name, cfg.User.Email), nil
}

// mergeConfig merges source config into destination config
// Only non-empty values from source override destination
func mergeConfig(dst, src *Config) {
	// Merge user config
	if src.User.Name != "" {
		dst.User.Name = src.User.Name
	}
	if src.User.Email != "" {
		dst.User.Email = src.User.Email
	}

	// Merge core config
	if src.Core.Editor != "" {
		dst.Core.Editor = src.Core.Editor
	}
	if src.Core.Pager != "" {
		dst.Core.Pager = src.Core.Pager
	}
	// AutoShelf is always merged (bool values)
	dst.Core.AutoShelf = src.Core.AutoShelf

	// Merge color config (bool values always merged)
	dst.Color.UI = src.Color.UI
	dst.Color.Status = src.Color.Status
	dst.Color.Diff = src.Color.Diff
}
