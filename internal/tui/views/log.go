package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/javanhut/Ivaldi-vcs/internal/engine"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// logKeyMap defines keybindings specific to the log view
type logKeyMap struct {
	ToggleFormat    key.Binding
	ToggleTimelines key.Binding
	Refresh         key.Binding
}

func defaultLogKeyMap() logKeyMap {
	return logKeyMap{
		ToggleFormat: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "toggle oneline/full"),
		),
		ToggleTimelines: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "toggle all timelines"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// LogModel is the log view model
type LogModel struct {
	ivaldiDir string
	viewport  viewport.Model
	keys      logKeyMap
	theme     style.Theme

	commits         []engine.CommitEntry
	currentTimeline string
	oneline         bool
	allTimelines    bool
	loading         bool
	err             error
	ready           bool
	width           int
	height          int
}

// logLoadedMsg carries loaded log data
type logLoadedMsg struct {
	commits  []engine.CommitEntry
	timeline string
	err      error
}

// NewLogModel creates a new log view
func NewLogModel(ivaldiDir string) *LogModel {
	return &LogModel{
		ivaldiDir: ivaldiDir,
		keys:      defaultLogKeyMap(),
		theme:     style.DefaultTheme(),
		loading:   true,
	}
}

// Init loads the initial log data
func (m *LogModel) Init() tea.Cmd {
	m.loading = true
	return m.loadLog()
}

