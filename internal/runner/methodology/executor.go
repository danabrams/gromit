package methodology

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// RenderFn renders an acceptance tests prompt from a prompt context.
type RenderFn func(ctx *prompt.Context) (string, error)

// InvokeFn executes a Claude invocation for acceptance tests.
// The facade wraps the full heartbeat/stall/provider logic into this callback.
type InvokeFn func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error

// ValidateDirectFn runs validation commands directly and returns the result.
type ValidateDirectFn func(ctx context.Context, commands []string, workDir string) (*claude.Result, error)

// DiagnosticInvokeFn executes a diagnostic Claude invocation with a prompt and tier.
type DiagnosticInvokeFn func(ctx context.Context, prompt string, tier string) (*claude.Result, error)

// RenderDiagnosticFn renders the ATDD diagnostic prompt from a prompt context.
type RenderDiagnosticFn func(ctx *prompt.DiagnosticContext) (string, error)

// CoverageValidateFn validates coverage for a test code snippet and criterion.
type CoverageValidateFn func(ctx context.Context, testCode string, criterion coverage.Criterion) (*coverage.ValidationResponse, error)

// Executor handles ATDD workflow phases: writing acceptance tests,
// verifying they fail before implementation, refactoring, and retry wrappers.
type Executor struct {
	cfg                *config.Config
	output             io.Writer
	renderFn           RenderFn
	invokeFn           InvokeFn
	validateFn         ValidateDirectFn
	coverageValidateFn CoverageValidateFn
	escalateTierFn     EscalateTierFn
	analyzeFn          AnalyzeFn
	getDiffFn          GetDiffFn
	renderRefactorFn   RenderRefactorFn
	refactorInvokeFn   RefactorInvokeFn
	diagnosticInvokeFn DiagnosticInvokeFn
	renderDiagnosticFn RenderDiagnosticFn
	gitResetFn         GitResetFn
	getGitHeadFn       GetGitHeadFn
}

// AcceptanceVerificationError captures failure details when post-build
// acceptance verification fails.
type AcceptanceVerificationError struct {
	Message  string
	Output   string
	ExitCode int
}

