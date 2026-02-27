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

func TestOnIterationComplete_UpdatesProgressPercentage(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 1,
				MaxIterations:    5,
			},
		},
	}
	event := &events.IterationCompleteEvent{
		Iteration: 1,
		BeadID:    "bead-123",
		Success:   true,
		Duration:  5 * time.Second,
		Time:      time.Unix(2000, 0),
	}

	store.OnIterationComplete(event)

	if store.Dashboard.RunProgress.CurrentIteration != 2 {
		t.Fatalf("CurrentIteration = %d, want 2", store.Dashboard.RunProgress.CurrentIteration)
	}
	if store.Dashboard.RunProgress.IterationPercent != 40 {
		t.Fatalf("IterationPercent = %d, want 40", store.Dashboard.RunProgress.IterationPercent)
	}
}

func TestOnBeadComplete_TracksRecentCompletion(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.BeadCompleteEvent{
		BeadID:    "bead-123",
		BeadTitle: "Test Feature",
		Duration:  5 * time.Second,
		Time:      time.Unix(3000, 0),
	}

	store.OnBeadComplete(event)

	if len(store.Dashboard.RecentCompletions) != 1 {
		t.Fatalf("expected 1 recent completion, got %d", len(store.Dashboard.RecentCompletions))
	}
	if store.Dashboard.RecentCompletions[0].BeadID != "bead-123" {
		t.Fatalf("BeadID = %q, want %q", store.Dashboard.RecentCompletions[0].BeadID, "bead-123")
	}
	if store.Dashboard.RecentCompletions[0].Status != "completed" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RecentCompletions[0].Status, "completed")
	}
}

func TestOnBeadFailed_TracksRecentFailure(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.BeadFailedEvent{
		BeadID:    "bead-456",
		BeadTitle: "Failed Feature",
		Error:     "test failed",
		Time:      time.Unix(4000, 0),
	}

	store.OnBeadFailed(event)

	if len(store.Dashboard.RecentCompletions) != 1 {
		t.Fatalf("expected 1 recent completion, got %d", len(store.Dashboard.RecentCompletions))
	}
	if store.Dashboard.RecentCompletions[0].Status != "failed" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RecentCompletions[0].Status, "failed")
	}
}

func TestRecentCompletions_LimitsTo10(t *testing.T) {
	t.Parallel()

	store := &Store{}
	for i := 0; i < 15; i++ {
		event := &events.BeadCompleteEvent{
			BeadID:    "bead-" + string(rune(i)),
			BeadTitle: "Feature",
			Duration:  time.Second,
			Time:      time.Unix(int64(5000+i), 0),
		}
		store.OnBeadComplete(event)
	}

	if len(store.Dashboard.RecentCompletions) != 10 {
		t.Fatalf("expected 10 recent completions, got %d", len(store.Dashboard.RecentCompletions))
	}
}
