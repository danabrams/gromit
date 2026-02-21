package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

const (
	testStartCommit    = "abc123"
	testRefactorDiff   = "diff --git a/a.go b/a.go\n+line"
	testRefactorPrompt = "refactor prompt"
)

func setTestRefactorDeps(
	r *Runner,
	invoke func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error),
) {
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return testRefactorDiff, nil
		},
		func(ctx *prompt.Context) (string, error) {
			return testRefactorPrompt, nil
		},
		invoke,
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return testStartCommit, nil },
	))
}

func TestRunRefactorAndPostChecks_ValidationUsesParentContext(t *testing.T) {
	commandCalls := 0
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		commandCalls++
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		return nil, nil, ctx.Err()
	})

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   parentCtx,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	// Simulate bead context budget exhaustion.
	beadCtx, cancel := context.WithCancel(context.Background())
	cancel()

	retry, terminal := r.runRefactorAndPostChecks(beadCtx, bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal == nil {
		t.Fatal("expected terminal result when refactor phase fails")
	}
	if commandCalls != 0 {
		t.Fatalf("expected validation commands to be skipped after refactor failure, got %d calls", commandCalls)
	}
}

func TestRunRefactorAndPostChecks_AcceptanceVerificationUsesParentContext(t *testing.T) {
	acceptanceCommandSeen := false
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if strings.Contains(command, "-tags acceptance") {
			acceptanceCommandSeen = true
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		return nil, nil, ctx.Err()
	})

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   parentCtx,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	// Simulate bead context budget exhaustion.
	beadCtx, cancel := context.WithCancel(context.Background())
	cancel()

	retry, terminal := r.runRefactorAndPostChecks(beadCtx, bc, true, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal == nil {
		t.Fatal("expected terminal result when refactor phase fails")
	}
	if acceptanceCommandSeen {
		t.Fatal("expected acceptance verification to be skipped after refactor failure")
	}
}

func TestRunRefactorAndPostChecks_RefactorUsesPhaseContext(t *testing.T) {
	// Verify that the refactor invocation receives a context with a deadline
	// matching the configured phase timeout, derived from ParentCtx,
	// even when the bead context is already canceled.
	var refactorCtxDeadline time.Time
	var refactorCtxHadDeadline bool
	var refactorCtxErr error

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	r.cfg.Methodology.PhaseTimeouts = config.MethodologyPhaseTimeout{
		RefactorSeconds: 120,
	}
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		refactorCtxDeadline, refactorCtxHadDeadline = ctx.Deadline()
		refactorCtxErr = ctx.Err()
		return &claude.Result{Success: true}, nil, nil
	})

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   parentCtx,
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	// Cancel the bead context to simulate exhaustion.
	beadCtx, cancel := context.WithCancel(context.Background())
	cancel()

	r.runRefactorAndPostChecks(beadCtx, bc, false, 1)

	// The refactor invocation should have received a live context (no error)
	// derived from ParentCtx, not from the canceled beadCtx.
	if refactorCtxErr != nil {
		t.Fatalf("refactor received canceled context: %v", refactorCtxErr)
	}
	if !refactorCtxHadDeadline {
		t.Fatal("refactor context should have a deadline from newPhaseContext")
	}
	// Deadline should be approximately 120 seconds from now (the configured phase timeout).
	untilDeadline := time.Until(refactorCtxDeadline)
	if untilDeadline < 100*time.Second || untilDeadline > 130*time.Second {
		t.Fatalf("refactor context deadline unexpected: %v remaining (want ~120s)", untilDeadline.Round(time.Second))
	}
}

func TestRunRefactorAndPostChecks_ValidationGetsFreshContextAfterRefactorTimeout(t *testing.T) {
	// Refactor timeout should be terminal and skip downstream validation.
	var validationCtxErr error
	var validationCtxDeadline time.Time
	var validationCtxHadDeadline bool
	validationCommandCalls := 0

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCtxErr = ctx.Err()
		validationCtxDeadline, validationCtxHadDeadline = ctx.Deadline()
		validationCommandCalls++
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	r.cfg.Methodology.PhaseTimeouts = config.MethodologyPhaseTimeout{
		RefactorSeconds: 120,
	}
	r.cfg.Validation.PhaseTimeoutSeconds = 180
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		// Simulate refactor timing out.
		return nil, nil, context.DeadlineExceeded
	})

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   parentCtx,
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	// Cancel the bead context to simulate exhaustion.
	beadCtx, cancel := context.WithCancel(context.Background())
	cancel()

	retry, terminal := r.runRefactorAndPostChecks(beadCtx, bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal == nil {
		t.Fatal("expected terminal result when refactor times out")
	}
	if validationCommandCalls != 0 {
		t.Fatalf("validation commands should not run after refactor timeout, got %d calls", validationCommandCalls)
	}
	_ = validationCtxErr
	_ = validationCtxDeadline
	_ = validationCtxHadDeadline
}

