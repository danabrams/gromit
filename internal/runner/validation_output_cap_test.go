package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

type stubValidationProvider struct {
	result *provider.Result
	err    error
}

func (p *stubValidationProvider) Name() string { return "stub-provider" }

func (p *stubValidationProvider) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return p.result, p.err
}

func (p *stubValidationProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

type countingValidationProvider struct {
	result *provider.Result
	err    error
	calls  int32
}

func (p *countingValidationProvider) Name() string { return "counting-provider" }

func (p *countingValidationProvider) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	atomic.AddInt32(&p.calls, 1)
	return p.result, p.err
}

func (p *countingValidationProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

func (p *countingValidationProvider) Calls() int32 {
	return atomic.LoadInt32(&p.calls)
}

type stubValidationRouter struct {
	provider execution.Provider
	model    string
}

func (r *stubValidationRouter) Select(phase, tier string) (execution.Provider, string) {
	return r.provider, r.model
}

func (r *stubValidationRouter) MarkUnavailable(name string) {}

func (r *stubValidationRouter) RecordOutcome(providerName, failureCategory string) {
}

type stubFailureAnalyzer struct{}

func (a *stubFailureAnalyzer) Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
	return &analyzer.Analysis{
		Category:    analyzer.CategoryLogic,
		Recoverable: false,
		RootCause:   "validation failure",
	}, nil
}

type stubValidationPolicy struct {
	maxAttempts    int
	shouldEscalate bool
}

func (p *stubValidationPolicy) SelectGate(consecutiveSuccesses int) policy.GateType {
	_ = consecutiveSuccesses
	return policy.GateFast
}

func (p *stubValidationPolicy) MaxRecoveryAttempts() int {
	return p.maxAttempts
}

func (p *stubValidationPolicy) ShouldEscalateRecovery() bool {
	return p.shouldEscalate
}

func (p *stubValidationPolicy) MandatoryCommandPrefixes() []string {
	return nil
}

// rendererFailBuild is a test renderer that always fails RenderBuild,
// allowing makeValidationExecuteFn to be exercised up to the render step.
type rendererFailBuild struct {
	mockRenderer
}

func (r *rendererFailBuild) RenderBuild(_ *prompt.Context) (string, error) {
	return "", fmt.Errorf("render error (intentional)")
}

type rendererShapeGreenCapture struct {
	mockRenderer
	renderCtx *prompt.Context
}

func (r *rendererShapeGreenCapture) ShapeGreenPhaseContext(ctx *prompt.Context) *prompt.Context {
	cloned := *ctx
	cloned.FailureContext = "green-shaped-context"
	cloned.ClaudeMD = ""
	return &cloned
}

func (r *rendererShapeGreenCapture) ShapeRedPhaseContext(ctx *prompt.Context) *prompt.Context {
	return ctx
}

func (r *rendererShapeGreenCapture) ShapeRefactorPhaseContext(ctx *prompt.Context) *prompt.Context {
	return ctx
}

func (r *rendererShapeGreenCapture) RenderBuild(ctx *prompt.Context) (string, error) {
	r.renderCtx = ctx
	return "", fmt.Errorf("render error (intentional)")
}

func setupValidationEscalationRunner(t *testing.T, result *provider.Result) (*config.Config, *Runner, func(context.Context, *runtypes.BeadContext) bool) {
	t.Helper()

	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			Enabled: true,
			Chain:   []string{provider.TierLow, provider.TierMedium},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	stubProvider := &stubValidationProvider{result: result}
	router := &stubValidationRouter{
		provider: stubProvider,
		model:    "stub-model",
	}

	r := &Runner{
		cfg:      cfg,
		renderer: &mockRenderer{},
		output:   io.Discard,
		invoker:  execution.NewInvoker(router, io.Discard, nil),
	}
	r.escalationHandler = escalation.NewHandler(cfg, &stubFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	return cfg, r, r.makeValidationExecuteFn()
}

func newValidationBeadContext(id string) *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: id, Title: "validation concurrent"},
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
		Tier:      provider.TierLow,
	}
}

// TestMakeValidationExecuteFn_TruncatesPrevFailure verifies that when
// makeValidationExecuteFn runs with a large Result.Output, the PrevFailure
// field assigned to PromptCtx is capped at ~50KB.
func TestMakeValidationExecuteFn_TruncatesPrevFailure(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	var logBuf strings.Builder
	r := &Runner{
		cfg:      cfg,
		renderer: &rendererFailBuild{},
		output:   &logBuf,
	}

	// Build Result.Output larger than 50KB
	line := strings.Repeat("y", 100) + "\n"
	var sb strings.Builder
	for sb.Len() < 60*1024 {
		sb.WriteString(line)
	}
	largeOutput := sb.String()

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-cap", Title: "Cap test"},
		Result:    &runtypes.IterationResult{Output: largeOutput},
		PromptCtx: &prompt.Context{},
	}

	fn := r.makeValidationExecuteFn()
	fn(context.Background(), bc)

	const maxAllowed = 55 * 1024 // 50KB cap + small overhead for marker
	if len(bc.PromptCtx.PrevFailure) > maxAllowed {
		t.Errorf("PrevFailure length %d exceeds cap %d; large Result.Output not truncated before prompt context",
			len(bc.PromptCtx.PrevFailure), maxAllowed)
	}
}

