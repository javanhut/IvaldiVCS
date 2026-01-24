package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// User config should be empty by default
	if cfg.User.Name != "" {
		t.Errorf("Expected empty user name, got %q", cfg.User.Name)
	}
	if cfg.User.Email != "" {
		t.Errorf("Expected empty user email, got %q", cfg.User.Email)
	}

	// Core config should have sensible defaults
	if cfg.Core.AutoShelf != true {
		t.Error("Expected AutoShelf to be true by default")
	}

	// Color config should be enabled by default
	if cfg.Color.UI != true {
		t.Error("Expected Color.UI to be true by default")
	}
	if cfg.Color.Status != true {
		t.Error("Expected Color.Status to be true by default")
	}
	if cfg.Color.Diff != true {
		t.Error("Expected Color.Diff to be true by default")
	}
}

func TestGetSetValue(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	ivaldiDir := filepath.Join(tempDir, ".ivaldi")
	if err := os.MkdirAll(ivaldiDir, 0755); err != nil {
		t.Fatalf("Failed to create .ivaldi directory: %v", err)
	}

	// Change to temp directory to use repo config
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Test setting and getting user.name
	testName := "Test User"
	if err := SetValue("user.name", testName, false); err != nil {
		t.Fatalf("Failed to set user.name: %v", err)
	}

	gotName, err := GetValue("user.name")
	if err != nil {
		t.Fatalf("Failed to get user.name: %v", err)
	}
	if gotName != testName {
		t.Errorf("user.name: expected %q, got %q", testName, gotName)
	}

	// Test setting and getting user.email
	testEmail := "test@example.com"
	if err := SetValue("user.email", testEmail, false); err != nil {
		t.Fatalf("Failed to set user.email: %v", err)
	}

	gotEmail, err := GetValue("user.email")
	if err != nil {
		t.Fatalf("Failed to get user.email: %v", err)
	}
	if gotEmail != testEmail {
		t.Errorf("user.email: expected %q, got %q", testEmail, gotEmail)
	}

	// Test setting and getting core.editor
	testEditor := "vim"
	if err := SetValue("core.editor", testEditor, false); err != nil {
		t.Fatalf("Failed to set core.editor: %v", err)
	}

	gotEditor, err := GetValue("core.editor")
	if err != nil {
		t.Fatalf("Failed to get core.editor: %v", err)
	}
	if gotEditor != testEditor {
		t.Errorf("core.editor: expected %q, got %q", testEditor, gotEditor)
	}

	// Test boolean value (core.autoshelf)
	if err := SetValue("core.autoshelf", "false", false); err != nil {
		t.Fatalf("Failed to set core.autoshelf: %v", err)
	}

	gotAutoShelf, err := GetValue("core.autoshelf")
	if err != nil {
		t.Fatalf("Failed to get core.autoshelf: %v", err)
	}
	if gotAutoShelf != "false" {
		t.Errorf("core.autoshelf: expected %q, got %q", "false", gotAutoShelf)
	}
}

func TestGetValueInvalidKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{
			name:    "single part key",
			key:     "username",
			wantErr: "invalid config key",
		},
		{
			name:    "three part key",
			key:     "user.name.first",
			wantErr: "invalid config key",
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: "invalid config key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetValue(tt.key)
			if err == nil {
				t.Error("Expected error for invalid key, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Error should contain %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestSetValueInvalidSection(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	ivaldiDir := filepath.Join(tempDir, ".ivaldi")
	if err := os.MkdirAll(ivaldiDir, 0755); err != nil {
		t.Fatalf("Failed to create .ivaldi directory: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{
			name:    "unknown section",
			key:     "invalid.setting",
			wantErr: "unknown config section",
		},
		{
			name:    "unknown user field",
			key:     "user.invalid",
			wantErr: "unknown user config field",
		},
		{
			name:    "unknown core field",
			key:     "core.invalid",
			wantErr: "unknown core config field",
		},
		{
			name:    "unknown color field",
			key:     "color.invalid",
			wantErr: "unknown color config field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetValue(tt.key, "value", false)
			if err == nil {
				t.Error("Expected error for invalid section/field, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Error should contain %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestGetAuthor(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	ivaldiDir := filepath.Join(tempDir, ".ivaldi")
	if err := os.MkdirAll(ivaldiDir, 0755); err != nil {
		t.Fatalf("Failed to create .ivaldi directory: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Also isolate from global config by changing HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Test that GetAuthor fails when user.name and user.email are not set
	_, err = GetAuthor()
	if err == nil {
		t.Error("Expected error when user.name and user.email not configured")
	}

	// Set user.name and user.email
	if err := SetValue("user.name", "Test User", false); err != nil {
		t.Fatalf("Failed to set user.name: %v", err)
	}
	if err := SetValue("user.email", "test@example.com", false); err != nil {
		t.Fatalf("Failed to set user.email: %v", err)
	}

	// Now GetAuthor should succeed
	author, err := GetAuthor()
	if err != nil {
		t.Fatalf("GetAuthor failed: %v", err)
	}

	expectedAuthor := "Test User <test@example.com>"
	if author != expectedAuthor {
		t.Errorf("GetAuthor: expected %q, got %q", expectedAuthor, author)
	}
}

// TestSetConfigValueErrorContext verifies Phase 11 error wrapping improvements.
// The error should contain context about what failed.
func TestSetConfigValueErrorContext(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	ivaldiDir := filepath.Join(tempDir, ".ivaldi")
	if err := os.MkdirAll(ivaldiDir, 0755); err != nil {
		t.Fatalf("Failed to create .ivaldi directory: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Make the .ivaldi directory read-only to cause a write error
	configPath := filepath.Join(ivaldiDir, "config")
	// Create the config file first
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Make it read-only
	if err := os.Chmod(configPath, 0444); err != nil {
		t.Fatalf("Failed to make config read-only: %v", err)
	}
	defer os.Chmod(configPath, 0644) // Restore for cleanup

	// Try to set a value - should fail with contextual error
	err = SetValue("user.name", "Test", false)
	if err == nil {
		t.Skip("Could not trigger write error - skipping context check")
	}

	// The error should contain context from Phase 11's %w wrapping
	errStr := err.Error()
	if !strings.Contains(errStr, "failed to save config") {
		t.Errorf("Error should contain 'failed to save config' context, got: %s", errStr)
	}
}

// TestErrorUnwrapping verifies that Phase 11's %w fixes enable proper error chain inspection.
func TestErrorUnwrapping(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	ivaldiDir := filepath.Join(tempDir, ".ivaldi")
	if err := os.MkdirAll(ivaldiDir, 0755); err != nil {
		t.Fatalf("Failed to create .ivaldi directory: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create the config file first
	configPath := filepath.Join(ivaldiDir, "config")
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Make it read-only to cause a write error
	if err := os.Chmod(configPath, 0444); err != nil {
		t.Fatalf("Failed to make config read-only: %v", err)
	}
	defer os.Chmod(configPath, 0644)

	// Try to set a value
	err = SetValue("user.name", "Test", false)
	if err == nil {
		t.Skip("Could not trigger write error - skipping unwrap test")
	}

	// Verify the error chain can be inspected using errors.Unwrap
	// The wrapped error should be accessible via the error chain
	unwrapped := errors.Unwrap(err)
	if unwrapped == nil {
		t.Error("Error should be unwrappable (wrapped with %w)")
	}

	// Verify errors.Is works with the wrapped error chain
	// We can't directly check for os.ErrPermission because the underlying
	// error might be different on different systems, but we can verify
	// that the chain is navigable
	if unwrapped != nil {
		// The unwrapped error should contain filesystem-related info
		unwrappedStr := unwrapped.Error()
		// This just verifies unwrapping works - the specific message varies by OS
		if len(unwrappedStr) == 0 {
			t.Error("Unwrapped error should have content")
		}
	}
}

func TestLoadConfigMerging(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	ivaldiDir := filepath.Join(tempDir, ".ivaldi")
	if err := os.MkdirAll(ivaldiDir, 0755); err != nil {
		t.Fatalf("Failed to create .ivaldi directory: %v", err)
	}

	// Create repo config with specific values
	repoConfig := `{
		"user": {"name": "Repo User", "email": ""},
		"core": {"editor": "nano"},
		"color": {"ui": false}
	}`
	if err := os.WriteFile(filepath.Join(ivaldiDir, "config"), []byte(repoConfig), 0644); err != nil {
		t.Fatalf("Failed to create repo config: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Load config and verify merging
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Repo config should override defaults where specified
	if cfg.User.Name != "Repo User" {
		t.Errorf("Expected user.name from repo config, got %q", cfg.User.Name)
	}
	if cfg.Core.Editor != "nano" {
		t.Errorf("Expected core.editor from repo config, got %q", cfg.Core.Editor)
	}
	// Color.UI should be false from repo config
	if cfg.Color.UI != false {
		t.Error("Expected color.ui to be false from repo config")
	}
}