// Update handles messages
func (m *LogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height)
			m.viewport.HighPerformanceRendering = false
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height
		}
		m.renderContent()
		return m, nil

	case logLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.commits = msg.commits
		m.currentTimeline = msg.timeline
		m.err = nil
		m.renderContent()
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			return m, m.loadLog()

		case key.Matches(msg, m.keys.ToggleFormat):
			m.oneline = !m.oneline
			m.renderContent()
			return m, nil

		case key.Matches(msg, m.keys.ToggleTimelines):
			m.allTimelines = !m.allTimelines
			m.loading = true
			return m, m.loadLog()
		}

		// Forward to viewport for scrolling
		if m.ready {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// View renders the log view
func (m *LogModel) View() string {
	if m.loading {
		return m.theme.Dim.Render("  Loading commit history...")
	}

	if m.err != nil {
		return m.theme.Error.Render(fmt.Sprintf("  Error: %v", m.err))
	}

	if !m.ready {
		return m.theme.Dim.Render("  Initializing...")
	}

	var b strings.Builder

	// Header
	b.WriteString("  ")
	if m.allTimelines {
		b.WriteString(m.theme.Title.Render("Commit History (all timelines)"))
	} else {
		b.WriteString(m.theme.Title.Render(fmt.Sprintf("Commit History — %s", m.currentTimeline)))
	}

	formatLabel := "full"
	if m.oneline {
		formatLabel = "oneline"
	}
	b.WriteString(m.theme.Dim.Render(fmt.Sprintf("  [%s]", formatLabel)))
	b.WriteString("\n")

	// Viewport content
	b.WriteString(m.viewport.View())

	// Scroll indicator
	scrollPct := m.viewport.ScrollPercent()
	b.WriteString("\n")
	b.WriteString(m.theme.Dim.Render(fmt.Sprintf("  %d commits  %.0f%%", len(m.commits), scrollPct*100)))

	return b.String()
}

// ShortHelp returns a short help string
func (m *LogModel) ShortHelp() string {
	return "j/k:scroll  o:oneline/full  t:all timelines  r:refresh"
}

// HasActiveInput returns whether the log view has an active input dialog
func (m *LogModel) HasActiveInput() bool {
	return false
}

// renderContent rebuilds the viewport content from commits
func (m *LogModel) renderContent() {
	if !m.ready {
		return
	}

	if len(m.commits) == 0 {
		m.viewport.SetContent(m.theme.Dim.Render("  No commits yet."))
		return
	}

	var content string
	if m.oneline {
		content = m.renderOneline()
	} else {
		content = m.renderFull()
	}

	m.viewport.SetContent(content)
}

// renderFull renders commits in full multi-line format
func (m *LogModel) renderFull() string {
	var b strings.Builder

	for i, entry := range m.commits {
		// Commit identifier line
		b.WriteString("  ")
		if entry.SealName != "" {
			b.WriteString(m.theme.Info.Render("seal "))
			b.WriteString(m.theme.Bold.Render(entry.SealName))
		} else {
			b.WriteString(m.theme.Info.Render("commit "))
			b.WriteString(m.theme.Bold.Render(entry.Hash))
		}

		if entry.IsMerge {
			b.WriteString(m.theme.Warning.Render(" (merge)"))
		}
		b.WriteString("\n")

		// Author
		b.WriteString("  ")
		b.WriteString(m.theme.Dim.Render("Author: "))
		b.WriteString(entry.Author)
		b.WriteString("\n")

		// Date
		relTime := engine.RelativeTime(entry.Time)
		b.WriteString("  ")
		b.WriteString(m.theme.Dim.Render("Date:   "))
		b.WriteString(entry.Time.Format("Mon Jan 2 15:04:05 2006"))
		b.WriteString(m.theme.Dim.Render(fmt.Sprintf(" (%s)", relTime)))
		b.WriteString("\n")

		// Timeline (if showing all)
		if m.allTimelines {
			b.WriteString("  ")
			b.WriteString(m.theme.Dim.Render("Timeline: "))
			b.WriteString(m.theme.Info.Render(entry.Timeline))
			b.WriteString("\n")
		}

		// Parents (for merge commits)
		if entry.IsMerge && len(entry.Parents) > 0 {
			b.WriteString("  ")
			b.WriteString(m.theme.Dim.Render("Parents: "))
			b.WriteString(m.theme.Dim.Render(strings.Join(entry.Parents, ", ")))
			b.WriteString("\n")
		}

		// Message
		b.WriteString("\n")
		// Indent each line of the message
		msgLines := strings.Split(entry.Message, "\n")
		for _, line := range msgLines {
			b.WriteString("      ")
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Separator between commits
		if i < len(m.commits)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderOneline renders commits in compact one-line format
func (m *LogModel) renderOneline() string {
	var b strings.Builder

	for _, entry := range m.commits {
		b.WriteString("  ")

		// Identifier (seal name or short hash)
		if entry.SealName != "" {
			name := entry.SealName
			if len(name) > 20 {
				name = name[:20]
			}
			b.WriteString(m.theme.Info.Render(padRight(name, 22)))
		} else {
			b.WriteString(m.theme.Info.Render(padRight(entry.Hash, 22)))
		}

		// Timeline indicator (if showing all)
		if m.allTimelines {
			label := entry.Timeline
			if len(label) > 12 {
				label = label[:12]
			}
			b.WriteString(m.theme.Dim.Render(padRight("["+label+"]", 15)))
		}

		// Relative time
		relTime := engine.RelativeTime(entry.Time)
		b.WriteString(m.theme.Dim.Render(padRight(relTime, 16)))

		// Message (first line, truncated)
		message := firstLine(entry.Message)
		maxMsgLen := m.width - 60
		if m.allTimelines {
			maxMsgLen -= 15
		}
		if maxMsgLen < 20 {
			maxMsgLen = 20
		}
		if len(message) > maxMsgLen {
			message = message[:maxMsgLen-3] + "..."
		}
		b.WriteString(message)

		// Merge indicator
		if entry.IsMerge {
			b.WriteString(m.theme.Warning.Render(" *"))
		}

		b.WriteString("\n")
	}

	return b.String()
}

// loadLog loads commit history asynchronously
func (m *LogModel) loadLog() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	allTimelines := m.allTimelines
	return func() tea.Msg {
		opts := engine.LogOptions{
			AllTimelines: allTimelines,
		}
		commits, timeline, err := engine.GetCommitHistory(ivaldiDir, opts)
		return logLoadedMsg{commits: commits, timeline: timeline, err: err}
	}
}

// firstLine returns the first line of a string
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
