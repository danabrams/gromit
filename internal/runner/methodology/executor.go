package methodology

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// RenderFn renders an acceptance tests prompt from a prompt context.
type RenderFn func(ctx *prompt.Context) (string, error)

// InvokeFn executes a Claude invocation for acceptance tests.
// The facade wraps the full heartbeat/stall/provider logic into this callback.
type InvokeFn func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error

// ValidateDirectFn runs validation commands directly and returns the result.
type ValidateDirectFn func(ctx context.Context, commands []string, workDir string) (*claude.Result, error)

// Executor handles ATDD workflow phases: writing acceptance tests,
// verifying they fail before implementation, refactoring, and retry wrappers.
type Executor struct {
	cfg              *config.Config
	output           io.Writer
	renderFn         RenderFn
	invokeFn         InvokeFn
	validateFn       ValidateDirectFn
	escalateTierFn   EscalateTierFn
	analyzeFn        AnalyzeFn
	getDiffFn        GetDiffFn
	renderRefactorFn RenderRefactorFn
	refactorInvokeFn RefactorInvokeFn
	gitResetFn       GitResetFn
	getGitHeadFn     GetGitHeadFn
}

// NewExecutor creates an Executor with narrow dependency interfaces.
// renderFn, invokeFn, and validateFn may be nil for test scenarios.
func NewExecutor(cfg *config.Config, output io.Writer, renderFn RenderFn, invokeFn InvokeFn, validateFn ValidateDirectFn) *Executor {
	return &Executor{
		cfg:        cfg,
		output:     output,
		renderFn:   renderFn,
		invokeFn:   invokeFn,
		validateFn: validateFn,
	}
}

// SetAnalyzeFn sets the analysis callback for VerifyTestsFailWithRetry.
func (e *Executor) SetAnalyzeFn(fn AnalyzeFn) {
	e.analyzeFn = fn
}

// SetGetDiffFn sets the git diff callback.
func (e *Executor) SetGetDiffFn(fn GetDiffFn) {
	e.getDiffFn = fn
}

// SetRefactorDeps sets the refactor-phase dependencies.
// Only sets refactor-specific fields; does not overwrite getDiffFn or validateFn
// if they were already set by the constructor or other setters.
func (e *Executor) SetRefactorDeps(deps RefactorDeps) {
	if deps.getDiffFn != nil {
		e.getDiffFn = deps.getDiffFn
	}
	e.renderRefactorFn = deps.renderRefactorFn
	e.refactorInvokeFn = deps.refactorInvokeFn
	if deps.validateFn != nil {
		e.validateFn = deps.validateFn
	}
	e.gitResetFn = deps.gitResetFn
	e.getGitHeadFn = deps.getGitHeadFn
}

// AcceptanceCommands returns a copy of the given commands with "-tags acceptance"
// injected into any "go test" command. Non-go-test commands are preserved as-is.
// If a command already contains "-tags acceptance", it is not modified.
func AcceptanceCommands(commands []string) []string {
	result := make([]string, len(commands))
	for i, cmd := range commands {
		if strings.HasPrefix(cmd, "go test ") && !strings.Contains(cmd, "-tags acceptance") {
			// Insert -tags acceptance right after "go test "
			result[i] = "go test -tags acceptance " + cmd[len("go test "):]
		} else {
			result[i] = cmd
		}
	}
	return result
}

// log writes a formatted message to the output writer.
func (e *Executor) log(format string, args ...interface{}) {
	if e.output != nil {
		fmt.Fprintf(e.output, format+"\n", args...)
	}
}

// RunAcceptanceTests renders the acceptance test prompt and invokes the LLM.
func (e *Executor) RunAcceptanceTests(ctx context.Context, bc *runtypes.BeadContext) error {
	if e.renderFn == nil {
		return fmt.Errorf("render function not configured")
	}

	acceptancePrompt, err := e.renderFn(bc.PromptCtx)
	if err != nil {
		return fmt.Errorf("rendering acceptance tests prompt: %w", err)
	}

	if e.invokeFn == nil {
		return fmt.Errorf("invoke function not configured")
	}

	return e.invokeFn(ctx, bc, acceptancePrompt)
}

// VerifyTestsFail runs validation and returns nil when validation fails (expected)
// or an error when validation passes (unexpected - tests should fail before implementation).
func (e *Executor) VerifyTestsFail(ctx context.Context, bc *runtypes.BeadContext) error {
	if !e.cfg.Validation.Enabled {
		return fmt.Errorf("validation is not enabled - cannot verify tests fail")
	}

	if e.validateFn == nil {
		return fmt.Errorf("validate function not configured")
	}

	e.log("Verifying acceptance tests fail (as expected)...")

	valResult, err := e.validateFn(ctx, AcceptanceCommands(e.cfg.Validation.Commands), bc.PromptCtx.WorkDir)
	if err != nil {
		return fmt.Errorf("validation invocation: %w", err)
	}
	if valResult == nil {
		return fmt.Errorf("validation returned no result")
	}

	// In ATDD, we expect tests to FAIL before implementation
	if claude.IsValidationPassed(valResult) {
		e.log("Unexpected: acceptance tests passed before implementation")
		e.log("Tests should fail until implementation makes them pass")
		return fmt.Errorf("acceptance tests passed before implementation - tests may not be covering new behavior")
	}

	e.log("Acceptance tests failed as expected")
	return nil
}

// VerifyAcceptanceTestsPass runs acceptance-tagged tests and expects them to pass.
// Called after a successful build to confirm the implementation satisfies the acceptance criteria.
func (e *Executor) VerifyAcceptanceTestsPass(ctx context.Context, bc *runtypes.BeadContext) error {
	if !e.cfg.Validation.Enabled {
		return fmt.Errorf("validation is not enabled - cannot verify acceptance tests pass")
	}

	if e.validateFn == nil {
		return fmt.Errorf("validate function not configured")
	}

	e.log("Verifying acceptance tests pass after implementation...")

	valResult, err := e.validateFn(ctx, AcceptanceCommands(e.cfg.Validation.Commands), bc.PromptCtx.WorkDir)
	if err != nil {
		return fmt.Errorf("acceptance validation invocation: %w", err)
	}
	if valResult == nil {
		return fmt.Errorf("acceptance validation returned no result")
	}

	if !claude.IsValidationPassed(valResult) {
		e.log("Acceptance tests failed after implementation")
		return fmt.Errorf("acceptance tests failed after implementation - implementation may not satisfy acceptance criteria")
	}

	e.log("Acceptance tests passed")
	return nil
}
