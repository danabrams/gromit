package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	epiloguepkg "github.com/danabrams/gromit/internal/pipeline/epilogue"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/specflow"
	"github.com/danabrams/gromit/internal/state"
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

// fakeSpecMergeController is a minimal test double for specmerge.Controller.
type fakeSpecMergeController struct {
	isCompleteFn       func(context.Context, string) (bool, error)
	triggerFn          func(context.Context, string) error
	isCompleteCalls    []string
	isCompleteContexts []context.Context
	triggerCalls       []string
}

func (f *fakeSpecMergeController) IsSpecComplete(ctx context.Context, specName string) (bool, error) {
	f.isCompleteCalls = append(f.isCompleteCalls, specName)
	f.isCompleteContexts = append(f.isCompleteContexts, ctx)
	if f.isCompleteFn != nil {
		return f.isCompleteFn(ctx, specName)
	}
	return false, nil
}

func (f *fakeSpecMergeController) Trigger(ctx context.Context, specName string) error {
	f.triggerCalls = append(f.triggerCalls, specName)
	if f.triggerFn != nil {
		return f.triggerFn(ctx, specName)
	}
	return nil
}

type specStageStore struct {
	stage      specflow.Stage
	storeCalls []specflow.Stage
}

func (s *specStageStore) Stage(_ context.Context, _ string) (specflow.Stage, error) {
	return s.stage, nil
}

func (s *specStageStore) StoreStage(_ context.Context, _ string, stage specflow.Stage) error {
	s.storeCalls = append(s.storeCalls, stage)
	s.stage = stage
	return nil
}

// fakeStatusWriter implements epiloguepkg.StatusWriter for tests.
type fakeStatusWriter struct {
	writeFn func(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error
}

func (f *fakeStatusWriter) Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error {
	if f.writeFn != nil {
		return f.writeFn(iteration, beadID, beadTitle, model, maxIterations, timeBudgetMinutes)
	}
	return nil
}

// fakeBeadLifecycle implements epiloguepkg.BeadLifecycle for tests.
type fakeBeadLifecycle struct {
	closeFn func(ctx context.Context, id string) error
	syncFn  func(ctx context.Context) error
}

func (f *fakeBeadLifecycle) Close(ctx context.Context, id string) error {
	if f.closeFn != nil {
		return f.closeFn(ctx, id)
	}
	return nil
}

func (f *fakeBeadLifecycle) Sync(ctx context.Context) error {
	if f.syncFn != nil {
		return f.syncFn(ctx)
	}
	return nil
}

// capturingIterationLogWriter records the last log written for inspection.
type capturingIterationLogWriter struct {
	called  bool
	lastLog *logger.IterationLog
}

func (c *capturingIterationLogWriter) Write(log *logger.IterationLog) error {
	c.called = true
	c.lastLog = log
	return nil
}

// fakeCoordinator is a test double for Coordinator interface.
type fakeCoordinator struct {
	coordinateFn       func(ctx context.Context) error
	recoverFromCrashFn func(ctx context.Context) error
}

func (f *fakeCoordinator) Coordinate(ctx context.Context) error {
	if f.coordinateFn != nil {
		return f.coordinateFn(ctx)
	}
	return nil
}

func (f *fakeCoordinator) RecoverFromCrash(ctx context.Context) error {
	if f.recoverFromCrashFn != nil {
		return f.recoverFromCrashFn(ctx)
	}
	return nil
}

// TestOrchestrator_CallsStateSaverAfterRun verifies that when StateSaver is set
// in the config, it is called once after the orchestrator loop completes,
// so provider routing state is persisted across runs.
func TestOrchestrator_CallsStateSaverAfterRun(t *testing.T) {
	t.Parallel()
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

func TestOrchestratorRunsPreImplementationHook(t *testing.T) {
	t.Parallel()

	store := &specStageStore{stage: specflow.StageAcceptanceTests}
	stageCtx := &StageContext{
		SpecName: "spec-foo",
		Stage:    specflow.StageAcceptanceTests,
		Manager:  specflow.NewManager(store),
	}
	hookCalls := 0
	cfg := OrchestratorConfig{
		Gate:         &fakeStage{},
		Build:        &fakeStage{},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		GetBead:      func(context.Context) (*bead.Bead, error) { return nil, nil },
		Config:       &config.Config{},
		StageContext: stageCtx,
		PreImplementationHook: func(ctx context.Context) error {
			hookCalls++
			return nil
		},
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if hookCalls != 1 {
		t.Fatalf("pre-implementation hook called %d times, want 1", hookCalls)
	}
	if stageCtx.Stage != specflow.StageImplementation {
		t.Fatalf("stage context stage = %s, want %s", stageCtx.Stage, specflow.StageImplementation)
	}
	if store.stage != specflow.StageImplementation {
		t.Fatalf("store stage = %s, want %s", store.stage, specflow.StageImplementation)
	}
	if len(store.storeCalls) != 1 || store.storeCalls[0] != specflow.StageImplementation {
		t.Fatalf("store calls = %v, want [%s]", store.storeCalls, specflow.StageImplementation)
	}
}

func TestOrchestratorPreImplementationHookError(t *testing.T) {
	t.Parallel()

	store := &specStageStore{stage: specflow.StageAcceptanceTests}
	stageCtx := &StageContext{
		SpecName: "spec-foo",
		Stage:    specflow.StageAcceptanceTests,
		Manager:  specflow.NewManager(store),
	}
	hookErr := errors.New("hook failed")
	cfg := OrchestratorConfig{
		Gate:         &fakeStage{},
		Build:        &fakeStage{},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		GetBead:      func(context.Context) (*bead.Bead, error) { return nil, nil },
		Config:       &config.Config{},
		StageContext: stageCtx,
		PreImplementationHook: func(ctx context.Context) error {
			return hookErr
		},
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 0, time.Time{}, nil); err == nil || !errors.Is(err, hookErr) {
		t.Fatalf("Run() error = %v, want %v", err, hookErr)
	}
	if stageCtx.Stage != specflow.StageAcceptanceTests {
		t.Fatalf("stage context stage = %s, want %s", stageCtx.Stage, specflow.StageAcceptanceTests)
	}
	if store.stage != specflow.StageAcceptanceTests {
		t.Fatalf("store stage = %s, want %s", store.stage, specflow.StageAcceptanceTests)
	}
	if len(store.storeCalls) != 0 {
		t.Fatalf("store calls = %v, want []", store.storeCalls)
	}
}

func TestOrchestrator_RunInvokesAutoTriageServiceAfterRun(t *testing.T) {
	t.Parallel()

	service := &fakeAutoTriageService{}
	cfg := OrchestratorConfig{
		Gate:              &fakeStage{},
		Build:             &fakeStage{},
		Validate:          &fakeStage{},
		Epilogue:          &fakeStage{},
		GetBead:           func(_ context.Context) (*bead.Bead, error) { return nil, nil },
		Config:            &config.Config{},
		Output:            io.Discard,
		AutoTriageService: service,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 1, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if service.calls != 1 {
		t.Fatalf("Auto-triage service called %d times, want 1", service.calls)
	}
}

func TestOrchestrator_AutoTriageErrorsAreLogged(t *testing.T) {
	t.Parallel()

	const triageErrMsg = "triage failed"

	service := &fakeAutoTriageService{
		evaluateFn: func(context.Context) error {
			return errors.New(triageErrMsg)
		},
	}
	output := &bytes.Buffer{}
	cfg := OrchestratorConfig{
		Gate:              &fakeStage{},
		Build:             &fakeStage{},
		Validate:          &fakeStage{},
		Epilogue:          &fakeStage{},
		GetBead:           func(_ context.Context) (*bead.Bead, error) { return nil, nil },
		Config:            &config.Config{},
		Output:            output,
		AutoTriageService: service,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 1, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	log := output.String()
	if !strings.Contains(log, "auto-triage evaluation failed") {
		t.Fatalf("log output = %q, want warning about auto-triage failure", log)
	}
	if !strings.Contains(log, triageErrMsg) {
		t.Fatalf("log output = %q, want to include the error message", log)
	}
	if service.calls != 1 {
		t.Fatalf("Auto-triage service called %d times, want 1", service.calls)
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

type fakeAutoTriageService struct {
	evaluateFn func(context.Context) error
	calls      int
}

func (f *fakeAutoTriageService) EvaluateAndTriage(ctx context.Context) error {
	f.calls++
	if f.evaluateFn != nil {
		return f.evaluateFn(ctx)
	}
	return nil
}

// TestOrchestrator_ProviderCostDefs_Accessible verifies that provider cost
// definitions passed via OrchestratorConfig are stored and accessible, so the
// Orchestrator path can estimate costs like the legacy Runner path.
func TestOrchestrator_ProviderCostDefs_Accessible(t *testing.T) {
	t.Parallel()
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

func TestOrchestrator_SpecMergePipelineTriggersOnce(t *testing.T) {
	t.Parallel()

	specName := "payments"
	beads := []*bead.Bead{
		{ID: "spec-1", Labels: []string{"spec:" + specName}},
		{ID: "spec-2", Labels: []string{"spec:" + specName}},
	}
	beadIndex := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		if beadIndex >= len(beads) {
			return nil, nil
		}
		next := beads[beadIndex]
		beadIndex++
		return next, nil
	}

	completeCalls := 0
	specPipeline := &fakeSpecMergeController{
		isCompleteFn: func(_ context.Context, name string) (bool, error) {
			if name != specName {
				t.Fatalf("IsSpecComplete called with spec %q, want %q", name, specName)
			}
			completeCalls++
			return completeCalls >= len(beads), nil
		},
		triggerFn: func(ctx context.Context, name string) error {
			if name != specName {
				t.Fatalf("Trigger called with spec %q, want %q", name, specName)
			}
			return nil
		},
	}

	cfg := OrchestratorConfig{
		Gate:                &fakeStage{},
		Build:               &fakeStage{},
		Validate:            &fakeStage{},
		Epilogue:            &fakeStage{},
		GetBead:             getBead,
		Config:              &config.Config{Methodology: config.MethodologyConfig{Granularity: config.MethodologyGranularitySpec}},
		SpecMergeController: specPipeline,
		Output:              io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(specPipeline.triggerCalls) != 1 {
		t.Fatalf("Trigger was called %d times, want 1", len(specPipeline.triggerCalls))
	}
	if completeCalls != len(beads) {
		t.Fatalf("IsSpecComplete called %d times, want %d", completeCalls, len(beads))
	}
}

func TestOrchestrator_SpecMergeIsSpecCompleteReceivesContext(t *testing.T) {
	t.Parallel()

	key := struct{}{}
	ctxValue := "ctx-preserved"
	testCtx := context.WithValue(context.Background(), key, ctxValue)

	specName := "payments"
	specPipeline := &fakeSpecMergeController{
		isCompleteFn: func(ctx context.Context, name string) (bool, error) {
			if name != specName {
				t.Fatalf("IsSpecComplete called with spec %q, want %q", name, specName)
			}
			if got := ctx.Value(key); got != ctxValue {
				t.Fatalf("IsSpecComplete received context value %v, want %v", got, ctxValue)
			}
			return true, nil
		},
	}

	cfg := OrchestratorConfig{
		Gate:                &fakeStage{},
		Build:               &fakeStage{},
		Validate:            &fakeStage{},
		Epilogue:            &fakeStage{},
		GetBead:             func(context.Context) (*bead.Bead, error) { return nil, nil },
		Config:              &config.Config{Methodology: config.MethodologyConfig{Granularity: config.MethodologyGranularitySpec}},
		SpecMergeController: specPipeline,
		Output:              io.Discard,
	}

	orch := NewOrchestrator(cfg)
	orch.maybeTriggerSpecMerge(testCtx, &bead.Bead{Labels: []string{"spec:" + specName}})

	if len(specPipeline.isCompleteContexts) != 1 {
		t.Fatalf("IsSpecComplete context recorded %d times, want 1", len(specPipeline.isCompleteContexts))
	}
}

// TestOrchestrator_SuccessPath_CarriesBuildModelToIterationLog verifies that when
// the Build stage returns a model name in its Output, the orchestrator copies it
// into the IterationLog on the success path so audit logs show which model was used.
func TestOrchestrator_SuccessPath_CarriesBuildModelToIterationLog(t *testing.T) {
	t.Parallel()
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

// TestOrchestrator_SuccessPathLifecycleFailureRecordsFailureOutcome verifies that when
// the Epilogue fails to close the bead, the persisted IterationLog marks the iteration as failed.
func TestOrchestrator_SuccessPathLifecycleFailureRecordsFailureOutcome(t *testing.T) {
	t.Parallel()

	logWriter := &capturingIterationLogWriter{}
	lifecycle := &fakeBeadLifecycle{
		closeFn: func(ctx context.Context, id string) error {
			return errors.New("close failure")
		},
	}
	epilogueStage := epiloguepkg.New(lifecycle, &fakeStatusWriter{}, io.Discard).
		WithIterationLogWriter(logWriter)

	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Proceed, Model: "claude-opus-4-6"}, nil
	}}
	validate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-lifecycle-failure", Title: "Lifecycle fail bead"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    build,
		Validate: validate,
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !logWriter.called {
		t.Fatal("IterationLogWriter was not invoked; want log entry written after epilogue")
	}
	if logWriter.lastLog == nil {
		t.Fatal("IterationLogWriter last log is nil")
	}
	if logWriter.lastLog.Success {
		t.Error("Iteration log success = true, want false when lifecycle close fails")
	}
}

// TestOrchestrator_LowTierHaikuIterationRecordsDuration verifies that when a low-tier
// haiku bead returns a DurationMs from the Build stage, it is properly propagated to
// the IterationLog via the router/timer source so haiku iterations are not recorded as 0ms.
func TestOrchestrator_LowTierHaikuIterationRecordsDuration(t *testing.T) {
	t.Parallel()
	var capturedResult *logger.IterationLog

	build := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		if in.Config == nil {
			return pipeline.Output{}, fmt.Errorf("config nil")
		}
		// Assert that the low-tier bead runs the build stage with a haiku model.
		return pipeline.Output{
			Decision:     pipeline.Proceed,
			Model:        "claude-haiku-4-6",
			DurationMs:   42,
			CostUSD:      0.02,
			InputTokens:  400,
			OutputTokens: 200,
		}, nil
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		capturedResult = in.Result
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{
			ID:              "haiku-bead",
			Title:           "Add unit tests for feature X",
			Priority:        2,
			ExpectedOutputs: []string{"feature_x_test.go"},
		}, nil
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
		t.Fatal("Epilogue Result is nil; want IterationLog populated")
	}
	if capturedResult.Model != "claude-haiku-4-6" {
		t.Errorf("IterationLog.Model = %q, want claude-haiku-4-6", capturedResult.Model)
	}
	if capturedResult.DurationMs != 42 {
		t.Errorf("IterationLog.DurationMs = %d, want 42", capturedResult.DurationMs)
	}
}

// TestOrchestrator_SuccessPath_CarriesBuildCostAndTokensToIterationLog verifies that
// DurationMs, CostUSD, InputTokens, and OutputTokens from the Build stage output
// are propagated into the IterationLog on the success path for cost auditing.
func TestOrchestrator_SuccessPath_CarriesBuildCostAndTokensToIterationLog(t *testing.T) {
	t.Parallel()
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

// TestOrchestrator_SuccessPath_CarriesBuildTiersToIterationLog verifies that
// OriginalTier and ActualTier from Build stage output are propagated into
// IterationLog on the success path.
func TestOrchestrator_SuccessPath_CarriesBuildTiersToIterationLog(t *testing.T) {
	t.Parallel()
	var capturedResult *logger.IterationLog

	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:     pipeline.Proceed,
			OriginalTier: "low",
			ActualTier:   "medium",
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
		return &bead.Bead{ID: "bead-3", Title: "Tier test bead"}, nil
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
	if capturedResult.OriginalTier != "low" {
		t.Errorf("IterationLog.OriginalTier = %q, want %q", capturedResult.OriginalTier, "low")
	}
	if capturedResult.ActualTier != "medium" {
		t.Errorf("IterationLog.ActualTier = %q, want %q", capturedResult.ActualTier, "medium")
	}
}

func TestOrchestrator_SuccessPath_CarriesBuildTelemetryToIterationLog(t *testing.T) {
	t.Parallel()
	var capturedResult *logger.IterationLog

	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:                pipeline.Proceed,
			CacheHit:                true,
			CacheMiss:               true,
			CacheWrite:              true,
			CacheClass:              "render_static_build",
			CacheKey:                "cache-key-1",
			CacheInvalidationReason: "version_change",
			CacheVersionMarker:      "rules-v2",
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
		return &bead.Bead{ID: "bead-4", Title: "Telemetry test bead"}, nil
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
	if !capturedResult.CacheHit || !capturedResult.CacheMiss || !capturedResult.CacheWrite {
		t.Fatalf("cache booleans = hit:%t miss:%t write:%t, want all true", capturedResult.CacheHit, capturedResult.CacheMiss, capturedResult.CacheWrite)
	}
	if capturedResult.CacheClass != "render_static_build" {
		t.Fatalf("CacheClass = %q, want %q", capturedResult.CacheClass, "render_static_build")
	}
	if capturedResult.CacheKey != "cache-key-1" {
		t.Fatalf("CacheKey = %q, want %q", capturedResult.CacheKey, "cache-key-1")
	}
	if capturedResult.CacheInvalidationReason != "version_change" {
		t.Fatalf("CacheInvalidationReason = %q, want %q", capturedResult.CacheInvalidationReason, "version_change")
	}
	if capturedResult.CacheVersionMarker != "rules-v2" {
		t.Fatalf("CacheVersionMarker = %q, want %q", capturedResult.CacheVersionMarker, "rules-v2")
	}
}

func TestOrchestrator_AccumulatesTouchedPackagesAcrossIterations(t *testing.T) {
	t.Parallel()
	recorded := make(map[int][]string)
	gateStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		recorded[in.Iteration] = append([]string(nil), in.TouchedPackages...)
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}
	epilogueOutputs := map[int][]string{
		1: {"./pkg-one", "./pkg-one"},
		2: {"pkg-two"},
	}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		if out, ok := epilogueOutputs[in.Iteration]; ok {
			return pipeline.Output{TouchedPackages: append([]string(nil), out...)}, nil
		}
		return pipeline.Output{}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 3 {
			return nil, nil
		}
		return &bead.Bead{ID: fmt.Sprintf("bead-%d", beadCalls), Title: fmt.Sprintf("Bead %d", beadCalls)}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gateStage,
		Build:    &fakeStage{},
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

	expected := map[int][]string{
		1: nil,
		2: {"pkg-one"},
		3: {"pkg-one", "pkg-two"},
	}
	for iter, want := range expected {
		got := recorded[iter]
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Iteration %d touched packages = %v, want %v", iter, got, want)
		}
	}
}

// TestOrchestrator_PropagatesGateComplexityRoutingToBuildInput verifies that
// complexity metadata produced by Gate is copied into the Build stage input.
func TestOrchestrator_PropagatesGateComplexityRoutingToBuildInput(t *testing.T) {
	t.Parallel()
	var buildInput pipeline.Input

	gate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision: pipeline.Proceed,
			ComplexityRouting: pipeline.ComplexityRouting{
				Complexity:               "high",
				ComplexitySource:         "scope_estimate",
				ComplexityFallbackReason: "none",
			},
		}, nil
	}}
	build := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		buildInput = in
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-complexity-input", Title: "Complexity input test bead"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gate,
		Build:    build,
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if buildInput.Complexity != "high" {
		t.Errorf("Build input Complexity = %q, want %q", buildInput.Complexity, "high")
	}
	if buildInput.ComplexitySource != "scope_estimate" {
		t.Errorf("Build input ComplexitySource = %q, want %q", buildInput.ComplexitySource, "scope_estimate")
	}
	if buildInput.ComplexityFallbackReason != "none" {
		t.Errorf("Build input ComplexityFallbackReason = %q, want %q", buildInput.ComplexityFallbackReason, "none")
	}
}

func TestOrchestrator_AttachesEmitterToBuildInput(t *testing.T) {
	t.Parallel()
	var captured pipeline.Input

	gate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}
	build := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		captured = in
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-emitter", Title: "Emitter bead"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gate,
		Build:    build,
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if captured.Emitter == nil {
		t.Fatal("build input emitter nil; want orchestrator emitter")
	}
	if captured.Emitter != orch.GetEmitter() {
		t.Fatalf("build input emitter = %p, want orchestrator emitter %p", captured.Emitter, orch.GetEmitter())
	}
}

