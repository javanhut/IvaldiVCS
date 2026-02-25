package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// DialogType indicates which dialog is open
type DialogType int

const (
	DialogNone DialogType = iota
	DialogSeal
	DialogPortal
)

// DialogSubmitMsg is sent when the user submits the dialog
type DialogSubmitMsg struct {
	Type  DialogType
	Value string
}

// DialogCancelMsg is sent when the user cancels the dialog
type DialogCancelMsg struct {
	Type DialogType
}

// Dialog is a text input overlay for single-line prompts
type Dialog struct {
	Type      DialogType
	Title     string
	textInput textinput.Model
	active    bool
}

// NewDialog creates a new dialog component
func NewDialog() Dialog {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 50
	return Dialog{
		textInput: ti,
	}
}

// Open opens the dialog with a title and placeholder
func (d *Dialog) Open(dialogType DialogType, title, placeholder string) tea.Cmd {
	d.Type = dialogType
	d.Title = title
	d.active = true
	d.textInput.Placeholder = placeholder
	d.textInput.SetValue("")
	d.textInput.Focus()
	return textinput.Blink
}

// Close closes the dialog
func (d *Dialog) Close() {
	d.active = false
	d.textInput.Blur()
}

// IsActive returns whether the dialog is open
func (d *Dialog) IsActive() bool {
	return d.active
}

// Update handles key events for the dialog. Returns a command and whether the
// event was consumed by the dialog.
func (d *Dialog) Update(msg tea.KeyMsg) (tea.Cmd, bool) {
	if !d.active {
		return nil, false
	}

	switch msg.String() {
	case "esc":
		dialogType := d.Type
		d.Close()
		return func() tea.Msg {
			return DialogCancelMsg{Type: dialogType}
		}, true

	case "enter":
		value := strings.TrimSpace(d.textInput.Value())
		dialogType := d.Type
		d.Close()
		if value == "" {
			return func() tea.Msg {
				return DialogCancelMsg{Type: dialogType}
			}, true
		}
		return func() tea.Msg {
			return DialogSubmitMsg{Type: dialogType, Value: value}
		}, true
	}

	var cmd tea.Cmd
	d.textInput, cmd = d.textInput.Update(msg)
	return cmd, true
}

// View renders the dialog
func (d Dialog) View(theme style.Theme) string {
	if !d.active {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(theme.Title.Render(d.Title))
	b.WriteString("\n  ")
	b.WriteString(d.textInput.View())
	b.WriteString("\n")
	b.WriteString(theme.Dim.Render("  enter: confirm  esc: cancel"))
	return b.String()
}
