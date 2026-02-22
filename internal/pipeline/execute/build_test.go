package execute_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/provider"
)

// fakeInvoker is a test double for execute.Invoker.
// Run panics to prove the Build stage never calls it; only StreamRun is used.
type fakeInvoker struct {
	streamRunFn func(ctx context.Context, prompt, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	streamCalls []streamCall
}

type streamCall struct {
	prompt string
	tier   string
}

func (f *fakeInvoker) Run(_ context.Context, _, _ string) (*provider.Result, error) {
	panic("Build stage must not call Run; use StreamRun instead")
}

func (f *fakeInvoker) StreamRun(ctx context.Context, prompt, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	f.streamCalls = append(f.streamCalls, streamCall{prompt: prompt, tier: tier})
	if f.streamRunFn != nil {
		return f.streamRunFn(ctx, prompt, tier, w, handler, onToolCall)
	}
	return &provider.Result{Success: true}, nil
}

// fakePromptRenderer is a test double for execute.PromptRenderer.
type fakePromptRenderer struct {
	renderBuildFn         func(title, desc string, failures []string) (string, error)
	renderTDDBuildFn      func(title, desc string, failures []string) (string, error)
	renderRefactorBuildFn func(title, desc string, failures []string) (string, error)
	lastMethodology       string
}

func (f *fakePromptRenderer) RenderBuild(title, desc string, failures []string) (string, error) {
	f.lastMethodology = "standard"
	if f.renderBuildFn != nil {
		return f.renderBuildFn(title, desc, failures)
	}
	return "standard build prompt", nil
}

func (f *fakePromptRenderer) RenderTDDBuild(title, desc string, failures []string) (string, error) {
	f.lastMethodology = "tdd"
	if f.renderTDDBuildFn != nil {
		return f.renderTDDBuildFn(title, desc, failures)
	}
	return "tdd build prompt", nil
}

func (f *fakePromptRenderer) RenderRefactorBuild(title, desc string, failures []string) (string, error) {
	f.lastMethodology = "refactor"
	if f.renderRefactorBuildFn != nil {
		return f.renderRefactorBuildFn(title, desc, failures)
	}
	return "refactor build prompt", nil
}

func makeBead(id, title string) *bead.Bead {
	return &bead.Bead{ID: id, Title: title, Labels: []string{}}
}

func makeBeadWithLabels(id, title string, labels []string) *bead.Bead {
	return &bead.Bead{ID: id, Title: title, Labels: labels}
}

func makeInput(b *bead.Bead, cfg *config.Config) pipeline.Input {
	return pipeline.Input{
		Bead:      b,
		Config:    cfg,
		Iteration: 1,
		Deadline:  time.Now().Add(time.Minute),
	}
}

func defaultConfig() *config.Config {
	return &config.Config{
		Models: config.ModelsConfig{
			P0: "high",
			P1: "medium",
			P2: "low",
		},
	}
}

// TestBuildStage_UsesStreamRun_NotRun verifies that the Build stage invokes the
// provider via StreamRun and never via Run (which panics on the fake).
func TestBuildStage_UsesStreamRun_NotRun(t *testing.T) {
	invoker := &fakeInvoker{}
	renderer := &fakePromptRenderer{}
	stage := execute.New(invoker, renderer, io.Discard)

	in := makeInput(makeBead("bead-1", "Implement feature X"), defaultConfig())

	// Run must not panic (which it would if Run were called on the fake).
	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}

	// StreamRun must have been called exactly once.
	if len(invoker.streamCalls) != 1 {
		t.Errorf("StreamRun called %d times, want 1", len(invoker.streamCalls))
	}
}

// TestBuildStage_SelectMethodology_Standard verifies that a bead with no methodology
// labels and no TDD config uses the standard methodology.
func TestBuildStage_SelectMethodology_Standard(t *testing.T) {
	cfg := defaultConfig()
	cfg.Methodology.TDD = false

	b := makeBead("bead-1", "Fix bug")
	m := execute.SelectMethodology(b, cfg)
	if m != execute.MethodologyStandard {
		t.Errorf("SelectMethodology = %v, want Standard", m)
	}
}