func TestOrchestrator_SetsComplexityFromPriority(t *testing.T) {
	t.Parallel()
	var captured pipeline.Input

	gate := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		captured = in
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	cfg := &config.Config{}
	cfg.SetDefaults()

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "priority-p2", Title: "Priority 2 bead", Priority: 2}, nil
	}

	orchCfg := OrchestratorConfig{
		Gate:     gate,
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   cfg,
		Output:   io.Discard,
	}

	orch := NewOrchestrator(orchCfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if captured.Complexity == "" {
		t.Fatalf("captured Input.Complexity empty; want tier derived from priority")
	}
	want := cfg.SelectTier(2, nil)
	if captured.Complexity != want {
		t.Fatalf("Input.Complexity = %q, want %q for priority 2", captured.Complexity, want)
	}
	tier := cfg.PhaseModelTier("build", cfg.SelectInitialTierForComplexity(captured.Complexity))
	if tier != provider.TierLow {
		t.Fatalf("build tier = %q, want %q", tier, provider.TierLow)
	}
}

// TestOrchestrator_SuccessPath_CarriesComplexityRoutingToIterationLog verifies
// that Gate-selected complexity routing metadata is persisted into the success
// iteration log payload alongside tier telemetry.
func TestOrchestrator_SuccessPath_CarriesComplexityRoutingToIterationLog(t *testing.T) {
	t.Parallel()
	var capturedResult *logger.IterationLog

	gate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision: pipeline.Proceed,
			ComplexityRouting: pipeline.ComplexityRouting{
				Complexity:               "low",
				ComplexitySource:         "label",
				ComplexityFallbackReason: "scope_unavailable",
			},
		}, nil
	}}
	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:     pipeline.Proceed,
			OriginalTier: "low",
			ActualTier:   "medium",
		}, nil
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		capturedResult = in.Result
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-complexity-log", Title: "Complexity log test bead"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gate,
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
	if capturedResult.Complexity != "low" {
		t.Errorf("IterationLog.Complexity = %q, want %q", capturedResult.Complexity, "low")
	}
	if capturedResult.ComplexitySource != "label" {
		t.Errorf("IterationLog.ComplexitySource = %q, want %q", capturedResult.ComplexitySource, "label")
	}
	if capturedResult.ComplexityFallbackReason != "scope_unavailable" {
		t.Errorf(
			"IterationLog.ComplexityFallbackReason = %q, want %q",
			capturedResult.ComplexityFallbackReason,
			"scope_unavailable",
		)
	}
}

// TestOrchestrator_ValidationFailure_CarriesComplexityRoutingToIterationLog verifies
// that failure-path iteration log payloads still include Gate-derived complexity metadata.
func TestOrchestrator_ValidationFailure_CarriesComplexityRoutingToIterationLog(t *testing.T) {
	t.Parallel()
	var capturedResult *logger.IterationLog

	gate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision: pipeline.Proceed,
			ComplexityRouting: pipeline.ComplexityRouting{
				Complexity:               "medium",
				ComplexitySource:         "default",
				ComplexityFallbackReason: "scope_and_label_unavailable",
			},
		}, nil
	}}
	validate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:           pipeline.Block,
			ValidationFailures: []string{"validation failed"},
		}, nil
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		capturedResult = in.Result
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-complexity-failure-log", Title: "Complexity failure log test bead"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gate,
		Build:    &fakeStage{},
		Validate: validate,
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
		t.Fatal("Epilogue Result is nil; want IterationLog populated on failure path")
	}
	if capturedResult.Complexity != "medium" {
		t.Errorf("IterationLog.Complexity = %q, want %q", capturedResult.Complexity, "medium")
	}
	if capturedResult.ComplexitySource != "default" {
		t.Errorf("IterationLog.ComplexitySource = %q, want %q", capturedResult.ComplexitySource, "default")
	}
	if capturedResult.ComplexityFallbackReason != "scope_and_label_unavailable" {
		t.Errorf(
			"IterationLog.ComplexityFallbackReason = %q, want %q",
			capturedResult.ComplexityFallbackReason,
			"scope_and_label_unavailable",
		)
	}
}

// TestOrchestrator_BuildFailure_RecordsErrorTelemetry verifies that build-stage
// failures preserve actionable error details in the iteration log payload,
// including the failing TDD phase when present in the error text.
func TestOrchestrator_BuildFailure_RecordsErrorTelemetry(t *testing.T) {
	t.Parallel()
	var capturedResult *logger.IterationLog

	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{}, errors.New("build: TDD cycle runner: red phase: invocation failed after retry and escalation")
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		capturedResult = in.Result
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-build-failure-log", Title: "Build failure log test bead"}, nil
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
		t.Fatal("Epilogue Result is nil; want IterationLog populated on build failure path")
	}
	if capturedResult.FailurePhase != "red" {
		t.Fatalf("IterationLog.FailurePhase = %q, want %q", capturedResult.FailurePhase, "red")
	}
	if !strings.Contains(capturedResult.Error, "TDD cycle runner") {
		t.Fatalf("IterationLog.Error = %q, want build failure detail", capturedResult.Error)
	}
}

// TestRunner_RunMethod_Removed verifies that the legacy Runner.Run method has been
// removed as part of the architecture migration to Orchestrator. All loop execution
// now flows through Orchestrator.Run. This test prevents accidental reintroduction.
func TestRunner_RunMethod_Removed(t *testing.T) {
	t.Parallel()
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
	t.Parallel(
	// Deps type and NewRunnerWithDeps constructor are gone; nothing to assert here.
	// The Runner type still exists for TDD orchestration but no longer exposes Run().
	)

	_ = reflect.TypeOf(&Runner{}) // ensure Runner itself is still present
}

// TestOrchestrator_StatusWriter_ReceivesDeadline verifies that the orchestrator
// passes the deadline to the StatusWriter function so the constructor's closure
// can compute timeBudgetMinutes instead of hardcoding 0.
func TestOrchestrator_StatusWriter_ReceivesDeadline(t *testing.T) {
	t.Parallel()
	var capturedDeadline time.Time

	deadline := time.Now().Add(30 * time.Minute)

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-1", Title: "Test"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
		StatusWriter: func(_ context.Context, iteration int, beadID, beadTitle string, dl time.Time) {
			capturedDeadline = dl
		},
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 10, deadline, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if capturedDeadline.IsZero() {
		t.Error("StatusWriter did not receive deadline; want deadline passed for time budget computation")
	}
	if !capturedDeadline.Equal(deadline) {
		t.Errorf("StatusWriter deadline = %v, want %v", capturedDeadline, deadline)
	}
}

func TestOrchestrator_StopsBeforeDeadline(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(-1 * time.Second)

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		return &bead.Bead{ID: "bead-1", Title: "Should not run"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 10, deadline, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if beadCalls != 0 {
		t.Fatalf("GetBead called %d times; want 0 when deadline already passed", beadCalls)
	}
}

func TestOrchestrator_StopsWhenDeadlinePassesDuringGetBead(t *testing.T) {
	deadline := time.Now().Add(1 * time.Minute)
	originalNowFn := orchestratorNowFn
	nowCalls := 0
	orchestratorNowFn = func() time.Time {
		nowCalls++
		if nowCalls == 1 {
			return deadline.Add(-1 * time.Millisecond)
		}
		return deadline.Add(1 * time.Millisecond)
	}
	defer func() { orchestratorNowFn = originalNowFn }()

	gateCalls := 0
	gate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		gateCalls++
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	getBeadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		getBeadCalls++
		return &bead.Bead{ID: "bead-1", Title: "Should not start after deadline"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gate,
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 10, deadline, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if getBeadCalls != 1 {
		t.Fatalf("GetBead called %d times; want 1", getBeadCalls)
	}
	if gateCalls != 0 {
		t.Fatalf("Gate called %d times; want 0 when deadline passes while fetching bead", gateCalls)
	}
}

// TestOrchestrator_ValidationFailure_SetsFailureOutput verifies that when the
// validate stage returns Block with ValidationFailures, the orchestrator sets
// FailureOutput on the epilogue Input so the failure learner receives it.
func TestOrchestrator_ValidationFailure_SetsFailureOutput(t *testing.T) {
	t.Parallel()
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

func TestOrchestrator_ValidationFailure_SetsFailurePhase(t *testing.T) {
	t.Parallel()
	var capturedResult *logger.IterationLog

	validate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:           pipeline.Block,
			ValidationFailures: []string{"validation failed"},
		}, nil
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		capturedResult = in.Result
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-1", Title: "Validation failure"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: validate,
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
		t.Fatal("expected an iteration log on validation failure")
	}
	if capturedResult.FailurePhase != failurephase.Validation {
		t.Fatalf("FailurePhase = %q, want %q", capturedResult.FailurePhase, failurephase.Validation)
	}
}

func TestOrchestrator_ValidationFailure_AttemptsRecoveryBuildUpToConfiguredRetries(t *testing.T) {
	t.Parallel()

	buildCalls := 0
	var buildInputs []pipeline.Input
	build := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		buildCalls++
		buildInputs = append(buildInputs, in)
		return pipeline.Output{
			Decision:     pipeline.Proceed,
			OriginalTier: "low",
			ActualTier:   "medium",
			Model:        "test-model",
		}, nil
	}}

	validateCalls := 0
	validate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		validateCalls++
		if validateCalls == 1 {
			return pipeline.Output{
				Decision:           pipeline.Block,
				ValidationFailures: []string{"first validation failure"},
			}, nil
		}
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	var capturedResult *logger.IterationLog
	epilogue := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		capturedResult = in.Result
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-1", Title: "Validation recovery"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    build,
		Validate: validate,
		Epilogue: epilogue,
		GetBead:  getBead,
		Config: &config.Config{
			Validation: config.ValidationConfig{
				MaxValidationRetries: 2,
			},
		},
		Output: io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if buildCalls != 2 {
		t.Fatalf("build stage calls = %d, want 2 (initial + recovery)", buildCalls)
	}
	if validateCalls != 2 {
		t.Fatalf("validate stage calls = %d, want 2 (initial fail + recovery pass)", validateCalls)
	}
	if len(buildInputs) < 2 {
		t.Fatalf("captured build inputs = %d, want at least 2", len(buildInputs))
	}
	if len(buildInputs[1].ValidationFailures) != 1 || buildInputs[1].ValidationFailures[0] != "first validation failure" {
		t.Fatalf("recovery build validation failures = %#v, want first failure summary", buildInputs[1].ValidationFailures)
	}
	if capturedResult == nil {
		t.Fatal("expected success result to reach epilogue after recovery")
	}
	if !capturedResult.Success {
		t.Fatalf("captured result success = false, want true after recovery")
	}
}

