package tui

import (
	"strings"
	"testing"
)

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
