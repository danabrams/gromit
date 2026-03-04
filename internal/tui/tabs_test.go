package tui

import (
	"strings"
	"testing"
)

func TestTabBarHighlightsActiveTab(t *testing.T) {
	active := TabSpecs
	bar := TabBar(active, 80)

	expectedActive := tabBarActiveStyle.Render(tabLabel(active))
	if !strings.Contains(bar, expectedActive) {
		t.Fatalf("TabBar did not highlight %s: %q", active, bar)
	}

	for _, tab := range tabOrder {
		if !strings.Contains(bar, tabLabel(tab)) {
			t.Fatalf("TabBar missing tab %s", tab)
		}
	}
}

func TestTabEntriesFallbacksToFirstTab(t *testing.T) {
	entries := tabEntries("invalid-tab")
	if len(entries) == 0 {
		t.Fatalf("expected at least one tab entry, got none")
	}

	if !entries[0].active || entries[0].tab != TabBacklog {
		t.Fatalf("expected first tab entry to be %+v with active=true, got %+v", TabBacklog, entries[0])
	}
}