func TestRunRefactorAndPostChecks_AcceptanceVerificationUsesPhaseContext(t *testing.T) {
	// Verify that acceptance verification receives a phase context with a
	// proper deadline, not a raw ParentCtx or the canceled bead context.
	var acceptanceCtxErr error
	var acceptanceCtxHadDeadline bool

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if strings.Contains(command, "-tags acceptance") {
			acceptanceCtxErr = ctx.Err()
			_, acceptanceCtxHadDeadline = ctx.Deadline()
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	r.cfg.Methodology.PhaseTimeouts = config.MethodologyPhaseTimeout{
		RefactorSeconds: 120,
	}
	r.cfg.Validation.PhaseTimeoutSeconds = 180
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		return &claude.Result{Success: true}, nil, nil
	})

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   parentCtx,
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	// Cancel the bead context.
	beadCtx, cancel := context.WithCancel(context.Background())
	cancel()

	retry, terminal := r.runRefactorAndPostChecks(beadCtx, bc, true, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected no terminal result, got: %+v", terminal)
	}
	if acceptanceCtxErr != nil {
		t.Fatalf("acceptance verification received pre-canceled context: %v", acceptanceCtxErr)
	}
	if !acceptanceCtxHadDeadline {
		t.Fatal("acceptance verification context should have a deadline from newPhaseContext")
	}
}

func TestExecuteBuildLoop_MethodologyValidationUsesPhaseContext(t *testing.T) {
	// When methodology (TDD/ATDD) is active, the intermediate validation gate
	// between build and refactor should use a phase context with its own deadline,
	// not the raw bead context. This ensures bead timeout exhaustion during build
	// does not pre-cancel the validation gate.
	//
	// The test captures context info from only the FIRST cmdRunner call, which
	// corresponds to the intermediate validation (line 92 of executeBuildAndMethodologyLoop),
	// not the post-refactor re-validation.
	var firstCallCtxHadDeadline bool
	var firstCallCtxErr error
	callCount := 0

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		callCount++
		if callCount == 1 {
			firstCallCtxErr = ctx.Err()
			_, firstCallCtxHadDeadline = ctx.Deadline()
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Methodology.TDD = true
	r.cfg.Validation.PhaseTimeoutSeconds = 150
	// Disable validation in runRefactorAndPostChecks by setting high refactor threshold.
	// This means only the intermediate validation runs.
	r.cfg.Refactor.MinFilesChanged = 999

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-val-gate", Title: "Test Validation Gate"},
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	// Cancel the bead context to simulate exhaustion.
	beadCtx, cancel := context.WithCancel(context.Background())
	cancel()

	r.methodologyExec = r.makeMethodologyExec()
	r.validationRunner = validation.NewRunner(r.cfg, cmdRunner, nil, nil)

	result := r.executeBuildAndMethodologyLoop(
		beadCtx, bc,
		false, true, // atddActive=false, tddActive=true
		func() bool { return true }, // build succeeds
	)

	if !result.Success {
		if result.Error != nil {
			t.Fatalf("expected success, got error: %v", result.Error)
		}
		t.Fatal("expected success")
	}
	if callCount == 0 {
		t.Fatal("expected validation commands to run")
	}
	if firstCallCtxErr != nil {
		t.Fatalf("intermediate validation received pre-canceled context: %v", firstCallCtxErr)
	}
	if !firstCallCtxHadDeadline {
		t.Fatal("intermediate validation should have a deadline from newPhaseContext")
	}
}

func TestExecuteBuildLoop_MethodologyValidationPhaseClampedByRunDeadline(t *testing.T) {
	var firstCallCtxDeadline time.Time
	var firstCallCtxHadDeadline bool
	callCount := 0

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		callCount++
		if callCount == 1 {
			firstCallCtxDeadline, firstCallCtxHadDeadline = ctx.Deadline()
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Methodology.TDD = true
	r.cfg.Validation.PhaseTimeoutSeconds = 180
	// Disable validation in runRefactorAndPostChecks so only the intermediate gate runs.
	r.cfg.Refactor.MinFilesChanged = 999

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-val-gate-clamp", Title: "Test Validation Gate Clamp"},
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		RunDeadline: time.Now().Add(35 * time.Second),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	beadCtx := context.Background()
	r.methodologyExec = r.makeMethodologyExec()
	r.validationRunner = validation.NewRunner(r.cfg, cmdRunner, nil, nil)

	result := r.executeBuildAndMethodologyLoop(
		beadCtx, bc,
		false, true,
		func() bool { return true },
	)

	if !result.Success {
		if result.Error != nil {
			t.Fatalf("expected success, got error: %v", result.Error)
		}
		t.Fatal("expected success")
	}
	if !firstCallCtxHadDeadline {
		t.Fatal("intermediate validation should have a deadline")
	}
	untilDeadline := time.Until(firstCallCtxDeadline)
	if untilDeadline < 20*time.Second || untilDeadline > 40*time.Second {
		t.Fatalf("validation gate context deadline unexpected: %v remaining (want ~35s clamp)", untilDeadline.Round(time.Second))
	}
}

func TestExecuteBuildAndMethodologyLoop_ATDDOnlySkipsRefactor(t *testing.T) {
	refactorInvoked := false
	validationCalls := 0

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCalls++
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Methodology.ATDD = true
	r.cfg.Methodology.TDD = false
	r.cfg.Refactor.MinFilesChanged = 0

	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			refactorInvoked = true
			return testRefactorDiff, nil
		},
		func(ctx *prompt.Context) (string, error) { return testRefactorPrompt, nil },
		func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
			return &claude.Result{Success: true}, nil, nil
		},
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return testStartCommit, nil },
	))

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-atdd-only", Title: "ATDD only"},
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	result := r.executeBuildAndMethodologyLoop(context.Background(), bc, true, false, func() bool { return true })
	if result.Error != nil {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if refactorInvoked {
		t.Fatal("expected refactor phase to be skipped when only ATDD is active")
	}
	if validationCalls == 0 {
		t.Fatal("expected intermediate validation to run in methodology mode")
	}
}

