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
