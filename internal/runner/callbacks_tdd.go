package runner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/tdd"
)

const (
	tddPhaseRed   = "red"
	tddPhaseGreen = "green"
)

type tddOrchestrator struct {
	runCyclesFn func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error
}

func (o *tddOrchestrator) RunCycles(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
	if o != nil && o.runCyclesFn != nil {
		return o.runCyclesFn(ctx, bc, tracker, criteria)
	}
	return fmt.Errorf("tdd orchestrator is not configured")
}

func (r *Runner) makeTDDOrchestrator() *tddOrchestrator {
	if r == nil {
		return nil
	}

	return &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			if bc == nil || bc.Bead == nil {
				return fmt.Errorf("bead context is nil")
			}

			var activeBC *runtypes.BeadContext
			var currentCoverageState string
			var lastRenderedPhase string
			var pendingCoverageCriterion *coverage.Criterion
			var lastSelfReport *coverage.SelfReport
			var lastFailingTestCode string
			invokeFn := r.makeInvokeFn()
			criteriaByNumber := criteriaIndexByNumber(criteria)

			orch := tdd.NewCycleOrchestrator(r.cfg, r.output, tdd.CycleOrchestratorDeps{
				RenderRedFn: func(handoff *tdd.RedHandoff, bc *runtypes.BeadContext) (string, error) {
					if r.renderer == nil {
						return "", fmt.Errorf("renderer not configured")
					}

					specExcerpt := handoff.SpecExcerpt
					currentCoverageState = ""
					if tracker != nil {
						nextCriterion := tracker.NextUncovered()
						if nextCriterion != nil {
							copied := *nextCriterion
							pendingCoverageCriterion = &copied
							currentCoverageState = tracker.FormatCoverageState(nextCriterion.Number)
							if strings.TrimSpace(specExcerpt) == "" {
								specExcerpt = currentCoverageState
							} else {
								specExcerpt = specExcerpt + "\n\n" + currentCoverageState
							}
						}
					}

					ctx := &prompt.TDDRedContext{
						BeadTitle:        bc.Bead.Title,
						SpecExcerpt:      specExcerpt,
						TestFileContents: handoff.TestFiles,
						APISurface:       handoff.APISurface,
						CycleSummary:     handoff.CycleSummary,
					}
					lastRenderedPhase = tddPhaseRed
					return r.renderer.RenderTDDRed(ctx)
				},
				RenderGreenFn: func(handoff *tdd.GreenHandoff, bc *runtypes.BeadContext) (string, error) {
					if r.renderer == nil {
						return "", fmt.Errorf("renderer not configured")
					}
					ctx := &prompt.TDDGreenContext{
						BeadTitle:         bc.Bead.Title,
						FailingTest:       handoff.FailingTest,
						TestFailureOutput: handoff.TestFailureOutput,
						ImplFileContents:  handoff.ImplFiles,
					}
					lastFailingTestCode = handoff.FailingTest
					lastRenderedPhase = tddPhaseGreen
					return r.renderer.RenderTDDGreen(ctx)
				},
				InvokeFn: func(ctx context.Context, promptText, tier string) error {
					if activeBC == nil {
						return fmt.Errorf("bead context is nil")
					}
					if r.escalationHandler == nil {
						return fmt.Errorf("escalation handler not configured")
					}
					activeBC.Tier = tier
					invResult, err := invokeFn(ctx, activeBC, promptText)
					if err != nil {
						return err
					}
					if invResult == nil || invResult.Result == nil {
						return fmt.Errorf("invocation returned nil result")
					}
					if !invResult.Result.Success {
						return fmt.Errorf("invocation failed: %s", runtypes.TruncateOutput(invResult.Result.Output))
					}
					selfReport, reportErr := coverage.ParseSelfReport(invResult.Result.Output)
					if reportErr == nil {
						lastSelfReport = selfReport
					}
					return nil
				},
				ValidateFn: func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
					validationCommands := commands
					if len(validationCommands) == 0 && r.cfg != nil {
						validationCommands = r.cfg.Validation.FastCommandsOrDefault()
					}
					validationWorkDir := workDir
					if strings.TrimSpace(validationWorkDir) == "" && activeBC != nil && activeBC.PromptCtx != nil {
						validationWorkDir = activeBC.PromptCtx.WorkDir
					}

					result, err := r.runDirectValidationCheck(ctx, validationCommands, validationWorkDir)
					if err != nil {
						return "", false, err
					}
					if result == nil {
						return "", false, fmt.Errorf("validation returned nil result")
					}
					passed := claude.IsValidationPassed(result)
					if !passed || tracker == nil || r.methodologyExec == nil || lastRenderedPhase != tddPhaseGreen {
						return result.Output, passed, nil
					}

					target := resolveCoverageTargetCriterion(pendingCoverageCriterion, lastSelfReport, criteriaByNumber)
					if target == nil {
						return result.Output, passed, nil
					}

					coverageResult, coverageErr := r.methodologyExec.ValidateCoverage(ctx, lastFailingTestCode, *target)
					if coverageErr != nil {
						return "", false, fmt.Errorf("coverage validation failed: %w", coverageErr)
					}
					if coverageResult != nil && coverageResult.Covers {
						tracker.MarkCovered(target.Number)
					} else {
						tracker.RecordRejection(target.Number)
					}
					updateIterationCoverageMetrics(activeBC.Result, tracker)

					if lastSelfReport != nil && len(lastSelfReport.Remaining) == 0 && !tracker.IsComplete() {
						r.log("TDD coverage disagreement: self-report done but tracker has unchecked criteria (%s)", currentCoverageState)
					}
					lastRenderedPhase = ""
					pendingCoverageCriterion = nil
					return result.Output, passed, nil
				},
				RunRefactorFn: func(ctx context.Context, bc *runtypes.BeadContext) error {
					if r.methodologyExec == nil {
						return fmt.Errorf("methodology executor not configured")
					}
					result := r.methodologyExec.RunRefactorPhaseWithResult(ctx, bc)
					if !result.Successful {
						return fmt.Errorf("refactor phase failed: %s", result.Reason)
					}
					return nil
				},
				EscalateTierFn: func(currentTier string) string {
					if r.escalationPolicy == nil {
						return ""
					}
					return r.escalationPolicy.NextTier(currentTier)
				},
				ReadFileFn: func(path string) (string, error) {
					content, err := os.ReadFile(path)
					if err != nil {
						return "", err
					}
					return string(content), nil
				},
				GetDiffFn: func() (string, error) {
					if activeBC == nil || activeBC.StartCommit == "" {
						return "", nil
					}
					return r.getDiff(activeBC.StartCommit)
				},
				GetGitHeadFn: r.getHead,
				GitResetFn:   r.resetHard,
			})

			maxCycles := resolveMaxTDDCycles(r.cfg)

			remaining, done := resolveInitialCycleState(bc.Bead.ExpectedOutputs, tracker)

			state := tdd.CycleState{
				CycleNumber: 0,
				MaxCycles:   maxCycles,
				Remaining:   remaining,
				Done:        done,
			}
			if bc.StartCommit != "" {
				if diff, err := r.getDiff(bc.StartCommit); err == nil {
					state.TouchedFiles = methodology.ParseDiffFiles(diff)
				}
			}

			activeBC = bc
			defer func() { activeBC = nil }()
			return orch.RunCycles(ctx, bc, state)
		},
	}
}

func criteriaIndexByNumber(criteria []coverage.Criterion) map[int]coverage.Criterion {
	index := make(map[int]coverage.Criterion, len(criteria))
	for _, criterion := range criteria {
		index[criterion.Number] = criterion
	}
	return index
}

func resolveCoverageTargetCriterion(
	pending *coverage.Criterion,
	report *coverage.SelfReport,
	criteriaByNumber map[int]coverage.Criterion,
) *coverage.Criterion {
	target := pending
	if report == nil || report.Targeting <= 0 {
		return target
	}
	criterion, ok := criteriaByNumber[report.Targeting]
	if !ok {
		return target
	}
	copied := criterion
	return &copied
}

func resolveInitialCycleState(expectedOutputs []string, tracker *coverage.CoverageTracker) ([]string, bool) {
	remaining := append([]string(nil), expectedOutputs...)
	done := len(remaining) == 0
	if tracker == nil {
		return remaining, done
	}

	uncovered := tracker.UncoveredCriteria()
	remaining = make([]string, 0, len(uncovered))
	for _, criterion := range uncovered {
		remaining = append(remaining, criterion.Text)
	}
	return remaining, tracker.IsComplete()
}
