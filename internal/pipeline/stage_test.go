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

func TestInputTouchedPackagesField(t *testing.T) {
	pkgs := []string{"internal/pipeline", "internal/runner"}
	in := pipeline.Input{TouchedPackages: pkgs}
	if len(in.TouchedPackages) != 2 {
		t.Errorf("Input.TouchedPackages: want 2 items, got %d", len(in.TouchedPackages))
	}
	if in.TouchedPackages[0] != "internal/pipeline" {
		t.Errorf("Input.TouchedPackages[0]: want %q, got %q", "internal/pipeline", in.TouchedPackages[0])
	}
	var zero pipeline.Input
	if zero.TouchedPackages != nil {
		t.Error("Input.TouchedPackages zero value: want nil, got non-nil")
	}
}

func TestOutputTouchedPackagesField(t *testing.T) {
	pkgs := []string{"internal/pipeline", "internal/runner"}
	out := pipeline.Output{TouchedPackages: pkgs}
	if len(out.TouchedPackages) != 2 {
		t.Errorf("Output.TouchedPackages: want 2 items, got %d", len(out.TouchedPackages))
	}
	if out.TouchedPackages[1] != "internal/runner" {
		t.Errorf("Output.TouchedPackages[1]: want %q, got %q", "internal/runner", out.TouchedPackages[1])
	}
	var zero pipeline.Output
	if zero.TouchedPackages != nil {
		t.Error("Output.TouchedPackages zero value: want nil, got non-nil")
	}
}

func TestOutputPhaseMetricsField(t *testing.T) {
	out := pipeline.Output{}
	out.PhaseMetrics = []pipeline.PhaseMetrics{
		{Phase: "red"},
		{Phase: "green"},
	}
	if len(out.PhaseMetrics) != 2 {
		t.Errorf("Output.PhaseMetrics: want 2 items, got %d", len(out.PhaseMetrics))
	}
	if out.PhaseMetrics[0].Phase != "red" {
		t.Errorf("Output.PhaseMetrics[0].Phase: want %q, got %q", "red", out.PhaseMetrics[0].Phase)
	}
}

func TestPhaseMetricSingularType(t *testing.T) {
	m := pipeline.PhaseMetric{Phase: "refactor"}
	if m.Phase != "refactor" {
		t.Errorf("PhaseMetric.Phase: want %q, got %q", "refactor", m.Phase)
	}
}
