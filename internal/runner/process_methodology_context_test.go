package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
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
