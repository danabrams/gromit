package tui

import (
	"strings"
	"testing"
)

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

func TestListModelRenderShowsCursorAndScroll(t *testing.T) {
	model := &ListModel{viewHeight: 2}
	items := []ListItem{
		&testListItem{title: "first", summary: "alpha"},
		&testListItem{title: "second", summary: "bravo"},
		&testListItem{title: "third", summary: "charlie"},
	}

	model.SetItems(items)
	model.MoveDown()
	model.MoveDown()

	if got, want := model.scrollOffset, 1; got != want {
		t.Fatalf("scrollOffset = %d, want %d", got, want)
	}

	rendered := model.Render(40)
	lines := strings.Split(rendered, "\n")
	if got, want := len(lines), 2; got != want {
		t.Fatalf("rendered lines = %d, want %d", got, want)
	}
	if got := lines[0]; !strings.HasPrefix(got, "> ") || !strings.Contains(got, "third") {
		t.Fatalf("expected cursor line containing third item, got %q", got)
	}
	if got := lines[1]; !strings.HasPrefix(got, "  ") || !strings.Contains(got, "charlie") {
		t.Fatalf("expected following line containing charlie summary, got %q", got)
	}
}
