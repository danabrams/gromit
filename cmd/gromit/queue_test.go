package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestGroupBeadsBySpec(t *testing.T) {
	beads := []*bead.Bead{
		{ID: "a", Labels: []string{"spec:auth"}},
		{ID: "b", Labels: []string{"priority:p1"}},
		{ID: "c", Labels: []string{"spec:api"}},
		{ID: "d", Labels: []string{"spec:auth"}},
	}

	grouped := groupBeadsBySpec(beads)

	if len(grouped["auth"]) != 2 {
		t.Fatalf("grouped[auth] count = %d, want 2", len(grouped["auth"]))
	}
	if len(grouped["api"]) != 1 {
		t.Fatalf("grouped[api] count = %d, want 1", len(grouped["api"]))
	}
	if len(grouped[""]) != 1 {
		t.Fatalf("grouped[(none)] count = %d, want 1", len(grouped[""]))
	}
}

func TestOrderedSpecKeys_PutsNoneLast(t *testing.T) {
	grouped := map[string][]*bead.Bead{
		"":     []*bead.Bead{{ID: "none"}},
		"auth": []*bead.Bead{{ID: "auth"}},
		"api":  []*bead.Bead{{ID: "api"}},
	}

	keys := orderedSpecKeys(grouped)
	if len(keys) != 3 {
		t.Fatalf("len(keys) = %d, want 3", len(keys))
	}
	if keys[0] != "api" || keys[1] != "auth" || keys[2] != "" {
		t.Fatalf("keys = %v, want [api auth \"\"]", keys)
	}
}

func TestColorizeLine(t *testing.T) {
	line := "hello"
	colored := colorizeLine(line, ansiGreen, true)
	if colored != ansiGreen+line+ansiReset {
		t.Fatalf("colored = %q", colored)
	}

	plain := colorizeLine(line, ansiGreen, false)
	if plain != line {
		t.Fatalf("plain = %q, want %q", plain, line)
	}
}

func TestGetReadyBeads_UsesReadyStatusFromBD(t *testing.T) {
	c := &bead.Client{
		RunFn: func(args ...string) (string, error) {
			want := []string{"list", "--json", "--status", "ready", "--sort", "priority", "--limit", "0"}
			if len(args) != len(want) {
				t.Fatalf("run args len = %d, want %d (%v)", len(args), len(want), args)
			}
			for i := range want {
				if args[i] != want[i] {
					t.Fatalf("run args[%d] = %q, want %q", i, args[i], want[i])
				}
			}
			return `[{"id":"task-ready","title":"Ready","priority":1,"issue_type":"task","status":"ready"}]`, nil
		},
	}

	ready, err := getReadyBeads(c)
	if err != nil {
		t.Fatalf("getReadyBeads() error = %v", err)
	}
	if len(ready) != 1 {
		t.Fatalf("len(ready) = %d, want 1", len(ready))
	}
	if ready[0].ID != "task-ready" {
		t.Fatalf("ready[0].ID = %q, want task-ready", ready[0].ID)
	}
}
