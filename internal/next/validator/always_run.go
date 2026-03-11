package validator

import "context"

// CheckResults aggregates results from multiple check executions.
type CheckResults struct {
	Results []CheckResult `json:"results"`
}

// AllPass returns true if every check passed.
func (cr CheckResults) AllPass() bool {
	for _, r := range cr.Results {
		if !r.Pass {
			return false
		}
	}
	return true
}

// PassCount returns the number of passing checks.
func (cr CheckResults) PassCount() int {
	n := 0
	for _, r := range cr.Results {
		if r.Pass {
			n++
		}
	}
	return n
}

// FailCount returns the number of failing checks.
func (cr CheckResults) FailCount() int {
	return len(cr.Results) - cr.PassCount()
}

// FailedChecks returns only the checks that failed.
func (cr CheckResults) FailedChecks() []CheckResult {
	var failed []CheckResult
	for _, r := range cr.Results {
		if !r.Pass {
			failed = append(failed, r)
		}
	}
	return failed
}

// RunAlwaysRun executes all always-run checks sequentially,
// collecting results for every check regardless of individual pass/fail.
func (r *Runner) RunAlwaysRun(ctx context.Context, checks []Check, workDir string) (CheckResults, error) {
	var results []CheckResult
	for _, c := range checks {
		res, err := r.RunCheck(ctx, c, workDir)
		if err != nil {
			return CheckResults{}, err
		}
		results = append(results, res)
	}
	return CheckResults{Results: results}, nil
}
