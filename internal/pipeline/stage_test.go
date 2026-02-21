package pipeline_test

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// stubStage is a minimal Stage implementation for compile-time verification.
type stubStage struct{}

func (s *stubStage) Run(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
	return pipeline.Output{}, nil
}

// Compile-time check: *stubStage must implement Stage.
var _ pipeline.Stage = (*stubStage)(nil)

func TestDecisionConstants(t *testing.T) {
	// Decision constants must be distinct values.
	if pipeline.Proceed == pipeline.Skip {
		t.Error("Proceed and Skip must be distinct")
	}
	if pipeline.Proceed == pipeline.Block {
		t.Error("Proceed and Block must be distinct")
	}
	if pipeline.Skip == pipeline.Block {
		t.Error("Skip and Block must be distinct")
	}
}

func TestDecisionZeroValueIsProceed(t *testing.T) {
	var d pipeline.Decision
	if d != pipeline.Proceed {
		t.Errorf("zero value Decision should be Proceed, got %v", d)
	}
}

func TestInputHoldsRequiredFields(t *testing.T) {
	b := &bead.Bead{ID: "test-bead", Title: "Test"}
	cfg := &config.Config{}
	dl := time.Now().Add(time.Minute)
	failures := []string{"failure 1", "failure 2"}

	in := pipeline.Input{
		Bead:               b,
		Config:             cfg,
		Iteration:          3,
		Deadline:           dl,
		ValidationFailures: failures,
	}

	if in.Bead != b {
		t.Error("Input.Bead not set correctly")
	}
	if in.Config != cfg {
		t.Error("Input.Config not set correctly")
	}
	if in.Iteration != 3 {
		t.Error("Input.Iteration not set correctly")
	}
	if !in.Deadline.Equal(dl) {
		t.Error("Input.Deadline not set correctly")
	}
	if len(in.ValidationFailures) != 2 {
		t.Errorf("Input.ValidationFailures: want 2 items, got %d", len(in.ValidationFailures))
	}
}

func TestOutputHoldsDecision(t *testing.T) {
	out := pipeline.Output{Decision: pipeline.Skip}
	if out.Decision != pipeline.Skip {
		t.Errorf("Output.Decision: want Skip, got %v", out.Decision)
	}
}

func TestOutputHoldsValidationFailures(t *testing.T) {
	failures := []string{"test failure"}
	out := pipeline.Output{ValidationFailures: failures}
	if len(out.ValidationFailures) != 1 {
		t.Errorf("Output.ValidationFailures: want 1 item, got %d", len(out.ValidationFailures))
	}
}

func TestInputBuildSucceededField(t *testing.T) {
	in := pipeline.Input{BuildSucceeded: true}
	if !in.BuildSucceeded {
		t.Error("Input.BuildSucceeded: want true, got false")
	}
	var zero pipeline.Input
	if zero.BuildSucceeded {
		t.Error("Input.BuildSucceeded zero value: want false, got true")
	}
}
