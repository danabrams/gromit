package validator

import "context"

// FinalResult captures the outcome of final validation,
// combining always-run checks with project-level checks.
type FinalResult struct {
	Pass          bool         `json:"pass"`
	AlwaysRun     CheckResults `json:"always_run"`
	ProjectChecks CheckResults `json:"project_checks"`
}

// RunFinal executes final validation: always-run checks plus project checks.
// Proof checks (task-level targeted checks) are NOT part of final validation.
// Overall pass requires both groups to pass.
func (r *Runner) RunFinal(ctx context.Context, alwaysRun []Check, projectChecks []Check, workDir string) (FinalResult, error) {
	ar, err := r.RunAlwaysRun(ctx, alwaysRun, workDir)
	if err != nil {
		return FinalResult{}, err
	}
	pc, err := r.RunAlwaysRun(ctx, projectChecks, workDir)
	if err != nil {
		return FinalResult{}, err
	}
	return FinalResult{
		Pass:          ar.AllPass() && pc.AllPass(),
		AlwaysRun:     ar,
		ProjectChecks: pc,
	}, nil
}
