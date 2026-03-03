package unstick

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/logger"
)

func TestAutoChecker_Check_RestartsOnSignals(t *testing.T) {
	ctx := context.Background()
	lastAttempt := time.Unix(42, 0).UTC()
	now := time.Unix(4242, 0).UTC()

	cases := []struct {
		name            string
		closedDep       bool
		metadataChanged bool
		wantReason      string
	}{
		{
			name:       "closed dependency signal",
			closedDep:  true,
			wantReason: RestartReasonClosedDependency,
		},
		{
			name:            "metadata changed signal",
			metadataChanged: true,
			wantReason:      RestartReasonMetadataChanged,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := NewStore(t.TempDir())
			stats := map[string]logger.BeadStats{
				"bead-1": {BeadID: "bead-1", LastAttempt: lastAttempt},
			}

			client := &mockBeadClient{
				closedDependency: tc.closedDep,
				metadataChanged:  tc.metadataChanged,
			}
			emitter := &fakeEmitter{}
			checker := &AutoChecker{
				Client: client,
				NowFn: func() time.Time {
					return now
				},
			}

			err := checker.Check(ctx, []*bead.Bead{{ID: "bead-1"}}, stats, store, emitter)
			if err != nil {
				t.Fatalf("unexpected Check error: %v", err)
			}

			point, ok := store.Get("bead-1")
			if !ok {
				t.Fatalf("expected restart point")
			}
			if point.Time != now {
				t.Fatalf("expected restart time %v, got %v", now, point.Time)
			}
			if point.Reason != tc.wantReason {
				t.Fatalf("expected reason %q, got %q", tc.wantReason, point.Reason)
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
			if evt.Reason != tc.wantReason {
				t.Fatalf("expected event reason %q, got %q", tc.wantReason, evt.Reason)
			}

			emitter.events = nil
		})
	}
}

type mockBeadClient struct {
	closedDependency bool
	metadataChanged  bool
}

func (m *mockBeadClient) ClosedDependencySignal(ctx context.Context, beadID string) (bool, error) {
	return m.closedDependency, nil
}

func (m *mockBeadClient) MetadataChangedSignal(ctx context.Context, beadID string) (bool, error) {
	return m.metadataChanged, nil
}

type fakeEmitter struct {
	events []events.Event
}

func (f *fakeEmitter) Emit(evt events.Event) {
	f.events = append(f.events, evt)
}