func (e *AcceptanceVerificationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
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

// SetCoverageValidateFn sets the coverage validation callback.
func (e *Executor) SetCoverageValidateFn(fn CoverageValidateFn) {
	e.coverageValidateFn = fn
}

// SetDiagnosticDeps sets the diagnostic-phase dependencies.
func (e *Executor) SetDiagnosticDeps(diagnosticInvokeFn DiagnosticInvokeFn, renderDiagnosticFn RenderDiagnosticFn) {
	e.diagnosticInvokeFn = diagnosticInvokeFn
	e.renderDiagnosticFn = renderDiagnosticFn
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

// ValidateCoverage delegates coverage validation to the configured callback.
func (e *Executor) ValidateCoverage(ctx context.Context, testCode string, criterion coverage.Criterion) (*coverage.ValidationResponse, error) {
	if e.coverageValidateFn == nil {
		return nil, fmt.Errorf("coverage validate function not configured")
	}
	return e.coverageValidateFn(ctx, testCode, criterion)
}

// AcceptanceCommands returns a copy of the given commands with "-tags acceptance"
// injected into any "go test" command. Non-go-test commands are preserved as-is.
// If a command already contains "-tags acceptance", it is not modified.
//
// When touchedPackages is non-empty, any command ending with "./..." is scoped to
// those packages (e.g. "./internal/runner/..."), which prevents unrelated package
// acceptance failures from blocking the current bead.
func AcceptanceCommands(commands []string, touchedPackages []string) []string {
	result := make([]string, len(commands))
	for i, cmd := range commands {
		if strings.HasPrefix(cmd, "go test ") && !strings.Contains(cmd, "-tags acceptance") {
			// Insert -tags acceptance right after "go test "
			result[i] = "go test -tags acceptance " + cmd[len("go test "):]
		} else {
			result[i] = cmd
		}
		result[i] = scopeAcceptanceGoTestCommand(result[i], touchedPackages)
	}
	return result
}

func scopeAcceptanceGoTestCommand(cmd string, touchedPackages []string) string {
	trimmed := strings.TrimSpace(cmd)
	if len(touchedPackages) == 0 {
		return trimmed
	}
	if !strings.HasPrefix(trimmed, "go test ") {
		return trimmed
	}
	if !strings.HasSuffix(trimmed, " ./...") {
		return trimmed
	}

	scopedTargets := buildScopedGoTestTargets(touchedPackages)
	if len(scopedTargets) == 0 {
		return trimmed
	}

	prefix := strings.TrimSuffix(trimmed, "./...")
	return strings.TrimSpace(prefix) + " " + strings.Join(scopedTargets, " ")
}

func buildScopedGoTestTargets(touchedPackages []string) []string {
	targets := make([]string, 0, len(touchedPackages))
	seen := make(map[string]struct{}, len(touchedPackages))

	for _, pkg := range touchedPackages {
		pkg = strings.TrimSpace(pkg)
		pkg = strings.TrimPrefix(pkg, "./")
		pkg = strings.TrimSuffix(pkg, "/...")
		pkg = strings.TrimSuffix(pkg, "/")
		if pkg == "" || strings.ContainsAny(pkg, " \t\n\r") {
			continue
		}
		target := "."
		if pkg != "." {
			target = "./" + pkg + "/..."
		}
		if _, ok := seen[pkg]; ok {
			continue
		}
		seen[pkg] = struct{}{}
		targets = append(targets, target)
	}

	slices.Sort(targets)
	return targets
}

func (e *Executor) refreshTouchedPackages(bc *runtypes.BeadContext) {
	if e == nil || bc == nil || e.getDiffFn == nil || strings.TrimSpace(bc.StartCommit) == "" {
		return
	}

	diff, err := e.getDiffFn(bc.StartCommit)
	if err != nil || strings.TrimSpace(diff) == "" {
		return
	}

	touched := DetectTouchedPackages(diff)
	if len(touched) > 0 {
		bc.TouchedPackages = touched
	}
}

// log writes a formatted message to the output writer.
func (e *Executor) log(format string, args ...interface{}) {
	if e.output != nil {
		_, _ = fmt.Fprintf(e.output, format+"\n", args...) // best-effort logging, explicitly discard error
	}
}

// RunAcceptanceTests renders the acceptance test prompt and invokes the LLM.
func (e *Executor) RunAcceptanceTests(ctx context.Context, bc *runtypes.BeadContext) error {
	start := time.Now()
	e.log("ATDD acceptance generation started (tier=%s bead=%s)", bc.Tier, bc.Bead.ID)

	if e.renderFn == nil {
		return fmt.Errorf("render function not configured")
	}

	acceptancePrompt, err := e.renderFn(bc.PromptCtx)
	if err != nil {
		e.log("ATDD acceptance generation failed after %s: render error: %v", time.Since(start).Round(time.Millisecond), err)
		return fmt.Errorf("rendering acceptance tests prompt: %w", err)
	}

	if e.invokeFn == nil {
		return fmt.Errorf("invoke function not configured")
	}

	if err := e.invokeFn(ctx, bc, acceptancePrompt); err != nil {
		e.log("ATDD acceptance generation failed after %s: %v", time.Since(start).Round(time.Millisecond), err)
		return err
	}
	e.log("ATDD acceptance generation completed in %s", time.Since(start).Round(time.Millisecond))
	return nil
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
	e.refreshTouchedPackages(bc)

	acceptanceCommands := AcceptanceCommands(e.cfg.Validation.FastCommandsOrDefault(), bc.TouchedPackages)
	e.log("ATDD verify command set: %s", strings.Join(acceptanceCommands, " && "))
	start := time.Now()

	valResult, err := e.validateFn(ctx, acceptanceCommands, bc.PromptCtx.WorkDir)
	if err != nil {
		e.log("ATDD verify invocation failed after %s: %v", time.Since(start).Round(time.Millisecond), err)
		return fmt.Errorf("validation invocation: %w", err)
	}
	if valResult == nil {
		e.log("ATDD verify invocation failed after %s: nil validation result", time.Since(start).Round(time.Millisecond))
		return fmt.Errorf("validation returned no result")
	}
	e.log("ATDD verify invocation completed in %s (exit_code=%d)", time.Since(start).Round(time.Millisecond), valResult.ExitCode)

	// In ATDD, we expect tests to FAIL before implementation
	if claude.IsValidationPassed(valResult) {
		e.log("Unexpected: acceptance tests passed before implementation")
		e.log("Tests should fail until implementation makes them pass")
		e.log("Validation output on unexpected pass: %s", summarizeAcceptanceFailureOutput(valResult.Output))
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
	e.refreshTouchedPackages(bc)

	valResult, err := e.validateFn(ctx, AcceptanceCommands(e.cfg.Validation.FastCommandsOrDefault(), bc.TouchedPackages), bc.PromptCtx.WorkDir)
	if err != nil {
		return fmt.Errorf("acceptance validation invocation: %w", err)
	}
	if valResult == nil {
		return fmt.Errorf("acceptance validation returned no result")
	}

	if !claude.IsValidationPassed(valResult) {
		e.log("Acceptance tests failed after implementation")
		return &AcceptanceVerificationError{
			Message:  "acceptance tests failed after implementation - implementation may not satisfy acceptance criteria",
			Output:   summarizeAcceptanceFailureOutput(valResult.Output),
			ExitCode: valResult.ExitCode,
		}
	}

	e.log("Acceptance tests passed")
	return nil
}

// CheckTestsFailWithDiagnostic verifies tests fail before implementation and,
// when they unexpectedly pass, runs a diagnostic prompt to classify the result.
func (e *Executor) CheckTestsFailWithDiagnostic(ctx context.Context, bc *runtypes.BeadContext) error {
	if !e.cfg.Validation.Enabled {
		return fmt.Errorf("validation is not enabled - cannot verify tests fail")
	}
	if e.validateFn == nil {
		return fmt.Errorf("validate function not configured")
	}

	e.refreshTouchedPackages(bc)
	acceptanceCommands := AcceptanceCommands(e.cfg.Validation.FastCommandsOrDefault(), bc.TouchedPackages)
	validationResult, err := e.validateFn(ctx, acceptanceCommands, bc.PromptCtx.WorkDir)
	if err != nil {
		return fmt.Errorf("validation invocation: %w", err)
	}
	if validationResult == nil {
		return fmt.Errorf("validation returned no result")
	}

	// Expected red-phase behavior.
	if !claude.IsValidationPassed(validationResult) {
		return nil
	}

	if e.getDiffFn == nil {
		return fmt.Errorf("get diff function not configured")
	}
	testDiff, err := e.getDiffFn(bc.StartCommit)
	if err != nil {
		return fmt.Errorf("getting test diff: %w", err)
	}

	if e.renderDiagnosticFn == nil {
		return fmt.Errorf("render diagnostic function not configured")
	}
	diagnosticPrompt, err := e.renderDiagnosticFn(buildDiagnosticContext(bc, testDiff, validationResult.Output))
	if err != nil {
		return fmt.Errorf("rendering diagnostic prompt: %w", err)
	}

	if e.diagnosticInvokeFn == nil {
		return fmt.Errorf("diagnostic invoke function not configured")
	}
	diagnosticResult, err := e.diagnosticInvokeFn(ctx, diagnosticPrompt, provider.TierLow)
	if err != nil {
		return ErrATDDAlreadyDone
	}
	if diagnosticResult == nil {
		return ErrATDDAlreadyDone
	}

	verdict, feedback := parseDiagnosticVerdict(diagnosticResult.Output)
	if verdict == diagnosticVerdictRewrite {
		return &ErrATDDRewrite{Feedback: feedback}
	}
	return ErrATDDAlreadyDone
}

func buildDiagnosticContext(bc *runtypes.BeadContext, testDiff string, testOutput string) *prompt.DiagnosticContext {
	return &prompt.DiagnosticContext{
		BeadTitle:          bc.Bead.Title,
		BeadDescription:    bc.Bead.Description,
		AcceptanceCriteria: bc.Bead.AcceptanceCriteria,
		TestDiff:           testDiff,
		TestOutput:         testOutput,
	}
}

func summarizeAcceptanceFailureOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}
	const maxChars = 2000
	if len(trimmed) <= maxChars {
		return trimmed
	}
	const side = 900
	return trimmed[:side] + "\n...[truncated]...\n" + trimmed[len(trimmed)-side:]
}