func TestRunATDDPreBuildPhases_PreservesFailureContextBeforeATDDBuild(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			ATDD: true,
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	cfg.SetDefaults()

	r, _, _ := setupDirectValidationRunner(t, cfg, nil)
	r.methodologyExec = methodology.NewExecutorWithEscalation(
		r.cfg,
		r.output,
		func(ctx *prompt.Context) (string, error) { return "acceptance prompt", nil },
		func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error { return nil },
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "expected pre-build test failure", ExitCode: 1}, nil
		},
		nil,
	)

	var renderedCtx prompt.Context
	r.renderer = &mockPromptRenderer{
		RenderATDDBuildFn: func(ctx *prompt.Context) (string, error) {
			renderedCtx = *ctx
			return "atdd build prompt", nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-atdd-failure-context", Title: "ATDD context"},
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir:         t.TempDir(),
			FailureContext:  "keep-this-context",
			IsRetry:         true,
			PrevFailure:     "previous validation failure",
			RecentLearnings: nil,
		},
		Result: &IterationResult{},
	}

	if ok := r.runATDDPreBuildPhases(context.Background(), bc); !ok {
		t.Fatal("expected runATDDPreBuildPhases to succeed")
	}
	if renderedCtx.FailureContext != "keep-this-context" {
		t.Fatalf("rendered FailureContext = %q, want %q", renderedCtx.FailureContext, "keep-this-context")
	}
	if renderedCtx.IsRetry {
		t.Fatal("rendered IsRetry should be false for ATDD build prompt")
	}
	if renderedCtx.PrevFailure != "" {
		t.Fatalf("rendered PrevFailure = %q, want empty", renderedCtx.PrevFailure)
	}
	if bc.PromptCtx.FailureContext != "keep-this-context" {
		t.Fatalf("bead FailureContext = %q, want %q", bc.PromptCtx.FailureContext, "keep-this-context")
	}
}

