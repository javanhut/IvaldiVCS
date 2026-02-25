package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// HelpModel is the help overlay model
type HelpModel struct{}

// NewHelpModel creates a new help model
func NewHelpModel() HelpModel {
	return HelpModel{}
}

type helpSection struct {
	title string
	keys  []helpEntry
}

type helpEntry struct {
	key  string
	desc string
}

// View renders the help overlay
func (h HelpModel) View(width, height int, theme style.Theme) string {
	sections := []helpSection{
		{
			title: "Global",
			keys: []helpEntry{
				{"esc", "Back / dismiss / quit"},
				{"q / ctrl+c", "Quit"},
				{"?", "Toggle help"},
				{"tab", "Next tab"},
				{"shift+tab", "Previous tab"},
				{"1-6", "Jump to tab"},
			},
		},
		{
			title: "Status View",
			keys: []helpEntry{
				{"j / k / arrows", "Navigate files"},
				{"space", "Toggle file staging"},
				{"a", "Gather all unstaged files"},
				{"u", "Ungather all staged files"},
				{"s", "Seal (commit) staged files"},
				{"i", "Show/hide ignored files"},
				{"r", "Refresh status"},
				{"g", "Jump to top"},
				{"G", "Jump to bottom"},
			},
		},
		{
			title: "Log View",
			keys: []helpEntry{
				{"j / k / arrows", "Scroll up/down"},
				{"o", "Toggle oneline/full format"},
				{"t", "Toggle all timelines"},
				{"r", "Refresh log"},
			},
		},
		{
			title: "Diff View",
			keys: []helpEntry{
				{"j / k / arrows", "Scroll up/down"},
				{"pgup / ctrl+u", "Page up"},
				{"pgdn / ctrl+d", "Page down"},
				{"n / p", "Next/prev file"},
				{"g / G", "Top/bottom"},
				{"s", "Toggle staged diff"},
				{"r", "Refresh diff"},
			},
		},
		{
			title: "Timeline View",
			keys: []helpEntry{
				{"j / k / arrows", "Navigate timelines"},
				{"enter", "Switch to timeline"},
				{"c", "Create new timeline"},
				{"d", "Remove timeline"},
				{"R", "Rename timeline"},
				{"g / G", "Top/bottom"},
				{"r", "Refresh"},
			},
		},
		{
			title: "Remote View",
			keys: []helpEntry{
				{"s", "Scout remote timelines"},
				{"u", "Upload (push) current"},
				{"y", "Sync (pull) current"},
				{"h", "Harvest all new timelines"},
				{"enter", "Harvest selected timeline"},
				{"p", "Set portal (owner/repo)"},
				{"j / k", "Navigate scout results"},
				{"r", "Refresh"},
			},
		},
		{
			title: "Fuse View",
			keys: []helpEntry{
				{"j / k / arrows", "Navigate timelines"},
				{"enter / f", "Fuse selected into current"},
				{"s", "Cycle merge strategy"},
				{"a", "Abort merge in progress"},
				{"g / G", "Top/bottom"},
				{"r", "Refresh"},
			},
		},
	}

	glossary := []helpEntry{
		{"Timeline", "Branch — a line of development"},
		{"Seal", "Commit — a snapshot of changes"},
		{"Gather", "Stage — mark files for next seal"},
		{"Ungather", "Unstage — remove from staging"},
		{"Fuse", "Merge — combine two timelines"},
		{"Portal", "Remote — link to GitHub repository"},
		{"Upload", "Push — send changes to remote"},
		{"Sync", "Pull — fetch changes from remote"},
		{"Harvest", "Fetch — download remote timelines"},
		{"Scout", "List remote timeline information"},
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(theme.Title.Render("  Ivaldi TUI — Keybinding Reference"))
	b.WriteString("\n\n")

	for _, section := range sections {
		b.WriteString("  ")
		b.WriteString(theme.SectionHead.Render(section.title))
		b.WriteString("\n")

		for _, entry := range section.keys {
			b.WriteString("    ")
			b.WriteString(theme.HelpKey.Render(padRight(entry.key, 20)))
			b.WriteString(theme.HelpDesc.Render(entry.desc))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Glossary
	b.WriteString("  ")
	b.WriteString(theme.SectionHead.Render("Glossary — Ivaldi Naming Conventions"))
	b.WriteString("\n")
	for _, entry := range glossary {
		b.WriteString("    ")
		b.WriteString(theme.HelpKey.Render(padRight(entry.key, 12)))
		b.WriteString(theme.HelpDesc.Render(entry.desc))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(theme.Dim.Render("  Press any key to dismiss"))

	// Wrap in bordered box
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	innerWidth := width - 4
	if innerWidth < 40 {
		innerWidth = 40
	}

	return boxStyle.Width(innerWidth).Render(b.String())
}

// padRight pads a string to the given width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
