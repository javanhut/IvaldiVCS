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

// remoteKeyMap defines keybindings for the remote view
type remoteKeyMap struct {
	Scout   key.Binding
	Upload  key.Binding
	Sync    key.Binding
	Harvest key.Binding
	Portal  key.Binding
	Refresh key.Binding
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
}

func defaultRemoteKeyMap() remoteKeyMap {
	return remoteKeyMap{
		Scout: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "scout remotes"),
		),
		Upload: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "upload (push)"),
		),
		Sync: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "sync (pull)"),
		),
		Harvest: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "harvest all new"),
		),
		Portal: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "set portal"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/up", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "move down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "harvest selected"),
		),
	}
}

// RemoteModel is the remote operations view model
type RemoteModel struct {
	workDir   string
	ivaldiDir string
	keys      remoteKeyMap
	theme     style.Theme
	dialog    components.Dialog

	portal  *engine.PortalInfo
	scout   *engine.ScoutResult
	loading bool
	busy    string // operation in progress description
	err     error
	msg     string // success message
	width   int
	height  int

	// Cursor for scout results (remote-only timelines)
	cursor       int
	harvestables []string
}

// Remote message types

type portalLoadedMsg struct {
	portal *engine.PortalInfo
	err    error
}

type scoutDoneMsg struct {
	result *engine.ScoutResult
	err    error
}

type uploadDoneMsg struct {
	result *engine.UploadResult
	err    error
}

type syncDoneMsg struct {
	result *engine.SyncResult
	err    error
}

type harvestDoneMsg struct {
	result *engine.HarvestResult
	err    error
}

type portalSetMsg struct {
	err error
}

// NewRemoteModel creates a new remote operations view
func NewRemoteModel(workDir, ivaldiDir string) *RemoteModel {
	return &RemoteModel{
		workDir:   workDir,
		ivaldiDir: ivaldiDir,
		keys:      defaultRemoteKeyMap(),
		theme:     style.DefaultTheme(),
		dialog:    components.NewDialog(),
		loading:   true,
	}
}

// Init loads portal info
func (m *RemoteModel) Init() tea.Cmd {
	m.loading = true
	return m.loadPortal()
}

