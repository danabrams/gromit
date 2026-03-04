package tui

import "testing"

type testListItem struct {
	title   string
	summary string
}

func (t *testListItem) Title() string { return t.title }

func (t *testListItem) Summary() string { return t.summary }

func TestListModelSetItemsClampsCursor(t *testing.T) {
	model := &ListModel{
		cursor:       5,
		scrollOffset: 3,
	}

	items := []ListItem{
		&testListItem{title: "one"},
		&testListItem{title: "two"},
	}

	model.SetItems(items)

	if got, want := model.cursor, len(items)-1; got != want {
		t.Fatalf("cursor = %d, want %d", got, want)
	}
	if got := model.scrollOffset; got != 0 {
		t.Fatalf("scrollOffset = %d, want 0", got)
	}
}
