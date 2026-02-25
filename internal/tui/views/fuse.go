package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
	"github.com/javanhut/Ivaldi-vcs/internal/engine"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// Available merge strategies in display order
var fuseStrategies = []diffmerge.StrategyType{
	diffmerge.StrategyAuto,
	diffmerge.StrategyOurs,
	diffmerge.StrategyTheirs,
	diffmerge.StrategyUnion,
	diffmerge.StrategyBase,
}

var fuseStrategyDescs = map[diffmerge.StrategyType]string{
	diffmerge.StrategyAuto:   "Intelligent chunk-level merge",
	diffmerge.StrategyOurs:   "Keep target timeline version",
	diffmerge.StrategyTheirs: "Accept source timeline version",
	diffmerge.StrategyUnion:  "Combine both versions",
	diffmerge.StrategyBase:   "Revert to common ancestor",
}

// fuseKeyMap defines keybindings for the fuse view
type fuseKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Select   key.Binding
	Strategy key.Binding
	Abort    key.Binding
	Refresh  key.Binding
	GoTop    key.Binding
	GoBottom key.Binding
}

func defaultFuseKeyMap() fuseKeyMap {
	return fuseKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/up", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "move down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "f"),
			key.WithHelp("enter/f", "fuse selected"),
		),
		Strategy: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "cycle strategy"),
		),
		Abort: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "abort merge"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		GoTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "go to top"),
		),
		GoBottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to bottom"),
		),
	}
}

// fuseState represents the current state of the fuse view
type fuseState int

const (
	fuseStateIdle      fuseState = iota // Selecting timeline to fuse
	fuseStateBusy                       // Operation in progress
	fuseStateResult                     // Showing fuse result
	fuseStateConflicts                  // Showing conflicts from merge in progress
)

// FuseModel is the fuse (merge) view model
type FuseModel struct {
	workDir   string
	ivaldiDir string
	keys      fuseKeyMap
	theme     style.Theme

	state         fuseState
	loading       bool
	busy          string
	err           error
	msg           string
	width         int
	height        int

	// Timeline selection
	timelines     []engine.TimelineInfo
	current       string
	cursor        int
	strategyIndex int // Index into fuseStrategies

	// Merge status
	fuseStatus *engine.FuseStatus
	lastResult *engine.FuseResult
}

// Fuse message types

type fuseTimelinesLoadedMsg struct {
	timelines []engine.TimelineInfo
	current   string
	status    *engine.FuseStatus
	err       error
}

type fuseDoneMsg struct {
	result *engine.FuseResult
	err    error
}

type fuseAbortDoneMsg struct {
	err error
}

// NewFuseModel creates a new fuse view
func NewFuseModel(workDir, ivaldiDir string) *FuseModel {
	return &FuseModel{
		workDir:   workDir,
		ivaldiDir: ivaldiDir,
		keys:      defaultFuseKeyMap(),
		theme:     style.DefaultTheme(),
		loading:   true,
	}
}

// Init loads timeline list and merge status
func (m *FuseModel) Init() tea.Cmd {
	m.loading = true
	return m.loadData()
}

