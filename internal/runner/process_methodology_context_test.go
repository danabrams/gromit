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
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

func TestRunRefactorAndPostChecks_ValidationUsesParentContext(t *testing.T) {
	commandCalls := 0
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		commandCalls++
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			return "refactor prompt", nil
		},
		func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
			return nil, ctx.Err()
		},
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: "abc123",
		ParentCtx:   parentCtx,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	// Simulate bead context budget exhaustion.
	beadCtx, cancel := context.WithCancel(context.Background())
	cancel()

	retry, terminal := r.runRefactorAndPostChecks(beadCtx, bc, false)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected terminal result to be nil when validation succeeds, got: %+v", terminal)
	}
	if commandCalls == 0 {
		t.Fatal("expected validation commands to run via parent context even when bead context is canceled")
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
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			return "refactor prompt", nil
		},
		func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
			return nil, ctx.Err()
		},
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: "abc123",
		ParentCtx:   parentCtx,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	// Simulate bead context budget exhaustion.
	beadCtx, cancel := context.WithCancel(context.Background())
	cancel()

	retry, terminal := r.runRefactorAndPostChecks(beadCtx, bc, true)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected terminal result to be nil when acceptance verification succeeds, got: %+v", terminal)
	}
	if !acceptanceCommandSeen {
		t.Fatal("expected acceptance validation command to run via parent context even when bead context is canceled")
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
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			return "refactor prompt", nil
		},
		func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
			refactorCtxDeadline, refactorCtxHadDeadline = ctx.Deadline()
			refactorCtxErr = ctx.Err()
			return &claude.Result{Success: true}, nil
		},
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: "abc123",
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

	r.runRefactorAndPostChecks(beadCtx, bc, false)

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
	// Core isolation test: even if refactor times out, validation should
	// receive a fresh context with its own deadline, not a pre-canceled one.
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
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			return "refactor prompt", nil
		},
		func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
			// Simulate refactor timing out.
			return nil, context.DeadlineExceeded
		},
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: "abc123",
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

	retry, terminal := r.runRefactorAndPostChecks(beadCtx, bc, false)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected no terminal result, got: %+v", terminal)
	}
	if validationCommandCalls == 0 {
		t.Fatal("validation commands should have run after refactor timeout")
	}
	if validationCtxErr != nil {
		t.Fatalf("validation received pre-canceled context: %v", validationCtxErr)
	}
	if !validationCtxHadDeadline {
		t.Fatal("validation context should have a deadline from newPhaseContext")
	}
	// Validation deadline should be approximately 180 seconds from now.
	untilDeadline := time.Until(validationCtxDeadline)
	if untilDeadline < 150*time.Second || untilDeadline > 200*time.Second {
		t.Fatalf("validation context deadline unexpected: %v remaining (want ~180s)", untilDeadline.Round(time.Second))
	}
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
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			return "refactor prompt", nil
		},
		func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
			return &claude.Result{Success: true}, nil
		},
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: "abc123",
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

	retry, terminal := r.runRefactorAndPostChecks(beadCtx, bc, true)
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
		StartCommit: "abc123",
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
		StartCommit: "abc123",
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
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			return "refactor prompt", nil
		},
		func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
			return &claude.Result{Success: true}, nil
		},
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: "abc123",
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		RunDeadline: time.Now().Add(35 * time.Second),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false)
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
		StartCommit: "abc123",
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
		StartCommit: "abc123",
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
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			return "refactor prompt", nil
		},
		func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
			return &claude.Result{Success: true}, nil
		},
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-val-timeout", Title: "Validation Timeout"},
		Tier:        provider.TierMedium,
		StartCommit: "abc123",
		ParentCtx:   context.Background(),
		BeadTimeout: 100 * time.Millisecond, // very short so the phase context expires quickly
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal == nil {
		t.Fatal("expected terminal result when validation times out")
	}
	if terminal.TimeoutPhase != "validation" {
		t.Fatalf("TimeoutPhase = %q, want %q", terminal.TimeoutPhase, "validation")
	}
}
