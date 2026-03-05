package tui

import (
	"reflect"
	"strings"
	"testing"
)

func TestPerTabNormalHintActions(t *testing.T) {
	testCases := []struct {
		name string
		tab  Tab
		want []string
	}{
		{name: "backlog", tab: TabBacklog, want: []string{"[r] refine", "[v] view", "[x] delete"}},
		{name: "specs", tab: TabSpecs, want: []string{"[p] plan", "[v] view", "[x] delete"}},
		{name: "plans", tab: TabPlans, want: []string{"[d] decompose", "[v] view", "[x] delete"}},
		{name: "queue", tab: TabQueue, want: []string{"[v] view"}},
		{name: "runloop", tab: TabRunLoop, want: []string{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := perTabNormalHintActions(tc.tab); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("perTabNormalHintActions(%q) = %v, want %v", tc.tab, got, tc.want)
			}
		})
	}
}

func TestRenderHintBarNormalState(t *testing.T) {
	testCases := []struct {
		name string
		tab  Tab
		want []string
	}{
		{
			name: "backlog",
			tab:  TabBacklog,
			want: []string{"[r] refine", "[v] view", "[x] delete", "[q] quit"},
		},
		{
			name: "specs",
			tab:  TabSpecs,
			want: []string{"[p] plan", "[v] view", "[x] delete", "[q] quit"},
		},
		{
			name: "plans",
			tab:  TabPlans,
			want: []string{"[d] decompose", "[v] view", "[x] delete", "[q] quit"},
		},
		{
			name: "queue",
			tab:  TabQueue,
			want: []string{"[v] view", "[q] quit"},
		},
		{
			name: "runloop",
			tab:  TabRunLoop,
			want: []string{"[q] quit"},
		},
	}

	for _, tc := range testCases {
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

func TestRenderHintBarHidesSelectionActionsWithoutSelection(t *testing.T) {
	testCases := []struct {
		name     string
		tab      Tab
		excludes []string
	}{
		{name: "backlog", tab: TabBacklog, excludes: []string{"[r] refine", "[v] view", "[x] delete"}},
		{name: "specs", tab: TabSpecs, excludes: []string{"[p] plan", "[v] view", "[x] delete"}},
		{name: "plans", tab: TabPlans, excludes: []string{"[d] decompose", "[v] view", "[x] delete"}},
		{name: "queue", tab: TabQueue, excludes: []string{"[v] view"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hint := RenderHintBar(tc.tab, false, false, false)
			if !strings.Contains(hint, "[q] quit") {
				t.Fatalf("RenderHintBar(%q, no selection) missing quit hint: %q", tc.tab, hint)
			}
			for _, exclude := range tc.excludes {
				if strings.Contains(hint, exclude) {
					t.Fatalf("RenderHintBar(%q, no selection) should not include %q: %q", tc.tab, exclude, hint)
				}
			}
		})
	}
}

func TestRenderHintBarUnknownTabFallsBackToQuit(t *testing.T) {
	if hint := RenderHintBar("mystery", true, false, false); !strings.Contains(hint, "[q] quit") {
		t.Fatalf("RenderHintBar(mystery) missing quit hint: %q", hint)
	}
}

func TestRenderHintBarNormalStateMatchesPerTabStrings(t *testing.T) {
	testTabs := []Tab{TabBacklog, TabSpecs, TabPlans, TabQueue, TabRunLoop}
	for _, tab := range testTabs {
		t.Run(string(tab), func(t *testing.T) {
			want, ok := normalStateHintStrings[tab]
			if !ok {
				t.Fatalf("missing normal hint string for %q", tab)
			}
			if got := RenderHintBar(tab, true, false, false); got != want {
				t.Fatalf("RenderHintBar(%q) = %q, want %q", tab, got, want)
			}
		})
	}
}

func TestStyledNormalHintStringMatchesActions(t *testing.T) {
	testTabs := []Tab{TabBacklog, TabSpecs, TabPlans, TabQueue, TabRunLoop}
	for _, tab := range testTabs {
		t.Run(string(tab), func(t *testing.T) {
			want := renderHintActions(normalHintActions(tab, true))
			if got := styledNormalHintString(tab); got != want {
				t.Fatalf("styledNormalHintString(%q) = %q, want %q", tab, got, want)
			}
		})
	}
}

func TestPerTabNormalHintStringsMatchRenderHintBar(t *testing.T) {
	for _, tab := range hintTabs() {
		t.Run(string(tab), func(t *testing.T) {
			got, ok := perTabNormalHintStrings[tab]
			if !ok {
				t.Fatalf("missing per-tab normal hint string for %q", tab)
			}
			if want := RenderHintBar(tab, true, false, false); got != want {
				t.Fatalf("perTabNormalHintStrings[%q] = %q, want %q", tab, got, want)
			}
		})
	}
}