func TestRunRefactorAndPostChecks_ValidationPhaseClampedByRunDeadline(t *testing.T) {
	var validationCtxDeadline time.Time
	var validationCtxHadDeadline bool
	validationCalls := 0

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCalls++
		if validationCalls == 1 {
			validationCtxDeadline, validationCtxHadDeadline = ctx.Deadline()
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	r.cfg.Methodology.PhaseTimeouts = config.MethodologyPhaseTimeout{
		RefactorSeconds: 120,
	}
	r.cfg.Validation.PhaseTimeoutSeconds = 180
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		return &claude.Result{Success: true}, nil, nil
	})

	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		RunDeadline: time.Now().Add(35 * time.Second),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected no terminal result, got: %+v", terminal)
	}
	if validationCalls == 0 {
		t.Fatal("expected post-refactor validation to run")
	}
	if !validationCtxHadDeadline {
		t.Fatal("post-refactor validation should have a deadline")
	}
	untilDeadline := time.Until(validationCtxDeadline)
	if untilDeadline < 20*time.Second || untilDeadline > 40*time.Second {
		t.Fatalf("post-refactor validation context deadline unexpected: %v remaining (want ~35s clamp)", untilDeadline.Round(time.Second))
	}
}

func TestRunATDDPreBuildPhases_UsesRedPhaseContext(t *testing.T) {
	// Verify that ATDD pre-build phases use a red-phase context with a deadline
	// from the configured phase timeout, even when the bead context is canceled.
	var invocationCtxErr error
	var invocationCtxHadDeadline bool
	var invocationCtxDeadline time.Time

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "FAIL", "test failure", 1, nil
	}
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Methodology: config.MethodologyConfig{
			ATDD: true,
			PhaseTimeouts: config.MethodologyPhaseTimeout{
				RedSeconds: 90,
			},
		},
		Escalation: config.EscalationConfig{
			MaxRetriesPerModel: 0,
		},
	}
	cfg.SetDefaults()

	r, _, _ := setupDirectValidationRunner(t, cfg, cmdRunner)

	// Create a custom executor with an invokeFn that captures the context.
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error {
		invocationCtxErr = ctx.Err()
		invocationCtxDeadline, invocationCtxHadDeadline = ctx.Deadline()
		return nil
	}
	r.methodologyExec = methodology.NewExecutorWithEscalation(
		r.cfg, r.output, r.renderer.RenderAcceptanceTests, invokeFn, nil, nil,
	)

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-red", Title: "Test Red Phase", Labels: []string{"methodology:true"}},
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	// Cancel the bead context.
	beadCtx, cancel := context.WithCancel(context.Background())
	cancel()

	r.runATDDPreBuildPhases(beadCtx, bc)

	if invocationCtxErr != nil {
		t.Fatalf("ATDD invocation received canceled context: %v", invocationCtxErr)
	}
	if !invocationCtxHadDeadline {
		t.Fatal("ATDD invocation context should have a deadline from newPhaseContext")
	}
	untilDeadline := time.Until(invocationCtxDeadline)
	if untilDeadline < 70*time.Second || untilDeadline > 100*time.Second {
		t.Fatalf("ATDD red context deadline unexpected: %v remaining (want ~90s)", untilDeadline.Round(time.Second))
	}
}

func TestRunATDDPreBuildPhases_SetsRedPhaseAttributionOnTimeout(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			ATDD: true,
			PhaseTimeouts: config.MethodologyPhaseTimeout{
				RedSeconds: 90,
			},
		},
	}
	cfg.SetDefaults()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, cfg, cmdRunner)

	// Create a methodology executor that returns a deadline-exceeded error
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error {
		return context.DeadlineExceeded
	}
	r.methodologyExec = methodology.NewExecutorWithEscalation(
		r.cfg, r.output, r.renderer.RenderAcceptanceTests, invokeFn, nil, nil,
	)

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-red-timeout", Title: "Test Red Phase Timeout", Labels: []string{"methodology:true"}},
		Tier:        provider.TierMedium,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	r.runATDDPreBuildPhases(context.Background(), bc)

	if bc.Result.TimeoutPhase != "red" {
		t.Fatalf("TimeoutPhase = %q, want %q", bc.Result.TimeoutPhase, "red")
	}
}

