package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/tdd"
)

// buildTDDCycleRunner creates a TDDCycleRunner adapter backed by a Runner with a
// configured tddOrchestrator. Inject the result into execute.Build via WithTDDCycleRunner.
func buildTDDCycleRunner(cfg *config.Config, renderer *prompt.Renderer, router *provider.Router, output io.Writer) execute.TDDCycleRunner {
	orch := tdd.NewCycleOrchestrator(cfg, output, tdd.CycleOrchestratorDeps{
		RenderRedFn:    buildRenderRedFn(cfg, renderer),
		RenderGreenFn:  buildRenderGreenFn(cfg, renderer),
		InvokeFn:       buildInvokeFn(router, output),
		ValidateFn:     buildValidateFn(cfg),
		RunRefactorFn:  buildRunRefactorFn(cfg, renderer, router, output),
		EscalateTierFn: func(currentTier string) string { return cfg.NextEscalationTier(currentTier) },
		GetDiffFn:      func() (string, error) { return getGitDiff("HEAD") },
		ReadFileFn:     readFileAsString,
		GetGitHeadFn:   gitHeadCommit,
		GitResetFn:     gitResetHard,
		LogPhaseFn: func(cycle int, phase, detail string) {
			_, _ = fmt.Fprintf(output, "  [tdd] cycle %d %s: %s\n", cycle, phase, detail)
		},
	})

	// layer3Invoke wraps the router to provide the invoke signature that
	// extractRequirementsViaLLM / applyLayer3Requirements expect.
	layer3Invoke := func(invokeCtx context.Context, prompt, tier string) (*provider.Result, error) {
		p, _ := router.Select("build", tier)
		if p == nil {
			return nil, fmt.Errorf("no provider available for tier %s", tier)
		}
		return p.Run(invokeCtx, prompt, tier)
	}

	r := &Runner{
		cfg: cfg,
		tddOrchestrator: &tddOrchestrator{
			runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
				if bc.Tier == "" {
					bc.Tier = cfg.SelectTier(bc.Bead.Priority, bc.Bead.Labels)
				}
				if bc.Model == "" {
					bc.Model = provider.TierToLegacyModel(bc.Tier)
				}
				remaining := bc.Bead.ExpectedOutputs
				if len(remaining) == 0 {
					remaining = tddExpectedOutputsOrTitle(bc.Bead)
				}
				// Layer 3: LLM-based requirement extraction when we only have
				// a single broad requirement (title fallback).
				if len(remaining) <= 1 {
					if extracted, _ := applyLayer3Requirements(ctx, cfg, remaining, bc.Bead.Title, bc.Bead.Description, layer3Invoke); len(extracted) > len(remaining) {
						remaining = extracted
					}
				}
				state := tdd.CycleState{
					MaxCycles: cfg.Methodology.MaxTDDCycles,
					Remaining: remaining,
				}
				return orch.RunCycles(ctx, bc, state)
			},
		},
	}
	return &TDDPipelineAdapter{runner: r}
}

func buildRenderRedFn(cfg *config.Config, renderer *prompt.Renderer) tdd.RenderRedFn {
	return func(handoff *tdd.RedHandoff, bc *runtypes.BeadContext) (string, error) {
		if bc != nil {
			bc.Tier = cfg.PhaseModelTier("red", bc.Tier)
		}
		rules, err := renderer.LoadRulesForPhase("red")
		if err != nil {
			return "", err
		}
		ctx := &prompt.TDDRedContext{
			BeadID:           bc.Bead.ID,
			BeadTitle:        bc.Bead.Title,
			SpecExcerpt:      handoff.SpecExcerpt,
			TestFileContents: handoff.TestFiles,
			APISurface:       handoff.APISurface,
			CycleSummary:     handoff.CycleSummary,
			Rules:            rules,
		}
		return renderer.RenderTDDRed(ctx)
	}
}

func buildRenderGreenFn(cfg *config.Config, renderer *prompt.Renderer) tdd.RenderGreenFn {
	return func(handoff *tdd.GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		if bc != nil {
			bc.Tier = cfg.PhaseModelTier("green", bc.Tier)
		}
		rules, err := renderer.LoadRulesForPhase("green")
		if err != nil {
			return "", err
		}
		ctx := &prompt.TDDGreenContext{
			BeadID:            bc.Bead.ID,
			BeadTitle:         bc.Bead.Title,
			FailingTest:       handoff.FailingTest,
			TestFailureOutput: handoff.TestFailureOutput,
			ImplFileContents:  handoff.ImplFiles,
			Rules:             rules,
		}
		return renderer.RenderTDDGreen(ctx)
	}
}

func buildInvokeFn(router *provider.Router, output io.Writer) tdd.InvokeFn {
	return func(ctx context.Context, promptText, tier string) error {
		p, _ := router.Select("build", tier)
		if p == nil {
			return fmt.Errorf("no provider available for tier %s", tier)
		}
		_, err := p.StreamRun(ctx, promptText, tier, output, nil, nil)
		return err
	}
}

