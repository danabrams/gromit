package queue

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
)

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
