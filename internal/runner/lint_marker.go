package runner

// RunnerLintBaselineAcceptanceMarker is a sentinel constant that indicates
// all golangci-lint baseline violations (errcheck/unused/staticcheck) have
// been resolved in the runner package.
//
// This marker is referenced by TestRunnerLintBaseline_ErrcheckUnusedStaticcheck
// in lint_baseline_acceptance_test.go.
const RunnerLintBaselineAcceptanceMarker = "runner-lint-baseline-clean"
