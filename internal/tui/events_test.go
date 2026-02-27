package tui

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
)

func TestOnRunStart_UpdatesRunProgress(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.RunStartEvent{
		MaxIterations: 5,
		TimeBudget:    10 * time.Minute,
		DryRun:        false,
		Time:          time.Unix(1000, 0),
	}

	store.OnRunStart(event)

	if store.Dashboard.RunProgress == nil {
		t.Fatalf("expected RunProgress to be set")
	}
	if store.Dashboard.RunProgress.MaxIterations != 5 {
		t.Fatalf("MaxIterations = %d, want 5", store.Dashboard.RunProgress.MaxIterations)
	}
	if store.Dashboard.RunProgress.CurrentIteration != 0 {
		t.Fatalf("CurrentIteration = %d, want 0", store.Dashboard.RunProgress.CurrentIteration)
	}
	if store.Dashboard.RunProgress.Status != "running" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RunProgress.Status, "running")
	}
}

func TestOnRunComplete_UpdatesRunProgress(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 3,
				MaxIterations:    5,
				Status:           "running",
			},
		},
	}
	event := &events.RunCompleteEvent{
		IterationsCompleted: 3,
		Reason:              "success",
		Time:                time.Unix(2000, 0),
	}

	store.OnRunComplete(event)

	if store.Dashboard.RunProgress.Status != "completed" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RunProgress.Status, "completed")
	}
}

func TestOnIterationStart_UpdatesActivePhase(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.IterationStartEvent{
		Iteration: 1,
		BeadID:    "bead-123",
		BeadTitle: "Test Feature",
		Time:      time.Unix(1000, 0),
	}

	store.OnIterationStart(event)

	if store.Dashboard.ActivePhase == nil {
		t.Fatalf("expected ActivePhase to be set")
	}
	if store.Dashboard.ActivePhase.BeadID != "bead-123" {
		t.Fatalf("BeadID = %q, want %q", store.Dashboard.ActivePhase.BeadID, "bead-123")
	}
	if store.Dashboard.ActivePhase.BeadTitle != "Test Feature" {
		t.Fatalf("BeadTitle = %q, want %q", store.Dashboard.ActivePhase.BeadTitle, "Test Feature")
	}
}