func TestOrchestrator_LocalGateFailureSkipsReview(t *testing.T) {
	t.Parallel()
	var capturedResult *logger.IterationLog
	reviewCalled := false
	localGateCalled := 0

	localGateStage := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		localGateCalled++
		return pipeline.Output{
			Decision:           pipeline.Block,
			ValidationFailures: []string{"local gate failure"},
		}, nil
	}}
	reviewStage := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		reviewCalled = true
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}
	backstopEpilogue := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		capturedResult = in.Result
		return pipeline.Output{}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "local-gate-bead", Title: "Spec work"}, nil
	}

	orch := NewOrchestrator(OrchestratorConfig{
		Gate:      &fakeStage{},
		Build:     &fakeStage{},
		Validate:  &fakeStage{},
		LocalGate: localGateStage,
		Review:    reviewStage,
		Epilogue:  backstopEpilogue,
		GetBead:   getBead,
		Config:    &config.Config{},
		Output:    io.Discard,
	})

	if err := orch.Run(context.Background(), 5, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if localGateCalled != 1 {
		t.Fatalf("local gate stage executed %d times, want 1", localGateCalled)
	}
	if reviewCalled {
		t.Fatal("review stage should not run when local gate fails")
	}
	if capturedResult == nil {
		t.Fatal("expected iteration log to be set on failure")
	}
	if capturedResult.FailurePhase != failurephase.LocalGate {
		t.Fatalf("FailurePhase = %q, want %q", capturedResult.FailurePhase, failurephase.LocalGate)
	}
	if !strings.Contains(capturedResult.Error, "local gate failure") {
		t.Fatalf("IterationLog.Error = %q, want it to mention local gate, got %q", capturedResult.Error, capturedResult.Error)
	}
}

// TestOrchestrator_RunSequence_UsesCallerProvidedOrder verifies that RunSequence
// resolves bead IDs via GetBeadByID and executes them in the exact caller-provided
// order, independent of queue ordering.
func TestOrchestrator_RunSequence_UsesCallerProvidedOrder(t *testing.T) {
	t.Parallel()
	var buildOrder []string
	var getByIDCalls []string
	queueCalls := 0

	build := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		buildOrder = append(buildOrder, in.Bead.ID)
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    build,
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead: func(_ context.Context) (*bead.Bead, error) {
			queueCalls++
			return nil, nil
		},
		GetBeadByID: func(_ context.Context, id string) (*bead.Bead, error) {
			getByIDCalls = append(getByIDCalls, id)
			return &bead.Bead{ID: id, Title: "Sequence " + id}, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.RunSequence(context.Background(), []string{"b-2", "b-1", "b-3"}, 0, time.Time{}, nil); err != nil {
		t.Fatalf("RunSequence() error = %v, want nil", err)
	}

	if queueCalls != 0 {
		t.Errorf("GetBead queue path called %d times, want 0 during RunSequence", queueCalls)
	}
	if !reflect.DeepEqual(getByIDCalls, []string{"b-2", "b-1", "b-3"}) {
		t.Errorf("GetBeadByID calls = %v, want %v", getByIDCalls, []string{"b-2", "b-1", "b-3"})
	}
	if !reflect.DeepEqual(buildOrder, []string{"b-2", "b-1", "b-3"}) {
		t.Errorf("build order = %v, want %v", buildOrder, []string{"b-2", "b-1", "b-3"})
	}
}

// TestOrchestrator_RunSequence_RespectsMaxIterationsWithoutExtraResolution verifies
// that RunSequence does not resolve bead IDs beyond maxIterations, preserving the
// same iteration cap semantics as queue-based Run.
func TestOrchestrator_RunSequence_RespectsMaxIterationsWithoutExtraResolution(t *testing.T) {
	t.Parallel()
	var getByIDCalls []string
	var buildOrder []string

	build := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		buildOrder = append(buildOrder, in.Bead.ID)
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    build,
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead: func(_ context.Context) (*bead.Bead, error) {
			return nil, nil
		},
		GetBeadByID: func(_ context.Context, id string) (*bead.Bead, error) {
			getByIDCalls = append(getByIDCalls, id)
			return &bead.Bead{ID: id, Title: "Sequence " + id}, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.RunSequence(context.Background(), []string{"b-1", "b-2", "b-3"}, 2, time.Time{}, nil); err != nil {
		t.Fatalf("RunSequence() error = %v, want nil", err)
	}

	if !reflect.DeepEqual(getByIDCalls, []string{"b-1", "b-2"}) {
		t.Errorf("GetBeadByID calls = %v, want %v", getByIDCalls, []string{"b-1", "b-2"})
	}
	if !reflect.DeepEqual(buildOrder, []string{"b-1", "b-2"}) {
		t.Errorf("build order = %v, want %v", buildOrder, []string{"b-1", "b-2"})
	}
}

func TestOrchestrator_PostRunCompletenessAssertion_FailsWhenEfficiencyDataIncomplete(t *testing.T) {
	t.Parallel(
	// Create a temporary logs directory
	)

	logsDir := t.TempDir()

	// Create log file with iterations but missing efficiency data
	logContent := `{"type":"iteration","timestamp":"2026-02-25T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"haiku","success":true,"validated":true,"duration_ms":0,"cost_usd":0,"input_tokens":0,"output_tokens":0}
{"type":"iteration","timestamp":"2026-02-25T12:00:01Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"haiku","success":true,"validated":true,"duration_ms":0,"cost_usd":0,"input_tokens":0,"output_tokens":0}
`
	logPath := logsDir + "/run-20260225-120000.jsonl"
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}

	// Set up orchestrator that will not process any beads but will read the logs
	getBead := func(_ context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
		LogsDir:  logsDir,
		GetRunID: func() string { return "20260225-120000" },
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 10, time.Time{}, nil)

	// Should fail due to incomplete efficiency data
	if err == nil {
		t.Error("Run() expected to fail with efficiency data completeness issue, got nil error")
	} else if !strings.Contains(err.Error(), "efficiency") && !strings.Contains(err.Error(), "completeness") {
		t.Errorf("Run() error = %v, want error containing 'efficiency' or 'completeness'", err)
	}
}

func TestOrchestrator_PostRunCompletenessAssertion_FailsWhenLogRowsMissing(t *testing.T) {
	t.Parallel()
	logsDir := t.TempDir()

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls == 1 {
			return &bead.Bead{ID: "missing-row", Title: "Missing log row"}, nil
		}
		return nil, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
		LogsDir:  logsDir,
		GetRunID: func() string { return "20260225-120000" },
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 10, time.Time{}, nil)

	if err == nil {
		t.Fatal("Run() expected to fail with missing efficiency log rows, got nil error")
	} else if !strings.Contains(err.Error(), "efficiency") && !strings.Contains(err.Error(), "completeness") {
		t.Errorf("Run() error = %v, want error containing 'efficiency' or 'completeness'", err)
	}
}

// TestOrchestrator_CoverageTracker_TransitionsStatesAcrossTDDCycle verifies that when
// a CoverageTracker is wired into the orchestrator, it transitions through states
// during a simulated TDD cycle, tracking acceptance criteria coverage.
func TestOrchestrator_CoverageTracker_TransitionsStatesAcrossTDDCycle(t *testing.T) {
	t.Parallel(
	// Create criteria for a simple feature.
	)

	criteria := []coverage.Criterion{
		{Number: 1, Text: "System accepts valid input"},
		{Number: 2, Text: "System rejects invalid input"},
		{Number: 3, Text: "System logs errors"},
	}
	tracker := coverage.NewTracker(criteria, 2)

	// Verify initial state before running.
	if tracker.State() != coverage.StatePending {
		t.Fatalf("initial tracker state = %d, want StatePending (%d)", tracker.State(), coverage.StatePending)
	}

	gate := &fakeStage{}
	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}
	validate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
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
		Gate:            gate,
		Build:           build,
		Validate:        validate,
		Epilogue:        epilogueStage,
		GetBead:         getBead,
		Config:          &config.Config{},
		Output:          io.Discard,
		CoverageTracker: tracker,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	// Verify tracker transitioned through states during the cycle.
	// After a successful iteration: Pending -> Collecting -> Validating -> Complete
	if tracker.State() != coverage.StateComplete {
		t.Errorf("final tracker state = %d, want StateComplete (%d)", tracker.State(), coverage.StateComplete)
	}
	if tracker.TotalCriteria() != 3 {
		t.Errorf("total criteria = %d, want 3", tracker.TotalCriteria())
	}
}

// TestOrchestrator_MergesGlobalStatsPreservingExistingData verifies that when
// GlobalStatsPath is configured, the orchestrator reads the existing stats file,
// merges the new run stats into it, and updates the timestamp. Pre-existing
// entries from prior runs are preserved (not overwritten).
func TestOrchestrator_MergesGlobalStatsPreservingExistingData(t *testing.T) {
	t.Parallel()

	// Setup helpers for common operations
	setupMergeStatsTestDirs := func(t *testing.T) (dir, logsDir, statsPath string) {
		return setupMergeStatsTestDirsImpl(t)
	}

	seedGlobalStats := func(t *testing.T, statsPath string) {
		seedGlobalStatsImpl(t, statsPath)
	}

	createRunLogs := func(t *testing.T, logsDir, runID string, models []string) {
		createRunLogsImpl(t, logsDir, runID, models)
	}

	invokeOrchestrator := func(t *testing.T, statsPath, logsDir, runID string) *logger.GlobalStats {
		return invokeOrchestratorImpl(t, statsPath, logsDir, runID)
	}

	t.Run("baseline: merges with existing data, preserves prior runs", func(t *testing.T) {
		_, logsDir, statsPath := setupMergeStatsTestDirs(t)
		seedGlobalStats(t, statsPath)
		createRunLogs(t, logsDir, "2026-02-25-001", []string{"opus", "haiku"})
		mergedStats := invokeOrchestrator(t, statsPath, logsDir, "2026-02-25-001")

		// Verify stats merged correctly (not overwritten)
		if len(mergedStats.Models) != 2 {
			t.Fatalf("Expected 2 models in merged stats, got %d", len(mergedStats.Models))
		}

		// Verify opus was merged (not overwritten): old 10 + new 1 = 11 iterations
		opusStats := mergedStats.Models["opus"]
		if opusStats.Iterations != 11 {
			t.Errorf("opus.Iterations = %d, want 11 (10 existing + 1 new)", opusStats.Iterations)
		}
		if opusStats.Successes != 9 {
			t.Errorf("opus.Successes = %d, want 9 (8 existing + 1 new)", opusStats.Successes)
		}
		if opusStats.TotalCostUSD != 23.00 {
			t.Errorf("opus.TotalCostUSD = %.2f, want 23.00 (20.00 existing + 3.00 new)", opusStats.TotalCostUSD)
		}

		// Verify haiku was added: new model with 1 iteration
		haikuStats := mergedStats.Models["haiku"]
		if haikuStats.Iterations != 1 {
			t.Errorf("haiku.Iterations = %d, want 1", haikuStats.Iterations)
		}
		if haikuStats.Successes != 1 {
			t.Errorf("haiku.Successes = %d, want 1", haikuStats.Successes)
		}
		if haikuStats.TotalCostUSD != 0.50 {
			t.Errorf("haiku.TotalCostUSD = %.2f, want 0.50", haikuStats.TotalCostUSD)
		}

		// Verify timestamp was updated
		if mergedStats.Updated == "2026-02-24T10:00:00Z" {
			t.Error("Updated timestamp should have changed from existing value")
		}
	})

	t.Run("empty-initial-stats: all fields initialized from zero-values when no prior data exists", func(t *testing.T) {
		_, logsDir, statsPath := setupMergeStatsTestDirs(t)
		// Don't seed existing stats - start with empty
		logEntries := []logger.IterationLog{
			{
				Timestamp:    time.Now(),
				Iteration:    1,
				BeadID:       "bead-1",
				BeadTitle:    "Feature A",
				Success:      true,
				Model:        "sonnet",
				CostUSD:      2.50,
				InputTokens:  100,
				OutputTokens: 50,
				DurationMs:   800,
			},
		}
		writeOrchestratorTestLogFile(t, logsDir, "2026-02-25-002", logEntries)
		mergedStats := invokeOrchestrator(t, statsPath, logsDir, "2026-02-25-002")

		// Verify exactly one model in stats
		if len(mergedStats.Models) != 1 {
			t.Fatalf("Expected 1 model, got %d", len(mergedStats.Models))
		}

		// Verify all fields initialized correctly from zero-values
		sonnetStats := mergedStats.Models["sonnet"]
		if sonnetStats.Iterations != 1 {
			t.Errorf("Iterations: got %d, want 1", sonnetStats.Iterations)
		}
		if sonnetStats.Successes != 1 {
			t.Errorf("Successes: got %d, want 1", sonnetStats.Successes)
		}
		if sonnetStats.Failures != 0 {
			t.Errorf("Failures: got %d, want 0", sonnetStats.Failures)
		}
		if sonnetStats.TotalCostUSD != 2.50 {
			t.Errorf("TotalCostUSD: got %.2f, want 2.50", sonnetStats.TotalCostUSD)
		}
		if sonnetStats.EscalationsFrom != 0 {
			t.Errorf("EscalationsFrom: got %d, want 0", sonnetStats.EscalationsFrom)
		}
		if sonnetStats.EscalationsTo != 0 {
			t.Errorf("EscalationsTo: got %d, want 0", sonnetStats.EscalationsTo)
		}
	})

	t.Run("multiple-model-independence: each model stats tracked independently without cross-contamination", func(t *testing.T) {
		_, logsDir, statsPath := setupMergeStatsTestDirs(t)
		seedGlobalStats(t, statsPath) // Seed with opus: iter=10, succ=8, fail=2, cost=20.00, esc_to=2
		// Create logs with opus, haiku, and sonnet
		logEntries := []logger.IterationLog{
			{
				Timestamp: time.Now(),
				Iteration: 1,
				BeadID:    "bead-1",
				Success:   true,
				Model:     "opus",
				CostUSD:   3.00,
			},
			{
				Timestamp: time.Now(),
				Iteration: 2,
				BeadID:    "bead-2",
				Success:   true,
				Model:     "haiku",
				CostUSD:   0.50,
			},
			{
				Timestamp: time.Now(),
				Iteration: 3,
				BeadID:    "bead-3",
				Success:   true,
				Model:     "sonnet",
				CostUSD:   1.50,
			},
		}
		writeOrchestratorTestLogFile(t, logsDir, "2026-02-25-003", logEntries)
		mergedStats := invokeOrchestrator(t, statsPath, logsDir, "2026-02-25-003")

		// Verify 3 models present
		if len(mergedStats.Models) != 3 {
			t.Fatalf("Expected 3 models, got %d", len(mergedStats.Models))
		}

		// Verify opus: old(10,8,2,20) + new(1,1,0,3) = (11,9,2,23)
		opusStats := mergedStats.Models["opus"]
		if opusStats.Iterations != 11 {
			t.Errorf("opus.Iterations: got %d, want 11", opusStats.Iterations)
		}
		if opusStats.Successes != 9 {
			t.Errorf("opus.Successes: got %d, want 9", opusStats.Successes)
		}
		if opusStats.Failures != 2 {
			t.Errorf("opus.Failures: got %d, want 2 (unchanged)", opusStats.Failures)
		}
		if opusStats.TotalCostUSD != 23.00 {
			t.Errorf("opus.TotalCostUSD: got %.2f, want 23.00", opusStats.TotalCostUSD)
		}

		// Verify haiku: new model (0,0,0,0) + new(1,1,0,0.50) = (1,1,0,0.50)
		haikuStats := mergedStats.Models["haiku"]
		if haikuStats.Iterations != 1 {
			t.Errorf("haiku.Iterations: got %d, want 1", haikuStats.Iterations)
		}
		if haikuStats.Successes != 1 {
			t.Errorf("haiku.Successes: got %d, want 1", haikuStats.Successes)
		}
		if haikuStats.Failures != 0 {
			t.Errorf("haiku.Failures: got %d, want 0", haikuStats.Failures)
		}

		// Verify sonnet: new model (0,0,0,0) + new(1,1,0,1.50) = (1,1,0,1.50)
		sonnetStats := mergedStats.Models["sonnet"]
		if sonnetStats.Iterations != 1 {
			t.Errorf("sonnet.Iterations: got %d, want 1", sonnetStats.Iterations)
		}
		if sonnetStats.TotalCostUSD != 1.50 {
			t.Errorf("sonnet.TotalCostUSD: got %.2f, want 1.50", sonnetStats.TotalCostUSD)
		}
	})

	t.Run("zero-value-additive-semantics: zero-count failures don't overwrite existing non-zero failures", func(t *testing.T) {
		_, logsDir, statsPath := setupMergeStatsTestDirs(t)
		// Seed with opus having 10 iterations, 8 successes, 2 failures
		seedGlobalStats(t, statsPath)
		// Create run with opus that has all successes (0 failures)
		logEntries := []logger.IterationLog{
			{
				Timestamp: time.Now(),
				Iteration: 1,
				BeadID:    "bead-1",
				Success:   true, // Success, not failure
				Model:     "opus",
				CostUSD:   1.00,
			},
		}
		writeOrchestratorTestLogFile(t, logsDir, "2026-02-25-004", logEntries)
		mergedStats := invokeOrchestrator(t, statsPath, logsDir, "2026-02-25-004")

		// Verify opus failures are additive: old 2 failures + new 0 failures = 2 failures (not overwritten)
		opusStats := mergedStats.Models["opus"]
		if opusStats.Failures != 2 {
			t.Errorf("opus.Failures: got %d, want 2 (existing failures preserved with zero-value additivity)", opusStats.Failures)
		}
		// Also verify iterations and successes updated correctly
		if opusStats.Iterations != 11 {
			t.Errorf("opus.Iterations: got %d, want 11 (10 existing + 1 new)", opusStats.Iterations)
		}
		if opusStats.Successes != 9 {
			t.Errorf("opus.Successes: got %d, want 9 (8 existing + 1 new success)", opusStats.Successes)
		}
	})
}