func buildValidateFn(cfg *config.Config) tdd.ValidateFn {
	return func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		if len(commands) == 0 {
			commands = cfg.Validation.Commands
		}
		if workDir == "" {
			workDir = "."
		}
		var allOutput strings.Builder
		for _, cmd := range commands {
			stdout, stderr, exitCode, err := defaultCmdRunner(ctx, cmd, workDir)
			allOutput.WriteString(stdout)
			allOutput.WriteString(stderr)
			if err != nil {
				return allOutput.String(), false, err
			}
			if exitCode != 0 {
				return allOutput.String(), false, nil
			}
		}
		return allOutput.String(), true, nil
	}
}

func buildRunRefactorFn(cfg *config.Config, renderer *prompt.Renderer, router *provider.Router, output io.Writer) tdd.RunRefactorFn {
	return func(ctx context.Context, bc *runtypes.BeadContext) error {
		if bc != nil {
			bc.Tier = cfg.PhaseModelTier("refactor", bc.Tier)
		}
		rules, err := renderer.LoadRulesForPhase("refactor")
		if err != nil {
			return err
		}
		refactorCtx := &prompt.Context{
			Bead: bc.Bead,
			Rules: rules,
		}
		promptText, err := renderer.RenderRefactor(refactorCtx)
		if err != nil {
			return err
		}
		p, _ := router.Select("refactor", bc.Tier)
		if p == nil {
			return fmt.Errorf("no provider available for tier %s", bc.Tier)
		}
		_, err = p.StreamRun(ctx, promptText, bc.Tier, output, nil, nil)
		return err
	}
}

func readFileAsString(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func gitHeadCommit() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitResetHard(commit string) error {
	return exec.Command("git", "reset", "--hard", commit).Run()
}

// optionalTDDCycleRunner returns a TDDCycleRunner when FreshContextPerCycle is
// enabled, or nil otherwise. The caller should inject the result into the Build
// stage via WithTDDCycleRunner.
func optionalTDDCycleRunner(cfg *config.Config, renderer *prompt.Renderer, router *provider.Router, output io.Writer) execute.TDDCycleRunner {
	if cfg == nil || !cfg.Methodology.FreshContextPerCycle {
		return nil
	}
	if cfg.ResolvedMethodologyAdapter().Value != "go" {
		return nil
	}
	return buildTDDCycleRunner(cfg, renderer, router, output)
}

func snapshotIterationUsage(result *runtypes.IterationResult) (costUSD float64, inputTokens int, outputTokens int) {
	if result == nil {
		return 0, 0, 0
	}
	return result.CostUSD, result.InputTokens, result.OutputTokens
}

func phaseUsageDelta(
	result *runtypes.IterationResult,
	beforeCostUSD float64,
	beforeInputTokens int,
	beforeOutputTokens int,
) (costUSD float64, inputTokens int, outputTokens int) {
	afterCostUSD, afterInputTokens, afterOutputTokens := snapshotIterationUsage(result)
	costUSD = afterCostUSD - beforeCostUSD
	inputTokens = afterInputTokens - beforeInputTokens
	outputTokens = afterOutputTokens - beforeOutputTokens
	if costUSD < 0 {
		costUSD = 0
	}
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	return costUSD, inputTokens, outputTokens
}

// appendTDDPhaseMetric records a per-invocation PhaseMetric for a TDD cycle.
// It computes the cost/token delta since the before-snapshot and appends a
// PhaseMetric entry to bc.Result.PhaseMetrics.
func appendTDDPhaseMetric(
	bc *runtypes.BeadContext,
	phase string,
	cycleNumber int,
	beforeCostUSD float64,
	beforeInputTokens int,
	beforeOutputTokens int,
	start time.Time,
) {
	if bc == nil || bc.Result == nil || bc.Bead == nil {
		return
	}
	costUSD, inputTokens, outputTokens := phaseUsageDelta(bc.Result, beforeCostUSD, beforeInputTokens, beforeOutputTokens)
	durationMs := int64(0)
	if !start.IsZero() {
		if d := time.Since(start).Milliseconds(); d > 0 {
			durationMs = d
		}
	}
	if cycleNumber < 1 {
		cycleNumber = 1
	}
	bc.Result.PhaseMetrics = append(bc.Result.PhaseMetrics, runtypes.PhaseMetric{
		Phase:        phase,
		CycleNumber:  cycleNumber,
		BeadID:       bc.Bead.ID,
		Model:        bc.Model,
		Tier:         bc.Tier,
		CostUSD:      costUSD,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		DurationMs:   durationMs,
		Success:      true,
	})
}

type tddOrchestrator struct {
	runCyclesFn func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error
}

func (o *tddOrchestrator) RunCycles(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
	if o != nil && o.runCyclesFn != nil {
		return o.runCyclesFn(ctx, bc, tracker, criteria)
	}
	return fmt.Errorf("tdd orchestrator is not configured")
}
