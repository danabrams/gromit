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
