package components

import (
	"fmt"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/engine"
	"github.com/javanhut/Ivaldi-vcs/internal/tui/style"
)

// DiffView renders a scrollable diff with syntax highlighting
type DiffView struct {
	lines  []diffLine
	offset int
	width  int
	height int
}

// diffLine is a pre-rendered diff line with a type for coloring
type diffLine struct {
	lineType engine.DiffLineType
	text     string
	isHeader bool
	isSep    bool
}

// NewDiffView creates a new diff view
func NewDiffView() DiffView {
	return DiffView{
		height: 20,
	}
}

// SetSize updates the display size
func (d *DiffView) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.fixScroll()
}

// SetContent builds the rendered lines from file diffs
func (d *DiffView) SetContent(files []engine.FileDiff, theme style.Theme) {
	d.lines = nil
	d.offset = 0

	for i, file := range files {
		// File header
		var header string
		switch file.Type {
		case engine.DiffAdded:
			header = fmt.Sprintf("+++ %s (new file)", file.Path)
		case engine.DiffRemoved:
			header = fmt.Sprintf("--- %s (deleted)", file.Path)
		case engine.DiffModified:
			header = fmt.Sprintf("--- a/%s\n+++ b/%s", file.Path, file.Path)
		}
		for _, h := range strings.Split(header, "\n") {
			d.lines = append(d.lines, diffLine{isHeader: true, text: h})
		}

		if file.IsBinary {
			d.lines = append(d.lines, diffLine{text: "  Binary file differs"})
		} else {
			// Hunks
			for _, hunk := range file.Hunks {
				hunkHeader := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
					hunk.OldStart, hunk.OldCount,
					hunk.NewStart, hunk.NewCount)
				d.lines = append(d.lines, diffLine{isHeader: true, text: hunkHeader})

				for _, line := range hunk.Lines {
					var prefix string
					switch line.Type {
					case engine.DiffLineContext:
						prefix = " "
					case engine.DiffLineAdd:
						prefix = "+"
					case engine.DiffLineRemove:
						prefix = "-"
					}
					d.lines = append(d.lines, diffLine{
						lineType: line.Type,
						text:     prefix + line.Content,
					})
				}
			}
		}

		// Stats line
		stats := fmt.Sprintf("  +%d -%d", file.AddedLines, file.RemovedLines)
		d.lines = append(d.lines, diffLine{text: stats})

		// Separator between files
		if i < len(files)-1 {
			d.lines = append(d.lines, diffLine{isSep: true, text: ""})
		}
	}
}

// ScrollUp scrolls up by one line
func (d *DiffView) ScrollUp() {
	if d.offset > 0 {
		d.offset--
	}
}

// ScrollDown scrolls down by one line
func (d *DiffView) ScrollDown() {
	max := len(d.lines) - d.height
	if max < 0 {
		max = 0
	}
	if d.offset < max {
		d.offset++
	}
}

// PageUp scrolls up by a page
func (d *DiffView) PageUp() {
	d.offset -= d.height
	if d.offset < 0 {
		d.offset = 0
	}
}

// PageDown scrolls down by a page
func (d *DiffView) PageDown() {
	max := len(d.lines) - d.height
	if max < 0 {
		max = 0
	}
	d.offset += d.height
	if d.offset > max {
		d.offset = max
	}
}

// ScrollToTop scrolls to the beginning
func (d *DiffView) ScrollToTop() {
	d.offset = 0
}

// ScrollToBottom scrolls to the end
func (d *DiffView) ScrollToBottom() {
	max := len(d.lines) - d.height
	if max < 0 {
		max = 0
	}
	d.offset = max
}

// NextFile jumps to the next file header
func (d *DiffView) NextFile() {
	for i := d.offset + 1; i < len(d.lines); i++ {
		if d.lines[i].isHeader && (i == 0 || d.lines[i-1].isSep || i == d.offset+1) {
			// Found the start of a file section
			if strings.HasPrefix(d.lines[i].text, "---") || strings.HasPrefix(d.lines[i].text, "+++") {
				d.offset = i
				d.fixScroll()
				return
			}
		}
	}
}

// PrevFile jumps to the previous file header
func (d *DiffView) PrevFile() {
	for i := d.offset - 1; i >= 0; i-- {
		if d.lines[i].isHeader {
			if strings.HasPrefix(d.lines[i].text, "---") || strings.HasPrefix(d.lines[i].text, "+++") {
				d.offset = i
				d.fixScroll()
				return
			}
		}
	}
}

// TotalLines returns the total number of lines
func (d *DiffView) TotalLines() int {
	return len(d.lines)
}

// Offset returns the current scroll offset
func (d *DiffView) Offset() int {
	return d.offset
}

// fixScroll ensures the offset is within bounds
func (d *DiffView) fixScroll() {
	max := len(d.lines) - d.height
	if max < 0 {
		max = 0
	}
	if d.offset > max {
		d.offset = max
	}
	if d.offset < 0 {
		d.offset = 0
	}
}

// View renders the visible portion of the diff
func (d DiffView) View(theme style.Theme) string {
	if len(d.lines) == 0 {
		return theme.Dim.Render("  No differences")
	}

	var b strings.Builder
	end := d.offset + d.height
	if end > len(d.lines) {
		end = len(d.lines)
	}

	for i := d.offset; i < end; i++ {
		line := d.lines[i]

		var rendered string
		switch {
		case line.isSep:
			rendered = ""
		case line.isHeader:
			rendered = theme.Bold.Render(line.text)
		case line.lineType == engine.DiffLineAdd:
			rendered = theme.Added.Render(line.text)
		case line.lineType == engine.DiffLineRemove:
			rendered = theme.Deleted.Render(line.text)
		default:
			rendered = line.text
		}

		b.WriteString("  ")
		b.WriteString(rendered)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
