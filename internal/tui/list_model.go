package tui

import (
	"strings"
	"unicode/utf8"
)

// ListModel tracks a selectable list of items with cursor and scroll state.
type ListModel struct {
	items        []ListItem
	cursor       int
	scrollOffset int
	viewHeight   int
}

// SetItems replaces the list contents and clamps cursor/scroll state.
func (l *ListModel) SetItems(items []ListItem) {
	if l == nil {
		return
	}
	prevScroll := l.scrollOffset
	l.items = append([]ListItem{}, items...)
	if len(l.items) == 0 {
		l.cursor = 0
		l.scrollOffset = 0
		return
	}
	if l.cursor >= len(l.items) {
		l.cursor = len(l.items) - 1
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
	l.scrollOffset = prevScroll
	l.ensureCursorVisible()
	maxOffset := max(0, len(l.items)-l.viewHeight)
	l.scrollOffset = clamp(l.scrollOffset, 0, maxOffset)
}

// MoveUp moves the cursor up by one row and adjusts scroll offset.
func (l *ListModel) MoveUp() {
	if l == nil {
		return
	}
	if len(l.items) == 0 {
		l.cursor = 0
		return
	}

	if l.cursor <= 0 {
		l.cursor = 0
	} else if l.cursor >= len(l.items) {
		l.cursor = len(l.items) - 1
	} else {
		l.cursor--
	}

	l.ensureCursorVisible()
}

// MoveDown moves the cursor down by one row and adjusts scroll offset.
func (l *ListModel) MoveDown() {
	if l == nil {
		return
	}
	if len(l.items) == 0 {
		l.cursor = 0
		return
	}

	if l.cursor >= len(l.items)-1 {
		l.cursor = len(l.items) - 1
	} else if l.cursor < 0 {
		l.cursor = 0
	} else {
		l.cursor++
	}

	l.ensureCursorVisible()
}

// Selected returns the currently selected item, if any.
func (l *ListModel) Selected() ListItem {
	if l == nil {
		return nil
	}
	if len(l.items) == 0 {
		l.cursor = 0
		return nil
	}
	if l.cursor < 0 {
		l.cursor = 0
	} else if l.cursor >= len(l.items) {
		l.cursor = len(l.items) - 1
	}
	return l.items[l.cursor]
}

// Render returns a line-per-item representation with cursor marker.
func (l *ListModel) Render(width int) string {
	if l == nil || len(l.items) == 0 {
		return ""
	}

	start := 0
	end := len(l.items)
	if l.viewHeight > 0 {
		start = clamp(l.scrollOffset, 0, len(l.items))
		end = clamp(start+l.viewHeight, start, len(l.items))
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			b.WriteRune('\n')
		}
		prefix := "  "
		if i == l.cursor {
			prefix = "> "
		}
		line := prefix + l.renderLine(l.items[i], width)
		b.WriteString(line)
	}

	return b.String()
}

func (l *ListModel) renderLine(item ListItem, width int) string {
	parts := []string{}
	if trimmed := strings.TrimSpace(item.Title()); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if summary := strings.TrimSpace(item.Summary()); summary != "" {
		parts = append(parts, summary)
	}
	line := strings.Join(parts, " · ")
	if line == "" {
		return ""
	}
	if width > 0 {
		line = truncate(line, width)
	}
	return line
}

func (l *ListModel) ensureCursorVisible() {
	if l == nil || l.viewHeight <= 0 {
		l.scrollOffset = 0
		return
	}
	if l.cursor < l.scrollOffset {
		l.scrollOffset = l.cursor
		return
	}
	maxVisible := l.scrollOffset + l.viewHeight - 1
	if maxVisible < 0 {
		maxVisible = 0
	}
	if l.cursor > maxVisible {
		l.scrollOffset = l.cursor - l.viewHeight + 1
	}
	l.scrollOffset = clamp(l.scrollOffset, 0, max(0, len(l.items)-l.viewHeight))
}

func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= width {
		return s
	}
	var b strings.Builder
	b.Grow(width)
	count := 0
	for _, r := range s {
		if count >= width {
			break
		}
		b.WriteRune(r)
		count++
	}
	return b.String()
}

func clamp(value, minVal, maxVal int) int {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