// TestBuildStage_SelectMethodology_TDD_FromConfig verifies that when the global TDD
// config flag is true, SelectMethodology returns TDD for a bead with no label override.
func TestBuildStage_SelectMethodology_TDD_FromConfig(t *testing.T) {
	cfg := defaultConfig()
	cfg.Methodology.TDD = true

	b := makeBead("bead-1", "Add tests")
	m := execute.SelectMethodology(b, cfg)
	if m != execute.MethodologyTDD {
		t.Errorf("SelectMethodology = %v, want TDD", m)
	}
}

// TestBuildStage_SelectMethodology_TDD_FromLabel verifies that a bead with label
// "tdd:true" uses TDD methodology even when global TDD config is false.
func TestBuildStage_SelectMethodology_TDD_FromLabel(t *testing.T) {
	cfg := defaultConfig()
	cfg.Methodology.TDD = false

	b := makeBeadWithLabels("bead-1", "Implement service", []string{"tdd:true"})
	m := execute.SelectMethodology(b, cfg)
	if m != execute.MethodologyTDD {
		t.Errorf("SelectMethodology = %v, want TDD", m)
	}
}

// TestBuildStage_SelectMethodology_TDD_DisabledByLabel verifies that a bead with label
// "tdd:false" uses standard methodology even when global TDD config is true.
func TestBuildStage_SelectMethodology_TDD_DisabledByLabel(t *testing.T) {
	cfg := defaultConfig()
	cfg.Methodology.TDD = true

	b := makeBeadWithLabels("bead-1", "Quick fix", []string{"tdd:false"})
	m := execute.SelectMethodology(b, cfg)
	if m != execute.MethodologyStandard {
		t.Errorf("SelectMethodology = %v, want Standard (tdd:false label should override)", m)
	}
}

// TestBuildStage_SelectMethodology_Refactor verifies that a bead with label
// "refactor:true" uses the refactor methodology.
func TestBuildStage_SelectMethodology_Refactor(t *testing.T) {
	cfg := defaultConfig()

	b := makeBeadWithLabels("bead-1", "Refactor auth module", []string{"refactor:true"})
	m := execute.SelectMethodology(b, cfg)
	if m != execute.MethodologyRefactor {
		t.Errorf("SelectMethodology = %v, want Refactor", m)
	}
}

// TestBuildStage_Run_UsesStandardPrompt verifies that the standard methodology uses
// the RenderBuild renderer.
func TestBuildStage_Run_UsesStandardPrompt(t *testing.T) {
	invoker := &fakeInvoker{}
	renderer := &fakePromptRenderer{}
	stage := execute.New(invoker, renderer, io.Discard)

	cfg := defaultConfig()
	cfg.Methodology.TDD = false
	in := makeInput(makeBead("bead-1", "Fix bug"), cfg)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if renderer.lastMethodology != "standard" {
		t.Errorf("renderer called with methodology %q, want %q", renderer.lastMethodology, "standard")
	}
}

// TestBuildStage_Run_UsesTDDPrompt verifies that when TDD is active, the Build stage
// uses the TDD-specific prompt renderer.
func TestBuildStage_Run_UsesTDDPrompt(t *testing.T) {
	invoker := &fakeInvoker{}
	renderer := &fakePromptRenderer{}
	stage := execute.New(invoker, renderer, io.Discard)

	cfg := defaultConfig()
	cfg.Methodology.TDD = true
	in := makeInput(makeBead("bead-1", "Implement service"), cfg)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if renderer.lastMethodology != "tdd" {
		t.Errorf("renderer called with methodology %q, want %q", renderer.lastMethodology, "tdd")
	}
}

