package runner

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/specflow"
	"github.com/danabrams/gromit/internal/tracker"
)

func TestNewStageAwarePreImplementationHookReturnsNilWhenNotSpecStage(t *testing.T) {
	t.Parallel()

	hook := newStageAwarePreImplementationHook(nil, nil, nil, nil, "", nil, nil)
	if hook != nil {
		t.Fatal("hook should be nil when stage context is missing")
	}

	hook = newStageAwarePreImplementationHook(&StageContext{
		SpecName: "",
		Stage:    specflow.StageImplementation,
	}, &config.Config{}, &fakePromptRenderer{}, execution.NewInvoker(&execution.noopRouter{}, io.Discard, nil), "", nil, nil)
	if hook != nil {
		t.Fatal("hook should be nil when spec stage is not acceptance-tests")
	}
}

func TestNewStageAwarePreImplementationHookRunsAcceptanceAuthoring(t *testing.T) {
	t.Parallel()

	store := &specStageStore{stage: specflow.StageAcceptanceTests}
	stageCtx := &StageContext{
		SpecName: "spec-foo",
		Stage:    specflow.StageAcceptanceTests,
		Manager:  specflow.NewManager(store),
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Methodology.ATDD = true
	cfg.Methodology.Granularity = config.MethodologyGranularitySpec

	router := &fakeRouter{provider: &fakeProvider{}}
	invoker := execution.NewInvoker(router, io.Discard, nil)

	renderer := &fakePromptRenderer{}
	hook := newStageAwarePreImplementationHook(stageCtx, cfg, renderer, invoker, "cache-key", nil, io.Discard)
	if hook == nil {
		t.Fatal("hook should not be nil for acceptance test stage")
	}

	if err := hook(context.Background()); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}

	if renderer.renderAcceptanceCalls != 1 {
		t.Fatalf("expected renderer.RenderAcceptanceTests to be called once, got %d", renderer.renderAcceptanceCalls)
	}
	if renderer.lastBuildBead == nil {
		t.Fatal("expected renderer.BuildContext to receive a bead")
	}
	if len(renderer.lastBuildBead.Labels) == 0 || renderer.lastBuildBead.Labels[0] != tracker.SpecLabelPrefix+stageCtx.SpecName {
		t.Fatalf("expected spec label %s, got %v", tracker.SpecLabelPrefix+stageCtx.SpecName, renderer.lastBuildBead.Labels)
	}
	if router.provider.calls != 1 {
		t.Fatalf("expected provider.StreamRun called once, got %d", router.provider.calls)
	}
	if router.provider.lastPrompt != renderer.renderedPrompt {
		t.Fatalf("expected provider prompt %q, got %q", renderer.renderedPrompt, router.provider.lastPrompt)
	}
	if router.provider.lastTier == "" {
		t.Fatal("expected provider tier to be set")
	}
}

type fakePromptRenderer struct {
	lastBuildBead       *bead.Bead
	renderAcceptanceCtx *prompt.Context
	renderAcceptanceCalls int
	renderedPrompt      string
}

func (f *fakePromptRenderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string, phase string) (*prompt.Context, error) {
	f.lastBuildBead = b
	if f.lastBuildBead == nil {
		return nil, fmt.Errorf("bead is nil")
	}
	f.renderAcceptanceCtx = &prompt.Context{
		Bead:     b,
		Spec:     fmt.Sprintf("spec-body for %s", b.Title),
		SpecName: tracker.SpecLabelPrefix + b.Title,
		WorkDir:  "/tmp",
		Model:    model,
	}
	return f.renderAcceptanceCtx, nil
}

func (f *fakePromptRenderer) RenderAcceptanceTests(ctx *prompt.Context) (string, error) {
	f.renderAcceptanceCalls++
	f.renderAcceptanceCtx = ctx
	f.renderedPrompt = "rendered-acceptance-prompt"
	return f.renderedPrompt, nil
}

func (*fakePromptRenderer) RenderBuild(string, string, []string) (string, error)              { return "", nil }
func (*fakePromptRenderer) RenderAnalyze(*prompt.AnalyzeContext) (string, error)              { return "", nil }
func (*fakePromptRenderer) RenderLearn(*prompt.LearnContext) (string, error)                  { return "", nil }
func (*fakePromptRenderer) RenderDecompose(*prompt.DecomposeContext) (string, error)          { return "", nil }
func (*fakePromptRenderer) RenderScope(*prompt.ScopeContext) (string, error)                  { return "", nil }
func (*fakePromptRenderer) RenderPrecheck(*prompt.PrecheckContext) (string, error)            { return "", nil }
func (*fakePromptRenderer) RenderSpecAcceptance(*prompt.SpecAcceptanceContext) (string, error) { return "", nil }
func (*fakePromptRenderer) RenderSpecGate(*prompt.SpecGateContext) (string, error)             { return "", nil }
func (*fakePromptRenderer) RenderReview(*prompt.ReviewContext) (string, error)                 { return "", nil }
func (*fakePromptRenderer) RenderThoroughReview(*prompt.ThoroughReviewContext) (string, error) { return "", nil }
func (*fakePromptRenderer) RenderATDDBuild(*prompt.Context) (string, error)                    { return "", nil }
func (*fakePromptRenderer) RenderATDDDiagnostic(*prompt.DiagnosticContext) (string, error)     { return "", nil }
func (*fakePromptRenderer) RenderTDDBuild(*prompt.Context) (string, error)                     { return "", nil }
func (*fakePromptRenderer) RenderTDDRed(*prompt.TDDRedContext) (string, error)                 { return "", nil }
func (*fakePromptRenderer) RenderTDDGreen(*prompt.TDDGreenContext) (string, error)             { return "", nil }
func (*fakePromptRenderer) RenderRefactor(*prompt.Context) (string, error)                     { return "", nil }
func (*fakePromptRenderer) RenderTestFix(*prompt.TestFixContext) (string, error)               { return "", nil }
func (*fakePromptRenderer) RenderCoverageValidation(*prompt.CoverageValidationContext) (string, error) {
	return "", nil
}
func (*fakePromptRenderer) LoadSpec(string) (string, error) { return "", nil }
func (*fakePromptRenderer) LoadClaudeMD() (string, error)   { return "", nil }
func (*fakePromptRenderer) LoadRules() (string, error)      { return "", nil }
func (*fakePromptRenderer) LoadRulesForPhase(string) (string, error) {
	return "", nil
}
func (*fakePromptRenderer) GetLearningsFile() *learnings.File {
	return nil
}
func (*fakePromptRenderer) SetSiblingTouchedPackagesResolver(prompt.SiblingTouchedPackagesResolver) {}
func (*fakePromptRenderer) LastDiagnostics() *prompt.PromptDiagnostics { return nil }

type fakeRouter struct {
	provider *fakeProvider
}

func (f *fakeRouter) Select(phase, tier string) (execution.Provider, string) {
	return f.provider, tier
}
func (*fakeRouter) MarkUnavailable(string)                                     {}
func (*fakeRouter) RecordOutcome(providerName, failureCategory string)       {}

type fakeProvider struct {
	calls      int
	lastPrompt string
	lastTier   string
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	f.calls++
	f.lastPrompt = prompt
	f.lastTier = tier
	return &provider.Result{Success: true}, nil
}

func (*fakeProvider) IsUsageLimitError(*provider.Result, error) bool { return false }