// Update handles messages
func (m *RemoteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case portalLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.portal = nil
			// Not an error - just no portal configured
			m.err = nil
			return m, nil
		}
		m.portal = msg.portal
		m.err = nil
		return m, nil

	case scoutDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.scout = msg.result
		m.err = nil
		m.harvestables = msg.result.RemoteOnly
		m.cursor = 0
		m.msg = fmt.Sprintf("Scouted %d remote timelines", msg.result.Total)
		return m, nil

	case uploadDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.msg = fmt.Sprintf("Uploaded '%s' to %s/%s", msg.result.Timeline, msg.result.Owner, msg.result.Repo)
		return m, nil

	case syncDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		if msg.result.NoChanges {
			m.msg = fmt.Sprintf("'%s' is already up to date", msg.result.Timeline)
		} else {
			total := len(msg.result.Added) + len(msg.result.Modified) + len(msg.result.Deleted)
			m.msg = fmt.Sprintf("Synced %d file(s) for '%s'", total, msg.result.Timeline)
		}
		return m, nil

	case harvestDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		if len(msg.result.Successful) == 0 && len(msg.result.Failed) == 0 {
			m.msg = "No new timelines to harvest"
		} else if len(msg.result.Failed) == 0 {
			m.msg = fmt.Sprintf("Harvested %d timeline(s)", len(msg.result.Successful))
		} else {
			m.msg = fmt.Sprintf("Harvested %d, failed %d", len(msg.result.Successful), len(msg.result.Failed))
		}
		// Refresh scout data
		return m, m.doScout()

	case portalSetMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.msg = "Portal configured"
		m.loading = true
		return m, m.loadPortal()

	case components.DialogSubmitMsg:
		if msg.Type == components.DialogPortal { // reuse DialogSeal type for portal input
			m.busy = "Configuring portal..."
			return m, m.setPortal(msg.Value)
		}
		return m, nil

	case components.DialogCancelMsg:
		return m, nil

	case tea.KeyMsg:
		// Handle dialog first
		if m.dialog.IsActive() {
			cmd, consumed := m.dialog.Update(msg)
			if consumed {
				return m, cmd
			}
		}

		// Don't process keys while busy
		if m.busy != "" {
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			m.msg = ""
			m.err = nil
			return m, m.loadPortal()

		case key.Matches(msg, m.keys.Scout):
			if m.portal == nil {
				m.err = fmt.Errorf("no portal configured. Press 'p' to add one")
				return m, nil
			}
			m.busy = "Scouting remote timelines..."
			m.msg = ""
			m.err = nil
			return m, m.doScout()

		case key.Matches(msg, m.keys.Upload):
			if m.portal == nil {
				m.err = fmt.Errorf("no portal configured")
				return m, nil
			}
			m.busy = "Uploading..."
			m.msg = ""
			m.err = nil
			return m, m.doUpload()

		case key.Matches(msg, m.keys.Sync):
			if m.portal == nil {
				m.err = fmt.Errorf("no portal configured")
				return m, nil
			}
			m.busy = "Syncing..."
			m.msg = ""
			m.err = nil
			return m, m.doSync()

		case key.Matches(msg, m.keys.Harvest):
			if m.portal == nil {
				m.err = fmt.Errorf("no portal configured")
				return m, nil
			}
			m.busy = "Harvesting all new timelines..."
			m.msg = ""
			m.err = nil
			return m, m.doHarvestAll()

		case key.Matches(msg, m.keys.Select):
			if len(m.harvestables) > 0 && m.cursor < len(m.harvestables) {
				name := m.harvestables[m.cursor]
				m.busy = fmt.Sprintf("Harvesting '%s'...", name)
				m.msg = ""
				m.err = nil
				return m, m.doHarvestOne(name)
			}
			return m, nil

		case key.Matches(msg, m.keys.Portal):
			cmd := m.dialog.Open(components.DialogPortal, "Portal (owner/repo):", "owner/repo")
			return m, cmd

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.harvestables)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the remote operations view
func (m *RemoteModel) View() string {
	if m.loading {
		return m.theme.Dim.Render("  Loading remote info...")
	}

	var b strings.Builder

	// Header
	b.WriteString("  ")
	b.WriteString(m.theme.Title.Render("Remote Operations"))
	b.WriteString("\n\n")

	// Portal section
	b.WriteString("  ")
	b.WriteString(m.theme.SectionHead.Render("Portal"))
	b.WriteString("\n")
	if m.portal != nil {
		b.WriteString("    ")
		b.WriteString(m.theme.Bold.Render(fmt.Sprintf("%s/%s", m.portal.Owner, m.portal.Repo)))
		if m.portal.HasAuth {
			b.WriteString(m.theme.Success.Render("  [authenticated]"))
		} else {
			b.WriteString(m.theme.Warning.Render("  [no auth]"))
		}
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(m.theme.Dim.Render(fmt.Sprintf("Timeline: %s", m.portal.Timeline)))
		b.WriteString("\n")
	} else {
		b.WriteString("    ")
		b.WriteString(m.theme.Dim.Render("No repository configured. Press 'p' to add one."))
		b.WriteString("\n")
	}

	// Scout results
	if m.scout != nil {
		b.WriteString("\n  ")
		b.WriteString(m.theme.SectionHead.Render("Scout Results"))
		b.WriteString("\n")

		if len(m.scout.RemoteOnly) > 0 {
			b.WriteString("    ")
			b.WriteString(m.theme.Info.Render(fmt.Sprintf("Available to harvest: %d", len(m.scout.RemoteOnly))))
			b.WriteString("\n")
			for i, name := range m.harvestables {
				b.WriteString("    ")
				if i == m.cursor {
					b.WriteString(m.theme.Cursor.Render("> "))
					b.WriteString(m.theme.Selected.Render(padRight(name, 30)))
				} else {
					b.WriteString("  ")
					b.WriteString(m.theme.Info.Render(padRight(name, 30)))
				}
				b.WriteString("\n")
			}
		}

		if len(m.scout.Both) > 0 {
			b.WriteString("    ")
			b.WriteString(m.theme.Success.Render(fmt.Sprintf("Synced locally: %d", len(m.scout.Both))))
			b.WriteString("  ")
			b.WriteString(m.theme.Dim.Render(strings.Join(m.scout.Both, ", ")))
			b.WriteString("\n")
		}

		if len(m.scout.LocalOnly) > 0 {
			b.WriteString("    ")
			b.WriteString(m.theme.Warning.Render(fmt.Sprintf("Local only: %d", len(m.scout.LocalOnly))))
			b.WriteString("  ")
			b.WriteString(m.theme.Dim.Render(strings.Join(m.scout.LocalOnly, ", ")))
			b.WriteString("\n")
		}
	}

	// Busy indicator
	if m.busy != "" {
		b.WriteString("\n  ")
		b.WriteString(m.theme.Info.Render("~ " + m.busy))
		b.WriteString("\n")
	}

	// Success message
	if m.msg != "" && m.busy == "" {
		b.WriteString("\n  ")
		b.WriteString(m.theme.Success.Render(m.msg))
		b.WriteString("\n")
	}

	// Error
	if m.err != nil && m.busy == "" {
		b.WriteString("\n  ")
		b.WriteString(m.theme.Error.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	}

	// Dialog overlay
	if m.dialog.IsActive() {
		b.WriteString(m.dialog.View(m.theme))
	}

	// Footer
	b.WriteString("\n")
	if m.portal != nil {
		b.WriteString(m.theme.Dim.Render("  s:scout  u:upload  y:sync  h:harvest all  enter:harvest selected  p:portal  r:refresh"))
	} else {
		b.WriteString(m.theme.Dim.Render("  p:set portal  r:refresh"))
	}

	return b.String()
}

// ShortHelp returns a short help string
func (m *RemoteModel) ShortHelp() string {
	return "s:scout  u:upload  y:sync  h:harvest  p:portal  r:refresh"
}

// Async commands

func (m *RemoteModel) loadPortal() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		portal, err := engine.GetPortalInfo(ivaldiDir)
		return portalLoadedMsg{portal: portal, err: err}
	}
}

