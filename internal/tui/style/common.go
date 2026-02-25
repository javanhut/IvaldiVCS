package style

import tea "github.com/charmbracelet/bubbletea"

// Tab identifiers
type TabID int

const (
	TabStatus TabID = iota
	TabLog
	TabDiff
	TabTimelines
	TabRemote
	TabFuse
)

// TabInfo describes a tab
type TabInfo struct {
	ID    TabID
	Label string
}

// AllTabs returns the ordered list of tabs
func AllTabs() []TabInfo {
	return []TabInfo{
		{TabStatus, "Status"},
		{TabLog, "Log"},
		{TabDiff, "Diff"},
		{TabTimelines, "Timelines"},
		{TabRemote, "Remote"},
		{TabFuse, "Fuse"},
	}
}

// View is the interface that all TUI views must implement
type View interface {
	tea.Model
	ShortHelp() string
	HasActiveInput() bool
}

// Messages

// ErrMsg signals an error to the root model
type ErrMsg struct {
	Err error
}

// RefreshMsg signals that views should reload their data
type RefreshMsg struct{}

// StatusUpdateMsg carries updated status data
type StatusUpdateMsg struct {
	Timeline  string
	SealName  string
	FileCount int
	Staged    int
	Modified  int
	Untracked int
	Deleted   int
}
