package loop

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/event"
)

func TestBeadLoopEmitsStageLifecycleEvents(t *testing.T) {
	t.Parallel()

	emitter := event.NewEmitter()
	ch := make(chan event.TypedEvent, 16)
	emitter.Subscribe(func(evt event.TypedEvent) {
		ch <- evt
	})

	config := BeadLoopConfig{
		Gate:     newRecordingStage("gate", nil),
		Build:    newRecordingStage("build", nil),
		Validate: newRecordingStage("validate", nil),
		Review:   newRecordingStage("review", nil),
		Epilogue: newRecordingStage("epilogue", nil),
		Emitter:  emitter,
	}

	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "stage-bead"}}
	if err := loop.Run(context.Background(), beads); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	events := collectLifecycleEvents(ch, 12)
	if _, ok := events[0].(event.BeadStartedEvent); !ok {
		t.Fatalf("first event = %T, want event.BeadStartedEvent", events[0])
	}
	if _, ok := events[len(events)-1].(event.BeadCompletedEvent); !ok {
		t.Fatalf("last event = %T, want event.BeadCompletedEvent", events[len(events)-1])
	}

	stageNames := []string{"gate", "build", "validate", "review", "epilogue"}
	idx := 1
	for _, name := range stageNames {
		started, ok := events[idx].(event.StageStartedEvent)
		if !ok {
			t.Fatalf("event[%d] = %T, want event.StageStartedEvent", idx, events[idx])
		}
		if started.StageName != name {
			t.Fatalf("stage started name = %q, want %q", started.StageName, name)
		}
		idx++

		completed, ok := events[idx].(event.StageCompletedEvent)
		if !ok {
			t.Fatalf("event[%d] = %T, want event.StageCompletedEvent", idx, events[idx])
		}
		if completed.StageName != name {
			t.Fatalf("stage completed name = %q, want %q", completed.StageName, name)
		}
		if !completed.Success {
			t.Fatalf("stage %s success = %v, want true", name, completed.Success)
		}
		idx++
	}
}
