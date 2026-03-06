package event

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
)

func TestLegacyEventsFromTyped(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name     string
		typed    TypedEvent
		wantType string
		check    func(events.Event) bool
	}{
		{
			name: "spec started",
			typed: &SpecStartedEvent{
				Event:    Event{Timestamp: now},
				SpecID:   "spec-id",
				Worktree: "worktree",
			},
			wantType: "*events.SpecStartedEvent",
		},
		{
			name: "stage started logs",
			typed: &StageStartedEvent{
				Event:     Event{Timestamp: now},
				StageName: "build",
				BeadID:    "bead-id",
				Iteration: 2,
			},
			wantType: "*events.LogEvent",
			check: func(evt events.Event) bool {
				logEvt, ok := evt.(*events.LogEvent)
				return ok &&
					strings.Contains(logEvt.Message, "build") &&
					strings.Contains(logEvt.Message, "iteration 2")
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			legacy := legacyEventsFromTyped(tc.typed)
			if len(legacy) == 0 {
				t.Fatalf("no legacy events produced")
			}
			gotType := fmt.Sprintf("%T", legacy[0])
			if gotType != tc.wantType {
				t.Fatalf("legacy event type = %s, want %s", gotType, tc.wantType)
			}
			if tc.check != nil && !tc.check(legacy[0]) {
				t.Fatalf("check failed for event %+v", legacy[0])
			}
		})
	}
}
