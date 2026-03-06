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

func TestBeadLoopRunsBeadsInDependencyOrder(t *testing.T) {
	t.Parallel()

	beadOrder := []string{}

	config := BeadLoopConfig{
		Gate:     &recordingStage{name: "gate", hook: func(id string) { beadOrder = append(beadOrder, id) }},
		Build:    newRecordingStage("build", nil),
		Validate: newRecordingStage("validate", nil),
		Review:   newRecordingStage("review", nil),
		Epilogue: newRecordingStage("epilogue", nil),
	}

	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{
		{ID: "child", DependsOn: []bead.Dependency{{ID: "root"}}},
		{ID: "root"},
	}

	if err := loop.Run(context.Background(), beads); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(beadOrder) != 2 {
		t.Fatalf("bead order = %v, want 2 entries", beadOrder)
	}
	if beadOrder[0] != "root" || beadOrder[1] != "child" {
		t.Fatalf("bead order = %v, want [root child]", beadOrder)
	}
}

func newRecordingStage(name string, order *[]string) stage.Stage {
	hook := func(string) {}
	if order != nil {
		hook = func(beadID string) {
			*order = append(*order, name+":"+beadID)
		}
	}
	return &recordingStage{name: name, hook: hook}
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

type noopStage struct {
	name string
}

func newNoopStage(name string) stage.Stage {
	return &noopStage{name: name}
}

func (s *noopStage) Name() string {
	return s.name
}

func (s *noopStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}
