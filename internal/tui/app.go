package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/components"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/views"
)

// Model is the root TUI model
type Model struct {
	workDir   string
	ivaldiDir string

	tabs      components.TabBar
	statusBar components.StatusBar
	keys      KeyMap
	theme     style.Theme

	activeTab style.TabID
	views     map[style.TabID]style.View
	showHelp  bool
	helpView  views.HelpModel

	width  int
	height int

	err error
}

// New creates a new root TUI model
func New(workDir, ivaldiDir string) Model {
	theme := style.DefaultTheme()
	keys := DefaultKeyMap()

	tabs := style.AllTabs()
	tabLabels := make([]string, len(tabs))
	for i, t := range tabs {
		tabLabels[i] = t.Label
	}

	m := Model{
		workDir:   workDir,
		ivaldiDir: ivaldiDir,
		tabs:      components.NewTabBar(tabLabels),
		statusBar: components.NewStatusBar(),
		keys:      keys,
		theme:     theme,
		activeTab: style.TabStatus,
		views:     make(map[style.TabID]style.View),
		helpView:  views.NewHelpModel(),
	}

	// Initialize views
	m.views[style.TabStatus] = views.NewStatusModel(workDir, ivaldiDir)
	m.views[style.TabLog] = views.NewLogModel(ivaldiDir)
	m.views[style.TabDiff] = views.NewDiffModel(workDir, ivaldiDir)
	m.views[style.TabTimelines] = views.NewTimelineModel(workDir, ivaldiDir)
	m.views[style.TabRemote] = views.NewRemoteModel(workDir, ivaldiDir)
	m.views[style.TabFuse] = views.NewFuseModel(workDir, ivaldiDir)

	return m
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	if v, ok := m.views[m.activeTab]; ok {
		return v.Init()
	}
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.statusBar.Width = msg.Width

		contentHeight := msg.Height - 4

		for id, v := range m.views {
			sizeMsg := tea.WindowSizeMsg{
				Width:  msg.Width,
				Height: contentHeight,
			}
			updated, cmd := v.Update(sizeMsg)
			m.views[id] = updated.(style.View)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Help) {
			m.showHelp = !m.showHelp
			return m, nil
		}

		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		allTabs := style.AllTabs()
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Tab):
			m.activeTab = style.TabID((int(m.activeTab) + 1) % len(allTabs))
			m.tabs.SetActive(int(m.activeTab))
			return m, m.initActiveView()

		case key.Matches(msg, m.keys.ShiftTab):
			n := len(allTabs)
			m.activeTab = style.TabID((int(m.activeTab) - 1 + n) % n)
			m.tabs.SetActive(int(m.activeTab))
			return m, m.initActiveView()

		case key.Matches(msg, m.keys.Tab1):
			return m, m.switchTab(style.TabStatus)
		case key.Matches(msg, m.keys.Tab2):
			return m, m.switchTab(style.TabLog)
		case key.Matches(msg, m.keys.Tab3):
			return m, m.switchTab(style.TabDiff)
		case key.Matches(msg, m.keys.Tab4):
			return m, m.switchTab(style.TabTimelines)
		case key.Matches(msg, m.keys.Tab5):
			return m, m.switchTab(style.TabRemote)
		case key.Matches(msg, m.keys.Tab6):
			return m, m.switchTab(style.TabFuse)
		}

	case style.StatusUpdateMsg:
		m.statusBar.Timeline = msg.Timeline
		m.statusBar.SealName = msg.SealName
		m.statusBar.Staged = msg.Staged
		m.statusBar.Modified = msg.Modified
		m.statusBar.Untracked = msg.Untracked
		return m, nil

	case style.ErrMsg:
		m.err = msg.Err
		return m, nil
	}

	// Forward to active view
	if v, ok := m.views[m.activeTab]; ok {
		updated, cmd := v.Update(msg)
		m.views[m.activeTab] = updated.(style.View)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Tab bar
	b.WriteString(m.tabs.View(m.theme))
	b.WriteString("\n")

	// Content area
	contentHeight := m.height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	allTabs := style.AllTabs()
	var content string
	if m.showHelp {
		content = m.helpView.View(m.width, contentHeight, m.theme)
	} else if v, ok := m.views[m.activeTab]; ok {
		content = v.View()
	} else {
		content = m.theme.Dim.Render(fmt.Sprintf("  %s view — coming soon", allTabs[m.activeTab].Label))
	}

	// Pad content to fill the available height
	contentLines := strings.Count(content, "\n") + 1
	if contentLines < contentHeight {
		content += strings.Repeat("\n", contentHeight-contentLines)
	}

	b.WriteString(content)

	// Error display
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(m.theme.Error.Render("Error: " + m.err.Error()))
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(m.statusBar.View(m.theme))

	return b.String()
}

// switchTab switches to the given tab
func (m *Model) switchTab(tab style.TabID) tea.Cmd {
	m.activeTab = tab
	m.tabs.SetActive(int(tab))
	return m.initActiveView()
}

// initActiveView initializes the active view if it exists
func (m *Model) initActiveView() tea.Cmd {
	if v, ok := m.views[m.activeTab]; ok {
		if m.width > 0 && m.height > 0 {
			contentHeight := m.height - 4
			updated, cmd := v.Update(tea.WindowSizeMsg{
				Width:  m.width,
				Height: contentHeight,
			})
			m.views[m.activeTab] = updated.(style.View)
			return tea.Batch(cmd, v.Init())
		}
		return v.Init()
	}
	return nil
}

// Run starts the TUI application
func Run(workDir, ivaldiDir string) error {
	m := New(workDir, ivaldiDir)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
