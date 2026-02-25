package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/javanhut/Ivaldi-vcs/internal/engine"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// timelineKeyMap defines keybindings for the timeline view
type timelineKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Top     key.Binding
	Bottom  key.Binding
	Switch  key.Binding
	Create  key.Binding
	Remove  key.Binding
	Rename  key.Binding
	Refresh key.Binding
}

func defaultTimelineKeyMap() timelineKeyMap {
	return timelineKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/up", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "move down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		Switch: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "switch to timeline"),
		),
		Create: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create timeline"),
		),
		Remove: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "remove timeline"),
		),
		Rename: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "rename timeline"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// inputMode tracks which input dialog is active
type inputMode int

const (
	inputNone inputMode = iota
	inputCreate
	inputRename
	inputConfirmRemove
)

// TimelineModel is the timeline view model
type TimelineModel struct {
	workDir   string
	ivaldiDir string
	keys      timelineKeyMap
	theme     style.Theme

	result  *engine.TimelineListResult
	loading bool
	err     error
	width   int
	height  int

	cursor int // cursor position in local timelines list

	// Input dialog state
	mode      inputMode
	textInput textinput.Model
	renameOld string // old name when renaming
}

// timelineLoadedMsg carries loaded timeline data
type timelineLoadedMsg struct {
	result *engine.TimelineListResult
	err    error
}

// timelineSwitchedMsg signals a timeline switch completed
type timelineSwitchedMsg struct {
	name string
	err  error
}

// timelineActionMsg signals a create/remove/rename completed
type timelineActionMsg struct {
	action string
	err    error
}

// NewTimelineModel creates a new timeline view
func NewTimelineModel(workDir, ivaldiDir string) *TimelineModel {
	ti := textinput.New()
	ti.CharLimit = 64
	ti.Width = 30

	return &TimelineModel{
		workDir:   workDir,
		ivaldiDir: ivaldiDir,
		keys:      defaultTimelineKeyMap(),
		theme:     style.DefaultTheme(),
		loading:   true,
		textInput: ti,
	}
}

// Init loads timeline data
func (m *TimelineModel) Init() tea.Cmd {
	m.loading = true
	return m.loadTimelines()
}

// Update handles messages
func (m *TimelineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case timelineLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.result = msg.result
		m.err = nil
		// Clamp cursor
		if m.result != nil && m.cursor >= len(m.result.Local) {
			m.cursor = len(m.result.Local) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

	case timelineSwitchedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.loading = true
		return m, m.loadTimelines()

	case timelineActionMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.loading = true
		return m, m.loadTimelines()

	case tea.KeyMsg:
		// Handle input mode first
		if m.mode != inputNone {
			return m.updateInput(msg)
		}

		switch {
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			return m, m.loadTimelines()

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if m.result != nil && m.cursor < len(m.result.Local)-1 {
				m.cursor++
			}
			return m, nil

		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
			return m, nil

		case key.Matches(msg, m.keys.Bottom):
			if m.result != nil && len(m.result.Local) > 0 {
				m.cursor = len(m.result.Local) - 1
			}
			return m, nil

		case key.Matches(msg, m.keys.Switch):
			if m.result != nil && len(m.result.Local) > 0 {
				selected := m.result.Local[m.cursor]
				if selected.IsCurrent {
					return m, nil // already on this timeline
				}
				m.loading = true
				return m, m.switchTimeline(selected.Name)
			}
			return m, nil

		case key.Matches(msg, m.keys.Create):
			m.mode = inputCreate
			m.textInput.Placeholder = "new timeline name"
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, m.keys.Remove):
			if m.result != nil && len(m.result.Local) > 0 {
				selected := m.result.Local[m.cursor]
				if selected.IsCurrent {
					m.err = fmt.Errorf("cannot remove current timeline")
					return m, nil
				}
				m.mode = inputConfirmRemove
				m.textInput.Placeholder = "type 'yes' to confirm"
				m.textInput.SetValue("")
				m.textInput.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case key.Matches(msg, m.keys.Rename):
			if m.result != nil && len(m.result.Local) > 0 {
				m.renameOld = m.result.Local[m.cursor].Name
				m.mode = inputRename
				m.textInput.Placeholder = "new name"
				m.textInput.SetValue("")
				m.textInput.Focus()
				return m, textinput.Blink
			}
			return m, nil
		}
	}

	return m, nil
}

