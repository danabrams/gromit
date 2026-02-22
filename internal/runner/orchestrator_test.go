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

// TestOrchestrator_CallsStateSaverAfterRun verifies that when StateSaver is set
// in the config, it is called once after the orchestrator loop completes,
// so provider routing state is persisted across runs.
func TestOrchestrator_CallsStateSaverAfterRun(t *testing.T) {
	saveCalled := false
	saver := &fakeStateSaver{saveFn: func() error {
		saveCalled = true
		return nil
	}}

	getBead := func(_ context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:       &fakeStage{},
		Build:      &fakeStage{},
		Validate:   &fakeStage{},
		Epilogue:   &fakeStage{},
		GetBead:    getBead,
		Config:     &config.Config{},
		Output:     io.Discard,
		StateSaver: saver,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 10, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !saveCalled {
		t.Error("StateSaver.Save() was not called; want provider state persisted after loop completes")
	}
}

// fakeStateSaver is a test double for StateSaver.
type fakeStateSaver struct {
	saveFn func() error
}

func (f *fakeStateSaver) Save() error {
	if f.saveFn != nil {
		return f.saveFn()
	}
	return nil
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
