package components

import (
	"fmt"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// StatusBar renders the bottom info bar
type StatusBar struct {
	Timeline  string
	SealName  string
	Staged    int
	Modified  int
	Untracked int
	Deleted   int
	Width     int
}

// NewStatusBar creates a new status bar
func NewStatusBar() StatusBar {
	return StatusBar{}
}

// View renders the status bar
func (s StatusBar) View(theme style.Theme) string {
	var parts []string

	if s.Timeline != "" {
		parts = append(parts,
			theme.StatusKey.Render("Timeline: ")+theme.StatusValue.Render(s.Timeline))
	}

	if s.SealName != "" {
		parts = append(parts,
			theme.StatusKey.Render("Seal: ")+theme.StatusValue.Render(s.SealName))
	}

	// File counts
	var counts []string
	if s.Staged > 0 {
		counts = append(counts, fmt.Sprintf("%d staged", s.Staged))
	}
	if s.Modified > 0 {
		counts = append(counts, fmt.Sprintf("%d modified", s.Modified))
	}
	if s.Untracked > 0 {
		counts = append(counts, fmt.Sprintf("%d untracked", s.Untracked))
	}
	if s.Deleted > 0 {
		counts = append(counts, fmt.Sprintf("%d deleted", s.Deleted))
	}
	if len(counts) > 0 {
		parts = append(parts, theme.StatusValue.Render(strings.Join(counts, ", ")))
	}

	divider := theme.StatusDivider.Render(" | ")
	content := strings.Join(parts, divider)

	// Add help hint on the right
	helpHint := theme.Help.Render("esc=back  ?=help")
	contentWidth := lipglossWidth(content)
	helpWidth := lipglossWidth(helpHint)

	padding := s.Width - contentWidth - helpWidth - 2
	if padding < 1 {
		padding = 1
	}

	bar := content + strings.Repeat(" ", padding) + helpHint

	return theme.StatusBar.Width(s.Width).Render(bar)
}

// lipglossWidth estimates the visible width of a styled string
func lipglossWidth(s string) int {
	clean := stripAnsi(s)
	return len([]rune(clean))
}

// stripAnsi removes ANSI escape sequences from a string
func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
