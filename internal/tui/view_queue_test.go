package tui

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/queue"
)

func TestRenderQueueViewShowsReadyBlockedAndStuck(t *testing.T) {
	ready := &bead.Bead{ID: "ready-1", Title: "Ready One"}
	blocked := &bead.Bead{ID: "blocked-1", Title: "Blocked One", Parent: "parent-1"}
	stuck := &bead.Bead{ID: "stuck-1", Title: "Stuck One"}

	store := &Store{
		Queue: QueueState{
			Snapshot: &QueueSnapshot{
				Ready:   []*bead.Bead{ready},
				Blocked: []*bead.Bead{blocked},
				Stuck:   []*bead.Bead{stuck},
				All:     []*bead.Bead{ready, blocked, stuck},
			},
		},
	}

	got := RenderQueueView(store, 0)

	expected := []string{
		"Ready One",
		"Blocked One",
		queue.GetReason(blocked, store.Queue.Snapshot.All),
		"Stuck One",
	}

	for _, substr := range expected {
		if !strings.Contains(got, substr) {
			t.Fatalf("expected %q in queue view, got %q", substr, got)
		}
	}
}
