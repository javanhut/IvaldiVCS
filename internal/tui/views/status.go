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

// statusKeyMap defines keybindings specific to the status view
type statusKeyMap struct {
	GatherAll   key.Binding
	UngatherAll key.Binding
	Refresh     key.Binding
	Seal        key.Binding
}

func defaultStatusKeyMap() statusKeyMap {
	return statusKeyMap{
		GatherAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "gather all"),
		),
		UngatherAll: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "ungather all"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Seal: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "seal (commit)"),
		),
	}
}

// StatusModel is the status view model
type StatusModel struct {
	workDir   string
	ivaldiDir string
	fileList  components.FileList
	dialog    components.Dialog
	keys      statusKeyMap
	theme     style.Theme
	width     int
	height    int
	status    *engine.StatusResult
	err       error
	loading   bool
	sealMsg   string // success message after seal
}

// statusLoadedMsg carries loaded status data
type statusLoadedMsg struct {
	result *engine.StatusResult
	err    error
}

// statusStagingDoneMsg signals staging operation completed
type statusStagingDoneMsg struct {
	err error
}

// sealDoneMsg signals seal operation completed
type sealDoneMsg struct {
	result *engine.SealResult
	err    error
}

// NewStatusModel creates a new status view
func NewStatusModel(workDir, ivaldiDir string) *StatusModel {
	fl := components.NewFileList()
	fl.Focus(true)

	return &StatusModel{
		workDir:   workDir,
		ivaldiDir: ivaldiDir,
		fileList:  fl,
		dialog:    components.NewDialog(),
		keys:      defaultStatusKeyMap(),
		theme:     style.DefaultTheme(),
		loading:   true,
	}
}

// Init loads the initial status data
func (m *StatusModel) Init() tea.Cmd {
	m.loading = true
	return m.loadStatus()
}

