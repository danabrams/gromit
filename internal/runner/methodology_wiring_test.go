package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	// blank import not needed — mockPromptRenderer is defined in interfaces_test.go
	// which is in the same package
)

// --- Test helpers ---

func newMethodologyWiringConfig() *config.Config {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 2,
		},
		Escalation: config.EscalationConfig{
			MaxRetriesPerModel: 1,
			MaxRetriesPerBead:  3,
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:        300,
			StallTimeout:       30,
			StallTimeoutActive: 10,
		},
		Methodology: config.MethodologyConfig{
			ATDD: true,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

// newTestEscalationHandler creates a minimal escalation handler for tests.
func newTestEscalationHandler(cfg *config.Config) *escalation.Handler {
	return escalation.NewHandler(
		cfg,
		nil, // analyzer
		nil, // beadClient
		nil, // decomposeFn
		nil, // createSubFn
		func(format string, args ...interface{}) {}, // logFn (no-op)
		nil, // showPartialProgressFn
	)
}

// --- NewRunnerWithDeps wires methodologyExec ---

// Expected failure: Runner struct does not have a methodologyExec field yet.
// After implementation, NewRunnerWithDeps will construct a methodology.Executor
// and assign it to r.methodologyExec via makeMethodologyExec().
func TestNewRunnerWithDeps_WiresMethodologyExec(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	var buf strings.Builder

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   newMockRouter(),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockRenderer{},
		CmdRunner: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "VALIDATION_PASSED", "", 0, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	// Expected failure: r.methodologyExec does not exist as a field on Runner.
	if r.methodologyExec == nil {
		t.Fatal("NewRunnerWithDeps should wire methodologyExec field on Runner")
	}
}

// --- processBead delegates ATDD acceptance generation to methodologyExec ---

// Expected failure: processBead currently calls r.runAcceptanceTestsWithRetry (local method)
// instead of r.methodologyExec.RunAcceptanceTestsWithRetry. After implementation,
// processBead will delegate to the methodology.Executor via the methodologyExec field.
func TestProcessBead_ATDD_DelegatesToMethodologyExec(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	var buf strings.Builder

	// Track which callbacks are invoked and what arguments they receive.
	var acceptancePromptReceived string
	validateCallCount := 0

	// Create a methodology.Executor with tracking callbacks
	exec := methodology.NewExecutorWithAnalysis(
		cfg,
		&buf,
		func(ctx *prompt.Context) (string, error) {
			return "acceptance-test-prompt-from-render", nil
		},
		func(ctx context.Context, bc *runtypes.BeadContext, p string) error {
			acceptancePromptReceived = p
			return nil
		},
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			validateCallCount++
			// Acceptance verification runs once at the end of the cycle.
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		},
		nil, // analyzeFn
		nil, // getDiffFn
	)

	mockProv := &mockProviderWithRouterTracking{
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
	}
	router := provider.NewSingleProviderRouter(mockProv)

	noopCmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "VALIDATION_PASSED", "", 0, nil
	}

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:     &mockBeadClient{},
		Router:    router,
		Renderer:  &mockRenderer{},
		CmdRunner: noopCmdRunner,
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	// Replace the auto-wired methodologyExec with our tracking version
	r.methodologyExec = exec

	b := &bead.Bead{
		ID:       "test-atdd-001",
		Title:    "Test ATDD delegation",
		Priority: 1,
		Labels:   []string{"methodology:true"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	// Verify the rendered acceptance test prompt flowed through the invokeFn callback
	if acceptancePromptReceived != "acceptance-test-prompt-from-render" {
		t.Errorf("processBead with ATDD should delegate acceptance tests to methodologyExec, "+
			"passing the rendered prompt through invokeFn; got prompt=%q", acceptancePromptReceived)
	}
	if validateCallCount == 0 {
		t.Error("processBead with ATDD should run end-of-cycle acceptance verification")
	}
	if result.Error != nil {
		t.Errorf("processBead should succeed, got error: %v", result.Error)
	}
}

// --- processBead does not short-circuit to already-done in ATDD pre-build phases ---

// Expected failure: processBead currently checks against the local errATDDAlreadyDone sentinel.
// After implementation, it will check against methodology.ErrATDDAlreadyDone (or use
// methodology.IsATDDAlreadyDone) since the sentinel now lives in the methodology package.
func TestProcessBead_ATDD_DoesNotMarkAlreadyDoneWithoutVerifyFailPhase(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	var buf strings.Builder

	// Acceptance verification passes at end-of-cycle.
	exec := methodology.NewExecutorWithAnalysis(
		cfg,
		&buf,
		func(ctx *prompt.Context) (string, error) {
			return "test prompt", nil
		},
		func(ctx context.Context, bc *runtypes.BeadContext, p string) error {
			return nil // acceptance tests succeed
		},
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		},
		nil,
		nil, // getDiffFn
	)

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   newMockRouter(),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockRenderer{},
		CmdRunner: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "VALIDATION_PASSED", "", 0, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	r.methodologyExec = exec

	b := &bead.Bead{
		ID:       "test-atdd-done-001",
		Title:    "Already done bead",
		Priority: 1,
		Labels:   []string{"methodology:true"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if result.AlreadyDone {
		t.Error("processBead should not mark AlreadyDone when verify-fail phase is disabled")
	}
	if !result.Success {
		t.Error("processBead should succeed when acceptance verification passes")
	}
}

// --- processBead delegates refactor through methodologyExec in full ATDD flow ---

// Expected failure: processBead's refactor path currently has a conditional that checks
// if methodologyExec != nil and falls back to r.runRefactorPhase otherwise. After
// implementation, the fallback is removed and refactor always goes through methodologyExec.
// This test exercises the full processBead flow (ATDD → build → validate → refactor) and
// verifies the refactor render callback on methodologyExec is invoked through processBead.
func TestProcessBead_FullFlow_RefactorDelegatesToMethodologyExec(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	var buf strings.Builder

	var refactorPromptRendered string

	// Track calls to end-of-cycle acceptance verification.
	fullFlowValidateCallCount := 0

	// Create a fully-wired executor using the same factory pattern as makeMethodologyExec
	exec := methodology.NewExecutorWithEscalation(
		cfg,
		&buf,
		func(ctx *prompt.Context) (string, error) {
			return "acceptance test prompt", nil
		},
		func(ctx context.Context, bc *runtypes.BeadContext, p string) error {
			return nil // acceptance tests succeed
		},
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			fullFlowValidateCallCount++
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		},
		func(bc *runtypes.BeadContext, nextTier string) {
			// no-op escalation
		},
	)

	// Wire refactor deps with tracking — use 4 files to exceed the default MinFilesChanged=3.
	// Pass nil for validateFn so it doesn't overwrite the ATDD validateFn.
	exec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line\ndiff --git a/b.go b/b.go\n+line\n" +
				"diff --git a/c.go b/c.go\n+line\ndiff --git a/d.go b/d.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			refactorPromptRendered = "refactor-prompt-via-methodologyExec"
			return "refactor prompt", nil
		},
		func(ctx context.Context, p string, tier string) (*claude.Result, error) {
			return &claude.Result{Success: true}, nil
		},
		nil, // validateFn — keep the ATDD validateFn (tests should fail before implementation)
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	mockProv := &mockProviderWithRouterTracking{
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
	}
	router := provider.NewSingleProviderRouter(mockProv)

	noopCmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "VALIDATION_PASSED", "", 0, nil
	}

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:     &mockBeadClient{},
		Router:    router,
		Renderer:  &mockRenderer{},
		CmdRunner: noopCmdRunner,
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	// Replace auto-wired methodologyExec with our tracking version
	r.methodologyExec = exec

	b := &bead.Bead{
		ID:       "test-full-refactor-001",
		Title:    "Full flow refactor delegation",
		Priority: 1,
		Labels:   []string{"methodology:true"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if result.Error != nil {
		t.Fatalf("processBead should succeed, got error: %v", result.Error)
	}
	if refactorPromptRendered != "refactor-prompt-via-methodologyExec" {
		t.Error("processBead refactor phase should delegate to methodologyExec.RunRefactorPhase " +
			"which calls the refactor render callback — refactor was not invoked through methodologyExec")
	}
}

// --- EscalateTierFn callback wraps escalation.Handler.EscalateTier ---

// Expected failure: makeMethodologyExec does not currently wire EscalateTierFn to
// escalation.Handler.EscalateTier. After implementation, the callback wraps
// escalation.Handler.EscalateTier so tier escalation during ATDD phases sets
// bc.Result.Escalated=true and moves bc.Tier up from the initial TierLow value.
// The provider always fails, so the full escalation chain runs (low→medium→high).
func TestMethodologyExec_EscalateTierFn_WrapsEscalationHandler(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Escalation.Enabled = true // Required for NextEscalationTier to return non-empty
	var buf strings.Builder

	// Use a provider that always fails, forcing retry exhaustion and escalation
	failingProv := &mockProviderWithRouterTracking{
		name: "failing-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{Success: false, Output: "build failed"}, nil
		},
		streamRunResult: &provider.Result{Success: false, Output: "build failed"},
	}
	router := provider.NewSingleProviderRouter(failingProv)

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   router,
		Renderer: &mockRenderer{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	// Expected failure: r.methodologyExec does not exist on Runner
	if r.methodologyExec == nil {
		t.Fatal("methodologyExec must be wired")
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-esc-001", Title: "Escalation test", Priority: 2},
		Tier:      provider.TierLow,
		Model:     "haiku",
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	// RunAcceptanceTestsWithRetry should exhaust retries at TierLow, then
	// escalate via the wired EscalateTierFn callback. The provider always fails,
	// so the full escalation chain runs (low → medium → high).
	_ = r.methodologyExec.RunAcceptanceTestsWithRetry(context.Background(), bc)

	// Verify escalation happened: tier must have moved up from TierLow.
	// The default escalation chain [haiku, sonnet, opus] maps to [low, medium, high].
	// Since the provider always fails, all tiers are exhausted, ending at TierHigh.
	if bc.Tier == provider.TierLow {
		t.Error("EscalateTierFn should be wired to escalation.Handler.EscalateTier, " +
			"updating bc.Tier from low to a higher tier after retry exhaustion")
	}
	// escalation.Handler.EscalateTier sets bc.Result.Escalated = true
	if !bc.Result.Escalated {
		t.Error("EscalateTierFn should set bc.Result.Escalated=true via escalation.Handler.EscalateTier")
	}
	// escalation.Handler.EscalateTier sets bc.Result.EscalatedTo to a model name
	if bc.Result.EscalatedTo == "" {
		t.Error("EscalateTierFn should set bc.Result.EscalatedTo via escalation.Handler.EscalateTier")
	}
}

// --- InvokeFn callback routes through the provider chain ---

// Expected failure: makeMethodologyExec does not currently wire InvokeFn to the
// provider chain. After implementation, the InvokeFn callback wraps the router's
// Select+Run path so ATDD invocations go through the same provider infrastructure.
func TestMethodologyExec_InvokeFn_WrapsExecutionInvoker(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	var buf strings.Builder

	var promptReceivedByProvider string
	var tierReceivedByProvider string
	mockProv := &mockProviderWithRouterTracking{
		name:            "test-provider",
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
	}
	mockProv.streamRunFn = func(ctx context.Context, p, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		promptReceivedByProvider = p
		tierReceivedByProvider = tier
		return &provider.Result{Success: true, Model: "test-model"}, nil
	}
	router := provider.NewSingleProviderRouter(mockProv)

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   router,
		Renderer: &mockRenderer{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	// Expected failure: r.methodologyExec does not exist on Runner
	if r.methodologyExec == nil {
		t.Fatal("methodologyExec must be wired")
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-invoke-001", Title: "Invoke test"},
		Tier:      "medium",
		Model:     "sonnet",
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	// RunAcceptanceTests renders via renderFn then invokes via invokeFn.
	// The auto-wired renderFn wraps renderer.RenderAcceptanceTests which returns
	// "mock acceptance tests prompt". InvokeFn should route that through the provider.
	err = r.methodologyExec.RunAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunAcceptanceTests returned unexpected error: %v", err)
	}

	// Verify the rendered prompt reached the provider (end-to-end wiring)
	if promptReceivedByProvider != "mock acceptance tests prompt" {
		t.Errorf("InvokeFn should route the rendered acceptance test prompt through the provider; "+
			"expected %q, got %q", "mock acceptance tests prompt", promptReceivedByProvider)
	}
	// Verify the tier was passed through
	if tierReceivedByProvider != "medium" {
		t.Errorf("InvokeFn should pass bc.Tier=%q to the provider; got %q",
			"medium", tierReceivedByProvider)
	}
}

func TestMethodologyExec_RenderAcceptanceTests_UsesRedShapedContext(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	var buf strings.Builder

	var renderedCtx *prompt.Context
	renderer := &shapeAwarePromptRenderer{
		mockPromptRenderer: mockPromptRenderer{
			RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
				renderedCtx = ctx
				return "acceptance prompt", nil
			},
		},
		shapeRedFn: func(ctx *prompt.Context) *prompt.Context {
			cloned := *ctx
			cloned.ClaudeMD = ""
			cloned.FailureContext = "red-shaped-context"
			return &cloned
		},
	}

	mockProv := &mockProviderWithRouterTracking{
		name:            "test-provider",
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
	}
	router := provider.NewSingleProviderRouter(mockProv)

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   router,
		Renderer: renderer,
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	bc := &runtypes.BeadContext{
		Bead:  &bead.Bead{ID: "test-red-shape-001", Title: "shape acceptance context"},
		Tier:  provider.TierMedium,
		Model: "sonnet",
		Result: &runtypes.IterationResult{
			Escalated: false,
		},
		PromptCtx: &prompt.Context{
			WorkDir:         t.TempDir(),
			ClaudeMD:        "large context that should be dropped",
			FailureContext:  "original failure context",
			PrevFailure:     "orig",
			RecentLearnings: nil,
		},
	}

	if err := r.methodologyExec.RunAcceptanceTests(context.Background(), bc); err != nil {
		t.Fatalf("RunAcceptanceTests returned error: %v", err)
	}
	if renderedCtx == nil {
		t.Fatal("expected RenderAcceptanceTests to receive a context")
	}
	if renderedCtx.FailureContext != "red-shaped-context" {
		t.Fatalf("expected red-shaped context to be passed to renderer, got %q", renderedCtx.FailureContext)
	}
	if renderedCtx.ClaudeMD != "" {
		t.Fatalf("expected shaped context to trim ClaudeMD, got %q", renderedCtx.ClaudeMD)
	}
}

type shapeAwarePromptRenderer struct {
	mockPromptRenderer
	shapeRedFn      func(ctx *prompt.Context) *prompt.Context
	shapeGreenFn    func(ctx *prompt.Context) *prompt.Context
	shapeRefactorFn func(ctx *prompt.Context) *prompt.Context
}

func (m *shapeAwarePromptRenderer) ShapeRedPhaseContext(ctx *prompt.Context) *prompt.Context {
	if m.shapeRedFn != nil {
		return m.shapeRedFn(ctx)
	}
	return ctx
}

func (m *shapeAwarePromptRenderer) ShapeGreenPhaseContext(ctx *prompt.Context) *prompt.Context {
	if m.shapeGreenFn != nil {
		return m.shapeGreenFn(ctx)
	}
	return ctx
}

func (m *shapeAwarePromptRenderer) ShapeRefactorPhaseContext(ctx *prompt.Context) *prompt.Context {
	if m.shapeRefactorFn != nil {
		return m.shapeRefactorFn(ctx)
	}
	return ctx
}

func TestMethodologyExec_InvokeFn_FailureIncludesProviderStderr(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Escalation.MaxRetriesPerModel = 0
	cfg.Escalation.Chain = []string{provider.TierLow}
	var buf strings.Builder

	mockProv := &mockProviderWithRouterTracking{name: "test-provider"}
	mockProv.streamRunFn = func(ctx context.Context, p, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return &provider.Result{
			Success:  false,
			Model:    "test-model",
			ExitCode: 7,
			Stdout:   "stdout details",
			Stderr:   "stderr details",
			Output:   "combined details",
		}, nil
	}
	router := provider.NewSingleProviderRouter(mockProv)

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   router,
		Renderer: &mockRenderer{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}
	if r.methodologyExec == nil {
		t.Fatal("methodologyExec must be wired")
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-invoke-fail-001", Title: "Invoke fail test"},
		Tier:      provider.TierLow,
		Model:     "haiku",
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	err = r.methodologyExec.RunAcceptanceTestsWithRetry(context.Background(), bc)
	if err == nil {
		t.Fatal("RunAcceptanceTestsWithRetry should fail when provider returns unsuccessful result")
	}
	if !strings.Contains(err.Error(), "stderr=stderr details") {
		t.Fatalf("expected stderr details in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "output=combined details") {
		t.Fatalf("expected output details in error, got: %v", err)
	}
}

func TestMethodologyExec_InvokeFn_CodexTransportFailureFallsBackToAlternateProvider(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Escalation.MaxRetriesPerModel = 0
	cfg.Escalation.Chain = []string{provider.TierLow}
	var buf strings.Builder

	codexCalls := 0
	claudeCalls := 0

	codexProv := &mockProviderWithRouterTracking{name: "codex"}
	codexProv.streamRunFn = func(ctx context.Context, p, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		codexCalls++
		return &provider.Result{
			Success:         false,
			Model:           "gpt-5.3-codex",
			ExitCode:        1,
			FailureCategory: provider.FailureCategoryTransportDisconnect,
			Stderr:          "stream disconnected before completion",
			Output:          "stream disconnected before completion",
		}, nil
	}

	claudeProv := &mockProviderWithRouterTracking{name: "claude"}
	claudeProv.streamRunFn = func(ctx context.Context, p, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		claudeCalls++
		return &provider.Result{
			Success: true,
			Model:   "sonnet",
			Output:  "ok",
		}, nil
	}

	router := provider.NewRouter(
		map[string]provider.Provider{
			"codex":  codexProv,
			"claude": claudeProv,
		},
		map[string]string{"build": "codex"},
		map[string]int{"codex": 100, "claude": 100},
		time.Minute,
		&crossReviewMockStateFile{},
	)

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   router,
		Renderer: &mockRenderer{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}
	if r.methodologyExec == nil {
		t.Fatal("methodologyExec must be wired")
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-invoke-fallback-001", Title: "Invoke fallback test"},
		Tier:      provider.TierLow,
		Model:     "haiku",
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	err = r.methodologyExec.RunAcceptanceTestsWithRetry(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunAcceptanceTestsWithRetry should succeed via fallback, got error: %v", err)
	}
	if codexCalls != 1 {
		t.Fatalf("expected codex to be called once, got %d", codexCalls)
	}
	if claudeCalls != 1 {
		t.Fatalf("expected fallback provider to be called once, got %d", claudeCalls)
	}
	if bc.Model == "haiku" {
		t.Fatalf("expected fallback to update bead context model, got %q", bc.Model)
	}
}

// --- Local ATDD methods deleted: nil methodologyExec errors ---

// Expected failure: Local methods runAcceptanceTestsWithRetry, verifyTestsFailWithRetry
// still exist on Runner in process.go. After deletion, nil methodologyExec with ATDD
// active must return an error mentioning "methodologyExec" (not fall back to local methods).
func TestProcessBead_ATDD_RequiresMethodologyExec(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	var buf strings.Builder

	r := &Runner{
		cfg:      cfg,
		beads:    &mockBeadClient{},
		router:   newMockRouter(),
		renderer: &mockRenderer{},
		output:   &buf,
		syncOut:  newSyncWriter(&buf),
		invoker:  newInvokerForTest(newMockRouter(), &buf, nil),
		// methodologyExec intentionally NOT set (nil)
	}
	r.escalationHandler = newTestEscalationHandler(cfg)

	b := &bead.Bead{
		ID:       "test-nil-meth-001",
		Title:    "Test nil methodology",
		Priority: 1,
		Labels:   []string{"methodology:true"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	// Expected failure: With local methods still present, nil methodologyExec
	// falls back to local methods and succeeds. After deletion, it errors.
	if result.Error == nil {
		t.Error("processBead with ATDD active but nil methodologyExec should return an error " +
			"(local ATDD methods should be deleted)")
	}
	if result.Error != nil && !strings.Contains(result.Error.Error(), "methodologyExec") {
		t.Errorf("error should mention methodologyExec not being wired, got: %v", result.Error)
	}
}

// --- Refactor path requires methodologyExec (no local fallback) ---

// Expected failure: processBead currently falls back to r.runRefactorPhase (local method)
// when methodologyExec is nil during refactor. After deletion, nil methodologyExec errors.
func TestProcessBead_RefactorPhase_RequiresMethodologyExec(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = false
	cfg.Methodology.TDD = true // Use TDD to trigger refactor path without ATDD
	cfg.Refactor.MinFilesChanged = 0
	var buf strings.Builder

	mockProv := &mockProviderWithRouterTracking{
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
	}
	router := provider.NewSingleProviderRouter(mockProv)

	noopCmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "VALIDATION_PASSED", "", 0, nil
	}

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:     &mockBeadClient{},
		Router:    router,
		Renderer:  &mockRenderer{},
		CmdRunner: noopCmdRunner,
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	// Set methodologyExec to nil to simulate it not being wired
	r.methodologyExec = nil

	b := &bead.Bead{
		ID:       "test-refactor-nil-001",
		Title:    "Test refactor nil methodology",
		Priority: 1,
		Labels:   []string{"methodology:true"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	// Expected failure: With local runRefactorPhase still present, nil methodologyExec
	// falls back to local method and succeeds. After deletion, it errors.
	if result.Error == nil {
		t.Error("processBead with TDD active but nil methodologyExec should error during " +
			"refactor phase (local runRefactorPhase should be deleted, no fallback)")
	}
	if result.Error != nil && !strings.Contains(result.Error.Error(), "methodologyExec") {
		t.Errorf("error should mention methodologyExec not being wired, got: %v", result.Error)
	}
}

// --- processBead updates build prompt to ATDD build after acceptance test phases ---

// Expected failure: processBead does not currently set FailureContext on the prompt
// context after ATDD phases succeed, nor does it call RenderATDDBuild to switch the
// build prompt. After implementation, processBead sets bc.PromptCtx.FailureContext
// to indicate acceptance tests are ready and re-renders via RenderATDDBuild.
func TestProcessBead_ATDD_SwitchesToATDDBuildPrompt(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	var buf strings.Builder

	// Create executor where ATDD phases succeed.
	exec := methodology.NewExecutorWithAnalysis(
		cfg,
		&buf,
		func(ctx *prompt.Context) (string, error) {
			return "acceptance test prompt", nil
		},
		func(ctx context.Context, bc *runtypes.BeadContext, p string) error {
			return nil
		},
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		},
		nil, nil,
	)

	// Track whether RenderATDDBuild is called and what FailureContext it receives
	atddBuildCalled := false
	var failureContextReceived string
	renderer := &mockPromptRenderer{
		RenderATDDBuildFn: func(ctx *prompt.Context) (string, error) {
			atddBuildCalled = true
			failureContextReceived = ctx.FailureContext
			return "mock atdd build prompt", nil
		},
	}

	mockProv := &mockProviderWithRouterTracking{
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
	}
	router := provider.NewSingleProviderRouter(mockProv)

	noopCmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "VALIDATION_PASSED", "", 0, nil
	}

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:     &mockBeadClient{},
		Router:    router,
		Renderer:  renderer,
		CmdRunner: noopCmdRunner,
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	r.methodologyExec = exec

	b := &bead.Bead{
		ID:       "test-atdd-build-001",
		Title:    "ATDD build prompt switch",
		Priority: 1,
		Labels:   []string{"methodology:true"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)
	if result.Error != nil {
		t.Fatalf("processBead should succeed, got error: %v", result.Error)
	}

	// Expected failure: processBead does not call RenderATDDBuild after ATDD phases.
	// After implementation, it sets FailureContext and calls RenderATDDBuild.
	if !atddBuildCalled {
		t.Error("after ATDD phases, processBead should call RenderATDDBuild (not RenderBuild) " +
			"for the main build prompt")
	}
	// Verify the FailureContext indicates acceptance tests are ready
	if !strings.Contains(failureContextReceived, "Acceptance tests") {
		t.Errorf("processBead should set FailureContext indicating acceptance tests are ready; "+
			"got %q", failureContextReceived)
	}
}

func TestProcessBead_ATDD_RetriesWhenAcceptanceVerificationFailsAfterRefactor(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	cfg.Escalation.MaxRetriesPerModel = 1
	cfg.Escalation.MaxRetriesPerBead = 3
	var buf strings.Builder

	validateCallCount := 0
	exec := methodology.NewExecutorWithAnalysis(
		cfg,
		&buf,
		func(ctx *prompt.Context) (string, error) {
			return "acceptance test prompt", nil
		},
		func(ctx context.Context, bc *runtypes.BeadContext, p string) error {
			return nil
		},
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			validateCallCount++
			switch validateCallCount {
			case 1:
				// First post-refactor acceptance verification: fail and trigger retry.
				return &claude.Result{Success: true, Output: "FAIL after refactor", ExitCode: 1}, nil
			case 2:
				// Retry post-refactor acceptance verification: pass.
				return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
			default:
				return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
			}
		},
		nil, nil,
	)

	streamRunCalls := 0
	mockProv := &mockProviderWithRouterTracking{}
	mockProv.streamRunFn = func(ctx context.Context, p, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		streamRunCalls++
		return &provider.Result{Success: true, Model: "test-model", Output: "done"}, nil
	}

	analyzeCalls := 0
	mockAnalyzerObj := &mockFailureAnalyzer{
		AnalyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			analyzeCalls++
			return &analyzer.Analysis{
				Category:    analyzer.CategoryLogic,
				Recoverable: true,
				RootCause:   "acceptance regression after refactor",
				Suggestion:  "Fix acceptance failure and retry implementation.",
			}, nil
		},
	}

	router := provider.NewSingleProviderRouter(mockProv)
	noopCmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "VALIDATION_PASSED", "", 0, nil
	}

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:     &mockBeadClient{},
		Router:    router,
		Analyzer:  mockAnalyzerObj,
		Renderer:  &mockPromptRenderer{},
		CmdRunner: noopCmdRunner,
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	r.methodologyExec = exec

	b := &bead.Bead{
		ID:       "test-atdd-retry-after-refactor-001",
		Title:    "ATDD retry after refactor acceptance failure",
		Priority: 1,
		Labels:   []string{"methodology:true"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)
	if result.Error != nil {
		t.Fatalf("processBead should recover and succeed after retry, got error: %v", result.Error)
	}
	if !result.Success {
		t.Fatal("processBead should report success after retry path succeeds")
	}
	if streamRunCalls < 2 {
		t.Errorf("expected at least 2 build invocations (initial + retry), got %d", streamRunCalls)
	}
	if analyzeCalls == 0 {
		t.Error("expected failure analysis to run for post-refactor acceptance verification failure")
	}
	if validateCallCount < 2 {
		t.Errorf("expected acceptance validation sequence to include retry path, got %d validate calls", validateCallCount)
	}
	if result.AcceptanceFailureSummary != "" {
		t.Errorf("expected acceptance failure summary to be cleared on eventual success, got %q", result.AcceptanceFailureSummary)
	}
}