// TestBuildStage_Run_UsesRefactorPrompt verifies that when refactor label is set,
// the Build stage uses the refactor-specific prompt renderer.
func TestBuildStage_Run_UsesRefactorPrompt(t *testing.T) {
	invoker := &fakeInvoker{}
	renderer := &fakePromptRenderer{}
	stage := execute.New(invoker, renderer, io.Discard)

	cfg := defaultConfig()
	in := makeInput(makeBeadWithLabels("bead-1", "Refactor auth", []string{"refactor:true"}), cfg)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if renderer.lastMethodology != "refactor" {
		t.Errorf("renderer called with methodology %q, want %q", renderer.lastMethodology, "refactor")
	}
}

// TestBuildStage_Run_PassesValidationFailuresToRenderer verifies that ValidationFailures
// from the input are forwarded to the prompt renderer.
func TestBuildStage_Run_PassesValidationFailuresToRenderer(t *testing.T) {
	var gotFailures []string
	invoker := &fakeInvoker{}
	renderer := &fakePromptRenderer{
		renderBuildFn: func(title, desc string, failures []string) (string, error) {
			gotFailures = failures
			return "prompt", nil
		},
	}
	stage := execute.New(invoker, renderer, io.Discard)

	cfg := defaultConfig()
	in := makeInput(makeBead("bead-1", "Fix"), cfg)
	in.ValidationFailures = []string{"test failed: foo_test.go:42", "vet error: unused import"}

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(gotFailures) != 2 {
		t.Errorf("renderer got %d failures, want 2", len(gotFailures))
	}
}

// TestShouldRunPostSuccess_Standard returns true for standard methodology.
func TestShouldRunPostSuccess_Standard(t *testing.T) {
	stage := execute.New(&fakeInvoker{}, &fakePromptRenderer{}, io.Discard)
	cfg := defaultConfig()
	cfg.Methodology.TDD = false

	b := makeBead("bead-1", "Fix bug")
	if !stage.ShouldRunPostSuccess(b, cfg) {
		t.Error("ShouldRunPostSuccess = false, want true for standard methodology")
	}
}

// TestShouldRunPostSuccess_Refactor returns true for refactor methodology.
func TestShouldRunPostSuccess_Refactor(t *testing.T) {
	stage := execute.New(&fakeInvoker{}, &fakePromptRenderer{}, io.Discard)
	cfg := defaultConfig()

	b := makeBeadWithLabels("bead-1", "Refactor", []string{"refactor:true"})
	if !stage.ShouldRunPostSuccess(b, cfg) {
		t.Error("ShouldRunPostSuccess = false, want true for refactor methodology")
	}
}

// TestShouldRunPostSuccess_TDD returns false for TDD methodology because the
// refactor phase must complete before post-success stages run.
func TestShouldRunPostSuccess_TDD(t *testing.T) {
	stage := execute.New(&fakeInvoker{}, &fakePromptRenderer{}, io.Discard)
	cfg := defaultConfig()
	cfg.Methodology.TDD = true

	b := makeBead("bead-1", "Implement feature")
	if stage.ShouldRunPostSuccess(b, cfg) {
		t.Error("ShouldRunPostSuccess = true, want false for TDD methodology")
	}
}

// TestBuildStage_Run_StreamRunReceivesTier verifies that StreamRun is called with the
// tier derived from the bead's priority.
func TestBuildStage_Run_StreamRunReceivesTier(t *testing.T) {
	invoker := &fakeInvoker{}
	stage := execute.New(invoker, &fakePromptRenderer{}, io.Discard)

	cfg := defaultConfig()
	cfg.Models.P1 = "medium"
	b := &bead.Bead{ID: "bead-1", Title: "feature", Priority: 1, Labels: []string{}}
	in := makeInput(b, cfg)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(invoker.streamCalls) != 1 {
		t.Fatalf("StreamRun called %d times, want 1", len(invoker.streamCalls))
	}
	if invoker.streamCalls[0].tier != "medium" {
		t.Errorf("StreamRun tier = %q, want %q", invoker.streamCalls[0].tier, "medium")
	}
}

