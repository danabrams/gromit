package unstick

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/logger"
)

func TestAutoChecker_Check_UnsticksOnGitLog(t *testing.T) {
	ctx := context.Background()
	lastAttempt := time.Unix(42, 0).UTC()
	now := time.Unix(4242, 0).UTC()

	store := NewStore(t.TempDir())
	stats := map[string]logger.BeadStats{
		"bead-1": {BeadID: "bead-1", LastAttempt: lastAttempt},
	}

	emitter := &fakeEmitter{}
	checker := &AutoChecker{
		GitLogFn: func(since time.Time) (bool, error) {
			if !since.Equal(lastAttempt) {
				t.Fatalf("git log called with wrong time: %v", since)
			}
			return true, nil
		},
		NowFn: func() time.Time {
			return now
		},
	}

	if err := checker.Check(ctx, []*bead.Bead{{ID: "bead-1"}}, stats, store, emitter); err != nil {
		t.Fatalf("unexpected Check error: %v", err)
	}

	point, ok := store.Get("bead-1")
	if !ok {
		t.Fatalf("expected restart point for bead-1")
	}
	if !point.Time.Equal(now) {
		t.Fatalf("expected restart time %v, got %v", now, point.Time)
	}
	if point.Reason != RestartReasonNewCommits {
		t.Fatalf("expected reason %q, got %q", RestartReasonNewCommits, point.Reason)
	}

	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
	evt, ok := emitter.events[0].(*events.BeadUnstickedEvent)
	if !ok {
		t.Fatalf("expected BeadUnstickedEvent, got %T", emitter.events[0])
	}
	if evt.BeadID != "bead-1" {
		t.Fatalf("expected bead-id bead-1, got %s", evt.BeadID)
	}
	if evt.Reason != RestartReasonNewCommits {
		t.Fatalf("expected event reason %q, got %q", RestartReasonNewCommits, evt.Reason)
	}
}

type fakeEmitter struct {
	events []events.Event
}

func (f *fakeEmitter) Emit(evt events.Event) {
	f.events = append(f.events, evt)
}
