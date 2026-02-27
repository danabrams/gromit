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