// TestBuildStage_EscalationDisabled_DoesNotRetryOnFailure verifies that when
// EscalationEnabled is false, a failed invocation is not retried on a higher tier.
func TestBuildStage_EscalationDisabled_DoesNotRetryOnFailure(t *testing.T) {
	callCount := 0
	invoker := &fakeInvoker{
		streamRunFn: func(_ context.Context, _, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
			callCount++
			return nil, fmt.Errorf("invocation failed")
		},
	}
	renderer := &fakePromptRenderer{}
	stage := execute.New(invoker, renderer, io.Discard)

	in := makeInput(makeBead("bead-1", "Fix bug"), defaultConfig())
	in.EscalationEnabled = false

	_, err := stage.Run(context.Background(), in)
	if err == nil {
		t.Fatal("Run() error = nil, want error on failed invocation")
	}
	if callCount != 1 {
		t.Errorf("StreamRun called %d times, want 1 (no escalation when disabled)", callCount)
	}
}

// TestBuildStage_EscalationEnabled_RetriesWithNextTierOnFailure verifies that when
// EscalationEnabled is true and the initial tier fails, the Build stage escalates
// to the next tier in the chain.
func TestBuildStage_EscalationEnabled_RetriesWithNextTierOnFailure(t *testing.T) {
	var calledTiers []string
	invoker := &fakeInvoker{
		streamRunFn: func(_ context.Context, _, tier string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
			calledTiers = append(calledTiers, tier)
			if tier == "low" {
				return nil, fmt.Errorf("low tier failed")
			}
			return &provider.Result{Success: true}, nil
		},
	}
	renderer := &fakePromptRenderer{}
	stage := execute.New(invoker, renderer, io.Discard)

	cfg := defaultConfig()
	cfg.Escalation.Enabled = true
	cfg.Escalation.Chain = []string{"low", "medium", "high"}
	cfg.Models.P2 = "low"

	b := &bead.Bead{ID: "bead-1", Title: "Fix bug", Priority: 2, Labels: []string{}}
	in := makeInput(b, cfg)
	in.EscalationEnabled = true

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil (should succeed after escalation)", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}
	if len(calledTiers) != 2 {
		t.Errorf("StreamRun called with tiers %v, want 2 calls (low, medium)", calledTiers)
	}
	if len(calledTiers) == 2 && (calledTiers[0] != "low" || calledTiers[1] != "medium") {
		t.Errorf("StreamRun tiers = %v, want [low medium]", calledTiers)
	}
}

// fakeTDDCycleRunner is a minimal implementation of TDDCycleRunner used to verify
// that the interface can be satisfied by a concrete type.
type fakeTDDCycleRunner struct{}

func (f *fakeTDDCycleRunner) RunCycles(_ context.Context, _ *bead.Bead, _ *config.Config) (execute.TDDCycleResult, error) {
	return execute.TDDCycleResult{}, nil
}

// TestTDDCycleRunner_InterfaceSatisfied verifies that TDDCycleRunner is a valid
// interface with a RunCycles method that concrete types can implement.
func TestTDDCycleRunner_InterfaceSatisfied(t *testing.T) {
	var _ execute.TDDCycleRunner = (*fakeTDDCycleRunner)(nil)
}

// TestTDDCycleResult_HoldsPhaseMetrics verifies that TDDCycleResult can carry
// per-phase metrics from multiple TDD cycle invocations.
func TestTDDCycleResult_HoldsPhaseMetrics(t *testing.T) {
	result := execute.TDDCycleResult{
		PhaseMetrics: []pipeline.PhaseMetric{
			{Phase: "red"},
			{Phase: "green"},
		},
	}
	if len(result.PhaseMetrics) != 2 {
		t.Errorf("TDDCycleResult.PhaseMetrics: want 2, got %d", len(result.PhaseMetrics))
	}
	if result.PhaseMetrics[1].Phase != "green" {
		t.Errorf("TDDCycleResult.PhaseMetrics[1].Phase: want %q, got %q", "green", result.PhaseMetrics[1].Phase)
	}
}

