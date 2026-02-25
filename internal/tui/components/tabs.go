package components

import (
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// TabBar represents a horizontal tab bar
type TabBar struct {
	Labels []string
	Active int
}

// NewTabBar creates a new tab bar with the given labels
func NewTabBar(labels []string) TabBar {
	return TabBar{
		Labels: labels,
		Active: 0,
	}
}

// SetActive sets the active tab index
func (t *TabBar) SetActive(index int) {
	if index >= 0 && index < len(t.Labels) {
		t.Active = index
	}
}

// View renders the tab bar
func (t TabBar) View(theme style.Theme) string {
	var tabs []string
	for i, label := range t.Labels {
		prefix := " "
		if i+1 <= 9 {
			prefix = string(rune('0' + i + 1))
		}
		display := prefix + ":" + label
		if i == t.Active {
			tabs = append(tabs, theme.ActiveTab.Render(display))
		} else {
			tabs = append(tabs, theme.InactiveTab.Render(display))
		}
	}
	row := strings.Join(tabs, " ")
	return theme.TabBar.Render(row)
}