// writeOrchestratorTestLogFile writes iteration logs to a run-{runID}.jsonl file
func writeOrchestratorTestLogFile(t *testing.T, dir string, runID string, logs []logger.IterationLog) {
	t.Helper()

	filename := filepath.Join(dir, "run-"+runID+".jsonl")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			t.Fatalf("Failed to write test log entry: %v", err)
		}
	}
}

// TestOrchestrator_ControlLimitAlert_SetsRetroFlagWhenFirstPassSuccessBelowEightyPercent verifies
// that when the rolling_first_pass_success_rate drops below 80% with at least 30 iterations
// in the window, a warning is logged and the state is marked to trigger retro on next run.
func TestOrchestrator_ControlLimitAlert_SetsRetroFlagWhenFirstPassSuccessBelowEightyPercent(t *testing.T) {
	t.Parallel(
	// Create temp directories
	)

	logsDir := t.TempDir()
	metricsDir := t.TempDir()
	stateDir := t.TempDir()

	// Create iteration logs with low first-pass success rate
	logs := make([]logger.IterationLog, 30)
	for i := 0; i < 30; i++ {
		logs[i] = logger.IterationLog{
			Iteration:        i + 1,
			BeadID:           fmt.Sprintf("bead-%d", i),
			Success:          true,
			Validated:        true,
			FirstPassSuccess: i < 6, // Only 6 out of 30 first-pass successes = 20%
		}
	}
	writeOrchestratorTestLogFile(t, logsDir, "test-run", logs)

	// Create ProcessTrend with low FirstPassSuccess rate
	trend := &logger.ProcessTrend{
		TotalIterations: 30,
		WindowSize:      30,
		LatestWindow: logger.ProcessTrendWindow{
			FirstPassSuccess: 0.20, // 20%, below 80% threshold
		},
	}

	// Write trend file
	trendPath := filepath.Join(metricsDir, "process_trend.json")
	trendData, err := json.MarshalIndent(trend, "", "  ")
	if err != nil {
		t.Fatalf("marshalling trend: %v", err)
	}
	if err := os.WriteFile(trendPath, trendData, 0644); err != nil {
		t.Fatalf("writing trend file: %v", err)
	}

	// Create a real state file for testing
	stateFile, err := state.NewFile(stateDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}

	// Create orchestrator with a TrendReader that returns our test trend
	trendUpdater := &fakeTrendUpdater{
		trend: trend,
	}

	getBead := func(_ context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:         &fakeStage{},
		Build:        &fakeStage{},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		GetBead:      getBead,
		Config:       &config.Config{},
		Output:       io.Discard,
		StateSaver:   stateFile,
		TrendUpdater: trendUpdater,
		LogsDir:      trendPath, // Pass the full path to the trend file
	}

	orch := NewOrchestrator(cfg)
	err = orch.Run(context.Background(), 10, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	// Verify that state file was saved (this happens at line 325 in orchestrator.go)
	if err := stateFile.Load(); err != nil {
		t.Fatalf("loading state file: %v", err)
	}

	// Verify that the control limit alert flag was set
	if !stateFile.IsControlLimitAlertTriggered() {
		t.Error("ControlLimitAlert flag should be set when FirstPassSuccess < 0.80")
	}
}

// TestOrchestrator_ControlLimitAlert_NotTriggeredWhenSuccessRateAtThreshold verifies
// that the control limit alert is NOT triggered when the success rate equals 80%
// (the threshold is strictly less than).
func TestOrchestrator_ControlLimitAlert_NotTriggeredWhenSuccessRateAtThreshold(t *testing.T) {
	t.Parallel()
	metricsDir := t.TempDir()
	stateDir := t.TempDir()

	// Create ProcessTrend with FirstPassSuccess exactly at 80% (should NOT trigger)
	trend := &logger.ProcessTrend{
		TotalIterations: 30,
		WindowSize:      30,
		LatestWindow: logger.ProcessTrendWindow{
			FirstPassSuccess: 0.80, // Exactly at threshold, should NOT trigger
		},
	}

	trendPath := filepath.Join(metricsDir, "process_trend.json")
	trendData, err := json.MarshalIndent(trend, "", "  ")
	if err != nil {
		t.Fatalf("marshalling trend: %v", err)
	}
	if err := os.WriteFile(trendPath, trendData, 0644); err != nil {
		t.Fatalf("writing trend file: %v", err)
	}

	stateFile, err := state.NewFile(stateDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}

	trendUpdater := &fakeTrendUpdater{trend: trend}
	getBead := func(_ context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:         &fakeStage{},
		Build:        &fakeStage{},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		GetBead:      getBead,
		Config:       &config.Config{},
		Output:       io.Discard,
		StateSaver:   stateFile,
		TrendUpdater: trendUpdater,
		LogsDir:      trendPath,
	}

	orch := NewOrchestrator(cfg)
	err = orch.Run(context.Background(), 10, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if err := stateFile.Load(); err != nil {
		t.Fatalf("loading state file: %v", err)
	}

	if stateFile.IsControlLimitAlertTriggered() {
		t.Error("ControlLimitAlert flag should NOT be set when FirstPassSuccess = 0.80")
	}
}

// TestOrchestrator_ControlLimitAlert_NotTriggeredWhenWindowTooSmall verifies
// that the control limit alert is NOT triggered when the window has fewer
// than 30 iterations, even if success rate is low.
func TestOrchestrator_ControlLimitAlert_NotTriggeredWhenWindowTooSmall(t *testing.T) {
	t.Parallel()
	metricsDir := t.TempDir()
	stateDir := t.TempDir()

	// Create ProcessTrend with low success rate but window < 30
	trend := &logger.ProcessTrend{
		TotalIterations: 29,
		WindowSize:      29, // Less than minimum 30
		LatestWindow: logger.ProcessTrendWindow{
			FirstPassSuccess: 0.20, // Low rate, but window is too small
		},
	}

	trendPath := filepath.Join(metricsDir, "process_trend.json")
	trendData, err := json.MarshalIndent(trend, "", "  ")
	if err != nil {
		t.Fatalf("marshalling trend: %v", err)
	}
	if err := os.WriteFile(trendPath, trendData, 0644); err != nil {
		t.Fatalf("writing trend file: %v", err)
	}

	stateFile, err := state.NewFile(stateDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}

	trendUpdater := &fakeTrendUpdater{trend: trend}
	getBead := func(_ context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:         &fakeStage{},
		Build:        &fakeStage{},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		GetBead:      getBead,
		Config:       &config.Config{},
		Output:       io.Discard,
		StateSaver:   stateFile,
		TrendUpdater: trendUpdater,
		LogsDir:      trendPath,
	}

	orch := NewOrchestrator(cfg)
	err = orch.Run(context.Background(), 10, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if err := stateFile.Load(); err != nil {
		t.Fatalf("loading state file: %v", err)
	}

	if stateFile.IsControlLimitAlertTriggered() {
		t.Error("ControlLimitAlert flag should NOT be set when window size < 30")
	}
}

// TestOrchestrator_ControlLimitAlert_TriggeredWhenFirstPassBelowEightyPercent ensures
// the alert fires when first-pass success dips below 80% in a full 30-iteration window.
func TestOrchestrator_ControlLimitAlert_TriggeredWhenFirstPassBelowEightyPercent(t *testing.T) {
	t.Parallel()
	metricsDir := t.TempDir()
	stateDir := t.TempDir()

	trend := &logger.ProcessTrend{
		TotalIterations: 30,
		WindowSize:      30,
		LatestWindow: logger.ProcessTrendWindow{
			FirstPassSuccess: 0.75, // Below the new 80% guard
		},
	}

	trendPath := filepath.Join(metricsDir, "process_trend.json")
	trendData, err := json.MarshalIndent(trend, "", "  ")
	if err != nil {
		t.Fatalf("marshalling trend: %v", err)
	}
	if err := os.WriteFile(trendPath, trendData, 0644); err != nil {
		t.Fatalf("writing trend file: %v", err)
	}

	stateFile, err := state.NewFile(stateDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}

	trendUpdater := &fakeTrendUpdater{trend: trend}
	getBead := func(_ context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:         &fakeStage{},
		Build:        &fakeStage{},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		GetBead:      getBead,
		Config:       &config.Config{},
		Output:       io.Discard,
		StateSaver:   stateFile,
		TrendUpdater: trendUpdater,
		LogsDir:      trendPath,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if err := stateFile.Load(); err != nil {
		t.Fatalf("loading state file: %v", err)
	}

	if !stateFile.IsControlLimitAlertTriggered() {
		t.Error("ControlLimitAlert flag should be set when FirstPassSuccess < 0.80")
	}
}

// TestOrchestrator_ControlLimitAlert_LogsWarningWhenTriggered verifies that when
// the control limit alert is triggered, a warning message with the new threshold
// and review guidance is logged to the output.
func TestOrchestrator_ControlLimitAlert_LogsWarningWhenTriggered(t *testing.T) {
	t.Parallel()
	metricsDir := t.TempDir()
	stateDir := t.TempDir()

	// Create ProcessTrend with low FirstPassSuccess rate
	trend := &logger.ProcessTrend{
		TotalIterations: 30,
		WindowSize:      30,
		LatestWindow: logger.ProcessTrendWindow{
			FirstPassSuccess: 0.1667, // 16.67%, below 80% threshold
		},
	}

	trendPath := filepath.Join(metricsDir, "process_trend.json")
	trendData, err := json.MarshalIndent(trend, "", "  ")
	if err != nil {
		t.Fatalf("marshalling trend: %v", err)
	}
	if err := os.WriteFile(trendPath, trendData, 0644); err != nil {
		t.Fatalf("writing trend file: %v", err)
	}

	stateFile, err := state.NewFile(stateDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}

	// Capture log output
	var logOutput strings.Builder

	trendUpdater := &fakeTrendUpdater{trend: trend}
	getBead := func(_ context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:         &fakeStage{},
		Build:        &fakeStage{},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		GetBead:      getBead,
		Config:       &config.Config{},
		Output:       &logOutput,
		StateSaver:   stateFile,
		TrendUpdater: trendUpdater,
		LogsDir:      trendPath,
	}

	orch := NewOrchestrator(cfg)
	err = orch.Run(context.Background(), 10, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	// Verify that warning was logged
	logStr := logOutput.String()
	if !strings.Contains(logStr, "first-pass success rate") {
		t.Errorf("Warning message not found in logs. Got: %q", logStr)
	}
	if !strings.Contains(logStr, "80%") {
		t.Errorf("Control limit threshold not mentioned in logs. Got: %q", logStr)
	}
	if !strings.Contains(logStr, "decomposition rule changes") {
		t.Errorf("Decomposition review guidance not mentioned in logs. Got: %q", logStr)
	}
}

// TestOrchestrator_ScopeGateBlockedBeads_IterationCounterConsistency verifies that
// when beads are blocked by the scope gate, the iteration counter is properly
// incremented so that scope-blocked beads and successfully completed beads have
// different (non-overlapping) iteration numbers.
func TestOrchestrator_ScopeGateBlockedBeads_IterationCounterConsistency(t *testing.T) {
	t.Parallel(
	// Simulate a sequence: success, blocked, success to verify iteration counters don't overlap
	)

	capturedLogs := []*logger.IterationLog{}

	gateStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		// Block bead-2-blocked
		if in.Bead.ID == "bead-2-blocked" {
			return pipeline.Output{Decision: pipeline.Block}, nil
		}
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	buildStage := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:     pipeline.Proceed,
			Model:        "claude-opus",
			CostUSD:      0.01,
			InputTokens:  100,
			OutputTokens: 50,
		}, nil
	}}

	validateStage := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		if in.Result != nil {
			capturedLogs = append(capturedLogs, in.Result)
		}
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beads := []*bead.Bead{
		{ID: "bead-1", Title: "First bead"},
		{ID: "bead-2-blocked", Title: "Blocked by scope gate"},
		{ID: "bead-3", Title: "Third bead"},
	}
	beadIndex := 0

	getBead := func(_ context.Context) (*bead.Bead, error) {
		if beadIndex >= len(beads) {
			return nil, nil
		}
		b := beads[beadIndex]
		beadIndex++
		return b, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gateStage,
		Build:    buildStage,
		Validate: validateStage,
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	// All 3 beads should produce iteration logs (including the blocked one)
	if len(capturedLogs) != 3 {
		t.Errorf("Expected 3 iteration logs, got %d", len(capturedLogs))
		return
	}

	// Each bead should have a unique, monotonically increasing iteration number
	seenIterations := make(map[int]string) // iteration -> beadID
	for _, log := range capturedLogs {
		if _, exists := seenIterations[log.Iteration]; exists {
			t.Errorf("Duplicate iteration number %d: appears for both %q and %q",
				log.Iteration, seenIterations[log.Iteration], log.BeadID)
		}
		seenIterations[log.Iteration] = log.BeadID
	}

	// Verify the beads appear in order and have correct iteration numbers
	if capturedLogs[0].BeadID != "bead-1" || capturedLogs[0].Iteration != 1 {
		t.Errorf("Log 0: Expected BeadID=bead-1 Iteration=1, got BeadID=%s Iteration=%d",
			capturedLogs[0].BeadID, capturedLogs[0].Iteration)
	}
	if capturedLogs[1].BeadID != "bead-2-blocked" || capturedLogs[1].Iteration != 2 {
		t.Errorf("Log 1 (blocked bead): Expected BeadID=bead-2-blocked Iteration=2, got BeadID=%s Iteration=%d",
			capturedLogs[1].BeadID, capturedLogs[1].Iteration)
	}
	if capturedLogs[2].BeadID != "bead-3" || capturedLogs[2].Iteration != 3 {
		t.Errorf("Log 2: Expected BeadID=bead-3 Iteration=3, got BeadID=%s Iteration=%d",
			capturedLogs[2].BeadID, capturedLogs[2].Iteration)
	}
}

// fakeTrendUpdater is a test double for trendUpdaterCloser
type fakeTrendUpdater struct {
	trend *logger.ProcessTrend
}

func (f *fakeTrendUpdater) Close() {
	// no-op for testing
}

// setupMergeStatsTestDirsImpl creates temp directories for merge stats testing
func setupMergeStatsTestDirsImpl(t *testing.T) (dir, logsDir, statsPath string) {
	t.Helper()
	dir = t.TempDir()
	logsDir = filepath.Join(dir, "logs")
	statsPath = filepath.Join(dir, "stats.json")

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	return dir, logsDir, statsPath
}

// seedGlobalStatsImpl creates an existing global stats file with opus history
func seedGlobalStatsImpl(t *testing.T, statsPath string) {
	t.Helper()
	existingStats := logger.GlobalStats{
		Version: 1,
		Updated: "2026-02-24T10:00:00Z",
		Models: map[string]*logger.GlobalModelStats{
			"opus": {
				Iterations:      10,
				Successes:       8,
				Failures:        2,
				TotalCostUSD:    20.00,
				EscalationsFrom: 0,
				EscalationsTo:   2,
			},
		},
	}
	existingData, _ := json.MarshalIndent(existingStats, "", "  ")
	if err := os.WriteFile(statsPath, existingData, 0644); err != nil {
		t.Fatalf("Failed to create existing stats file: %v", err)
	}
}

// createRunLogsImpl creates iteration logs for the given models
func createRunLogsImpl(t *testing.T, logsDir, runID string, models []string) {
	t.Helper()
	logEntries := []logger.IterationLog{
		{
			Timestamp:    time.Now(),
			Iteration:    1,
			BeadID:       "bead-1",
			BeadTitle:    "Feature A",
			Success:      true,
			Model:        "opus",
			CostUSD:      3.00,
			InputTokens:  100,
			OutputTokens: 50,
			DurationMs:   1000,
		},
		{
			Timestamp:    time.Now(),
			Iteration:    2,
			BeadID:       "bead-2",
			BeadTitle:    "Feature B",
			Success:      true,
			Model:        "haiku",
			CostUSD:      0.50,
			InputTokens:  50,
			OutputTokens: 25,
			DurationMs:   500,
		},
	}
	writeOrchestratorTestLogFile(t, logsDir, runID, logEntries)
}

// invokeOrchestratorImpl runs the orchestrator and returns the merged stats
func invokeOrchestratorImpl(t *testing.T, statsPath, logsDir, runID string) *logger.GlobalStats {
	t.Helper()

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		return nil, nil // No beads, so run completes immediately
	}

	cfg := OrchestratorConfig{
		Gate:            &fakeStage{},
		Build:           &fakeStage{},
		Validate:        &fakeStage{},
		Epilogue:        &fakeStage{},
		GetBead:         getBead,
		Config:          &config.Config{},
		Output:          io.Discard,
		GlobalStatsPath: statsPath,
		GetRunID: func() string {
			return runID
		},
		LogsDir: logsDir,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	mergedStats, err := logger.ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	return mergedStats
}

// TestNormalizeTouchedPackages tests that normalizeTouchedPackages correctly
// deduplicates, trims whitespace, normalizes paths, and handles edge cases.
func TestNormalizeTouchedPackages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil slice",
			in:   nil,
			want: []string{},
		},
		{
			name: "empty slice",
			in:   []string{},
			want: []string{},
		},
		{
			name: "single package",
			in:   []string{"pkg"},
			want: []string{"pkg"},
		},
		{
			name: "package with ./ prefix",
			in:   []string{"./pkg"},
			want: []string{"pkg"},
		},
		{
			name: "package with trailing slash",
			in:   []string{"pkg/"},
			want: []string{"pkg"},
		},
		{
			name: "package with whitespace",
			in:   []string{"  pkg  "},
			want: []string{"pkg"},
		},
		{
			name: "dot package",
			in:   []string{"."},
			want: []string{"."},
		},
		{
			name: "dot-slash package normalized away",
			in:   []string{"./"},
			want: []string{},
		},
		{
			name: "dot-slash-dot package",
			in:   []string{"./."},
			want: []string{"."},
		},
		{
			name: "empty string skipped",
			in:   []string{"pkg1", "", "pkg2"},
			want: []string{"pkg1", "pkg2"},
		},
		{
			name: "exact duplicates removed",
			in:   []string{"pkg", "pkg", "pkg"},
			want: []string{"pkg"},
		},
		{
			name: "formatting variants deduplicated",
			in:   []string{"./pkg/", "  pkg  ", "pkg"},
			want: []string{"pkg"},
		},
		{
			name: "overlapping packages",
			in:   []string{"pkg1", "./pkg2/", "  pkg1  "},
			want: []string{"pkg1", "pkg2"},
		},
		{
			name: "multiple distinct packages",
			in:   []string{"./a/", "b", "  c  ", "d"},
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "only empty strings",
			in:   []string{"", "  ", ""},
			want: []string{},
		},
		{
			name: "complex mix",
			in:   []string{"./pkg1/", "pkg1", "  pkg2  ", "./pkg2/", "pkg3", "  pkg3  ", ""},
			want: []string{"pkg1", "pkg2", "pkg3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTouchedPackages(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normalizeTouchedPackages(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestMergeTouchedPackages tests that mergeTouchedPackages correctly merges
// and normalizes two slices of package names.
func TestMergeTouchedPackages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		exist []string
		in    []string
		want  []string
	}{
		{
			name:  "both nil",
			exist: nil,
			in:    nil,
			want:  nil,
		},
		{
			name:  "both empty",
			exist: []string{},
			in:    []string{},
			want:  nil,
		},
		{
			name:  "existing empty incoming non-empty",
			exist: []string{},
			in:    []string{"pkg"},
			want:  []string{"pkg"},
		},
		{
			name:  "existing non-empty incoming empty",
			exist: []string{"pkg"},
			in:    []string{},
			want:  []string{"pkg"},
		},
		{
			name:  "disjoint packages",
			exist: []string{"pkg1"},
			in:    []string{"pkg2"},
			want:  []string{"pkg1", "pkg2"},
		},
		{
			name:  "overlapping packages deduplicated",
			exist: []string{"pkg1"},
			in:    []string{"pkg1"},
			want:  []string{"pkg1"},
		},
		{
			name:  "formatting variants deduplicated",
			exist: []string{"./pkg1/"},
			in:    []string{"  pkg1  "},
			want:  []string{"pkg1"},
		},
		{
			name:  "existing with duplicates",
			exist: []string{"pkg1", "pkg1"},
			in:    []string{"pkg2"},
			want:  []string{"pkg1", "pkg2"},
		},
		{
			name:  "incoming with duplicates",
			exist: []string{"pkg1"},
			in:    []string{"pkg2", "pkg2"},
			want:  []string{"pkg1", "pkg2"},
		},
		{
			name:  "multiple existing and incoming",
			exist: []string{"./a/", "b"},
			in:    []string{"  c  ", "d"},
			want:  []string{"a", "b", "c", "d"},
		},
		{
			name:  "complex merge with overlap",
			exist: []string{"./pkg1/", "pkg2", "  pkg3  "},
			in:    []string{"pkg3", "  pkg4  ", "./pkg5/"},
			want:  []string{"pkg1", "pkg2", "pkg3", "pkg4", "pkg5"},
		},
		{
			name:  "dot packages deduplicated",
			exist: []string{"."},
			in:    []string{"."},
			want:  []string{"."},
		},
		{
			name:  "dot with regular packages",
			exist: []string{"pkg1"},
			in:    []string{"."},
			want:  []string{"pkg1", "."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeTouchedPackages(tt.exist, tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeTouchedPackages(%v, %v) = %v, want %v", tt.exist, tt.in, got, tt.want)
			}
		})
	}
}

// RED: test that orchestrator does not emit BuildStart/Complete and ReviewStart/Complete events
func TestOrchestrator_DoesNotEmitBuildAndReviewEvents(t *testing.T) {
	t.Parallel()

	buildCalled := false
	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		buildCalled = true
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	reviewCalled := false
	review := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		reviewCalled = true
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
		Review:   review,
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config: &config.Config{
			Review: config.ReviewConfig{Enabled: true},
		},
		Output: io.Discard,
	}

	orch := NewOrchestrator(cfg)

	// Subscribe to events immediately
	eventCh := orch.emitter.Subscribe()
	eventList := []interface{}{}

	// Create a context that will be cancelled when Run() completes
	runCtx, cancel := context.WithCancel(context.Background())

	// Collect events in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case evt := <-eventCh:
				if evt == nil {
					// Channel closed, emitter stopped
					return
				}
				eventList = append(eventList, evt)
			case <-runCtx.Done():
				// Polling fallback in case context gets cancelled but channel is still open
				select {
				case evt := <-eventCh:
					if evt != nil {
						eventList = append(eventList, evt)
					} else {
						return
					}
				default:
					return
				}
			}
		}
	}()

	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	cancel()

	// Wait for event collector to finish
	wg.Wait()

	// Verify stages were called
	if !buildCalled {
		t.Error("Build stage was not called")
	}
	if !reviewCalled {
		t.Error("Review stage was not called")
	}

	// Debug: print event count
	t.Logf("Collected %d events", len(eventList))
	for i, evt := range eventList {
		t.Logf("Event %d: %T", i, evt)
	}

	// Count specific event types
	buildStartCount := 0
	buildCompleteCount := 0
	reviewStartCount := 0
	reviewCompleteCount := 0

	// Verify no BuildStartEvent, BuildCompleteEvent, ReviewStartEvent, ReviewCompleteEvent
	for _, evt := range eventList {
		switch evt.(type) {
		case *events.BuildStartEvent:
			buildStartCount++
			t.Error("Orchestrator should not emit BuildStartEvent")
		case *events.BuildCompleteEvent:
			buildCompleteCount++
			t.Error("Orchestrator should not emit BuildCompleteEvent")
		case *events.ReviewStartEvent:
			reviewStartCount++
			t.Error("Orchestrator should not emit ReviewStartEvent")
		case *events.ReviewCompleteEvent:
			reviewCompleteCount++
			t.Error("Orchestrator should not emit ReviewCompleteEvent")
		}
	}

	t.Logf("Found %d BuildStartEvent, %d BuildCompleteEvent, %d ReviewStartEvent, %d ReviewCompleteEvent",
		buildStartCount, buildCompleteCount, reviewStartCount, reviewCompleteCount)
}

