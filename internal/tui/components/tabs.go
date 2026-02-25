package components

import (
	"fmt"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// TabBar represents a horizontal tab bar
type TabBar struct {
	Labels []string
	Active int
	Width  int
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

	// Brand prefix
	tabs = append(tabs, theme.Brand.Render("Ivaldi"))
	tabs = append(tabs, theme.TabSeparator.Render("│"))

	for i, label := range t.Labels {
		display := fmt.Sprintf("%d %s", i+1, label)
		if i == t.Active {
			tabs = append(tabs, theme.ActiveTab.Render(display))
		} else {
			tabs = append(tabs, theme.InactiveTab.Render(display))
		}
		if i < len(t.Labels)-1 {
			tabs = append(tabs, theme.TabSeparator.Render("│"))
		}
	}
	row := strings.Join(tabs, "")
	return theme.TabBar.Render(row)
}