// Update handles messages
func (m *FuseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case fuseTimelinesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.timelines = msg.timelines
		m.current = msg.current
		m.fuseStatus = msg.status
		m.err = nil

		// If merge in progress, show conflicts state
		if m.fuseStatus != nil && m.fuseStatus.InProgress {
			m.state = fuseStateConflicts
		} else {
			m.state = fuseStateIdle
		}

		// Reset cursor if out of bounds
		if m.cursor >= len(m.timelines) {
			m.cursor = 0
		}
		return m, nil

	case fuseDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.err = msg.err
			m.state = fuseStateIdle
			return m, nil
		}
		m.err = nil
		m.lastResult = msg.result

		if msg.result.Type == engine.FuseMergeConflicts {
			m.state = fuseStateConflicts
			// Reload to get merge status
			return m, m.loadData()
		}

		m.state = fuseStateResult
		return m, nil

	case fuseAbortDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.msg = "Merge aborted"
		m.state = fuseStateIdle
		m.fuseStatus = nil
		m.lastResult = nil
		return m, m.loadData()

	case tea.KeyMsg:
		if m.loading || m.busy != "" {
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			m.msg = ""
			m.err = nil
			return m, m.loadData()

		case key.Matches(msg, m.keys.Abort):
			if m.fuseStatus != nil && m.fuseStatus.InProgress {
				m.busy = "Aborting merge..."
				m.msg = ""
				m.err = nil
				return m, m.doAbort()
			}
			return m, nil

		case key.Matches(msg, m.keys.Strategy):
			if m.state == fuseStateIdle {
				m.strategyIndex = (m.strategyIndex + 1) % len(fuseStrategies)
			}
			return m, nil

		case key.Matches(msg, m.keys.Select):
			if m.state == fuseStateIdle && len(m.timelines) > 0 && m.cursor < len(m.timelines) {
				source := m.timelines[m.cursor].Name
				strategy := string(fuseStrategies[m.strategyIndex])
				m.busy = fmt.Sprintf("Fusing %s into %s...", source, m.current)
				m.msg = ""
				m.err = nil
				m.state = fuseStateBusy
				return m, m.doFuse(source, m.current, strategy)
			}
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if m.state == fuseStateIdle && m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if m.state == fuseStateIdle && m.cursor < len(m.timelines)-1 {
				m.cursor++
			}
			return m, nil

		case key.Matches(msg, m.keys.GoTop):
			if m.state == fuseStateIdle {
				m.cursor = 0
			}
			return m, nil

		case key.Matches(msg, m.keys.GoBottom):
			if m.state == fuseStateIdle && len(m.timelines) > 0 {
				m.cursor = len(m.timelines) - 1
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the fuse view
func (m *FuseModel) View() string {
	if m.loading {
		return m.theme.Dim.Render("  Loading fuse info...")
	}

	var b strings.Builder

	// Header
	b.WriteString("  ")
	b.WriteString(m.theme.Title.Render("Fuse (Merge)"))
	b.WriteString("\n\n")

	// Show merge-in-progress state
	if m.fuseStatus != nil && m.fuseStatus.InProgress {
		m.renderMergeInProgress(&b)
	} else if m.state == fuseStateResult && m.lastResult != nil {
		m.renderResult(&b)
	} else {
		m.renderTimelineSelection(&b)
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

	return b.String()
}

func (m *FuseModel) renderTimelineSelection(b *strings.Builder) {
	// Current timeline
	b.WriteString("  ")
	b.WriteString(m.theme.SectionHead.Render("Target"))
	b.WriteString("  ")
	if m.current != "" {
		b.WriteString(m.theme.Bold.Render(m.current))
	} else {
		b.WriteString(m.theme.Dim.Render("(no current timeline)"))
	}
	b.WriteString("\n")

	// Strategy
	b.WriteString("  ")
	b.WriteString(m.theme.SectionHead.Render("Strategy"))
	b.WriteString("  ")
	strategy := fuseStrategies[m.strategyIndex]
	b.WriteString(m.theme.Info.Render(string(strategy)))
	b.WriteString(m.theme.Dim.Render(" — " + fuseStrategyDescs[strategy]))
	b.WriteString("\n\n")

	// Source timeline selection
	b.WriteString("  ")
	b.WriteString(m.theme.SectionHead.Render("Select Source Timeline"))
	b.WriteString("\n")

	if len(m.timelines) == 0 {
		b.WriteString("    ")
		b.WriteString(m.theme.Dim.Render("No other timelines available"))
		b.WriteString("\n")
		return
	}

	for i, tl := range m.timelines {
		b.WriteString("    ")
		if i == m.cursor {
			b.WriteString(m.theme.Cursor.Render("> "))
			name := tl.Name
			if tl.IsButterfly {
				name += " [butterfly]"
			}
			b.WriteString(m.theme.Selected.Render(padRight(name, 30)))
			if tl.Hash != "" {
				b.WriteString(m.theme.Dim.Render(" " + tl.Hash))
			}
		} else {
			b.WriteString("  ")
			name := tl.Name
			if tl.IsButterfly {
				name += " [butterfly]"
			}
			b.WriteString(m.theme.Info.Render(padRight(name, 30)))
			if tl.Hash != "" {
				b.WriteString(m.theme.Dim.Render(" " + tl.Hash))
			}
		}
		b.WriteString("\n")
	}
}

func (m *FuseModel) renderMergeInProgress(b *strings.Builder) {
	b.WriteString("  ")
	b.WriteString(m.theme.Warning.Render("Merge in progress"))
	b.WriteString("\n\n")

	b.WriteString("    ")
	b.WriteString(m.theme.Bold.Render(fmt.Sprintf("%s -> %s", m.fuseStatus.SourceTimeline, m.fuseStatus.TargetTimeline)))
	b.WriteString("\n")

	if len(m.fuseStatus.Conflicts) > 0 {
		b.WriteString("\n  ")
		b.WriteString(m.theme.SectionHead.Render("Conflicts"))
		b.WriteString("\n")

		for _, path := range m.fuseStatus.Conflicts {
			b.WriteString("    ")
			b.WriteString(m.theme.Error.Render("CONFLICT: "))
			b.WriteString(m.theme.Bold.Render(path))
			b.WriteString("\n")
		}

		b.WriteString("\n    ")
		b.WriteString(m.theme.Dim.Render(fmt.Sprintf("%d file(s) with conflicts", len(m.fuseStatus.Conflicts))))
		b.WriteString("\n")
	}

	b.WriteString("\n  ")
	b.WriteString(m.theme.SectionHead.Render("Options"))
	b.WriteString("\n")
	b.WriteString("    ")
	b.WriteString(m.theme.Info.Render("a"))
	b.WriteString(m.theme.Dim.Render(" — Abort merge"))
	b.WriteString("\n")
	b.WriteString("    ")
	b.WriteString(m.theme.Dim.Render("Use CLI 'ivaldi fuse --continue' for interactive conflict resolution"))
	b.WriteString("\n")
}

func (m *FuseModel) renderResult(b *strings.Builder) {
	result := m.lastResult

	switch result.Type {
	case engine.FuseFastForward:
		b.WriteString("  ")
		b.WriteString(m.theme.Success.Render("Fast-forward merge complete"))
		b.WriteString("\n\n")
		b.WriteString("    ")
		b.WriteString(m.theme.Bold.Render(fmt.Sprintf("%s fast-forwarded to %s", result.Target, result.Source)))
		b.WriteString("\n")

	case engine.FuseMergeSuccess:
		b.WriteString("  ")
		b.WriteString(m.theme.Success.Render("Three-way merge complete"))
		b.WriteString("\n\n")
		b.WriteString("    ")
		b.WriteString(m.theme.Bold.Render(fmt.Sprintf("%s fused into %s", result.Source, result.Target)))
		b.WriteString("\n")
		if result.SealName != "" {
			b.WriteString("    ")
			b.WriteString(m.theme.Dim.Render("Seal: "))
			b.WriteString(m.theme.Info.Render(result.SealName))
			b.WriteString("\n")
		}

		// Change stats
		if result.Added > 0 || result.Modified > 0 || result.Removed > 0 {
			b.WriteString("\n  ")
			b.WriteString(m.theme.SectionHead.Render("Changes"))
			b.WriteString("\n")
			if result.Added > 0 {
				b.WriteString("    ")
				b.WriteString(m.theme.Success.Render(fmt.Sprintf("+ %d file(s)", result.Added)))
				b.WriteString("\n")
			}
			if result.Modified > 0 {
				b.WriteString("    ")
				b.WriteString(m.theme.Info.Render(fmt.Sprintf("~ %d file(s)", result.Modified)))
				b.WriteString("\n")
			}
			if result.Removed > 0 {
				b.WriteString("    ")
				b.WriteString(m.theme.Error.Render(fmt.Sprintf("- %d file(s)", result.Removed)))
				b.WriteString("\n")
			}
		}

	case engine.FuseMergeConflicts:
		b.WriteString("  ")
		b.WriteString(m.theme.Warning.Render("Merge has conflicts"))
		b.WriteString("\n\n")
		for _, path := range result.Conflicts {
			b.WriteString("    ")
			b.WriteString(m.theme.Error.Render("CONFLICT: "))
			b.WriteString(m.theme.Bold.Render(path))
			b.WriteString("\n")
		}
	}
}

// ShortHelp returns a short help string
func (m *FuseModel) ShortHelp() string {
	return "j/k:navigate  enter:fuse  s:strategy  a:abort  r:refresh"
}

// HasActiveInput returns whether the fuse view has an active input dialog
func (m *FuseModel) HasActiveInput() bool {
	return false
}

// Async commands

func (m *FuseModel) loadData() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		// Get timelines
		result, err := engine.ListTimelines(ivaldiDir)
		if err != nil {
			return fuseTimelinesLoadedMsg{err: err}
		}

		// Filter out current timeline from selection list
		var selectable []engine.TimelineInfo
		for _, tl := range result.Local {
			if !tl.IsCurrent {
				selectable = append(selectable, tl)
			}
		}

		// Get merge status
		fuseStatus, _ := engine.GetFuseStatus(ivaldiDir)

		return fuseTimelinesLoadedMsg{
			timelines: selectable,
			current:   result.Current,
			status:    fuseStatus,
		}
	}
}

func (m *FuseModel) doFuse(source, target, strategy string) tea.Cmd {
	ivaldiDir := m.ivaldiDir
	workDir := m.workDir
	return func() tea.Msg {
		result, err := engine.FuseTimelines(ivaldiDir, workDir, source, target, strategy)
		return fuseDoneMsg{result: result, err: err}
	}
}

func (m *FuseModel) doAbort() tea.Cmd {
	ivaldiDir := m.ivaldiDir
	return func() tea.Msg {
		err := engine.AbortFuse(ivaldiDir)
		return fuseAbortDoneMsg{err: err}
	}
}