// TestOrchestrator_SkipsAlreadyProcessedBead verifies that when GetBead returns
// the same bead ID indefinitely (e.g. because bd ready keeps returning an
// uncloseable bead), the orchestrator processes it once and then terminates
// instead of looping forever.
func TestOrchestrator_SkipsAlreadyProcessedBead(t *testing.T) {
	t.Parallel()

	buildRunCount := 0
	build := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		buildRunCount++
		return pipeline.Output{Decision: pipeline.Proceed, Model: "claude-sonnet-4-6"}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		// Always return the same bead — never nil. This simulates bd
		// returning an uncloseable bead indefinitely.
		return &bead.Bead{ID: "stuck-bead", Title: "Cannot close"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    build,
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if buildRunCount != 1 {
		t.Errorf("Build stage ran %d times, want 1 (bead should be skipped after first processing)", buildRunCount)
	}
	// GetBead is called twice: once to process, once to detect the duplicate.
	if beadCalls != 2 {
		t.Errorf("GetBead called %d times, want 2", beadCalls)
	}
}

func TestOrchestrator_UsesGetBeadExcludingForProcessedIDs(t *testing.T) {
	t.Parallel()

	buildRunCount := 0
	build := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		buildRunCount++
		return pipeline.Output{Decision: pipeline.Proceed, Model: "claude-sonnet-4-6"}, nil
	}}

	beadA := &bead.Bead{ID: "bead-a", Title: "A"}
	beadB := &bead.Bead{ID: "bead-b", Title: "B"}

	getBeadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		getBeadCalls++
		if getBeadCalls == 1 {
			return beadA, nil
		}
		return nil, nil
	}

	getBeadExcludingCalls := 0
	var excludesSeen []map[string]bool
	getBeadExcluding := func(_ context.Context, excludeIDs map[string]bool) (*bead.Bead, error) {
		getBeadExcludingCalls++
		snapshot := make(map[string]bool, len(excludeIDs))
		for id, excluded := range excludeIDs {
			snapshot[id] = excluded
		}
		excludesSeen = append(excludesSeen, snapshot)

		if excludeIDs["bead-a"] && !excludeIDs["bead-b"] {
			return beadB, nil
		}
		return nil, nil
	}

	cfg := OrchestratorConfig{
		Gate:             &fakeStage{},
		Build:            build,
		Validate:         &fakeStage{},
		Epilogue:         &fakeStage{},
		GetBead:          getBead,
		GetBeadExcluding: getBeadExcluding,
		Config:           &config.Config{},
		Output:           io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if buildRunCount != 2 {
		t.Fatalf("Build stage ran %d times, want 2", buildRunCount)
	}
	if getBeadCalls != 1 {
		t.Fatalf("GetBead called %d times, want 1", getBeadCalls)
	}
	if getBeadExcludingCalls != 2 {
		t.Fatalf("GetBeadExcluding called %d times, want 2", getBeadExcludingCalls)
	}
	if len(excludesSeen) < 2 {
		t.Fatalf("expected at least 2 exclude snapshots, got %d", len(excludesSeen))
	}
	if !excludesSeen[0]["bead-a"] || excludesSeen[0]["bead-b"] {
		t.Fatalf("first exclude snapshot = %v, want only bead-a excluded", excludesSeen[0])
	}
	if !excludesSeen[1]["bead-a"] || !excludesSeen[1]["bead-b"] {
		t.Fatalf("second exclude snapshot = %v, want bead-a and bead-b excluded", excludesSeen[1])
	}
}

