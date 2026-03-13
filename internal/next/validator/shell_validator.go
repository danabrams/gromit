package validator

import "context"

// ShellValidator implements stages.FinalValidator by delegating to a Runner.
// It performs shell-only validation with no LLM dependency.
type ShellValidator struct {
	runner *Runner
}

// NewShellValidator creates a ShellValidator. Panics if runner is nil.
func NewShellValidator(runner *Runner) *ShellValidator {
	if runner == nil {
		panic("validator: runner must not be nil")
	}
	return &ShellValidator{runner: runner}
}

// RunFinal delegates to the underlying Runner.RunFinal.
func (sv *ShellValidator) RunFinal(ctx context.Context, alwaysRun []Check, projectChecks []Check, workDir string) (FinalResult, error) {
	return sv.runner.RunFinal(ctx, alwaysRun, projectChecks, workDir)
}
