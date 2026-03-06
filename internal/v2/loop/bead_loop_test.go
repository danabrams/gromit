package loop

import (
    "context"
    "testing"

    "github.com/danabrams/gromit/internal/bead"
    "github.com/danabrams/gromit/internal/v2/stage"
)

func TestBeadLoopRunsStagesInOrder(t *testing.T) {
    t.Parallel()

    order := []string{}
    beads := []*bead.Bead{{ID: "bead-1"}}

    config := BeadLoopConfig{
        Gate:     newRecordingStage("gate", &order),
        Build:    newRecordingStage("build", &order),
        Validate: newRecordingStage("validate", &order),
        Review:   newRecordingStage("review", &order),
        Epilogue: newRecordingStage("epilogue", &order),
    }
    loop, err := NewBeadLoop(config)
    if err != nil {
        t.Fatalf("NewBeadLoop: %v", err)
    }

    if err := loop.Run(context.Background(), beads); err != nil {
        t.Fatalf("Run failed: %v", err)
    }

    expected := []string{"gate:bead-1", "build:bead-1", "validate:bead-1", "review:bead-1", "epilogue:bead-1"}
    for i, name := range expected {
        if i >= len(order) {
            t.Fatalf("missing stage execution %s", name)
        }
        if order[i] != name {
            t.Fatalf("stage %d = %s, want %s", i, order[i], name)
        }
    }
}

func newRecordingStage(name string, order *[]string) stage.Stage {
    return &recordingStage{name: name, hook: func(beadID string) {
        *order = append(*order, name+":"+beadID)
    }}
}

type recordingStage struct {
    name string
    hook func(string)
}

func (s *recordingStage) Name() string {
    return s.name
}

func (s *recordingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
    if s.hook != nil {
        s.hook(req.Bead.ID)
    }
    return &stage.Result{Decision: stage.DecisionProceed}, nil
}