func TestMakeValidationExecuteFn_UsesGreenShapedContextForRenderBuild(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	renderer := &rendererShapeGreenCapture{}
	var logBuf strings.Builder
	r := &Runner{
		cfg:      cfg,
		renderer: renderer,
		output:   &logBuf,
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "test-shape-green", Title: "green shaping"},
		Result: &runtypes.IterationResult{
			Output: "validation failure output",
		},
		PromptCtx: &prompt.Context{
			FailureContext: "original context",
			ClaudeMD:       "project-wide payload",
		},
	}

	fn := r.makeValidationExecuteFn()
	if ok := fn(context.Background(), bc); ok {
		t.Fatal("expected validation execute to fail when RenderBuild errors")
	}
	if renderer.renderCtx == nil {
		t.Fatal("expected RenderBuild to be called")
	}
	if renderer.renderCtx.FailureContext != "green-shaped-context" {
		t.Fatalf("expected green-shaped context in RenderBuild, got %q", renderer.renderCtx.FailureContext)
	}
	if renderer.renderCtx.ClaudeMD != "" {
		t.Fatalf("expected shaped context to trim ClaudeMD, got %q", renderer.renderCtx.ClaudeMD)
	}
}

func TestMakeValidationExecuteFn_SetsPromptContextBeadWhenMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	renderer := &rendererShapeGreenCapture{}
	r := &Runner{
		cfg:      cfg,
		renderer: renderer,
		output:   io.Discard,
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "full-gate", Title: "full validation gate"},
		Result: &runtypes.IterationResult{Output: "validation failed"},
		PromptCtx: &prompt.Context{
			FailureContext: "existing context",
		},
	}

	fn := r.makeValidationExecuteFn()
	_ = fn(context.Background(), bc)

	if renderer.renderCtx == nil {
		t.Fatal("expected RenderBuild to be called")
	}
	if renderer.renderCtx.Bead == nil {
		t.Fatal("expected PromptCtx.Bead to be populated before RenderBuild")
	}
	if renderer.renderCtx.Bead.ID != "full-gate" {
		t.Fatalf("expected PromptCtx.Bead.ID %q, got %q", "full-gate", renderer.renderCtx.Bead.ID)
	}
}

func TestMakeValidationExecuteFn_DisablesEscalationDuringValidation(t *testing.T) {
	_, _, fn := setupValidationEscalationRunner(t, &provider.Result{
		Success:  false,
		Output:   "build failed",
		ExitCode: 1,
	})
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-validation-escalation", Title: "validation escalation"},
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
		Tier:      provider.TierLow,
	}

	if ok := fn(context.Background(), bc); ok {
		t.Fatal("expected validation execute to fail with stub provider failure")
	}
	if bc.Tier != provider.TierMedium {
		t.Fatalf("expected tier to escalate to %q, got %q", provider.TierMedium, bc.Tier)
	}
	if !bc.Result.Escalated {
		t.Fatalf("expected Escalated=true after validation escalation")
	}
}

func TestMakeValidationExecuteFn_RespectsMaxRecoveryAttempts(t *testing.T) {
	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			Enabled: true,
			Chain:   []string{provider.TierLow, provider.TierMedium},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	countingProvider := &countingValidationProvider{
		result: &provider.Result{
			Success:  false,
			Output:   "build failed",
			ExitCode: 1,
		},
	}
	router := &stubValidationRouter{
		provider: countingProvider,
		model:    "stub-model",
	}

	r := &Runner{
		cfg:              cfg,
		renderer:         &mockRenderer{},
		output:           io.Discard,
		invoker:          execution.NewInvoker(router, io.Discard, nil),
		validationPolicy: &stubValidationPolicy{maxAttempts: 1, shouldEscalate: true},
	}
	r.escalationHandler = escalation.NewHandler(cfg, &stubFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-max-attempts", Title: "validation attempts"},
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
		Tier:      provider.TierLow,
	}

	fn := r.makeValidationExecuteFn()
	if ok := fn(context.Background(), bc); ok {
		t.Fatal("expected validation execute to fail with stub provider failure")
	}
	if got := countingProvider.Calls(); got != 1 {
		t.Fatalf("expected 1 validation attempt, got %d", got)
	}
	if bc.Tier != provider.TierLow {
		t.Fatalf("expected tier to remain %q, got %q", provider.TierLow, bc.Tier)
	}
	if bc.Result.Escalated {
		t.Fatalf("expected Escalated=false when max attempts is 1")
	}
}

func TestMakeValidationExecuteFn_ConcurrentDoesNotMutateEscalationConfig(t *testing.T) {
	cfg, _, fn := setupValidationEscalationRunner(t, &provider.Result{
		Success:  false,
		Output:   "build failed",
		ExitCode: 1,
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			bc := newValidationBeadContext(fmt.Sprintf("test-validation-%d", idx))
			_ = fn(context.Background(), bc)
		}(i)
	}
	wg.Wait()

	if !cfg.Escalation.Enabled {
		t.Fatalf("expected Escalation.Enabled to remain true after concurrent validation")
	}
}
