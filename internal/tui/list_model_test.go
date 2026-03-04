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

func TestListModelMoveUpClampsCursor(t *testing.T) {
	model := &ListModel{
		items: []ListItem{
			&testListItem{title: "first"},
			&testListItem{title: "second"},
		},
		cursor: -5,
	}

    model.MoveUp()

	if got, want := model.cursor, 0; got != want {
		t.Fatalf("cursor = %d, want %d", got, want)
	}
}

func TestListModelMoveDownClampsCursor(t *testing.T) {
	model := &ListModel{
		items: []ListItem{
			&testListItem{title: "first"},
			&testListItem{title: "second"},
		},
		cursor: 5,
	}

	model.MoveDown()

	if got, want := model.cursor, len(model.items)-1; got != want {
		t.Fatalf("cursor = %d, want %d", got, want)
	}
}

func TestListModelSelectedClampsCursor(t *testing.T) {
	items := []ListItem{
		&testListItem{title: "alpha"},
		&testListItem{title: "omega"},
	}
	model := &ListModel{
		items:  items,
		cursor: len(items) + 2,
	}

	selected := model.Selected()
	if selected == nil {
		t.Fatal("expected selected item, got nil")
	}
	if got, want := selected.Title(), "omega"; got != want {
		t.Fatalf("selected title = %q, want %q", got, want)
	}
	if got, want := model.cursor, len(items)-1; got != want {
		t.Fatalf("cursor = %d, want %d", got, want)
	}
}

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

	cursorLines := []string{}
	for _, line := range lines {
		if strings.HasPrefix(line, "> ") {
			cursorLines = append(cursorLines, line)
		}
	}
	if len(cursorLines) != 1 {
		t.Fatalf("expected 1 cursor line, got %d", len(cursorLines))
	}
	if !strings.Contains(cursorLines[0], "third") {
		t.Fatalf("cursor line missing third item, got %q", cursorLines[0])
	}

	otherLine := lines[0]
	if strings.HasPrefix(otherLine, "> ") {
		otherLine = lines[1]
	}
	if !strings.Contains(otherLine, "second") {
		t.Fatalf("non-cursor line missing second item, got %q", otherLine)
	}
}

func TestListModelSetItemsPreservesScrollOffset(t *testing.T) {
	model := &ListModel{
		viewHeight:   2,
		scrollOffset: 1,
		cursor:       1,
	}

	items := []ListItem{
		&testListItem{title: "first"},
		&testListItem{title: "second"},
		&testListItem{title: "third"},
	}

	model.SetItems(items)
	if got, want := model.scrollOffset, 1; got != want {
		t.Fatalf("scrollOffset = %d, want %d", got, want)
	}

	model.scrollOffset = 5
	items = []ListItem{
		&testListItem{title: "one"},
		&testListItem{title: "two"},
	}
	model.SetItems(items)
	if got, want := model.scrollOffset, 0; got != want {
		t.Fatalf("scrollOffset = %d after shrinking, want %d", got, want)
	}
}
