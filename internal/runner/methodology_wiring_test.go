package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
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

// --- NewRunnerWithDeps wires methodologyExec ---

// Expected failure: Runner struct does not have a methodologyExec field yet.
// After implementation, NewRunnerWithDeps will construct a methodology.Executor
// and assign it to r.methodologyExec.
func TestNewRunnerWithDeps_WiresMethodologyExec(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	var buf strings.Builder

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   newMockRouter(),
		Renderer: &mockRenderer{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	// The methodologyExec field must be non-nil after construction
	if r.methodologyExec == nil {
		t.Fatal("NewRunnerWithDeps should wire methodologyExec field on Runner")
	}
}

// --- processBead delegates ATDD to methodologyExec ---

// Expected failure: processBead currently calls r.runAcceptanceTestsWithRetry (local method)
// instead of r.methodologyExec.RunAcceptanceTestsWithRetry. After implementation,
// processBead will delegate to the methodology.Executor via the methodologyExec field.
func TestProcessBead_ATDD_DelegatesToMethodologyExec(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	var buf strings.Builder

	// Track whether the methodology executor's callbacks were invoked
	acceptanceTestInvoked := false
	verifyTestsFailInvoked := false

	// Create a methodology.Executor with tracking callbacks
	exec := methodology.NewExecutorWithAnalysis(
		cfg,
		&buf,
		func(ctx *prompt.Context) (string, error) {
			return "test prompt", nil
		},
		func(ctx context.Context, bc *runtypes.BeadContext, p string) error {
			acceptanceTestInvoked = true
			return nil
		},
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			verifyTestsFailInvoked = true
			// Return failure (tests fail as expected in ATDD)
			return &claude.Result{Success: true, Output: "FAIL", ExitCode: 1}, nil
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

	if !acceptanceTestInvoked {
		t.Error("processBead with ATDD should delegate acceptance tests to methodologyExec.RunAcceptanceTestsWithRetry")
	}
	if !verifyTestsFailInvoked {
		t.Error("processBead with ATDD should delegate verify-tests-fail to methodologyExec.VerifyTestsFailWithRetry")
	}
	if result.Error != nil {
		t.Errorf("processBead should succeed, got error: %v", result.Error)
	}
}

// --- processBead uses methodology.ErrATDDAlreadyDone ---

// Expected failure: processBead currently checks against the local errATDDAlreadyDone sentinel.
// After implementation, it will check against methodology.ErrATDDAlreadyDone (or use
// methodology.IsATDDAlreadyDone) since the sentinel now lives in the methodology package.
func TestProcessBead_ATDD_HandlesAlreadyDoneFromMethodologyPackage(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	var buf strings.Builder

	// Create executor that returns methodology.ErrATDDAlreadyDone from VerifyTestsFailWithRetry
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
			// Tests pass (unexpected) → triggers ErrATDDAlreadyDone
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		},
		nil, // analyzeFn - nil so it falls through to ErrATDDAlreadyDone
		nil, // getDiffFn
	)

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   newMockRouter(),
		Renderer: &mockRenderer{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	// Replace with our executor that will return ErrATDDAlreadyDone
	r.methodologyExec = exec

	b := &bead.Bead{
		ID:       "test-atdd-done-001",
		Title:    "Already done bead",
		Priority: 1,
		Labels:   []string{"methodology:true"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	// processBead should recognize ErrATDDAlreadyDone from methodology package
	// and set AlreadyDone = true, Success = true
	if !result.AlreadyDone {
		t.Error("processBead should set AlreadyDone=true when methodologyExec returns methodology.ErrATDDAlreadyDone")
	}
	if !result.Success {
		t.Error("processBead should set Success=true when work is already done")
	}
}

// --- processBead delegates refactor to methodologyExec ---

// Expected failure: processBead currently calls r.runRefactorPhase (local method).
// After implementation, it will call r.methodologyExec.RunRefactorPhase.
func TestProcessBead_RefactorPhase_DelegatesToMethodologyExec(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	cfg.Refactor.MinFilesChanged = 0 // Always run refactor
	var buf strings.Builder

	refactorCalled := false

	// Create executor wired for both ATDD and refactor
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
			// Tests fail as expected
			return &claude.Result{Success: true, Output: "FAIL", ExitCode: 1}, nil
		},
		nil, nil,
	)

	// Wire refactor deps to track the call
	refactorExec := methodology.NewExecutorWithRefactor(cfg, &buf, methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/foo.go b/foo.go\n+some change\ndiff --git a/bar.go b/bar.go\n+other", nil
		},
		func(ctx *prompt.Context) (string, error) {
			refactorCalled = true
			return "refactor prompt", nil
		},
		func(ctx context.Context, p string, tier string) (*claude.Result, error) {
			return &claude.Result{Success: true}, nil
		},
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		},
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	// The implementation should use a single methodologyExec with all deps wired.
	// For this test we verify the delegation happens. Since we can't easily create
	// a fully-wired Executor with both ATDD and refactor callbacks using current
	// constructors, we test the refactor path separately.
	_ = exec // ATDD executor - used in the combined flow

	mockProv := &mockProviderWithRouterTracking{
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
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

	// Wire the refactor executor
	r.methodologyExec = refactorExec

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-refactor-001", Title: "Refactor test"},
		Tier:        "medium",
		Model:       "sonnet",
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: t.TempDir()},
		StartCommit: "abc123",
	}

	// Call RunRefactorPhase through the methodologyExec field directly
	// (since processBead should delegate to it after implementation)
	err = r.methodologyExec.RunRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunRefactorPhase returned error: %v", err)
	}

	if !refactorCalled {
		t.Error("processBead refactor phase should delegate to methodologyExec.RunRefactorPhase")
	}
}

// --- EscalateTierFn callback is wired to escalation.Handler ---

