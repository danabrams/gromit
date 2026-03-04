package tui

// ListModel tracks a selectable list of items with cursor state.
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
	l.items = append([]ListItem{}, items...)
	if len(l.items) == 0 {
		l.cursor = 0
		l.scrollOffset = 0
		return
	}
	l.scrollOffset = 0
	if l.cursor >= len(l.items) {
		l.cursor = len(l.items) - 1
	}
	if l.scrollOffset < 0 {
		l.scrollOffset = 0
	}
}
