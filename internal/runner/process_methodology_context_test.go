package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/claude"
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