// updateInput handles key events when an input dialog is active
func (m *TimelineModel) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = inputNone
		m.textInput.Blur()
		return m, nil

	case "enter":
		value := strings.TrimSpace(m.textInput.Value())
		m.textInput.Blur()
		mode := m.mode
		m.mode = inputNone

		switch mode {
		case inputCreate:
			if value == "" {
				return m, nil
			}
			m.loading = true
			return m, m.createTimeline(value)

		case inputRename:
			if value == "" {
				return m, nil
			}
			m.loading = true
			return m, m.renameTimeline(m.renameOld, value)

		case inputConfirmRemove:
			if value == "yes" && m.result != nil && len(m.result.Local) > 0 {
				selected := m.result.Local[m.cursor]
				m.loading = true
				return m, m.removeTimeline(selected.Name)
			}
			return m, nil
		}
		return m, nil
	}

	// Forward to text input
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// View renders the timeline view
func (m *TimelineModel) View() string {
	if m.loading {
		return m.theme.Dim.Render("  Loading timelines...")
	}

	if m.err != nil {
		return m.theme.Error.Render(fmt.Sprintf("  Error: %v", m.err))
	}

	if m.result == nil {
		return m.theme.Dim.Render("  No timeline data")
	}

	var b strings.Builder

	// Header
	b.WriteString("  ")
	b.WriteString(m.theme.Title.Render("Timelines"))
	b.WriteString("  ")
	b.WriteString(m.theme.Dim.Render(fmt.Sprintf("(current: %s)", m.result.Current)))
	b.WriteString("\n\n")

	// Local timelines
	b.WriteString("  ")
	b.WriteString(m.theme.SectionHead.Render("Local"))
	b.WriteString("\n")

	if len(m.result.Local) == 0 {
		b.WriteString(m.theme.Dim.Render("    No local timelines\n"))
	} else {
		for i, tl := range m.result.Local {
			b.WriteString("  ")

			// Cursor
			if i == m.cursor {
				b.WriteString(m.theme.Cursor.Render("> "))
			} else {
				b.WriteString("  ")
			}

			// Current marker
			if tl.IsCurrent {
				b.WriteString(m.theme.Success.Render("* "))
			} else {
				b.WriteString("  ")
			}

			// Name
			name := tl.Name
			if tl.IsButterfly {
				name += " [butterfly]"
			}

			if i == m.cursor {
				b.WriteString(m.theme.Selected.Render(padRight(name, 30)))
			} else if tl.IsCurrent {
				b.WriteString(m.theme.Success.Render(padRight(name, 30)))
			} else {
				b.WriteString(padRight(name, 30))
			}

			// Hash
			if tl.Hash != "" {
				b.WriteString(m.theme.Dim.Render(" " + tl.Hash))
			}

			// Description
			if tl.Description != "" {
				b.WriteString(m.theme.Dim.Render("  " + truncate(tl.Description, 40)))
			}

			b.WriteString("\n")
		}
	}

	// Remote timelines
	if len(m.result.Remote) > 0 {
		b.WriteString("\n  ")
		b.WriteString(m.theme.SectionHead.Render("Remote"))
		b.WriteString("\n")
		for _, tl := range m.result.Remote {
			b.WriteString("      ")
			b.WriteString(m.theme.Info.Render(padRight(tl.Name, 30)))
			if tl.Description != "" {
				b.WriteString(m.theme.Dim.Render("  " + truncate(tl.Description, 40)))
			}
			b.WriteString("\n")
		}
	}

	// Tags
	if len(m.result.Tags) > 0 {
		b.WriteString("\n  ")
		b.WriteString(m.theme.SectionHead.Render("Tags"))
		b.WriteString("\n")
		for _, tl := range m.result.Tags {
			b.WriteString("      ")
			b.WriteString(m.theme.Warning.Render(padRight(tl.Name, 30)))
			if tl.Description != "" {
				b.WriteString(m.theme.Dim.Render("  " + truncate(tl.Description, 40)))
			}
			b.WriteString("\n")
		}
	}

	// Input dialog
	if m.mode != inputNone {
		b.WriteString("\n")
		switch m.mode {
		case inputCreate:
			b.WriteString(m.theme.Title.Render("  Create timeline: "))
		case inputRename:
			b.WriteString(m.theme.Title.Render(fmt.Sprintf("  Rename '%s' to: ", m.renameOld)))
		case inputConfirmRemove:
			if m.result != nil && m.cursor < len(m.result.Local) {
				b.WriteString(m.theme.Warning.Render(
					fmt.Sprintf("  Remove '%s'? ", m.result.Local[m.cursor].Name)))
			}
		}
		b.WriteString(m.textInput.View())
		b.WriteString("\n")
		b.WriteString(m.theme.Dim.Render("  (enter to confirm, esc to cancel)"))
	}

	return b.String()
}

// ShortHelp returns a short help string
func (m *TimelineModel) ShortHelp() string {
	return "j/k:navigate  enter:switch  c:create  d:remove  R:rename  r:refresh"
}

// HasActiveInput returns whether the timeline view has an active input dialog
func (m *TimelineModel) HasActiveInput() bool {
	return m.mode != inputNone
}

// loadTimelines loads timeline data asynchronously
func (m *TimelineModel) loadTimelines() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		result, err := engine.ListTimelines(ivaldiDir)
		return timelineLoadedMsg{result: result, err: err}
	}
}

// switchTimeline switches to a timeline asynchronously
func (m *TimelineModel) switchTimeline(name string) tea.Cmd {
	workDir := m.workDir
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		err := engine.SwitchTimeline(ivaldiDir, workDir, name)
		return timelineSwitchedMsg{name: name, err: err}
	}
}

// createTimeline creates a new timeline asynchronously
func (m *TimelineModel) createTimeline(name string) tea.Cmd {
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		err := engine.CreateTimeline(ivaldiDir, name)
		return timelineActionMsg{action: "create", err: err}
	}
}

// removeTimeline removes a timeline asynchronously
func (m *TimelineModel) removeTimeline(name string) tea.Cmd {
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		err := engine.RemoveTimeline(ivaldiDir, name)
		return timelineActionMsg{action: "remove", err: err}
	}
}

// renameTimeline renames a timeline asynchronously
func (m *TimelineModel) renameTimeline(oldName, newName string) tea.Cmd {
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		err := engine.RenameTimeline(ivaldiDir, oldName, newName, false)
		return timelineActionMsg{action: "rename", err: err}
	}
}

// truncate shortens a string to maxLen with ellipsis
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