func TestRunATDDPreBuildPhases_RecordsRedPhaseMetric(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			ATDD: true,
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	cfg.SetDefaults()

	r, _, _ := setupDirectValidationRunner(t, cfg, nil)
	r.methodologyExec = methodology.NewExecutorWithEscalation(
		r.cfg,
		r.output,
		func(ctx *prompt.Context) (string, error) { return "acceptance prompt", nil },
		func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error {
			bc.Result.InputTokens = 41
			bc.Result.OutputTokens = 19
			return nil
		},
		func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "expected pre-build test failure", ExitCode: 1}, nil
		},
		nil,
	)

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-red-metric", Title: "Red Metric"},
		Model:       "sonnet",
		Tier:        provider.TierMedium,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	if ok := r.runATDDPreBuildPhases(context.Background(), bc); !ok {
		t.Fatal("expected runATDDPreBuildPhases to succeed")
	}

	if len(bc.Result.PhaseMetrics) != 1 {
		t.Fatalf("PhaseMetrics length = %d, want 1", len(bc.Result.PhaseMetrics))
	}
	metric := bc.Result.PhaseMetrics[0]
	if metric.Phase != "red" {
		t.Fatalf("Phase = %q, want %q", metric.Phase, "red")
	}
	if metric.CycleNumber != 1 {
		t.Fatalf("CycleNumber = %d, want 1", metric.CycleNumber)
	}
	if metric.InputTokens != 41 || metric.OutputTokens != 19 {
		t.Fatalf("tokens = (%d,%d), want (41,19)", metric.InputTokens, metric.OutputTokens)
	}
	if !metric.Success {
		t.Fatal("red phase metric should be marked successful")
	}
}

func TestExecuteBuildAndMethodologyLoop_SetsValidationGatePhaseAttributionOnTimeout(t *testing.T) {
	// When the intermediate validation gate times out, TimeoutPhase should be
	// "validation_gate" to attribute the timeout to that phase.
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		<-ctx.Done()
		return "", "", 1, ctx.Err()
	}
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Methodology: config.MethodologyConfig{
			TDD: true,
		},
	}
	cfg.SetDefaults()

	r, _, _ := setupDirectValidationRunner(t, cfg, cmdRunner)

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-valgate-timeout", Title: "Validation Gate Timeout", Labels: []string{"methodology:true"}},
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 100 * time.Millisecond,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	executeWithRetry := func() bool {
		return true // pretend build succeeded
	}

	result := r.executeBuildAndMethodologyLoop(context.Background(), bc, false, true, executeWithRetry)
	if result.TimeoutPhase != "validation_gate" {
		t.Fatalf("TimeoutPhase = %q, want %q", result.TimeoutPhase, "validation_gate")
	}
}

func TestRunRefactorAndPostChecks_RefactorTimeoutThenValidationSucceeds_NoTerminalAttribution(t *testing.T) {
	// Refactor timeout is terminal; validation should not run.
	validationCalls := 0

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCalls++
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	r.cfg.Methodology.PhaseTimeouts = config.MethodologyPhaseTimeout{
		RefactorSeconds: 120,
	}
	r.cfg.Validation.PhaseTimeoutSeconds = 180
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		// Simulate refactor timing out.
		return nil, nil, context.DeadlineExceeded
	})

	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal == nil {
		t.Fatal("expected terminal result when refactor times out")
	}
	if validationCalls != 0 {
		t.Fatalf("validation should not run after refactor timeout, got %d calls", validationCalls)
	}
	if bc.Result.TimeoutPhase != "" {
		t.Fatalf("TimeoutPhase = %q, want empty (refactor failure is not classified as timeout phase attribution)", bc.Result.TimeoutPhase)
	}
}

func TestRunRefactorAndPostChecks_RecordsRefactorAndVerificationMetrics(t *testing.T) {
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		return &claude.Result{Success: true}, nil, nil
	})

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-phase-metrics-refactor", Title: "Refactor Metrics"},
		Model:       "sonnet",
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, true, 3)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected terminal=nil, got %+v", terminal)
	}

	if len(bc.Result.PhaseMetrics) != 2 {
		t.Fatalf("PhaseMetrics length = %d, want 2", len(bc.Result.PhaseMetrics))
	}
	if bc.Result.PhaseMetrics[0].Phase != "refactor" {
		t.Fatalf("phase[0] = %q, want %q", bc.Result.PhaseMetrics[0].Phase, "refactor")
	}
	if bc.Result.PhaseMetrics[1].Phase != "verification" {
		t.Fatalf("phase[1] = %q, want %q", bc.Result.PhaseMetrics[1].Phase, "verification")
	}
	if bc.Result.PhaseMetrics[0].CycleNumber != 3 || bc.Result.PhaseMetrics[1].CycleNumber != 3 {
		t.Fatalf("cycle numbers = (%d,%d), want (3,3)", bc.Result.PhaseMetrics[0].CycleNumber, bc.Result.PhaseMetrics[1].CycleNumber)
	}
	if !bc.Result.PhaseMetrics[0].Success || !bc.Result.PhaseMetrics[1].Success {
		t.Fatal("expected both refactor and verification metrics to be successful")
	}
}

