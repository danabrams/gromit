package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
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
			want := []string{"ready", "--json", "--sort", "priority", "--limit", "0"}
			if len(args) != len(want) {
				t.Fatalf("run args len = %d, want %d (%v)", len(args), len(want), args)
			}
			for i := range want {
				if args[i] != want[i] {
					t.Fatalf("run args[%d] = %q, want %q", i, args[i], want[i])
				}
			}
			return `[{"id":"task-ready","title":"Ready","priority":1,"issue_type":"task","status":"open"}]`, nil
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

func TestGetReason_FromDependencies(t *testing.T) {
	b := &bead.Bead{
		ID: "b1",
		Dependencies: []bead.Dependency{
			{ID: "dep-a"},
			{ID: "dep-b"},
		},
	}
	got := getReason(b, nil)
	want := "blocked by: dep-a, dep-b"
	if got != want {
		t.Fatalf("getReason() = %q, want %q", got, want)
	}
}

func TestGetReason_FromDependencyCount(t *testing.T) {
	count := 3
	b := &bead.Bead{ID: "b1", DependencyCount: &count}
	got := getReason(b, nil)
	want := "blocked by 3 dependencies"
	if got != want {
		t.Fatalf("getReason() = %q, want %q", got, want)
	}
}

func TestFindStuckBeadIDs(t *testing.T) {
	stats := map[string]logger.BeadStats{
		"a": {BeadID: "a", Failures: 1},
		"b": {BeadID: "b", Failures: 3},
		"c": {BeadID: "c", Failures: 4},
	}

	stuck := findStuckBeadIDs(stats, 3)
	if len(stuck) != 2 {
		t.Fatalf("len(stuck) = %d, want 2", len(stuck))
	}
	if !stuck["b"] || !stuck["c"] {
		t.Fatalf("stuck map missing expected IDs: %v", stuck)
	}
	if stuck["a"] {
		t.Fatalf("stuck map should not include a: %v", stuck)
	}
}

func TestPartitionQueueBeads_SeparatesReadyBlockedAndStuck(t *testing.T) {
	readyInput := []*bead.Bead{
		{ID: "ready-1", Priority: 1, Title: "Ready 1"},
		{ID: "stuck-ready", Priority: 0, Title: "Stuck But Ready"},
	}
	all := []*bead.Bead{
		{ID: "stuck-ready", Priority: 0, Title: "Stuck But Ready"},
		{ID: "ready-1", Priority: 1, Title: "Ready 1"},
		{ID: "blocked-1", Priority: 2, Title: "Blocked 1"},
		{ID: "stuck-blocked", Priority: 2, Title: "Stuck Blocked"},
	}
	stats := map[string]logger.BeadStats{
		"stuck-ready":   {BeadID: "stuck-ready", Failures: 3},
		"stuck-blocked": {BeadID: "stuck-blocked", Failures: 4},
	}

	ready, blocked, stuck := partitionQueueBeads(readyInput, all, stats, 3)

	if len(ready) != 1 || ready[0].ID != "ready-1" {
		t.Fatalf("ready = %+v, want [ready-1]", ready)
	}
	if len(blocked) != 1 || blocked[0].ID != "blocked-1" {
		t.Fatalf("blocked = %+v, want [blocked-1]", blocked)
	}
	if len(stuck) != 2 || stuck[0].ID != "stuck-ready" || stuck[1].ID != "stuck-blocked" {
		t.Fatalf("stuck = %+v, want [stuck-ready stuck-blocked]", stuck)
	}
}

func TestEnrichReadyBeads_MergesLabelsFromOpenList(t *testing.T) {
	ready := []*bead.Bead{
		{ID: "r1", Labels: []string{"tdd:true"}},
		{ID: "r2", Labels: nil},
	}
	open := []*bead.Bead{
		{ID: "r1", Labels: []string{"spec:alpha", "backend"}},
		{ID: "r2", Labels: []string{"spec:beta"}},
	}

	enriched := enrichReadyBeads(ready, open)
	if bead.FindSpecLabel(enriched[0].Labels) != "alpha" {
		t.Fatalf("r1 spec = %q, want alpha (labels=%v)", bead.FindSpecLabel(enriched[0].Labels), enriched[0].Labels)
	}
	if bead.FindSpecLabel(enriched[1].Labels) != "beta" {
		t.Fatalf("r2 spec = %q, want beta (labels=%v)", bead.FindSpecLabel(enriched[1].Labels), enriched[1].Labels)
	}
}
