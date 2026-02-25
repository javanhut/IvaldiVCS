package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/javanhut/Ivaldi-vcs/internal/engine"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/components"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// diffKeyMap defines keybindings specific to the diff view
type diffKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	NextFile key.Binding
	PrevFile key.Binding
	Toggle   key.Binding
	Refresh  key.Binding
}

func defaultDiffKeyMap() diffKeyMap {
	return diffKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/up", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		NextFile: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next file"),
		),
		PrevFile: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prev file"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle staged"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// DiffModel is the diff view model
type DiffModel struct {
	workDir   string
	ivaldiDir string
	diffView  components.DiffView
	keys      diffKeyMap
	theme     style.Theme

	result  *engine.DiffResult
	staged  bool
	loading bool
	err     error
	width   int
	height  int
}

// diffLoadedMsg carries loaded diff data
type diffLoadedMsg struct {
	result *engine.DiffResult
	err    error
}

// NewDiffModel creates a new diff view
func NewDiffModel(workDir, ivaldiDir string) *DiffModel {
	return &DiffModel{
		workDir:   workDir,
		ivaldiDir: ivaldiDir,
		diffView:  components.NewDiffView(),
		keys:      defaultDiffKeyMap(),
		theme:     style.DefaultTheme(),
		loading:   true,
	}
}

// Init loads the initial diff data
func (m *DiffModel) Init() tea.Cmd {
	m.loading = true
	return m.loadDiff()
}

// Update handles messages
func (m *DiffModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reserve space for header + footer
		viewHeight := msg.Height - 4
		if viewHeight < 1 {
			viewHeight = 1
		}
		m.diffView.SetSize(msg.Width, viewHeight)
		return m, nil

	case diffLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.result = msg.result
		m.err = nil
		m.diffView.SetContent(msg.result.Files, m.theme)
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			return m, m.loadDiff()

		case key.Matches(msg, m.keys.Toggle):
			m.staged = !m.staged
			m.loading = true
			return m, m.loadDiff()

		case key.Matches(msg, m.keys.Up):
			m.diffView.ScrollUp()
			return m, nil

		case key.Matches(msg, m.keys.Down):
			m.diffView.ScrollDown()
			return m, nil

		case key.Matches(msg, m.keys.PageUp):
			m.diffView.PageUp()
			return m, nil

		case key.Matches(msg, m.keys.PageDown):
			m.diffView.PageDown()
			return m, nil

		case key.Matches(msg, m.keys.Top):
			m.diffView.ScrollToTop()
			return m, nil

		case key.Matches(msg, m.keys.Bottom):
			m.diffView.ScrollToBottom()
			return m, nil

		case key.Matches(msg, m.keys.NextFile):
			m.diffView.NextFile()
			return m, nil

		case key.Matches(msg, m.keys.PrevFile):
			m.diffView.PrevFile()
			return m, nil
		}
	}

	return m, nil
}

// View renders the diff view
func (m *DiffModel) View() string {
	if m.loading {
		return m.theme.Dim.Render("  Computing diff...")
	}

	if m.err != nil {
		return m.theme.Error.Render(fmt.Sprintf("  Error: %v", m.err))
	}

	if m.result == nil {
		return m.theme.Dim.Render("  No diff data")
	}

	var b strings.Builder

	// Header
	b.WriteString("  ")
	modeLabel := "working directory vs HEAD"
	if m.staged {
		modeLabel = "staged vs HEAD"
	}
	b.WriteString(m.theme.Title.Render(fmt.Sprintf("Diff — %s", modeLabel)))

	if len(m.result.Files) > 0 {
		var statParts []string
		if m.result.Stats.Added > 0 {
			statParts = append(statParts, m.theme.Added.Render(fmt.Sprintf("+%d", m.result.Stats.Added)))
		}
		if m.result.Stats.Modified > 0 {
			statParts = append(statParts, m.theme.Modified.Render(fmt.Sprintf("~%d", m.result.Stats.Modified)))
		}
		if m.result.Stats.Removed > 0 {
			statParts = append(statParts, m.theme.Deleted.Render(fmt.Sprintf("-%d", m.result.Stats.Removed)))
		}
		b.WriteString("  ")
		b.WriteString(strings.Join(statParts, " "))
	}
	b.WriteString("\n")

	// Diff content
	if len(m.result.Files) == 0 {
		b.WriteString(m.theme.Success.Render("  No differences"))
	} else {
		b.WriteString(m.diffView.View(m.theme))
	}

	// Footer with scroll info
	total := m.diffView.TotalLines()
	if total > 0 {
		b.WriteString("\n")
		offset := m.diffView.Offset()
		pct := 0
		if total > 0 {
			pct = (offset * 100) / total
		}
		b.WriteString(m.theme.Dim.Render(
			fmt.Sprintf("  %d files  %d lines  %d%%",
				len(m.result.Files), total, pct)))
	}

	return b.String()
}

// ShortHelp returns a short help string
func (m *DiffModel) ShortHelp() string {
	return "j/k:scroll  n/p:next/prev file  s:staged  g/G:top/bottom  r:refresh"
}

// HasActiveInput returns whether the diff view has an active input dialog
func (m *DiffModel) HasActiveInput() bool {
	return false
}

// loadDiff loads diff data asynchronously
func (m *DiffModel) loadDiff() tea.Cmd {
	workDir := m.workDir
	ivaldiDir := m.ivaldiDir
	staged := m.staged
	return func() tea.Msg {
		opts := engine.DiffOptions{
			Staged: staged,
		}
		result, err := engine.ComputeDiff(ivaldiDir, workDir, opts)
		return diffLoadedMsg{result: result, err: err}
	}
}