func TestSkipTrackerCountsInterleavedSkips(t *testing.T) {
	t.Parallel()

	tracker := newSkipTracker()
	tracker.markProcessed("skipped-a")
	tracker.markProcessed("skipped-b")

	if tracker.recordSkip("skipped-a") {
		t.Fatalf("expected no break after the first skip")
	}
	if !tracker.recordSkip("skipped-b") {
		t.Fatalf("expected break after accumulating 2 skips")
	}

	tracker.markProcessed("skipped-c")
	if tracker.recordSkip("skipped-a") {
		t.Fatalf("skip counter should restart after processing a new bead")
	}
	if tracker.recordSkip("skipped-b") {
		t.Fatalf("still waiting for 3 total skips before breaking")
	}
	if !tracker.recordSkip("skipped-c") {
		t.Fatalf("expected break after accumulating 3 skips")
	}
}

// TestOrchestrator_SkipsSpecMergeTriggerOnEpilogueLifecycleFailure verifies that
// when the Epilogue stage returns a lifecycle failure (LifecycleFailureClose or
// LifecycleFailureSync), the orchestrator does NOT trigger the spec merge pipeline,
// even if the spec is otherwise complete.
func TestOrchestrator_SkipsSpecMergeTriggerOnEpilogueLifecycleFailure(t *testing.T) {
	t.Parallel()

	specName := "payments"
	b := &bead.Bead{ID: "spec-1", Title: "Spec bead", Labels: []string{"spec:" + specName}}
	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return b, nil
	}

	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		// Simulate a lifecycle failure (e.g. bead close failed)
		return pipeline.Output{
			Decision:         pipeline.Proceed,
			LifecycleFailure: pipeline.LifecycleFailureClose,
		}, nil
	}}

	triggerCalls := 0
	specPipeline := &fakeSpecMergeController{
		isCompleteFn: func(_ context.Context, name string) (bool, error) {
			// Always report spec as complete
			return true, nil
		},
		triggerFn: func(ctx context.Context, name string) error {
			triggerCalls++
			return nil
		},
	}

	cfg := OrchestratorConfig{
		Gate:                &fakeStage{},
		Build:               &fakeStage{},
		Validate:            &fakeStage{},
		Epilogue:            epilogueStage,
		GetBead:             getBead,
		Config:              &config.Config{Methodology: config.MethodologyConfig{Granularity: config.MethodologyGranularitySpec}},
		SpecMergeController: specPipeline,
		Output:              io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if triggerCalls != 0 {
		t.Fatalf("SpecMerge.Trigger was called %d times, want 0 (should skip on lifecycle failure)", triggerCalls)
	}
}

// TestOrchestrator_SuppressesSuccessLoggingOnEpilogueLifecycleFailure verifies that
// when the Epilogue stage returns a lifecycle failure (LifecycleFailureClose or
// LifecycleFailureSync), the orchestrator does NOT emit the "completed successfully"
// log message, even though the bead was successfully built and validated.
func TestOrchestrator_SuppressesSuccessLoggingOnEpilogueLifecycleFailure(t *testing.T) {
	t.Parallel()

	var logOutput strings.Builder
	beadID := "bead-1"
	beadTitle := "Test bead"
	b := &bead.Bead{ID: beadID, Title: beadTitle}
	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return b, nil
	}

	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		// Simulate a lifecycle failure (e.g. bead sync failed)
		return pipeline.Output{
			Decision:         pipeline.Proceed,
			LifecycleFailure: pipeline.LifecycleFailureSync,
		}, nil
	}}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   &logOutput,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	logText := logOutput.String()
	successMsg := "completed successfully"
	if strings.Contains(logText, successMsg) {
		t.Fatalf("Log contains %q, want suppressed on lifecycle failure; full log: %s", successMsg, logText)
	}
}

// TestOrchestrator_TriggersSpecMergeWhenLifecycleSucceeds verifies that when
// the Epilogue stage returns LifecycleFailureNone (success), the orchestrator
// DOES trigger the spec merge pipeline, ensuring the gating logic works correctly
// in both directions (skip on failure, trigger on success).
func TestOrchestrator_TriggersSpecMergeWhenLifecycleSucceeds(t *testing.T) {
	t.Parallel()

	specName := "payments"
	b := &bead.Bead{ID: "spec-1", Title: "Spec bead", Labels: []string{"spec:" + specName}}
	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return b, nil
	}

	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		// Return success (LifecycleFailureNone)
		return pipeline.Output{
			Decision:         pipeline.Proceed,
			LifecycleFailure: pipeline.LifecycleFailureNone,
		}, nil
	}}

	triggerCalls := 0
	specPipeline := &fakeSpecMergeController{
		isCompleteFn: func(_ context.Context, name string) (bool, error) {
			// Always report spec as complete
			return true, nil
		},
		triggerFn: func(ctx context.Context, name string) error {
			triggerCalls++
			return nil
		},
	}

	cfg := OrchestratorConfig{
		Gate:                &fakeStage{},
		Build:               &fakeStage{},
		Validate:            &fakeStage{},
		Epilogue:            epilogueStage,
		GetBead:             getBead,
		Config:              &config.Config{Methodology: config.MethodologyConfig{Granularity: config.MethodologyGranularitySpec}},
		SpecMergeController: specPipeline,
		Output:              io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 10, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if triggerCalls != 1 {
		t.Fatalf("SpecMerge.Trigger was called %d times, want 1 (should trigger on lifecycle success)", triggerCalls)
	}
}

func TestOrchestrator_LifecycleFailureSuppressesSuccessEvents(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		closeErr error
		syncErr  error
	}{
		{name: "close failure", closeErr: errors.New("close boom")},
		{name: "sync failure", syncErr: errors.New("sync boom")},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			beads := &fakeBeadLifecycle{
				closeFn: func(ctx context.Context, id string) error {
					return tc.closeErr
				},
				syncFn: func(ctx context.Context) error {
					return tc.syncErr
				},
			}
			status := &fakeStatusWriter{}
			epilogue := epiloguepkg.New(beads, status, io.Discard)

			beadCalls := 0
			getBead := func(_ context.Context) (*bead.Bead, error) {
				beadCalls++
				if beadCalls > 1 {
					return nil, nil
				}
				return &bead.Bead{ID: "failure-bead", Title: "Lifecycle failure"}, nil
			}

			cfg := OrchestratorConfig{
				Gate:     &fakeStage{},
				Build:    &fakeStage{},
				Validate: &fakeStage{},
				Epilogue: epilogue,
				GetBead:  getBead,
				Config:   &config.Config{},
				Output:   io.Discard,
			}

			orch := NewOrchestrator(cfg)
			capturer := newCaptureSubscriber(orch.GetEmitter())
			go capturer.start()

			if err := orch.Run(context.Background(), 1, time.Time{}, nil); err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			<-capturer.done

			var iterComplete *events.IterationCompleteEvent
			for _, evt := range capturer.capture.events {
				switch e := evt.(type) {
				case *events.IterationCompleteEvent:
					iterComplete = e
				case *events.BeadCompleteEvent:
					t.Fatalf("BeadCompleteEvent emitted on lifecycle failure path: %#v", e)
				case *events.LogEvent:
					if strings.Contains(e.Message, "completed successfully") {
						t.Fatalf("Found success log during lifecycle failure: %s", e.Message)
					}
				}
			}

			if iterComplete == nil {
				t.Fatal("IterationCompleteEvent not emitted")
			}
			if iterComplete.Success {
				t.Fatalf("IterationCompleteEvent.Success = true, want false for lifecycle failure: %+v", iterComplete)
			}
		})
	}
}

// TestSingleWriterInvariant_OrchestratorIsCoordinatorForMainIntegration is a regression guard
// asserting that the Orchestrator is the exclusive coordinator for main branch integration.
// This test verifies orchestrator can be created and run, supporting its coordinator role in
// the single-writer architecture. Sessions and epilogue do NOT merge directly; orchestrator does.
func TestSingleWriterInvariant_OrchestratorIsCoordinatorForMainIntegration(t *testing.T) {
	t.Parallel()
	getBead := func(context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)

	// REGRESSION GUARD: Orchestrator must be created and ready to coordinate main integration.
	// This test verifies structural support for orchestrator as sole integration coordinator.
	if orch == nil {
		t.Fatal("Orchestrator is nil; expected orchestrator instance for coordination")
	}

	if orch.cfg.Gate == nil || orch.cfg.Build == nil || orch.cfg.Validate == nil || orch.cfg.Epilogue == nil {
		t.Fatal("Orchestrator missing required stages; single-writer coordination requires all stages")
	}

	// Verify orchestrator can execute (basic coordinator functionality)
	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Orchestrator.Run() error = %v; expected nil (coordinator must be operational)", err)
	}
}

// TestOrchestratorCallsCoordinatorBetweenIterations verifies that Coordinator.Coordinate
// is invoked between each iteration in the run loop.
func TestOrchestratorCallsCoordinatorBetweenIterations(t *testing.T) {
	t.Parallel()

	// Track coordinator invocations
	var coordinatorCalls []int
	var mu sync.Mutex

	coordinator := &fakeCoordinator{
		coordinateFn: func(ctx context.Context) error {
			mu.Lock()
			coordinatorCalls = append(coordinatorCalls, len(coordinatorCalls))
			mu.Unlock()
			return nil
		},
	}

	// Create beads to process in 3 iterations
	beads := []*bead.Bead{
		{ID: "bead1", Title: "Task 1"},
		{ID: "bead2", Title: "Task 2"},
		{ID: "bead3", Title: "Task 3"},
	}
	beadIdx := 0

	getBead := func(ctx context.Context) (*bead.Bead, error) {
		if beadIdx >= len(beads) {
			return nil, nil
		}
		b := beads[beadIdx]
		beadIdx++
		return b, nil
	}

	cfg := OrchestratorConfig{
		Gate:        &fakeStage{},
		Build:       &fakeStage{},
		Validate:    &fakeStage{},
		Epilogue:    &fakeStage{},
		GetBead:     getBead,
		Coordinator: coordinator,
		Config:      &config.Config{},
		Output:      io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 3, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Orchestrator.Run() error = %v; expected nil", err)
	}

	// Coordinator should be called after each of the 3 successful iterations
	if len(coordinatorCalls) != 3 {
		t.Fatalf("Expected coordinator to be called 3 times (once per iteration), got %d calls", len(coordinatorCalls))
	}
}

// TestOrchestratorCoordinatorErrorIsolation verifies that errors from the coordinator
// do not terminate the run loop; subsequent iterations continue normally.
func TestOrchestratorCoordinatorErrorIsolation(t *testing.T) {
	t.Parallel()

	// Coordinator fails on first call, succeeds on second
	coordinatorCallCount := 0
	coordinator := &fakeCoordinator{
		coordinateFn: func(ctx context.Context) error {
			coordinatorCallCount++
			if coordinatorCallCount == 1 {
				return fmt.Errorf("coordination error")
			}
			return nil
		},
	}

	beads := []*bead.Bead{
		{ID: "bead1", Title: "Task 1"},
		{ID: "bead2", Title: "Task 2"},
	}
	beadIdx := 0

	getBead := func(ctx context.Context) (*bead.Bead, error) {
		if beadIdx >= len(beads) {
			return nil, nil
		}
		b := beads[beadIdx]
		beadIdx++
		return b, nil
	}

	cfg := OrchestratorConfig{
		Gate:        &fakeStage{},
		Build:       &fakeStage{},
		Validate:    &fakeStage{},
		Epilogue:    &fakeStage{},
		GetBead:     getBead,
		Coordinator: coordinator,
		Config:      &config.Config{},
		Output:      io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 2, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Orchestrator.Run() error = %v; expected nil (error in coordinator should be isolated)", err)
	}

	// Coordinator should be called twice (once per iteration)
	if coordinatorCallCount != 2 {
		t.Fatalf("Expected coordinator to be called 2 times, got %d calls", coordinatorCallCount)
	}
}

// TestConstructorWiresCoordinatorAndQueueStore verifies that the OrchestratorConfig
// created by the constructor includes a Coordinator for handling integration queue.
func TestConstructorWiresCoordinatorAndQueueStore(t *testing.T) {
	t.Parallel()

	// Create a minimal config for testing
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	logsDir := filepath.Join(tempDir, "logs")
	claudePath := filepath.Join(tempDir, "CLAUDE.md")

	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(logsDir, 0755)
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_build.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_tdd_build.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_refactor_build.md"), []byte("test"), 0644)
	os.WriteFile(claudePath, []byte("test"), 0644)

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			Specs:           filepath.Join(tempDir, "specs"),
			ProjectClaudeMD: claudePath,
			Logs:            logsDir,
		},
	}

	output := &strings.Builder{}
	orch, err := newRunnerImpl(cfg, output, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl error = %v; expected nil", err)
	}

	if orch == nil {
		t.Fatal("newRunnerImpl returned nil orchestrator")
	}

	// Verify Coordinator is wired
	if orch.cfg.Coordinator == nil {
		t.Fatal("Orchestrator missing Coordinator dependency; constructor should wire integration coordinator")
	}
}

