package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// fakeStage is a test double for pipeline.Stage.
type fakeStage struct {
	runFn func(ctx context.Context, in pipeline.Input) (pipeline.Output, error)
}

func (f *fakeStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if f.runFn != nil {
		return f.runFn(ctx, in)
	}
	return pipeline.Output{Decision: pipeline.Proceed}, nil
}

// TestOrchestrator_ValidationFailure_SetsFailureOutput verifies that when the
// validate stage returns Block with ValidationFailures, the orchestrator sets
// FailureOutput on the epilogue Input so the failure learner receives it.
func TestOrchestrator_ValidationFailure_SetsFailureOutput(t *testing.T) {
	var capturedInput pipeline.Input

	gate := &fakeStage{}
	build := &fakeStage{}
	validate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:           pipeline.Block,
			ValidationFailures: []string{"--- FAIL: TestFoo", "FAIL\tpkg/foo"},
		}, nil
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		capturedInput = in
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-1", Title: "Test bead"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gate,
		Build:    build,
		Validate: validate,
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 10, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if capturedInput.FailureOutput == "" {
		t.Error("Epilogue input FailureOutput is empty; want validation failure text set by orchestrator")
	}
	if !strings.Contains(capturedInput.FailureOutput, "FAIL: TestFoo") {
		t.Errorf("FailureOutput = %q, want it to contain validation failure text", capturedInput.FailureOutput)
	}
}
