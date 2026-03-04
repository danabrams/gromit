package tui

import (
	"strings"
	"testing"
)

func TestRenderHintBarNormalState(t *testing.T) {
	for _, tc := range normalHintTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			hint := RenderHintBar(tc.tab, true, false, false)
			for _, want := range tc.want {
				if !strings.Contains(hint, want) {
					t.Fatalf("RenderHintBar(%q) missing %q in %q", tc.tab, want, hint)
				}
			}
		})
	}
}

func TestRenderHintBarDetailViewOverride(t *testing.T) {
	testCases := []struct {
		name string
		tab  Tab
		want []string
	}{
		{name: "backlog", tab: TabBacklog, want: []string{"[q] quit", "[esc] back"}},
		{name: "specs", tab: TabSpecs, want: []string{"[q] quit", "[esc] back"}},
		{name: "plans", tab: TabPlans, want: []string{"[q] quit", "[esc] back"}},
		{name: "queue", tab: TabQueue, want: []string{"[q] quit", "[esc] back"}},
		{name: "runloop", tab: TabRunLoop, want: []string{"[q] quit"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hint := RenderHintBar(tc.tab, true, true, false)
			for _, want := range tc.want {
				if !strings.Contains(hint, want) {
					t.Fatalf("RenderHintBar(%q, detailView) missing %q in %q", tc.tab, want, hint)
				}
			}
		})
	}
}

func TestRenderHintBarConfirmationOverride(t *testing.T) {
	testCases := []struct {
		name string
		tab  Tab
	}{
		{name: "backlog", tab: TabBacklog},
		{name: "specs", tab: TabSpecs},
		{name: "plans", tab: TabPlans},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hint := RenderHintBar(tc.tab, true, false, true)
			if !strings.Contains(hint, "[y/n] confirm delete") {
				t.Fatalf("RenderHintBar(%q, confirm) missing confirmation text, got %q", tc.tab, hint)
			}
		})
	}
}