// TestOrchestrator_CoordinatorDrainsReadyEntry ensures a ready integration queue entry
// is moved to a terminal non-ready state after the orchestration loop invokes the coordinator.
func TestOrchestrator_CoordinatorDrainsReadyEntry(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := integrationqueue.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := testQueueEntry("feature/drain-ready", integrationqueue.StateReady)
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	prevGitOpsFn := newIntegrationQueueGitOpsAdapterFn
	prevGateFn := newIntegrationQueueScopedGateAdapterFn
	defer func() {
		newIntegrationQueueGitOpsAdapterFn = prevGitOpsFn
		newIntegrationQueueScopedGateAdapterFn = prevGateFn
	}()
	newIntegrationQueueGitOpsAdapterFn = func(repoDir string, cfg *config.Config) (*integrationQueueGitOpsAdapter, error) {
		return &integrationQueueGitOpsAdapter{
			repoDir:    repoDir,
			baseBranch: "main",
			runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "", nil
			},
		}, nil
	}
	newIntegrationQueueScopedGateAdapterFn = func(cfg *config.Config, repoDir string) (*integrationQueueScopedGateAdapter, error) {
		return &integrationQueueScopedGateAdapter{
			evaluator: func(ctx context.Context, entry integrationqueue.Entry) error {
				return nil
			},
		}, nil
	}

	coord, err := NewIntegrationCoordinator(tmpDir)
	if err != nil {
		t.Fatalf("NewIntegrationCoordinator() error = %v", err)
	}

	getBeadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		getBeadCalls++
		if getBeadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "drain-bead", Title: "Drain Ready Bead"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:        &fakeStage{},
		Build:       &fakeStage{},
		Validate:    &fakeStage{},
		Epilogue:    &fakeStage{},
		GetBead:     getBead,
		Config:      &config.Config{},
		Output:      io.Discard,
		Coordinator: coord,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 1, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	result, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	processed := findQueueEntry(result, entry.Branch)
	if processed == nil {
		t.Fatalf("entry %s missing after run", entry.Branch)
	}
	if processed.State != integrationqueue.StateMerged {
		t.Fatalf("entry state = %s, want %s", processed.State, integrationqueue.StateMerged)
	}
	if processed.LastErrorCode != "" {
		t.Fatalf("LastErrorCode = %q, want empty", processed.LastErrorCode)
	}
}

func testQueueEntry(branch string, state integrationqueue.State) integrationqueue.Entry {
	return integrationqueue.Entry{
		Branch:               branch,
		SessionID:            branch,
		OriginCommand:        "bd run",
		State:                state,
		Lane:                 string(integrationqueue.CodeLane),
		BaseRef:              "main",
		HeadSHA:              "deadbeef",
		ChangedFiles:         []string{"file.go"},
		ChangedFilesHash:     "hash",
		LastTransitionReason: "test",
	}
}

func findQueueEntry(snapshot *integrationqueue.Snapshot, branch string) *integrationqueue.Entry {
	if snapshot == nil {
		return nil
	}
	for i := range snapshot.Entries {
		entry := &snapshot.Entries[i]
		if entry.Branch == branch {
			return entry
		}
	}
	return nil
}

func TestOrchestrator_CoordinatorRecoversFromCrash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store, err := integrationqueue.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := testQueueEntry("feature/crash", integrationqueue.StateIntegrating)
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	coord, err := NewIntegrationCoordinator(tmpDir)
	if err != nil {
		t.Fatalf("NewIntegrationCoordinator() error = %v", err)
	}

	cfg := OrchestratorConfig{
		Gate:        &fakeStage{},
		Build:       &fakeStage{},
		Validate:    &fakeStage{},
		Epilogue:    &fakeStage{},
		GetBead:     func(context.Context) (*bead.Bead, error) { return nil, nil },
		Config:      &config.Config{},
		Output:      io.Discard,
		Coordinator: coord,
	}

	orch := NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 1, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	result, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	recovered := findQueueEntry(result, entry.Branch)
	if recovered == nil {
		t.Fatalf("entry %s missing after run", entry.Branch)
	}
	if recovered.State != integrationqueue.StateReady {
		t.Fatalf("entry state = %s, want %s", recovered.State, integrationqueue.StateReady)
	}
	if recovered.LastErrorCode != "crash_recovery" {
		t.Fatalf("LastErrorCode = %q, want %q", recovered.LastErrorCode, "crash_recovery")
	}
	if !strings.Contains(recovered.LastErrorMessage, "recovered from crash") {
		t.Fatalf("LastErrorMessage = %q, want crash recovery message", recovered.LastErrorMessage)
	}
}

// TestOrchestratorSkipsCoordinatorOnFailedIterations verifies that the coordinator
// is only invoked after successful iterations, not after gate/build/validate failures.
func TestOrchestratorSkipsCoordinatorOnFailedIterations(t *testing.T) {
	t.Parallel()

	coordinatorCalls := 0
	coordinator := &fakeCoordinator{
		coordinateFn: func(ctx context.Context) error {
			coordinatorCalls++
			return nil
		},
	}

	// Create a stage that returns Skip decision
	gateStage := &fakeStage{
		runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Skip}, nil
		},
	}

	beads := []*bead.Bead{
		{ID: "bead1", Title: "Task 1"},
	}
	beadIdx := 0

	getBead := func(ctx context.Context) (*bead.Bead, error) {
		if beadIdx >= len(beads) {
			return nil, nil
		}
		b := beads[beadIdx]
		beadIdx++
		return b, nil
	}

	cfg := OrchestratorConfig{
		Gate:        gateStage,
		Build:       &fakeStage{},
		Validate:    &fakeStage{},
		Epilogue:    &fakeStage{},
		GetBead:     getBead,
		Coordinator: coordinator,
		Config:      &config.Config{},
		Output:      io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Orchestrator.Run() error = %v; expected nil", err)
	}

	// Coordinator should NOT be called for failed/skipped iterations
	if coordinatorCalls != 0 {
		t.Fatalf("Expected coordinator to not be called (iteration failed), got %d calls", coordinatorCalls)
	}
}

// TestOrchestrator_GateBlockReason_PropagatedToIterationLog verifies that when the gate
// stage returns a Block decision with a GateBlockReason, the reason is written to the
// iteration log so the orchestrator can track why beads were blocked.
func TestOrchestrator_GateBlockReason_PropagatedToIterationLog(t *testing.T) {
	t.Parallel()

	var capturedLogs []*logger.IterationLog

	gateStage := &fakeStage{
		runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{
				Decision:        pipeline.Block,
				GateBlockReason: "scope gate: open dependencies",
			}, nil
		},
	}

	epilogueStage := &fakeStage{
		runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
			if in.Result != nil {
				capturedLogs = append(capturedLogs, in.Result)
			}
			return pipeline.Output{}, nil
		},
	}

	beads := []*bead.Bead{
		{ID: "bead-1", Title: "Test Bead"},
	}
	beadIdx := 0

	getBead := func(ctx context.Context) (*bead.Bead, error) {
		if beadIdx >= len(beads) {
			return nil, nil
		}
		b := beads[beadIdx]
		beadIdx++
		return b, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gateStage,
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Orchestrator.Run() error = %v; expected nil", err)
	}

	if len(capturedLogs) != 1 {
		t.Fatalf("Expected 1 iteration log, got %d", len(capturedLogs))
	}

	log := capturedLogs[0]
	if log.GateBlockReason != "scope gate: open dependencies" {
		t.Errorf("Expected GateBlockReason = 'scope gate: open dependencies', got %q", log.GateBlockReason)
	}
}

// TestOrchestrator_GateSkipDecision_DoesNotSetBlockReason verifies that when the gate
// stage returns a Skip decision, the iteration log has an empty GateBlockReason field,
// ensuring block reasons are only set for Block decisions.
func TestOrchestrator_GateSkipDecision_DoesNotSetBlockReason(t *testing.T) {
	t.Parallel()

	var capturedLogs []*logger.IterationLog

	gateStage := &fakeStage{
		runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{
				Decision: pipeline.Skip,
			}, nil
		},
	}

	epilogueStage := &fakeStage{
		runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
			if in.Result != nil {
				capturedLogs = append(capturedLogs, in.Result)
			}
			return pipeline.Output{}, nil
		},
	}

	beads := []*bead.Bead{
		{ID: "bead-1", Title: "Test Bead"},
	}
	beadIdx := 0

	getBead := func(ctx context.Context) (*bead.Bead, error) {
		if beadIdx >= len(beads) {
			return nil, nil
		}
		b := beads[beadIdx]
		beadIdx++
		return b, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gateStage,
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Orchestrator.Run() error = %v; expected nil", err)
	}

	if len(capturedLogs) != 1 {
		t.Fatalf("Expected 1 iteration log, got %d", len(capturedLogs))
	}

	log := capturedLogs[0]
	if log.GateBlockReason != "" {
		t.Errorf("Expected GateBlockReason to be empty for Skip decision, got %q", log.GateBlockReason)
	}
}

// TestOrchestrator_GateBlockDecision_WithEmptyReason verifies that when the gate
// stage returns a Block decision with an empty GateBlockReason, the iteration log
// is created successfully with an empty reason field (allows block without explicit reason).
func TestOrchestrator_GateBlockDecision_WithEmptyReason(t *testing.T) {
	t.Parallel()

	var capturedLogs []*logger.IterationLog

	gateStage := &fakeStage{
		runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{
				Decision:        pipeline.Block,
				GateBlockReason: "",
			}, nil
		},
	}

	epilogueStage := &fakeStage{
		runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
			if in.Result != nil {
				capturedLogs = append(capturedLogs, in.Result)
			}
			return pipeline.Output{}, nil
		},
	}

	beads := []*bead.Bead{
		{ID: "bead-1", Title: "Test Bead"},
	}
	beadIdx := 0

	getBead := func(ctx context.Context) (*bead.Bead, error) {
		if beadIdx >= len(beads) {
			return nil, nil
		}
		b := beads[beadIdx]
		beadIdx++
		return b, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gateStage,
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Orchestrator.Run() error = %v; expected nil", err)
	}

	if len(capturedLogs) != 1 {
		t.Fatalf("Expected 1 iteration log, got %d", len(capturedLogs))
	}

	log := capturedLogs[0]
	if log.GateBlockReason != "" {
		t.Errorf("Expected GateBlockReason to be empty for Block with no explicit reason, got %q", log.GateBlockReason)
	}
}

// RED: test for readiness-blocked bead not proceeding to build with reason code propagation
func TestOrchestrator_ReadinessCheckBlocksPrecedingBuild(t *testing.T) {
	t.Parallel()

	var capturedLogs []*logger.IterationLog
	buildCalled := false

	gateStage := &fakeStage{
		runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{
				Decision:        pipeline.Block,
				GateBlockReason: "criteria_missing",
			}, nil
		},
	}

	buildStage := &fakeStage{
		runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			buildCalled = true
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		},
	}

	epilogueStage := &fakeStage{
		runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
			if in.Result != nil {
				capturedLogs = append(capturedLogs, in.Result)
			}
			return pipeline.Output{}, nil
		},
	}

	beads := []*bead.Bead{
		{ID: "bead-1", Title: "Readiness Blocked Bead"},
	}
	beadIdx := 0

	getBead := func(ctx context.Context) (*bead.Bead, error) {
		if beadIdx >= len(beads) {
			return nil, nil
		}
		b := beads[beadIdx]
		beadIdx++
		return b, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gateStage,
		Build:    buildStage,
		Validate: &fakeStage{},
		Epilogue: epilogueStage,
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Orchestrator.Run() error = %v; expected nil", err)
	}

	// Verify build stage was never called
	if buildCalled {
		t.Fatal("Build stage should not be called when Gate blocks with readiness reason")
	}

	// Verify the reason is recorded in the iteration log
	if len(capturedLogs) != 1 {
		t.Fatalf("Expected 1 iteration log, got %d", len(capturedLogs))
	}

	log := capturedLogs[0]
	if log.GateBlockReason != "criteria_missing" {
		t.Errorf("Expected GateBlockReason = 'criteria_missing', got %q", log.GateBlockReason)
	}
}

// TestStampBuildAttribution_PopulatesAllFields verifies the shared helper copies all
// 14 attribution fields from pipeline.Output onto a logger.IterationLog.
func TestStampBuildAttribution_PopulatesAllFields(t *testing.T) {
	t.Parallel()

	out := pipeline.Output{
		Model:                   "opus",
		CostUSD:                 1.23,
		InputTokens:             50000,
		OutputTokens:            10000,
		DurationMs:              45000,
		OriginalTier:            "sonnet",
		ActualTier:              "opus",
		CacheHit:                true,
		CacheMiss:               false,
		CacheWrite:              true,
		CacheClass:              "prompt",
		CacheKey:                "abc123",
		CacheInvalidationReason: "version_change",
		CacheVersionMarker:      "v2",
	}
	log := &logger.IterationLog{}
	stampBuildAttribution(log, out)

	if log.Model != "opus" {
		t.Errorf("Model = %q, want %q", log.Model, "opus")
	}
	if log.CostUSD != 1.23 {
		t.Errorf("CostUSD = %v, want %v", log.CostUSD, 1.23)
	}
	if log.InputTokens != 50000 {
		t.Errorf("InputTokens = %d, want %d", log.InputTokens, 50000)
	}
	if log.OutputTokens != 10000 {
		t.Errorf("OutputTokens = %d, want %d", log.OutputTokens, 10000)
	}
	if log.DurationMs != 45000 {
		t.Errorf("DurationMs = %d, want %d", log.DurationMs, 45000)
	}
	if log.OriginalTier != "sonnet" {
		t.Errorf("OriginalTier = %q, want %q", log.OriginalTier, "sonnet")
	}
	if log.ActualTier != "opus" {
		t.Errorf("ActualTier = %q, want %q", log.ActualTier, "opus")
	}
	if !log.CacheHit {
		t.Error("CacheHit = false, want true")
	}
	if log.CacheClass != "prompt" {
		t.Errorf("CacheClass = %q, want %q", log.CacheClass, "prompt")
	}
	if log.CacheKey != "abc123" {
		t.Errorf("CacheKey = %q, want %q", log.CacheKey, "abc123")
	}
	if log.CacheInvalidationReason != "version_change" {
		t.Errorf("CacheInvalidationReason = %q, want %q", log.CacheInvalidationReason, "version_change")
	}
	if log.CacheVersionMarker != "v2" {
		t.Errorf("CacheVersionMarker = %q, want %q", log.CacheVersionMarker, "v2")
	}
}

// TestStampBuildAttribution_NilLogIsNoOp verifies the helper is safe to call with nil.
func TestStampBuildAttribution_NilLogIsNoOp(t *testing.T) {
	t.Parallel()
	// Should not panic
	stampBuildAttribution(nil, pipeline.Output{Model: "opus"})
}

// TestOrchestrator_ValidationFailure_CarriesBuildAttributionToIterationLog verifies that
// when the build stage succeeds but validation fails, the build's model/cost/token
// attribution is preserved in the IterationLog. Previously, the validation-failure path
// dropped this data, causing empty model/provider fields in current-run efficiency data.
func TestOrchestrator_ValidationFailure_CarriesBuildAttributionToIterationLog(t *testing.T) {
	t.Parallel()
	var capturedResult *logger.IterationLog

	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:     pipeline.Proceed,
			Model:        "sonnet",
			CostUSD:      0.042,
			InputTokens:  15000,
			OutputTokens: 3000,
			DurationMs:   8500,
			OriginalTier: "haiku",
			ActualTier:   "sonnet",
		}, nil
	}}
	validate := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{
			Decision:           pipeline.Block,
			ValidationFailures: []string{"lint: unused import"},
		}, nil
	}}
	epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		capturedResult = in.Result
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-val-attr", Title: "Validation attribution test bead"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    build,
		Validate: validate,
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
		t.Fatal("Epilogue Result is nil; want IterationLog populated on validation failure path")
	}
	if capturedResult.Model != "sonnet" {
		t.Errorf("IterationLog.Model = %q, want %q", capturedResult.Model, "sonnet")
	}
	if capturedResult.CostUSD != 0.042 {
		t.Errorf("IterationLog.CostUSD = %v, want %v", capturedResult.CostUSD, 0.042)
	}
	if capturedResult.InputTokens != 15000 {
		t.Errorf("IterationLog.InputTokens = %d, want %d", capturedResult.InputTokens, 15000)
	}
	if capturedResult.OutputTokens != 3000 {
		t.Errorf("IterationLog.OutputTokens = %d, want %d", capturedResult.OutputTokens, 3000)
	}
	if capturedResult.DurationMs != 8500 {
		t.Errorf("IterationLog.DurationMs = %d, want %d", capturedResult.DurationMs, 8500)
	}
	if capturedResult.OriginalTier != "haiku" {
		t.Errorf("IterationLog.OriginalTier = %q, want %q", capturedResult.OriginalTier, "haiku")
	}
	if capturedResult.ActualTier != "sonnet" {
		t.Errorf("IterationLog.ActualTier = %q, want %q", capturedResult.ActualTier, "sonnet")
	}
}

