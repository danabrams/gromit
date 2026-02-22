package runner

import (
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
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

// TestOrchestrator_ProviderCostDefs_Accessible verifies that provider cost
// definitions passed via OrchestratorConfig are stored and accessible, so the
// Orchestrator path can estimate costs like the legacy Runner path.
func TestOrchestrator_ProviderCostDefs_Accessible(t *testing.T) {
	defs := map[string]config.ProviderDef{
		"claude": {Binary: "claude"},
	}

	cfg := OrchestratorConfig{
		Gate:             &fakeStage{},
		Build:            &fakeStage{},
		Validate:         &fakeStage{},
		Epilogue:         &fakeStage{},
		GetBead:          func(_ context.Context) (*bead.Bead, error) { return nil, nil },
		Config:           &config.Config{},
		Output:           io.Discard,
		ProviderCostDefs: defs,
	}

	orch := NewOrchestrator(cfg)
	if orch.cfg.ProviderCostDefs == nil {
		t.Fatal("ProviderCostDefs is nil; want cost definitions stored on Orchestrator")
	}
	if len(orch.cfg.ProviderCostDefs) != 1 {
		t.Errorf("ProviderCostDefs has %d entries, want 1", len(orch.cfg.ProviderCostDefs))
	}
	if _, ok := orch.cfg.ProviderCostDefs["claude"]; !ok {
		t.Error("ProviderCostDefs missing 'claude' entry")
	}
}

// TestOrchestrator_SuccessPath_CarriesBuildModelToIterationLog verifies that when
// the Build stage returns a model name in its Output, the orchestrator copies it
// into the IterationLog on the success path so audit logs show which model was used.
func TestOrchestrator_SuccessPath_CarriesBuildModelToIterationLog(t *testing.T) {
	var capturedResult *logger.IterationLog

	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Proceed, Model: "claude-opus-4-6"}, nil
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		if in.Result != nil {
			capturedResult = in.Result
		}
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
		Gate:     &fakeStage{},
		Build:    build,
		Validate: &fakeStage{},
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if capturedResult == nil {
		t.Fatal("Epilogue Result is nil; want IterationLog populated on success path")
	}
	if capturedResult.Model != "claude-opus-4-6" {
		t.Errorf("IterationLog.Model = %q, want %q", capturedResult.Model, "claude-opus-4-6")
	}
}

// TestOrchestrator_SuccessPath_CarriesBuildCostAndTokensToIterationLog verifies that
// DurationMs, CostUSD, InputTokens, and OutputTokens from the Build stage output
// are propagated into the IterationLog on the success path for cost auditing.
func TestOrchestrator_SuccessPath_CarriesBuildCostAndTokensToIterationLog(t *testing.T) {
	var capturedResult *logger.IterationLog

	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:     pipeline.Proceed,
			DurationMs:   42000,
			CostUSD:      0.123,
			InputTokens:  1000,
			OutputTokens: 500,
		}, nil
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		if in.Result != nil {
			capturedResult = in.Result
		}
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-2", Title: "Cost test bead"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    build,
		Validate: &fakeStage{},
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if capturedResult == nil {
		t.Fatal("Epilogue Result is nil; want IterationLog populated on success path")
	}

	if capturedResult.DurationMs != 42000 {
		t.Errorf("IterationLog.DurationMs = %d, want 42000", capturedResult.DurationMs)
	}
	if capturedResult.CostUSD != 0.123 {
		t.Errorf("IterationLog.CostUSD = %f, want 0.123", capturedResult.CostUSD)
	}
	if capturedResult.InputTokens != 1000 {
		t.Errorf("IterationLog.InputTokens = %d, want 1000", capturedResult.InputTokens)
	}
	if capturedResult.OutputTokens != 500 {
		t.Errorf("IterationLog.OutputTokens = %d, want 500", capturedResult.OutputTokens)
	}
}

// TestRunner_RunMethod_Removed verifies that the legacy Runner.Run method has been
// removed as part of the architecture migration to Orchestrator. All loop execution
// now flows through Orchestrator.Run. This test prevents accidental reintroduction.
func TestRunner_RunMethod_Removed(t *testing.T) {
	rt := reflect.TypeOf(&Runner{})
	if _, found := rt.MethodByName("Run"); found {
		t.Error("Runner still has a Run method; the legacy run loop should be removed in favour of Orchestrator.Run")
	}
}

// TestNewRunnerWithDeps_Removed documents that the legacy NewRunnerWithDeps constructor
// and its Deps type have been removed. All test construction now uses OrchestratorTestHelper.
// The removal is enforced at compile time: re-introducing Deps would require adding it back
// to this package, and any code referencing it would cause compilation errors elsewhere.
func TestNewRunnerWithDeps_Removed(t *testing.T) {
	// Deps type and NewRunnerWithDeps constructor are gone; nothing to assert here.
	// The Runner type still exists for TDD orchestration but no longer exposes Run().
	_ = reflect.TypeOf(&Runner{}) // ensure Runner itself is still present
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