func (m *RemoteModel) doScout() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	workDir := m.workDir
	return func() tea.Msg {
		result, err := engine.Scout(ivaldiDir, workDir)
		return scoutDoneMsg{result: result, err: err}
	}
}

func (m *RemoteModel) doUpload() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	workDir := m.workDir
	return func() tea.Msg {
		result, err := engine.Upload(ivaldiDir, workDir, false)
		return uploadDoneMsg{result: result, err: err}
	}
}

func (m *RemoteModel) doSync() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	workDir := m.workDir
	return func() tea.Msg {
		result, err := engine.SyncTimeline(ivaldiDir, workDir, "")
		return syncDoneMsg{result: result, err: err}
	}
}

func (m *RemoteModel) doHarvestAll() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	workDir := m.workDir
	return func() tea.Msg {
		result, err := engine.HarvestTimelines(ivaldiDir, workDir, nil)
		return harvestDoneMsg{result: result, err: err}
	}
}

func (m *RemoteModel) doHarvestOne(name string) tea.Cmd {
	ivaldiDir := m.ivaldiDir
	workDir := m.workDir
	return func() tea.Msg {
		result, err := engine.HarvestTimelines(ivaldiDir, workDir, []string{name})
		return harvestDoneMsg{result: result, err: err}
	}
}

func (m *RemoteModel) setPortal(ownerRepo string) tea.Cmd {
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		err := engine.SetPortal(ivaldiDir, ownerRepo)
		return portalSetMsg{err: err}
	}
}