// TestBuildStage_EscalationEnabled_FailsWhenAllTiersExhausted verifies that when
// EscalationEnabled is true but all tiers in the chain fail, the stage returns an error.
func TestBuildStage_EscalationEnabled_FailsWhenAllTiersExhausted(t *testing.T) {
	callCount := 0
	invoker := &fakeInvoker{
		streamRunFn: func(_ context.Context, _, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
			callCount++
			return nil, fmt.Errorf("always fails")
		},
	}
	renderer := &fakePromptRenderer{}
	stage := execute.New(invoker, renderer, io.Discard)

	cfg := defaultConfig()
	cfg.Escalation.Enabled = true
	cfg.Escalation.Chain = []string{"low", "medium"}
	cfg.Models.P2 = "low"

	b := &bead.Bead{ID: "bead-1", Title: "Fix bug", Priority: 2, Labels: []string{}}
	in := makeInput(b, cfg)
	in.EscalationEnabled = true

	_, err := stage.Run(context.Background(), in)
	if err == nil {
		t.Fatal("Run() error = nil, want error when all tiers fail")
	}
	if callCount != 2 {
		t.Errorf("StreamRun called %d times, want 2 (low + medium)", callCount)
	}
}

// trackingTDDCycleRunner is a test double for TDDCycleRunner that records calls.
type trackingTDDCycleRunner struct {
	runCyclesFn func(ctx context.Context, b *bead.Bead, cfg *config.Config) (execute.TDDCycleResult, error)
}

func (f *trackingTDDCycleRunner) RunCycles(ctx context.Context, b *bead.Bead, cfg *config.Config) (execute.TDDCycleResult, error) {
	if f.runCyclesFn != nil {
		return f.runCyclesFn(ctx, b, cfg)
	}
	return execute.TDDCycleResult{}, nil
}

// TestBuildRun_TDD_FreshContext_DelegatesToTDDCycleRunner verifies that when
// methodology=TDD and FreshContextPerCycle is true, Build.Run() delegates to
// the injected TDDCycleRunner instead of calling StreamRun directly.
func TestBuildRun_TDD_FreshContext_DelegatesToTDDCycleRunner(t *testing.T) {
	var runCyclesCalled bool
	runner := &trackingTDDCycleRunner{
		runCyclesFn: func(_ context.Context, _ *bead.Bead, _ *config.Config) (execute.TDDCycleResult, error) {
			runCyclesCalled = true
			return execute.TDDCycleResult{}, nil
		},
	}

	invoker := &fakeInvoker{}
	renderer := &fakePromptRenderer{}
	stage := execute.New(invoker, renderer, io.Discard).WithTDDCycleRunner(runner)

	cfg := defaultConfig()
	cfg.Methodology.TDD = true
	cfg.Methodology.FreshContextPerCycle = true
	in := makeInput(makeBead("bead-1", "Implement TDD feature"), cfg)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !runCyclesCalled {
		t.Error("TDDCycleRunner.RunCycles was not called, but should have been")
	}
	if len(invoker.streamCalls) != 0 {
		t.Errorf("StreamRun called %d times, want 0 (should delegate to TDDCycleRunner)", len(invoker.streamCalls))
	}
}

