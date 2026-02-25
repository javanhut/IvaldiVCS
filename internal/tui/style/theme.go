package style

import "github.com/charmbracelet/lipgloss"

// Theme holds all the styles used in the TUI
type Theme struct {
	// Tab bar
	ActiveTab   lipgloss.Style
	InactiveTab lipgloss.Style
	TabBar      lipgloss.Style

	// Status bar
	StatusBar     lipgloss.Style
	StatusKey     lipgloss.Style
	StatusValue   lipgloss.Style
	StatusDivider lipgloss.Style

	// File statuses
	Staged    lipgloss.Style
	Added     lipgloss.Style
	Modified  lipgloss.Style
	Deleted   lipgloss.Style
	Untracked lipgloss.Style
	Ignored   lipgloss.Style

	// General
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Selected    lipgloss.Style
	Cursor      lipgloss.Style
	Dim         lipgloss.Style
	Bold        lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	Warning     lipgloss.Style
	Info        lipgloss.Style
	Help        lipgloss.Style
	HelpKey     lipgloss.Style
	HelpDesc    lipgloss.Style
	SectionHead lipgloss.Style

	// Viewport
	ViewportBorder lipgloss.Style

	// Timeline header (status view)
	TimelineHeader lipgloss.Style
	TimelineName   lipgloss.Style
	SealBadge      lipgloss.Style
	FileCountBadge lipgloss.Style

	// Separators
	SectionDivider lipgloss.Style
	TabSeparator   lipgloss.Style

	// Brand
	Brand lipgloss.Style
}

// DefaultTheme returns the default TUI theme
func DefaultTheme() Theme {
	return Theme{
		// Tab bar
		ActiveTab: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 2),
		InactiveTab: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(0, 2),
		TabBar: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("#444444")),

		// Status bar
		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Background(lipgloss.Color("#333333")).
			Padding(0, 1),
		StatusKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true),
		StatusValue: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD")),
		StatusDivider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")),

		// File statuses
		Staged: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")),
		Added: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true),
		Modified: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5599FF")),
		Deleted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")),
		Untracked: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF55")),
		Ignored: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")),

		// General
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")),
		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")),
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color("#444444")).
			Foreground(lipgloss.Color("#FFFFFF")),
		Cursor: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true),
		Dim: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")),
		Bold: lipgloss.NewStyle().
			Bold(true),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF55")),
		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#55FFFF")),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")),
		HelpKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true),
		HelpDesc: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")),
		SectionHead: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			MarginTop(1),

		// Viewport
		ViewportBorder: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")),

		// Timeline header (status view)
		TimelineHeader: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1),
		TimelineName: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")),
		SealBadge: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#55FFFF")),
		FileCountBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")),
		SectionDivider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444444")),
		TabSeparator: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444444")),
		Brand: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1),
	}
}