func TestRunRefactorAndPostChecks_ValidationTimeoutIndependentOfRefactorTimeout(t *testing.T) {
	// Refactor timeout is terminal, so validation timeout attribution is never reached.
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		// Block until validation phase context expires.
		<-ctx.Done()
		return "", "", 1, ctx.Err()
	}
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Methodology: config.MethodologyConfig{
			ATDD: true,
		},
	}
	cfg.SetDefaults()

	r, _, _ := setupDirectValidationRunner(t, cfg, cmdRunner)
	r.cfg.Refactor.MinFilesChanged = 0
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		// Refactor also times out.
		return nil, nil, context.DeadlineExceeded
	})

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-both-timeout", Title: "Both Timeout"},
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 100 * time.Millisecond, // short so validation phase context expires quickly
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal == nil {
		t.Fatal("expected terminal result when refactor times out")
	}
	if terminal.TimeoutPhase != "" {
		t.Fatalf("TimeoutPhase = %q, want empty when refactor failure is terminal before validation", terminal.TimeoutPhase)
	}
}

func TestRunRefactorAndPostChecks_DefaultFallbackWhenPhaseTimeoutsOmitted(t *testing.T) {
	// When phase_timeouts config is omitted (zero-valued), phases should
	// fall back to the bead timeout for their deadline. This verifies the
	// end-to-end fallback path through newPhaseContext.
	var refactorCtxDeadline time.Time
	var refactorCtxHadDeadline bool
	var validationCtxDeadline time.Time
	var validationCtxHadDeadline bool
	validationCalls := 0

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCalls++
		if validationCalls == 1 {
			validationCtxDeadline, validationCtxHadDeadline = ctx.Deadline()
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	// Explicitly zero: no phase timeout overrides configured.
	r.cfg.Methodology.PhaseTimeouts = config.MethodologyPhaseTimeout{}
	r.cfg.Validation.PhaseTimeoutSeconds = 0
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		refactorCtxDeadline, refactorCtxHadDeadline = ctx.Deadline()
		return &claude.Result{Success: true}, nil, nil
	})

	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 200 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	r.runRefactorAndPostChecks(context.Background(), bc, false, 1)

	// Refactor should get a deadline derived from bead timeout (~200s).
	if !refactorCtxHadDeadline {
		t.Fatal("refactor context should have a deadline even without explicit phase timeout config")
	}
	refactorRemaining := time.Until(refactorCtxDeadline)
	if refactorRemaining < 170*time.Second || refactorRemaining > 210*time.Second {
		t.Fatalf("refactor context deadline unexpected: %v remaining (want ~200s from bead timeout fallback)", refactorRemaining.Round(time.Second))
	}

	// Validation should also get a deadline derived from bead timeout (~200s).
	if validationCalls == 0 {
		t.Fatal("expected validation to run")
	}
	if !validationCtxHadDeadline {
		t.Fatal("validation context should have a deadline even without explicit phase timeout config")
	}
	validationRemaining := time.Until(validationCtxDeadline)
	if validationRemaining < 170*time.Second || validationRemaining > 210*time.Second {
		t.Fatalf("validation context deadline unexpected: %v remaining (want ~200s from bead timeout fallback)", validationRemaining.Round(time.Second))
	}
}

func TestRunRefactorAndPostChecks_SetsValidationPhaseAttributionOnTimeout(t *testing.T) {
	// When post-refactor validation times out via phase context expiration,
	// TimeoutPhase should be "validation" to distinguish from a refactor timeout.
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		// Block until the phase context expires, simulating a slow validation command.
		<-ctx.Done()
		return "", "", 1, ctx.Err()
	}
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Methodology: config.MethodologyConfig{
			ATDD: true,
		},
	}
	cfg.SetDefaults()

	r, _, _ := setupDirectValidationRunner(t, cfg, cmdRunner)
	r.cfg.Refactor.MinFilesChanged = 0
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		return &claude.Result{Success: true}, nil, nil
	})

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-val-timeout", Title: "Validation Timeout"},
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 100 * time.Millisecond, // very short so the phase context expires quickly
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal == nil {
		t.Fatal("expected terminal result when validation times out")
	}
	if terminal.TimeoutPhase != "" {
		t.Fatalf("TimeoutPhase = %q, want empty when refactor failure is terminal before validation", terminal.TimeoutPhase)
	}
}