// TestBuildRun_TDD_FreshContext_ReturnsPhaseMetricsInOutput verifies that when
// Build.Run() delegates to TDDCycleRunner, the returned Output.PhaseMetrics
// contains the aggregated phase metrics from the TDDCycleResult.
func TestBuildRun_TDD_FreshContext_ReturnsPhaseMetricsInOutput(t *testing.T) {
	phases := []pipeline.PhaseMetric{
		{Phase: "red", DurationMs: 100, Model: "haiku"},
		{Phase: "green", DurationMs: 200, Model: "haiku"},
	}
	runner := &trackingTDDCycleRunner{
		runCyclesFn: func(_ context.Context, _ *bead.Bead, _ *config.Config) (execute.TDDCycleResult, error) {
			return execute.TDDCycleResult{PhaseMetrics: phases}, nil
		},
	}

	stage := execute.New(&fakeInvoker{}, &fakePromptRenderer{}, io.Discard).WithTDDCycleRunner(runner)

	cfg := defaultConfig()
	cfg.Methodology.TDD = true
	cfg.Methodology.FreshContextPerCycle = true
	in := makeInput(makeBead("bead-1", "Implement TDD feature"), cfg)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(out.PhaseMetrics) != 2 {
		t.Fatalf("Output.PhaseMetrics: want 2, got %d", len(out.PhaseMetrics))
	}
	if out.PhaseMetrics[0].Phase != "red" {
		t.Errorf("Output.PhaseMetrics[0].Phase = %q, want %q", out.PhaseMetrics[0].Phase, "red")
	}
	if out.PhaseMetrics[1].Phase != "green" {
		t.Errorf("Output.PhaseMetrics[1].Phase = %q, want %q", out.PhaseMetrics[1].Phase, "green")
	}
}

// TestBuildRun_TDD_FreshContext_NilRunner_FallsBackToStreamRun verifies that when
// methodology=TDD and FreshContextPerCycle=true but no TDDCycleRunner is injected,
// Build.Run() falls back to the existing single-invocation StreamRun path.
func TestBuildRun_TDD_FreshContext_NilRunner_FallsBackToStreamRun(t *testing.T) {
	invoker := &fakeInvoker{}
	stage := execute.New(invoker, &fakePromptRenderer{}, io.Discard) // no WithTDDCycleRunner

	cfg := defaultConfig()
	cfg.Methodology.TDD = true
	cfg.Methodology.FreshContextPerCycle = true
	in := makeInput(makeBead("bead-1", "Implement TDD feature"), cfg)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(invoker.streamCalls) != 1 {
		t.Errorf("StreamRun called %d times, want 1 (nil TDDCycleRunner should fall back to StreamRun)", len(invoker.streamCalls))
	}
}

// TestBuildRun_TDD_FreshContextFalse_UsesSingleInvocationPath verifies that when
// methodology=TDD but FreshContextPerCycle=false, Build.Run() uses StreamRun
// even when a TDDCycleRunner is injected.
func TestBuildRun_TDD_FreshContextFalse_UsesSingleInvocationPath(t *testing.T) {
	runCyclesCalled := false
	runner := &trackingTDDCycleRunner{
		runCyclesFn: func(_ context.Context, _ *bead.Bead, _ *config.Config) (execute.TDDCycleResult, error) {
			runCyclesCalled = true
			return execute.TDDCycleResult{}, nil
		},
	}

	invoker := &fakeInvoker{}
	stage := execute.New(invoker, &fakePromptRenderer{}, io.Discard).WithTDDCycleRunner(runner)

	cfg := defaultConfig()
	cfg.Methodology.TDD = true
	cfg.Methodology.FreshContextPerCycle = false
	in := makeInput(makeBead("bead-1", "Implement TDD feature"), cfg)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if runCyclesCalled {
		t.Error("TDDCycleRunner.RunCycles was called, but should not be when FreshContextPerCycle=false")
	}
	if len(invoker.streamCalls) != 1 {
		t.Errorf("StreamRun called %d times, want 1 (FreshContextPerCycle=false should use single-invocation path)", len(invoker.streamCalls))
	}
}

