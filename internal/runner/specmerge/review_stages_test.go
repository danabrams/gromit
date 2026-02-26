package specmerge_test

import (
    "context"
    "errors"
    "io"
    "testing"

    "github.com/danabrams/gromit/internal/provider"
    "github.com/danabrams/gromit/internal/prompt"
    "github.com/danabrams/gromit/internal/runner/specmerge"
)

func TestRunStage2SpecConformance_RendersSpecDiffAndReturnsResult(t *testing.T) {
    ctx := context.Background()
    fakeRenderer := &capturingRenderer{specContent: "# Spec\n- Do the thing", rulesForPhase: "phase rules"}
    router := &fakeRouter{
        selectFn: func(phase, tier string) (provider.Provider, string) {
            if phase != "spec_conformance" {
                t.Fatalf("phase = %q, want spec_conformance", phase)
            }
            if tier != "high" {
                t.Fatalf("tier = %q, want high", tier)
            }
            return &fakeProvider{runFn: func(ctx context.Context, promptText, tier string) (*provider.Result, error) {
                if promptText == "" {
                    return nil, errors.New("prompt missing")
                }
                if tier != "high" {
                    return nil, errors.New("unexpected tier")
                }
                return &provider.Result{Output: `{"passed":true,"summary":"Spec ready"}`}, nil
            }}, "opus"
        },
    }

    deps := specmerge.ReviewStageDependencies{Router: router, Renderer: fakeRenderer}
    result, provResult, err := specmerge.RunStage2SpecConformance(ctx, deps, "payments", "diff --git", "high")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result == nil {
        t.Fatal("expected review result")
    }
    if provResult == nil {
        t.Fatal("expected provider result")
    }
    if result.Summary != "Spec ready" {
        t.Errorf("summary = %q, want Spec ready", result.Summary)
    }
    if fakeRenderer.renderCtx == nil {
        t.Fatal("RenderReview was not called")
    }
    if fakeRenderer.renderCtx.Spec != fakeRenderer.specContent {
        t.Errorf("spec = %q, want %q", fakeRenderer.renderCtx.Spec, fakeRenderer.specContent)
    }
    if fakeRenderer.renderCtx.Diff != "diff --git" {
        t.Errorf("diff = %q, want diff --git", fakeRenderer.renderCtx.Diff)
    }
    if fakeRenderer.lastRulesPhase != "spec_conformance" {
        t.Errorf("rules phase = %q", fakeRenderer.lastRulesPhase)
    }
}

// capturingRenderer records context supplied to RenderReview.
type capturingRenderer struct {
    specContent    string
    rulesForPhase  string
    lastRulesPhase string
    renderCtx      *prompt.ReviewContext
}

func (r *capturingRenderer) LoadSpec(name string) (string, error) {
    if name != "payments" {
        return "", errors.New("unexpected spec name")
    }
    return r.specContent, nil
}

func (r *capturingRenderer) LoadRulesForPhase(phase string) (string, error) {
    r.lastRulesPhase = phase
    return r.rulesForPhase, nil
}

func (r *capturingRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
    r.renderCtx = ctx
    return "rendered prompt", nil
}

// fakeRouter satisfies the subset of provider.Router needed for the tests.
type fakeRouter struct {
    selectFn func(phase, tier string) (provider.Provider, string)
}

func (f *fakeRouter) Select(phase, tier string) (provider.Provider, string) {
    return f.selectFn(phase, tier)
}

// fakeProvider is a minimal provider implementation used by the test.
type fakeProvider struct {
    runFn func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
}

func (f *fakeProvider) Name() string                       { return "fake" }
func (f *fakeProvider) ModelForTier(tier string) string     { return "test-model" }
func (f *fakeProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
    return f.runFn(ctx, prompt, tier)
}
func (f *fakeProvider) StreamRun(ctx context.Context, prompt string, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
    return nil, provider.ErrStreamNotSupported
}
func (f *fakeProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
    return nil, errors.New("not implemented")
}
func (f *fakeProvider) IsUsageLimitError(result *provider.Result, err error) bool { return false }
func (f *fakeProvider) IsValidationPassed(result *provider.Result) bool          { return false }
func (f *fakeProvider) IsScopeTooLarge(result *provider.Result) (bool, string)      { return false, "" }