// Update handles messages
func (m *StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listHeight := msg.Height - 4
		if listHeight < 1 {
			listHeight = 1
		}
		m.fileList.SetSize(msg.Width, listHeight)
		return m, nil

	case statusLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.status = msg.result
		m.err = nil
		m.rebuildFileList()

		return m, func() tea.Msg {
			return style.StatusUpdateMsg{
				Timeline:  msg.result.Timeline,
				SealName:  msg.result.SealName,
				FileCount: msg.result.FileCount,
				Staged:    len(msg.result.Staged),
				Modified:  len(msg.result.Modified),
				Untracked: len(msg.result.Untracked),
				Deleted:   len(msg.result.Deleted),
			}
		}

	case statusStagingDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, m.loadStatus()

	case sealDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.sealMsg = fmt.Sprintf("Sealed '%s' on %s (%s)", msg.result.SealName, msg.result.Timeline, msg.result.Hash)
		return m, m.loadStatus()

	case components.DialogSubmitMsg:
		if msg.Type == components.DialogSeal {
			m.loading = true
			m.sealMsg = ""
			return m, m.createSeal(msg.Value)
		}
		return m, nil

	case components.DialogCancelMsg:
		return m, nil

	case tea.KeyMsg:
		// Handle dialog input first
		if m.dialog.IsActive() {
			cmd, consumed := m.dialog.Update(msg)
			if consumed {
				return m, cmd
			}
		}

		switch {
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			m.sealMsg = ""
			return m, m.loadStatus()

		case key.Matches(msg, m.keys.Seal):
			if m.status != nil && len(m.status.Staged) > 0 {
				cmd := m.dialog.Open(components.DialogSeal, "Seal message:", "describe your changes")
				return m, cmd
			}
			m.err = fmt.Errorf("no files staged. Gather files first")
			return m, nil

		case key.Matches(msg, m.keys.GatherAll):
			return m, m.gatherAll()

		case key.Matches(msg, m.keys.UngatherAll):
			return m, m.ungatherAll()

		case key.Matches(msg, m.fileList.Keys.Toggle):
			item := m.fileList.SelectedItem()
			if item != nil {
				return m, m.toggleFile(item.Info)
			}
		}

		cmd := m.fileList.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the status view
func (m *StatusModel) View() string {
	if m.loading {
		return m.theme.Dim.Render("  Loading status...")
	}

	if m.err != nil {
		return m.theme.Error.Render(fmt.Sprintf("  Error: %v", m.err))
	}

	if m.status == nil {
		return m.theme.Dim.Render("  No status data")
	}

	var b strings.Builder

	// Seal success message
	if m.sealMsg != "" {
		b.WriteString("  ")
		b.WriteString(m.theme.Success.Render(m.sealMsg))
		b.WriteString("\n\n")
	}

	total := len(m.status.Files)
	if total == 0 {
		b.WriteString(m.theme.Success.Render("  Working directory clean"))
		b.WriteString("\n\n")
		b.WriteString(m.theme.Dim.Render("  Press r to refresh"))
	} else {
		var parts []string
		if len(m.status.Staged) > 0 {
			parts = append(parts, m.theme.Staged.Render(fmt.Sprintf("%d staged", len(m.status.Staged))))
		}
		if len(m.status.Modified) > 0 {
			parts = append(parts, m.theme.Modified.Render(fmt.Sprintf("%d modified", len(m.status.Modified))))
		}
		if len(m.status.Untracked) > 0 {
			parts = append(parts, m.theme.Untracked.Render(fmt.Sprintf("%d untracked", len(m.status.Untracked))))
		}
		if len(m.status.Deleted) > 0 {
			parts = append(parts, m.theme.Deleted.Render(fmt.Sprintf("%d deleted", len(m.status.Deleted))))
		}

		b.WriteString("  ")
		b.WriteString(strings.Join(parts, m.theme.Dim.Render(" | ")))
		b.WriteString("\n\n")

		b.WriteString(m.fileList.View(m.theme))
	}

	// Dialog overlay
	if m.dialog.IsActive() {
		b.WriteString(m.dialog.View(m.theme))
	}

	return b.String()
}

// ShortHelp returns a short help string for the status bar
func (m *StatusModel) ShortHelp() string {
	return "j/k:nav  space:toggle  a:gather all  u:ungather all  s:seal  r:refresh"
}

// rebuildFileList rebuilds the file list from current status
func (m *StatusModel) rebuildFileList() {
	if m.status == nil {
		m.fileList.SetItems(nil)
		return
	}

	var items []components.FileItem

	for _, f := range m.status.Staged {
		items = append(items, components.FileItem{Info: f})
	}
	for _, f := range m.status.Modified {
		items = append(items, components.FileItem{Info: f})
	}
	for _, f := range m.status.Deleted {
		items = append(items, components.FileItem{Info: f})
	}
	for _, f := range m.status.Untracked {
		items = append(items, components.FileItem{Info: f})
	}

	m.fileList.SetItems(items)
}

// loadStatus loads status data asynchronously
func (m *StatusModel) loadStatus() tea.Cmd {
	workDir := m.workDir
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		result, err := engine.GetFileStatuses(workDir, ivaldiDir)
		return statusLoadedMsg{result: result, err: err}
	}
}

// toggleFile toggles the staging state of a file
func (m *StatusModel) toggleFile(info engine.FileStatusInfo) tea.Cmd {
	workDir := m.workDir
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		var err error
		switch info.Status {
		case engine.StatusStaged, engine.StatusAdded:
			err = engine.UngatherFiles(ivaldiDir, []string{info.Path})
		case engine.StatusModified, engine.StatusUntracked, engine.StatusDeleted:
			err = engine.GatherFiles(workDir, ivaldiDir, []string{info.Path})
		}
		return statusStagingDoneMsg{err: err}
	}
}

// gatherAll stages all unstaged files
func (m *StatusModel) gatherAll() tea.Cmd {
	workDir := m.workDir
	ivaldiDir := m.ivaldiDir
	files := m.status.Files
	return func() tea.Msg {
		err := engine.GatherAllUnstaged(workDir, ivaldiDir, files)
		return statusStagingDoneMsg{err: err}
	}
}

// ungatherAll removes all files from staging
func (m *StatusModel) ungatherAll() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		err := engine.UngatherAll(ivaldiDir)
		return statusStagingDoneMsg{err: err}
	}
}

// createSeal creates a seal (commit) asynchronously
func (m *StatusModel) createSeal(message string) tea.Cmd {
	workDir := m.workDir
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		result, err := engine.CreateSeal(ivaldiDir, workDir, message)
		return sealDoneMsg{result: result, err: err}
	}
}