// TestBuildRun_BuildStrategyTDDLabel_DelegatesToTDDCycleRunner verifies that
// a bead-level build_strategy:tdd label overrides global methodology defaults
// and forces the TDD cycle runner path.
func TestBuildRun_BuildStrategyTDDLabel_DelegatesToTDDCycleRunner(t *testing.T) {
	runCyclesCalled := false
	runner := &trackingTDDCycleRunner{
		runCyclesFn: func(_ context.Context, _ *bead.Bead, _ *config.Config) (execute.TDDCycleResult, error) {
			runCyclesCalled = true
			return execute.TDDCycleResult{}, nil
		},
	}

	invoker := &fakeInvoker{}
	stage := execute.New(invoker, &fakePromptRenderer{}, io.Discard).WithTDDCycleRunner(runner)

	cfg := defaultConfig()
	cfg.Methodology.TDD = false
	cfg.Methodology.BuildStrategy = "single_pass"
	cfg.Methodology.FreshContextPerCycle = true
	in := makeInput(makeBeadWithLabels("bead-1", "Implement feature", []string{"build_strategy:tdd"}), cfg)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !runCyclesCalled {
		t.Error("TDDCycleRunner.RunCycles was not called, want build_strategy:tdd to force TDD path")
	}
	if len(invoker.streamCalls) != 0 {
		t.Errorf("StreamRun called %d times, want 0 when TDD cycle runner is used", len(invoker.streamCalls))
	}
}

// TestBuildStage_Run_PopulatesOutputMetadataFromProviderResult verifies that the
// Build stage copies Model, DurationMs, CostUSD, InputTokens, and OutputTokens
// from the successful StreamRun provider result into the returned Output.
func TestBuildStage_Run_PopulatesOutputMetadataFromProviderResult(t *testing.T) {
	invoker := &fakeInvoker{
		streamRunFn: func(_ context.Context, _, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:      true,
				Model:        "claude-opus-4-5",
				Duration:     2500 * time.Millisecond,
				CostUSD:      0.042,
				InputTokens:  1200,
				OutputTokens: 800,
			}, nil
		},
	}
	stage := execute.New(invoker, &fakePromptRenderer{}, io.Discard)

	out, err := stage.Run(context.Background(), makeInput(makeBead("bead-1", "Add feature"), defaultConfig()))
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if out.Model != "claude-opus-4-5" {
		t.Errorf("Output.Model = %q, want %q", out.Model, "claude-opus-4-5")
	}
	if out.DurationMs != 2500 {
		t.Errorf("Output.DurationMs = %d, want 2500", out.DurationMs)
	}
	if out.CostUSD != 0.042 {
		t.Errorf("Output.CostUSD = %f, want 0.042", out.CostUSD)
	}
	if out.InputTokens != 1200 {
		t.Errorf("Output.InputTokens = %d, want 1200", out.InputTokens)
	}
	if out.OutputTokens != 800 {
		t.Errorf("Output.OutputTokens = %d, want 800", out.OutputTokens)
	}
}

// TestBuildStage_Run_UsesBuildPhaseTierOverride verifies that build invocations
// route through PhaseModelTier("build", beadTier) before StreamRun.
func TestBuildStage_Run_UsesBuildPhaseTierOverride(t *testing.T) {
	invoker := &fakeInvoker{}
	stage := execute.New(invoker, &fakePromptRenderer{}, io.Discard)

	cfg := defaultConfig()
	cfg.Models.P1 = "medium"
	cfg.Methodology.PhaseModels.Build = "low"
	b := &bead.Bead{ID: "bead-1", Title: "feature", Priority: 1, Labels: []string{}}
	in := makeInput(b, cfg)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(invoker.streamCalls) != 1 {
		t.Fatalf("StreamRun called %d times, want 1", len(invoker.streamCalls))
	}
	if invoker.streamCalls[0].tier != "low" {
		t.Errorf("StreamRun tier = %q, want %q", invoker.streamCalls[0].tier, "low")
	}
}
