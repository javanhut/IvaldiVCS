// Package colors provides terminal color support for Ivaldi VCS output.
//
// This package provides:
// - ANSI color codes for terminal output
// - Functions to colorize text based on file status
// - Automatic color detection and fallback for non-color terminals
// - Consistent color scheme across all Ivaldi commands
package colors

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// ANSI color codes
const (
	ColorReset = "\033[0m"
	ColorBold  = "\033[1m"
	ColorDim   = "\033[2m"

	// Regular colors
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
	ColorGray    = "\033[90m"

	// Bright colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
)

// colorEnabled determines if color output should be used
var colorEnabled = shouldUseColor()

// shouldUseColor determines if the terminal supports colors
func shouldUseColor() bool {
	// Check if NO_COLOR environment variable is set
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Force color if FORCE_COLOR is set
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}

	// On Windows, check if we're in a modern terminal
	if runtime.GOOS == "windows" {
		// Check for Windows Terminal, VS Code terminal, etc.
		term := strings.ToLower(os.Getenv("TERM"))
		wt := os.Getenv("WT_SESSION")
		vscode := os.Getenv("VSCODE_PID")

		if wt != "" || vscode != "" || strings.Contains(term, "color") || strings.Contains(term, "xterm") {
			return true
		}
		return false
	}

	// On Unix-like systems, check TERM environment variable
	term := strings.ToLower(os.Getenv("TERM"))
	if term == "dumb" || term == "" {
		return false
	}

	// Check if stdout is a terminal
	if fileInfo, err := os.Stdout.Stat(); err == nil {
		return (fileInfo.Mode() & os.ModeCharDevice) != 0
	}

	return true
}

// SetColorEnabled allows manual control of color output
func SetColorEnabled(enabled bool) {
	colorEnabled = enabled
}

// IsColorEnabled returns whether colors are currently enabled
func IsColorEnabled() bool {
	return colorEnabled
}

// colorize applies color to text if colors are enabled
func colorize(text, color string) string {
	if !colorEnabled {
		return text
	}
	return color + text + ColorReset
}

// Status-based coloring functions
func Added(text string) string {
	return colorize(text, BrightGreen)
}

func Modified(text string) string {
	return colorize(text, BrightBlue)
}

func Deleted(text string) string {
	return colorize(text, BrightRed)
}

func Untracked(text string) string {
	return colorize(text, BrightYellow)
}

func Ignored(text string) string {
	return colorize(text, ColorGray)
}

func Staged(text string) string {
	return colorize(text, ColorGreen)
}

// Generic color functions
func Red(text string) string {
	return colorize(text, BrightRed)
}

func Green(text string) string {
	return colorize(text, BrightGreen)
}

func Blue(text string) string {
	return colorize(text, BrightBlue)
}

func Yellow(text string) string {
	return colorize(text, BrightYellow)
}

func Cyan(text string) string {
	return colorize(text, BrightCyan)
}

func Magenta(text string) string {
	return colorize(text, BrightMagenta)
}

func White(text string) string {
	return colorize(text, BrightWhite)
}

func Gray(text string) string {
	return colorize(text, ColorGray)
}

func Bold(text string) string {
	if !colorEnabled {
		return text
	}
	return ColorBold + text + ColorReset
}

func Dim(text string) string {
	if !colorEnabled {
		return text
	}
	return ColorDim + text + ColorReset
}

// Status prefixes with colors
func AddedPrefix() string {
	return Added("A")
}

func ModifiedPrefix() string {
	return Modified("M")
}

func DeletedPrefix() string {
	return Deleted("D")
}

func UntrackedPrefix() string {
	return Untracked("?")
}

func IgnoredPrefix() string {
	return Ignored("!")
}

func StagedPrefix() string {
	return Staged("S")
}

// Colorize file status text with appropriate prefix
func ColorizeFileStatus(status, filePath string) string {
	switch strings.ToLower(status) {
	case "added", "new file":
		return fmt.Sprintf("  %s  %s", AddedPrefix(), Green(filePath))
	case "modified":
		return fmt.Sprintf("  %s  %s", ModifiedPrefix(), Blue(filePath))
	case "deleted":
		return fmt.Sprintf("  %s  %s", DeletedPrefix(), Red(filePath))
	case "untracked":
		return fmt.Sprintf("  %s  %s", UntrackedPrefix(), Yellow(filePath))
	case "ignored":
		return fmt.Sprintf("  %s  %s", IgnoredPrefix(), Gray(filePath))
	case "staged":
		return fmt.Sprintf("  %s  %s", StagedPrefix(), Green(filePath))
	default:
		return fmt.Sprintf("     %s", filePath)
	}
}

// Section headers with colors
func SectionHeader(text string) string {
	return Bold(text)
}

func ErrorText(text string) string {
	return Red(text)
}

func SuccessText(text string) string {
	return Green(text)
}

func InfoText(text string) string {
	return Cyan(text)
}

func WarningText(text string) string {
	return Yellow(text)
}
