package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
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

// TestOrchestrator_LowTierHaikuIterationRecordsDuration verifies that when a low-tier
// haiku bead returns a DurationMs from the Build stage, it is properly propagated to
// the IterationLog via the router/timer source so haiku iterations are not recorded as 0ms.
func TestOrchestrator_LowTierHaikuIterationRecordsDuration(t *testing.T) {
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

// TestOrchestrator_SuccessPath_CarriesComplexityRoutingToIterationLog verifies
// that Gate-selected complexity routing metadata is persisted into the success
// iteration log payload alongside tier telemetry.
func TestOrchestrator_SuccessPath_CarriesComplexityRoutingToIterationLog(t *testing.T) {
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

// TestOrchestrator_StatusWriter_ReceivesDeadline verifies that the orchestrator
// passes the deadline to the StatusWriter function so the constructor's closure
// can compute timeBudgetMinutes instead of hardcoding 0.
func TestOrchestrator_StatusWriter_ReceivesDeadline(t *testing.T) {
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
		StatusWriter: func(iteration int, beadID, beadTitle string, dl time.Time) {
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

// TestOrchestrator_RunSequence_UsesCallerProvidedOrder verifies that RunSequence
// resolves bead IDs via GetBeadByID and executes them in the exact caller-provided
// order, independent of queue ordering.
func TestOrchestrator_RunSequence_UsesCallerProvidedOrder(t *testing.T) {
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
	// Create a temporary logs directory
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

// TestOrchestrator_CoverageTracker_TransitionsStatesAcrossTDDCycle verifies that when
// a CoverageTracker is wired into the orchestrator, it transitions through states
// during a simulated TDD cycle, tracking acceptance criteria coverage.
func TestOrchestrator_CoverageTracker_TransitionsStatesAcrossTDDCycle(t *testing.T) {
	// Create criteria for a simple feature.
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
	dir := t.TempDir()
	logsDir := dir + "/logs"
	statsPath := dir + "/stats.json"
	runID := "2026-02-25-001"

	// Create logs directory for the run
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create existing global stats file with opus history
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

	// Create iteration logs for this run (one successful opus iteration, one successful haiku)
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

	// Set up orchestrator with no iterations (we just care about stats merging)
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

	// Verify global stats were merged (not overwritten)
	mergedStats, err := logger.ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	// Should have 2 models: opus (merged) and haiku (new)
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
