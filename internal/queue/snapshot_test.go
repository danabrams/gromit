package queue

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
)

func TestFindStuckBeadIDs(t *testing.T) {
	t.Parallel()
	stats := map[string]logger.BeadStats{
		"a": {BeadID: "a", Failures: 1},
		"b": {BeadID: "b", Failures: 3},
		"c": {BeadID: "c", Failures: 4},
	}

	stuck := FindStuckBeadIDs(stats, 3)
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
	t.Parallel()
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

	ready, blocked, stuck := PartitionQueueBeads(readyInput, all, stats, 3)

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

func TestPartitionQueueBeads_DoesNotClassifyInProgressAsBlocked(t *testing.T) {
	t.Parallel()
	readyInput := []*bead.Bead{}
	all := []*bead.Bead{
		{ID: "progress-1", Status: "in_progress", Priority: 1, Title: "Active Work"},
	}

	ready, blocked, stuck := PartitionQueueBeads(readyInput, all, map[string]logger.BeadStats{}, 3)

	if len(ready) != 0 {
		t.Fatalf("ready = %+v, want empty", ready)
	}
	if len(blocked) != 0 {
		t.Fatalf("blocked = %+v, want empty", blocked)
	}
	if len(stuck) != 0 {
		t.Fatalf("stuck = %+v, want empty", stuck)
	}
}

func TestGetReason_FromDependencies(t *testing.T) {
	t.Parallel()
	b := &bead.Bead{
		ID: "b1",
		Dependencies: []bead.Dependency{
			{ID: "dep-a"},
			{ID: "dep-b"},
		},
	}
	got := GetReason(b, nil)
	want := "blocked by: dep-a, dep-b"
	if got != want {
		t.Fatalf("GetReason() = %q, want %q", got, want)
	}
}

func TestDependencyIDs_TrimsAndSkipsEmpty(t *testing.T) {
	t.Parallel()
	input := []bead.Dependency{
		{ID: "dep-1"},
		{ID: "  "},
		{ID: "dep-2"},
		{ID: ""},
		{ID: "dep-3"},
	}
	if got := DependencyIDs(input); len(got) != 3 || got[0] != "dep-1" || got[1] != "dep-2" || got[2] != "dep-3" {
		t.Fatalf("DependencyIDs() = %v", got)
	}
}

func TestGetReason_FromDependencyCount(t *testing.T) {
	t.Parallel()
	count := 3
	b := &bead.Bead{ID: "b1", DependencyCount: &count}
	got := GetReason(b, nil)
	want := "blocked by 3 dependencies"
	if got != want {
		t.Fatalf("GetReason() = %q, want %q", got, want)
	}
}

func TestEnrichReadyBeads_MergesLabelsFromOpenList(t *testing.T) {
	t.Parallel()
	ready := []*bead.Bead{
		{ID: "r1", Labels: []string{"tdd:true"}},
		{ID: "r2", Labels: nil},
	}
	open := []*bead.Bead{
		{ID: "r1", Labels: []string{"spec:alpha", "backend"}},
		{ID: "r2", Labels: []string{"spec:beta"}},
	}

	enriched := EnrichReadyBeads(ready, open)
	if bead.FindSpecLabel(enriched[0].Labels) != "alpha" {
		t.Fatalf("r1 spec = %q, want alpha (labels=%v)", bead.FindSpecLabel(enriched[0].Labels), enriched[0].Labels)
	}
	if bead.FindSpecLabel(enriched[1].Labels) != "beta" {
		t.Fatalf("r2 spec = %q, want beta (labels=%v)", bead.FindSpecLabel(enriched[1].Labels), enriched[1].Labels)
	}
}
