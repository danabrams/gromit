package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
)

func TestRenderDashboardViewIncludesSections(t *testing.T) {
	now := time.Date(2026, time.February, 20, 12, 0, 0, 0, time.UTC)
	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 2,
				MaxIterations:    5,
				IterationPercent: 40,
				Status:           "running",
			},
			RecentCompletions: []*Completion{
				{
					BeadID:    "alpha",
					BeadTitle: "Alpha",
					Status:    "completed",
					Time:      now,
				},
			},
			HealthIndicator: &HealthIndicator{
				LastEventType:   "phase-change",
				LastEventTime:   now,
				IsHealthy:       true,
				HasStalledBeads: false,
			},
		},
		Queue: QueueState{
			Snapshot: &QueueSnapshot{
				Ready:   []*bead.Bead{{ID: "ready-1", Title: "Ready One"}},
				Blocked: []*bead.Bead{{ID: "blocked-1", Title: "Blocked One"}},
				Stuck:   []*bead.Bead{{ID: "stuck-1", Title: "Stuck One"}},
			},
		},
	}

	got := RenderDashboardView(store, 0)

	expected := []string{
		"Iteration 2/5",
		"Queue depth: ready=1 blocked=1 stuck=1",
		"[completed] Alpha",
		"Health: healthy",
		"Last event: phase-change",
	}

	for _, substr := range expected {
		if !strings.Contains(got, substr) {
			t.Fatalf("expected %q in view output, got %q", substr, got)
		}
	}
}