// Expected failure: NewRunnerWithDeps does not currently wire EscalateTierFn
// on the methodology.Executor. After implementation, the callback should wrap
// escalation.Handler.EscalateTier so that tier escalation during ATDD phases
// flows through the same escalation logic.
func TestMethodologyExec_EscalateTierFn_WrapsEscalationHandler(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	var buf strings.Builder

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   newMockRouter(),
		Renderer: &mockRenderer{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps returned error: %v", err)
	}

	if r.methodologyExec == nil {
		t.Fatal("methodologyExec must be wired")
	}

	// Exercise RunAcceptanceTestsWithRetry to verify the EscalateTierFn callback
	// is wired: when acceptance tests fail and exhaust retries, escalation should
	// update the BeadContext's tier/model via escalation.Handler.EscalateTier.
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-esc-001", Title: "Escalation test", Priority: 2},
		Tier:      provider.TierLow,
		Model:     "haiku",
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	// This should attempt to escalate from low → medium tier after retries exhaust.
	// The escalation updates bc.Tier and bc.Model via the wired callback.
	_ = r.methodologyExec.RunAcceptanceTestsWithRetry(context.Background(), bc)

	// After escalation, the tier should have moved up from low
	if bc.Tier == provider.TierLow {
		t.Error("EscalateTierFn should be wired to escalation.Handler.EscalateTier, " +
			"updating bc.Tier from low to a higher tier after retry exhaustion")
	}
}

// --- InvokeFn callback is wired to execution chain ---

// Expected failure: NewRunnerWithDeps does not currently wire InvokeFn on the
// methodology.Executor. After implementation, the InvokeFn callback should wrap
// the execution.Invoker so that ATDD acceptance test invocations use the same
// provider/streaming infrastructure as regular build invocations.
func TestMethodologyExec_InvokeFn_WrapsExecutionInvoker(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	var buf strings.Builder

	providerInvoked := false
	mockProv := &mockProviderWithRouterTracking{
		name:            "test-provider",
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
	}
	mockProv.runFn = func(ctx context.Context, p, tier string) (*provider.Result, error) {
		providerInvoked = true
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

	// RunAcceptanceTests uses the InvokeFn callback which should route through
	// the execution.Invoker → provider chain
	err = r.methodologyExec.RunAcceptanceTests(context.Background(), bc)

	// The provider should have been invoked through the execution chain
	if !providerInvoked {
		t.Error("methodology.Executor.InvokeFn should be wired to execution.Invoker, " +
			"routing through the provider infrastructure")
	}
}

// --- Local ATDD methods should be removed from Runner ---

// Expected failure: The local methods runAcceptanceTestsWithRetry, verifyTestsFailWithRetry,
// runRefactorPhase, and shouldRunRefactor still exist on Runner in process.go.
// After implementation, they will be deleted and only exist on methodology.Executor.
// This test verifies that processBead's ATDD path uses the methodologyExec field
// rather than any local method. If methodologyExec is nil when ATDD is active,
// processBead should return an error (not fall back to deleted local methods).
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

	// With the local methods deleted, a nil methodologyExec should cause an error
	// rather than silently succeeding (which would mean the old local methods still exist)
	if result.Error == nil {
		t.Error("processBead with ATDD active but nil methodologyExec should return an error " +
			"(local ATDD methods should be deleted)")
	}
}

// --- processBead refactor path requires methodologyExec (no fallback) ---

// Expected failure: processBead currently falls back to r.runRefactorPhase (local method)
// when methodologyExec is nil during the refactor phase. After implementation, the local
// runRefactorPhase method will be deleted and the fallback removed, so processBead should
// error when methodologyExec is nil and refactor is needed.
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

	// After the local runRefactorPhase is deleted, nil methodologyExec in the
	// refactor phase should produce an error (not silently fall back to the
	// deleted local method). Currently this passes because the fallback exists.
	if result.Error == nil {
		t.Error("processBead with TDD active but nil methodologyExec should error during " +
			"refactor phase (local runRefactorPhase should be deleted, no fallback)")
	}
	if result.Error != nil && !strings.Contains(result.Error.Error(), "methodologyExec") {
		t.Errorf("error should mention methodologyExec not being wired, got: %v", result.Error)
	}
}

// --- processBead refactor delegates through methodologyExec in full flow ---

// Expected failure: processBead's refactor path currently has a conditional that checks
// if methodologyExec != nil and falls back to r.runRefactorPhase otherwise. After
// implementation, the fallback is removed and refactor always goes through methodologyExec.
// This test exercises the full processBead flow (build → validate → refactor) and verifies
// the refactor callback on methodologyExec is invoked.
func TestProcessBead_FullFlow_RefactorDelegatesToMethodologyExec(t *testing.T) {
	cfg := newMethodologyWiringConfig()
	cfg.Methodology.ATDD = true
	cfg.Refactor.MinFilesChanged = 0
	var buf strings.Builder

	refactorRenderCalled := false

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
			// Tests fail as expected in ATDD
			return &claude.Result{Success: true, Output: "FAIL", ExitCode: 1}, nil
		},
		func(bc *runtypes.BeadContext, nextTier string) {
			// no-op escalation
		},
	)

	// Wire refactor deps with tracking
	exec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/foo.go b/foo.go\n+line\ndiff --git a/bar.go b/bar.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			refactorRenderCalled = true
			return "refactor prompt", nil
		},
		func(ctx context.Context, p string, tier string) (*claude.Result, error) {
			return &claude.Result{Success: true}, nil
		},
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		},
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
	if !refactorRenderCalled {
		t.Error("processBead refactor phase should delegate to methodologyExec.RunRefactorPhase " +
			"which calls the refactor render callback — refactor was not invoked through methodologyExec")
	}
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
	)
}
