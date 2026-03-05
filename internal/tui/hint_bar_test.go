package tui

import "testing"

func TestNormalHintActions(t *testing.T) {
	testCases := []struct {
		name string
		tab  Tab
		want []string
	}{
		{name: "backlog", tab: TabBacklog, want: []string{"[r] refine", "[v] view", "[x] delete", "[q] quit"}},
		{name: "specs", tab: TabSpecs, want: []string{"[p] plan", "[v] view", "[x] delete", "[q] quit"}},
		{name: "plans", tab: TabPlans, want: []string{"[d] decompose", "[v] view", "[x] delete", "[q] quit"}},
		{name: "queue", tab: TabQueue, want: []string{"[v] view", "[q] quit"}},
		{name: "runloop", tab: TabRunLoop, want: []string{"[q] quit"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalHintActions(tc.tab, true)
			if len(got) != len(tc.want) {
				t.Fatalf("normalHintActions(%q, true) = %v, want %v", tc.tab, got, tc.want)
			}
			for i, action := range tc.want {
				if got[i] != action {
					t.Fatalf("normalHintActions(%q, true)[%d] = %q, want %q", tc.tab, i, got[i], action)
				}
			}
		})
	}
}

func TestDetailViewHintActions(t *testing.T) {
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
			got := detailViewHintActions(tc.tab)
			if len(got) != len(tc.want) {
				t.Fatalf("detailViewHintActions(%q) = %v, want %v", tc.tab, got, tc.want)
			}
			for i, action := range tc.want {
				if got[i] != action {
					t.Fatalf("detailViewHintActions(%q)[%d] = %q, want %q", tc.tab, i, got[i], action)
				}
			}
		})
	}
}

func TestConfirmationHintActions(t *testing.T) {
	testCases := []struct {
		name string
		tab  Tab
		want []string
	}{
		{name: "backlog", tab: TabBacklog, want: []string{"[y/n] confirm delete"}},
		{name: "specs", tab: TabSpecs, want: []string{"[y/n] confirm delete"}},
		{name: "plans", tab: TabPlans, want: []string{"[y/n] confirm delete"}},
		{name: "queue", tab: TabQueue, want: []string{}},
		{name: "runloop", tab: TabRunLoop, want: []string{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := confirmationHintActions(tc.tab)
			if len(got) != len(tc.want) {
				t.Fatalf("confirmationHintActions(%q) = %v, want %v", tc.tab, got, tc.want)
			}
			for i, action := range tc.want {
				if got[i] != action {
					t.Fatalf("confirmationHintActions(%q)[%d] = %q, want %q", tc.tab, i, got[i], action)
				}
			}
		})
	}
}

func TestHintBarDetailViewOverrides(t *testing.T) {
	for tab, want := range hintBarDetailStateStrings {
		t.Run(string(tab), func(t *testing.T) {
			if got := HintBar(tab, true, false); got != want {
				t.Fatalf("HintBar(%q, detail view) = %q, want %q", tab, got, want)
			}
		})
	}
}

func TestHintBarConfirmationOverrides(t *testing.T) {
	tabs := []Tab{TabBacklog, TabSpecs, TabPlans}
	for _, tab := range tabs {
		t.Run(string(tab), func(t *testing.T) {
			got := HintBar(tab, false, true)
			if got != hintBarConfirmationStateStrings[tab] {
				t.Fatalf("HintBar(%q, confirmation) = %q, want %q", tab, got, hintBarConfirmationStateStrings[tab])
			}
		})
	}
}

func TestHintBarNormalStateOutputs(t *testing.T) {
	for tab, want := range hintBarNormalStateStrings {
		t.Run(string(tab), func(t *testing.T) {
			if got := HintBar(tab, false, false); got != want {
				t.Fatalf("HintBar(%q, normal) = %q, want %q", tab, got, want)
			}
		})
	}
}

func TestHintBarNormalStateStrings(t *testing.T) {
	for _, tab := range hintTabs() {
		t.Run(string(tab), func(t *testing.T) {
			want := expectedHintString(tab, hintBarStateNormal)
			if got := HintBar(tab, false, false); got != want {
				t.Fatalf("HintBar(%q, normal) = %q, want %q", tab, got, want)
			}
		})
	}
}

func TestRenderHintBarNormalStateStrings(t *testing.T) {
	for _, tab := range hintTabs() {
		t.Run(string(tab), func(t *testing.T) {
			want, ok := hintBarNormalStateStrings[tab]
			if !ok {
				t.Fatalf("missing normal hint string for %q", tab)
			}
			if got := RenderHintBar(tab, true, false, false); got != want {
				t.Fatalf("RenderHintBar(%q, normal) = %q, want %q", tab, got, want)
			}
		})
	}
}

func TestRenderHintBarDetailStateStrings(t *testing.T) {
	for _, tab := range hintTabs() {
		t.Run(string(tab), func(t *testing.T) {
			want, ok := hintBarDetailStateStrings[tab]
			if !ok {
				t.Fatalf("missing detail hint string for %q", tab)
			}
			if got := RenderHintBar(tab, true, true, false); got != want {
				t.Fatalf("RenderHintBar(%q, detail view) = %q, want %q", tab, got, want)
			}
		})
	}
}

func TestRenderHintBarConfirmationStateStrings(t *testing.T) {
	for _, tab := range hintTabs() {
		t.Run(string(tab), func(t *testing.T) {
			want := expectedConfirmationHintString(tab)
			if got := RenderHintBar(tab, true, false, true); got != want {
				t.Fatalf("RenderHintBar(%q, confirmation) = %q, want %q", tab, got, want)
			}
		})
	}
}

type hintBarState int

const (
	hintBarStateNormal hintBarState = iota
	hintBarStateDetail
	hintBarStateConfirmation
)

func expectedHintString(tab Tab, state hintBarState) string {
	switch state {
	case hintBarStateNormal:
		if hint, ok := hintBarNormalStateStrings[tab]; ok {
			return hint
		}
	}
	return ""
}