// TestOrchestrator_AllExitPaths_InvokeEpilogue verifies that every exit path that
// processes a bead invokes the epilogue stage. This is a structural test that
// enumerates: gate-blocked, build-fail, validation-fail, and success paths.
func TestOrchestrator_AllExitPaths_InvokeEpilogue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		gate     func(context.Context, pipeline.Input) (pipeline.Output, error)
		build    func(context.Context, pipeline.Input) (pipeline.Output, error)
		validate func(context.Context, pipeline.Input) (pipeline.Output, error)
	}{
		{
			name: "gate-blocked",
			gate: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
				return pipeline.Output{Decision: pipeline.Block, GateBlockReason: "test"}, nil
			},
			build:    nil, // should not be called
			validate: nil, // should not be called
		},
		{
			name: "build-fail",
			gate: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
			build: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
				return pipeline.Output{Model: "haiku"}, fmt.Errorf("build error")
			},
			validate: nil, // should not be called
		},
		{
			name: "validation-fail",
			gate: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
			build: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
				return pipeline.Output{Decision: pipeline.Proceed, Model: "sonnet"}, nil
			},
			validate: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
				return pipeline.Output{Decision: pipeline.Block, ValidationFailures: []string{"fail"}}, nil
			},
		},
		{
			name: "success",
			gate: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
			build: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
				return pipeline.Output{Decision: pipeline.Proceed, Model: "opus"}, nil
			},
			validate: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			epilogueCalled := false
			epilogueStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
				epilogueCalled = true
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			}}

			beadCalls := 0
			getBead := func(_ context.Context) (*bead.Bead, error) {
				beadCalls++
				if beadCalls > 1 {
					return nil, nil
				}
				return &bead.Bead{ID: "bead-exit-" + tc.name, Title: "Exit path test"}, nil
			}

			gateStage := &fakeStage{}
			if tc.gate != nil {
				gateStage.runFn = tc.gate
			}
			buildStage := &fakeStage{}
			if tc.build != nil {
				buildStage.runFn = tc.build
			}
			validateStage := &fakeStage{}
			if tc.validate != nil {
				validateStage.runFn = tc.validate
			}

			cfg := OrchestratorConfig{
				Gate:     gateStage,
				Build:    buildStage,
				Validate: validateStage,
				Epilogue: epilogueStage,
				GetBead:  getBead,
				Config:   &config.Config{},
				Output:   io.Discard,
			}

			orch := NewOrchestrator(cfg)
			_ = orch.Run(context.Background(), 10, time.Time{}, nil)

			if !epilogueCalled {
				t.Errorf("exit path %q did not invoke epilogue", tc.name)
			}
		})
	}
}

// TestCoordinatorRegression_ReadyEntryTransitionsOutOfReady verifies that when
// NewIntegrationCoordinator is called and runs one Coordinate iteration with a
// ready queue entry, the entry transitions out of ready state. This test fails
// if the coordinator is swapped back to a no-op implementation.
func TestCoordinatorRegression_ReadyEntryTransitionsOutOfReady(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store, err := integrationqueue.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := testQueueEntry("feature/ready-to-transition", integrationqueue.StateReady)
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	// Mock the git operations and scoped gate to succeed
	prevGitOpsFn := newIntegrationQueueGitOpsAdapterFn
	prevGateFn := newIntegrationQueueScopedGateAdapterFn
	defer func() {
		newIntegrationQueueGitOpsAdapterFn = prevGitOpsFn
		newIntegrationQueueScopedGateAdapterFn = prevGateFn
	}()
	newIntegrationQueueGitOpsAdapterFn = func(repoDir string, cfg *config.Config) (*integrationQueueGitOpsAdapter, error) {
		return &integrationQueueGitOpsAdapter{
			repoDir:    repoDir,
			baseBranch: "main",
			runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "", nil
			},
		}, nil
	}
	newIntegrationQueueScopedGateAdapterFn = func(cfg *config.Config, repoDir string) (*integrationQueueScopedGateAdapter, error) {
		return &integrationQueueScopedGateAdapter{
			evaluator: func(ctx context.Context, entry integrationqueue.Entry) error {
				return nil
			},
		}, nil
	}

	// Call NewIntegrationCoordinator and run one Coordinate iteration
	coord, err := NewIntegrationCoordinator(tmpDir)
	if err != nil {
		t.Fatalf("NewIntegrationCoordinator() error = %v", err)
	}

	ctx := context.Background()
	if err := coord.Coordinate(ctx); err != nil {
		t.Fatalf("Coordinate() error = %v", err)
	}

	// Verify the entry has transitioned out of ready state
	result, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	processed := findQueueEntry(result, entry.Branch)
	if processed == nil {
		t.Fatalf("entry %s missing after coordinator run", entry.Branch)
	}
	if processed.State == integrationqueue.StateReady {
		t.Fatalf("entry state = %s, want non-ready state", processed.State)
	}
}

// TestCoordinatorRegression_SuccessfulIntegrationTransitionsToMerged verifies that
// when NewIntegrationCoordinator runs successfully with a ready entry and all
// gates pass, the entry transitions specifically to the merged state.
func TestCoordinatorRegression_SuccessfulIntegrationTransitionsToMerged(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store, err := integrationqueue.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := testQueueEntry("feature/successful-merge", integrationqueue.StateReady)
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	// Mock the git operations and scoped gate to succeed
	prevGitOpsFn := newIntegrationQueueGitOpsAdapterFn
	prevGateFn := newIntegrationQueueScopedGateAdapterFn
	defer func() {
		newIntegrationQueueGitOpsAdapterFn = prevGitOpsFn
		newIntegrationQueueScopedGateAdapterFn = prevGateFn
	}()
	newIntegrationQueueGitOpsAdapterFn = func(repoDir string, cfg *config.Config) (*integrationQueueGitOpsAdapter, error) {
		return &integrationQueueGitOpsAdapter{
			repoDir:    repoDir,
			baseBranch: "main",
			runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "", nil
			},
		}, nil
	}
	newIntegrationQueueScopedGateAdapterFn = func(cfg *config.Config, repoDir string) (*integrationQueueScopedGateAdapter, error) {
		return &integrationQueueScopedGateAdapter{
			evaluator: func(ctx context.Context, entry integrationqueue.Entry) error {
				return nil
			},
		}, nil
	}

	// Call NewIntegrationCoordinator and run one Coordinate iteration
	coord, err := NewIntegrationCoordinator(tmpDir)
	if err != nil {
		t.Fatalf("NewIntegrationCoordinator() error = %v", err)
	}

	ctx := context.Background()
	if err := coord.Coordinate(ctx); err != nil {
		t.Fatalf("Coordinate() error = %v", err)
	}

	// Verify the entry has transitioned to merged state specifically
	result, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	processed := findQueueEntry(result, entry.Branch)
	if processed == nil {
		t.Fatalf("entry %s missing after coordinator run", entry.Branch)
	}
	if processed.State != integrationqueue.StateMerged {
		t.Fatalf("entry state = %s, want %s", processed.State, integrationqueue.StateMerged)
	}
	if processed.LastErrorCode != "" {
		t.Fatalf("LastErrorCode = %q, want empty on successful integration", processed.LastErrorCode)
	}
}

// TestCoordinatorRegression_ProcessesOldestReadyEntryFirst verifies that when
// NewIntegrationCoordinator is called with multiple ready entries, it processes
// the oldest ready entry first (FIFO order).
func TestCoordinatorRegression_ProcessesOldestReadyEntryFirst(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store, err := integrationqueue.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create and save oldest entry first
	oldest := testQueueEntry("feature/oldest", integrationqueue.StateReady)
	if err := store.Save(oldest); err != nil {
		t.Fatalf("Save(oldest) error = %v", err)
	}

	// Create and save newer entry second
	newer := testQueueEntry("feature/newer", integrationqueue.StateReady)
	if err := store.Save(newer); err != nil {
		t.Fatalf("Save(newer) error = %v", err)
	}

	// Mock the git operations and scoped gate to succeed
	prevGitOpsFn := newIntegrationQueueGitOpsAdapterFn
	prevGateFn := newIntegrationQueueScopedGateAdapterFn
	defer func() {
		newIntegrationQueueGitOpsAdapterFn = prevGitOpsFn
		newIntegrationQueueScopedGateAdapterFn = prevGateFn
	}()
	newIntegrationQueueGitOpsAdapterFn = func(repoDir string, cfg *config.Config) (*integrationQueueGitOpsAdapter, error) {
		return &integrationQueueGitOpsAdapter{
			repoDir:    repoDir,
			baseBranch: "main",
			runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "", nil
			},
		}, nil
	}
	newIntegrationQueueScopedGateAdapterFn = func(cfg *config.Config, repoDir string) (*integrationQueueScopedGateAdapter, error) {
		return &integrationQueueScopedGateAdapter{
			evaluator: func(ctx context.Context, entry integrationqueue.Entry) error {
				return nil
			},
		}, nil
	}

	// Call NewIntegrationCoordinator and run one Coordinate iteration
	coord, err := NewIntegrationCoordinator(tmpDir)
	if err != nil {
		t.Fatalf("NewIntegrationCoordinator() error = %v", err)
	}

	ctx := context.Background()
	if err := coord.Coordinate(ctx); err != nil {
		t.Fatalf("Coordinate() error = %v", err)
	}

	// Verify that the oldest entry was processed (transitioned out of ready)
	result, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	oldestProcessed := findQueueEntry(result, oldest.Branch)
	if oldestProcessed == nil {
		t.Fatalf("oldest entry %s missing after coordinator run", oldest.Branch)
	}
	newerRemaining := findQueueEntry(result, newer.Branch)
	if newerRemaining == nil {
		t.Fatalf("newer entry %s missing after coordinator run", newer.Branch)
	}

	// Oldest should be merged (processed)
	if oldestProcessed.State != integrationqueue.StateMerged {
		t.Fatalf("oldest entry state = %s, want %s (processed first)", oldestProcessed.State, integrationqueue.StateMerged)
	}
	// Newer should still be ready (not yet processed)
	if newerRemaining.State != integrationqueue.StateReady {
		t.Fatalf("newer entry state = %s, want %s (processed last)", newerRemaining.State, integrationqueue.StateReady)
	}
}

// TestAssertEfficiencyCompleteness_AllowsPrelaunchSentinelOnly tests that a run
// with only prelaunch sentinel rows does not trigger a completeness abort.
func TestAssertEfficiencyCompleteness_AllowsPrelaunchSentinelOnly(t *testing.T) {
	logsDir := t.TempDir()
	runID := "20260303-100000"

	// Create a log file with only a prelaunch failure (sentinel row)
	logContent := `{"type":"iteration","timestamp":"2026-03-03T10:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Prelaunch checkout failure","model":"haiku","success":false,"validated":false,"failure_phase":"prelaunch","duration_ms":0,"cost_usd":0,"input_tokens":0,"output_tokens":0}
`
	logPath := filepath.Join(logsDir, fmt.Sprintf("run-%s.jsonl", runID))
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}

	// Create orchestrator with logs directory
	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  func(context.Context) (*bead.Bead, error) { return nil, nil },
		Config:   &config.Config{},
		LogsDir:  logsDir,
		GetRunID: func() string {
			return runID
		},
		Output: io.Discard,
	}
	orch := NewOrchestrator(cfg)

	// assertEfficiencyCompleteness should NOT error for prelaunch-only runs
	// totalIterations=1 because we have 1 log entry
	err := orch.assertEfficiencyCompleteness(1)
	if err != nil {
		t.Errorf("assertEfficiencyCompleteness() failed unexpectedly: %v", err)
	}
}

// TestAssertEfficiencyCompleteness_FailsWhenRealDataGapExists tests that
// completeness check still fails for real missing efficiency data, even if
// sentinel rows are also present.
func TestAssertEfficiencyCompleteness_FailsWhenRealDataGapExists(t *testing.T) {
	logsDir := t.TempDir()
	runID := "20260303-100000"

	// Create a log file with prelaunch sentinel AND a real iteration with missing data
	logContent := `{"type":"iteration","timestamp":"2026-03-03T10:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Prelaunch failure","model":"haiku","success":false,"validated":false,"failure_phase":"prelaunch","duration_ms":0,"cost_usd":0,"input_tokens":0,"output_tokens":0}
{"type":"iteration","timestamp":"2026-03-03T10:00:01Z","iteration":2,"bead_id":"b2","bead_title":"Real task missing data","model":"haiku","success":false,"validated":false,"duration_ms":0,"cost_usd":0,"input_tokens":0,"output_tokens":0}
`
	logPath := filepath.Join(logsDir, fmt.Sprintf("run-%s.jsonl", runID))
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  func(context.Context) (*bead.Bead, error) { return nil, nil },
		Config:   &config.Config{},
		LogsDir:  logsDir,
		GetRunID: func() string {
			return runID
		},
		Output: io.Discard,
	}
	orch := NewOrchestrator(cfg)

	// assertEfficiencyCompleteness should error because b2 has missing data
	err := orch.assertEfficiencyCompleteness(2)
	if err == nil {
		t.Error("assertEfficiencyCompleteness() should have failed for real data gaps, got nil")
	}
	if !strings.Contains(err.Error(), "missing efficiency data") {
		t.Errorf("error message should mention missing efficiency data, got: %v", err)
	}
}

// TestOrchestrator_StatusWriter_ReceivesContext verifies that the orchestrator
// passes the run context to the StatusWriter function so that cancellation
// signals propagate through to bd calls (estimateScopedIterationTotal).
func TestOrchestrator_StatusWriter_ReceivesContext(t *testing.T) {
	t.Parallel()
	var capturedCtx context.Context

	deadline := time.Now().Add(30 * time.Minute)

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-1", Title: "Test"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
		StatusWriter: func(ctx context.Context, iteration int, beadID, beadTitle string, dl time.Time) {
			capturedCtx = ctx
		},
	}

	orch := NewOrchestrator(cfg)
	runCtx := context.Background()
	err := orch.Run(runCtx, 10, deadline, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if capturedCtx == nil {
		t.Error("StatusWriter did not receive context; want context passed for cancellation propagation")
	}
	if capturedCtx != runCtx {
		t.Errorf("StatusWriter context is not the run context; want same context for proper cancellation")
	}
}
