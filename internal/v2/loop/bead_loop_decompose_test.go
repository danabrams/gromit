package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/triage"
)

// errorDecomposeStage returns a configured error from Run.
type errorDecomposeStage struct {
	name string
	err  error
}

func (s *errorDecomposeStage) Name() string { return s.name }

func (s *errorDecomposeStage) Run(_ context.Context, _ *stage.Request) (*stage.Result, error) {
	return nil, s.err
}

// nilArtifactsDecomposeStage returns a Result whose Artifacts field is nil.
type nilArtifactsDecomposeStage struct {
	name string
}

func (s *nilArtifactsDecomposeStage) Name() string { return s.name }

func (s *nilArtifactsDecomposeStage) Run(_ context.Context, _ *stage.Request) (*stage.Result, error) {
	return &stage.Result{
		Decision:  stage.DecisionProceed,
		Artifacts: nil, // not a *stage.DecomposeArtifacts
	}, nil
}

func TestDecomposeAndRunSubBeads_NilArtifacts(t *testing.T) {
	t.Parallel()

	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	triageStage := &fakeTriageStage{
		name:     "triage",
		category: triage.CategoryDecompose,
	}
	decomposeStage := &nilArtifactsDecomposeStage{name: "decompose"}

	cfg := BeadLoopConfig{
		Gate:      newNoopStage("gate"),
		Build:     buildStage,
		Validate:  newNoopStage("validate"),
		Review:    newNoopStage("review"),
		Epilogue:  newNoopStage("epilogue"),
		Triage:    triageStage,
		Decompose: decomposeStage,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "nil-artifacts", Labels: []string{generation.Format(0)}}}
	_, err = loop.Run(context.Background(), beads, nil)
	if err == nil {
		t.Fatal("expected error when decompose returns nil artifacts, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected artifacts type") {
		t.Fatalf("error should mention unexpected artifacts type, got: %v", err)
	}
}

func TestDecomposeAndRunSubBeads_ZeroSubBeads(t *testing.T) {
	t.Parallel()

	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	triageStage := &fakeTriageStage{
		name:     "triage",
		category: triage.CategoryDecompose,
	}
	decomposeStage := &fakeBeadLoopDecomposeStage{
		name:  "decompose",
		beads: []*bead.Bead{}, // zero sub-beads
	}

	cfg := BeadLoopConfig{
		Gate:      newNoopStage("gate"),
		Build:     buildStage,
		Validate:  newNoopStage("validate"),
		Review:    newNoopStage("review"),
		Epilogue:  newNoopStage("epilogue"),
		Triage:    triageStage,
		Decompose: decomposeStage,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "zero-sub", Labels: []string{generation.Format(0)}}}
	_, err = loop.Run(context.Background(), beads, nil)
	if err == nil {
		t.Fatal("expected error when decompose produces zero sub-beads, got nil")
	}
	if !strings.Contains(err.Error(), "decomposition produced no sub-beads") {
		t.Fatalf("error should mention no sub-beads, got: %v", err)
	}
}

func TestDecomposeAndRunSubBeads_StageError(t *testing.T) {
	t.Parallel()

	decompErr := fmt.Errorf("decompose infrastructure failure")
	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	triageStage := &fakeTriageStage{
		name:     "triage",
		category: triage.CategoryDecompose,
	}
	decomposeStage := &errorDecomposeStage{
		name: "decompose",
		err:  decompErr,
	}

	cfg := BeadLoopConfig{
		Gate:      newNoopStage("gate"),
		Build:     buildStage,
		Validate:  newNoopStage("validate"),
		Review:    newNoopStage("review"),
		Epilogue:  newNoopStage("epilogue"),
		Triage:    triageStage,
		Decompose: decomposeStage,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "err-decompose", Labels: []string{generation.Format(0)}}}
	_, err = loop.Run(context.Background(), beads, nil)
	if err == nil {
		t.Fatal("expected error when decompose stage errors, got nil")
	}
	if !errors.Is(err, decompErr) && !strings.Contains(err.Error(), decompErr.Error()) {
		t.Fatalf("error should propagate decompose error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "decompose") {
		t.Fatalf("error should mention decompose, got: %v", err)
	}
}
