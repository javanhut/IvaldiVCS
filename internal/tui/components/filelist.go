package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/javanhut/Ivaldi-vcs/internal/engine"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// FileItem represents a file in the list
type FileItem struct {
	Info     engine.FileStatusInfo
	Selected bool
}

// FileList is a selectable, scrollable file list with toggle
type FileList struct {
	Items    []FileItem
	cursor   int
	offset   int // scroll offset
	height   int
	width    int
	focused  bool

	Keys FileListKeyMap
}

// FileListKeyMap defines keybindings for the file list
type FileListKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Toggle key.Binding
	Top    key.Binding
	Bottom key.Binding
}

// DefaultFileListKeyMap returns default keybindings
func DefaultFileListKeyMap() FileListKeyMap {
	return FileListKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/up", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "move down"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle staging"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
		),
	}
}

// NewFileList creates a new file list
func NewFileList() FileList {
	return FileList{
		Keys:   DefaultFileListKeyMap(),
		height: 20,
	}
}

// SetItems updates the list items
func (f *FileList) SetItems(items []FileItem) {
	f.Items = items
	if f.cursor >= len(items) {
		f.cursor = len(items) - 1
	}
	if f.cursor < 0 {
		f.cursor = 0
	}
	f.fixScroll()
}

// SetSize updates the visible dimensions
func (f *FileList) SetSize(width, height int) {
	f.width = width
	f.height = height
	f.fixScroll()
}

// Focus sets whether the list is focused
func (f *FileList) Focus(focused bool) {
	f.focused = focused
}

// Cursor returns the current cursor position
func (f *FileList) Cursor() int {
	return f.cursor
}

// SelectedItem returns the item under the cursor, or nil
func (f *FileList) SelectedItem() *FileItem {
	if f.cursor >= 0 && f.cursor < len(f.Items) {
		return &f.Items[f.cursor]
	}
	return nil
}

// ToggleCurrent toggles the selected state of the item under the cursor
func (f *FileList) ToggleCurrent() {
	if f.cursor >= 0 && f.cursor < len(f.Items) {
		f.Items[f.cursor].Selected = !f.Items[f.cursor].Selected
	}
}

// Update handles key messages
func (f *FileList) Update(msg tea.Msg) tea.Cmd {
	if !f.focused || len(f.Items) == 0 {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, f.Keys.Up):
			if f.cursor > 0 {
				f.cursor--
				f.fixScroll()
			}
		case key.Matches(msg, f.Keys.Down):
			if f.cursor < len(f.Items)-1 {
				f.cursor++
				f.fixScroll()
			}
		case key.Matches(msg, f.Keys.Top):
			f.cursor = 0
			f.fixScroll()
		case key.Matches(msg, f.Keys.Bottom):
			f.cursor = len(f.Items) - 1
			f.fixScroll()
		case key.Matches(msg, f.Keys.Toggle):
			f.ToggleCurrent()
		}
	}

	return nil
}

// fixScroll ensures the cursor is visible
func (f *FileList) fixScroll() {
	if f.height <= 0 {
		return
	}
	if f.cursor < f.offset {
		f.offset = f.cursor
	}
	if f.cursor >= f.offset+f.height {
		f.offset = f.cursor - f.height + 1
	}
}

// View renders the file list
func (f FileList) View(theme style.Theme) string {
	if len(f.Items) == 0 {
		return theme.Dim.Render("  No files")
	}

	var b strings.Builder
	end := f.offset + f.height
	if end > len(f.Items) {
		end = len(f.Items)
	}

	for i := f.offset; i < end; i++ {
		item := f.Items[i]
		isCursor := i == f.cursor && f.focused

		// Cursor indicator
		prefix := "  "
		if isCursor {
			prefix = theme.Cursor.Render("> ")
		}

		// Status indicator
		statusStr := statusLabel(item.Info.Status, theme)

		// File path
		pathStr := styleFilePath(item.Info.Path, item.Info.Status, theme)

		line := fmt.Sprintf("%s%s %s", prefix, statusStr, pathStr)

		if isCursor {
			line = theme.Selected.Render(line)
		}

		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// statusLabel returns a styled status label
func statusLabel(status engine.FileStatus, theme style.Theme) string {
	switch status {
	case engine.StatusAdded:
		return theme.Added.Render("[new]      ")
	case engine.StatusStaged:
		return theme.Staged.Render("[staged]   ")
	case engine.StatusModified:
		return theme.Modified.Render("[modified] ")
	case engine.StatusDeleted:
		return theme.Deleted.Render("[deleted]  ")
	case engine.StatusUntracked:
		return theme.Untracked.Render("[untracked]")
	case engine.StatusIgnored:
		return theme.Ignored.Render("[ignored]  ")
	default:
		return "           "
	}
}

// styleFilePath styles a file path based on status
func styleFilePath(path string, status engine.FileStatus, theme style.Theme) string {
	switch status {
	case engine.StatusAdded:
		return theme.Added.Render(path)
	case engine.StatusStaged:
		return theme.Staged.Render(path)
	case engine.StatusModified:
		return theme.Modified.Render(path)
	case engine.StatusDeleted:
		return theme.Deleted.Render(path)
	case engine.StatusUntracked:
		return theme.Untracked.Render(path)
	case engine.StatusIgnored:
		return theme.Ignored.Render(path)
	default:
		return path
	}
}
