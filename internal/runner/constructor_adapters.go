package runner

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/worktree"
)

// Adapter types for bridging existing infrastructure to pipeline stage interfaces.

// invokerAdapter wraps *provider.Router to satisfy execute.Invoker.
type invokerAdapter struct {
	router *provider.Router
	output io.Writer
}

func (a *invokerAdapter) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if a.router == nil {
		return nil, fmt.Errorf("router is nil")
	}
	p, _ := a.router.Select("build", tier)
	if p == nil {
		return nil, fmt.Errorf("no provider available for tier %s", tier)
	}
	return p.Run(ctx, prompt, tier)
}

func (a *invokerAdapter) StreamRun(ctx context.Context, prompt, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if a.router == nil {
		return nil, fmt.Errorf("router is nil")
	}
	p, _ := a.router.Select("build", tier)
	if p == nil {
		return nil, fmt.Errorf("no provider available for tier %s", tier)
	}
	return p.StreamRun(ctx, prompt, tier, w, handler, onToolCall)
}

// renderAdapter wraps prompt.Renderer to satisfy execute.PromptRenderer.
type renderAdapter struct {
	r *prompt.Renderer
}

func (a *renderAdapter) RenderBuild(title, description string, validationFailures []string) (string, error) {
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: validationFailures,
	}
	return a.r.RenderBuild(ctx)
}

func (a *renderAdapter) RenderTDDBuild(title, description string, validationFailures []string) (string, error) {
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: validationFailures,
	}
	return a.r.RenderTDDBuild(ctx)
}

func (a *renderAdapter) RenderRefactorBuild(title, description string, validationFailures []string) (string, error) {
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: validationFailures,
	}
	return a.r.RenderRefactor(ctx)
}

// cmdRunnerAdapter wraps a command runner function to satisfy validate.CommandRunner.
type cmdRunnerAdapter struct {
	runner func(ctx context.Context, command, workDir string) (string, string, int, error)
}

func (a *cmdRunnerAdapter) Run(ctx context.Context, command, workDir string) (string, string, int, error) {
	return a.runner(ctx, command, workDir)
}

// reviewInvokerAdapter wraps *provider.Router to satisfy review.Invoker.
type reviewInvokerAdapter struct {
	router  *provider.Router
	syncOut *syncWriter
}

func (a *reviewInvokerAdapter) StreamRun(ctx context.Context, prompt string, model string, w io.Writer) (string, error) {
	if a.router == nil {
		return "", fmt.Errorf("router is nil")
	}
	p, _ := a.router.Select("review", "high")
	if p == nil {
		return "", fmt.Errorf("no provider available for review")
	}
	result, err := p.Run(ctx, prompt, "high")
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("review invoker returned nil result")
	}
	return result.Output, nil
}

// beadCreatorAdapter wraps bead.Client to satisfy review.BeadCreator.
type beadCreatorAdapter struct {
	beads *bead.Client
}

func (a *beadCreatorAdapter) Create(title string, priority int, labels []string, outputs []string) (string, error) {
	b, err := a.beads.Create(title, priority, labels, outputs)
	if err != nil {
		return "", err
	}
	if b == nil {
		return "", fmt.Errorf("beads.Create returned nil")
	}
	return b.ID, nil
}

// reviewRendererAdapter wraps prompt.Renderer to satisfy review.PromptRenderer.
type reviewRendererAdapter struct {
	r *prompt.Renderer
}

func (a *reviewRendererAdapter) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	return a.r.RenderReview(ctx)
}

func (a *reviewRendererAdapter) LoadRulesForPhase(phase string) (string, error) {
	return a.r.LoadRulesForPhase(phase)
}

// beadLifecycleAdapter wraps bead.Client to satisfy epilogue.BeadLifecycle.
type beadLifecycleAdapter struct {
	beads *bead.Client
}

func (a *beadLifecycleAdapter) Close(id string) error {
	return a.beads.Close(id)
}

func (a *beadLifecycleAdapter) Sync() error {
	return a.beads.Sync()
}

// statusWriterAdapter wraps runner.StatusWriter to satisfy epilogue.StatusWriter.
type statusWriterAdapter struct {
	sw *StatusWriter
}

func (a *statusWriterAdapter) Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error {
	if a.sw == nil {
		return nil
	}
	return a.sw.Write(iteration, beadID, beadTitle, model, true, maxIterations, timeBudgetMinutes)
}

// worktreeMergerAdapter wraps worktree.Manager to satisfy epilogue.WorktreeMerger.
type worktreeMergerAdapter struct {
	mgr *worktree.Manager
}

func (a *worktreeMergerAdapter) PendingBranches() ([]string, error) {
	if a == nil || a.mgr == nil {
		return nil, fmt.Errorf("worktree manager is nil")
	}
	return a.mgr.PendingBranches()
}

func (a *worktreeMergerAdapter) MergeBack(branch string) error {
	if a == nil || a.mgr == nil {
		return fmt.Errorf("worktree manager is nil")
	}
	return a.mgr.MergeBack(branch)
}

// epilogueCommandRunnerAdapter wraps a command runner function to satisfy epilogue.CommandRunner.
type epilogueCommandRunnerAdapter struct {
	runner func(ctx context.Context, command, workDir string) (string, string, int, error)
}

func (a *epilogueCommandRunnerAdapter) Run(ctx context.Context, command string) (string, string, int, error) {
	return a.runner(ctx, command, "")
}

// iterationLogWriterAdapter wraps *logger.Logger to satisfy epilogue.IterationLogWriter.
type iterationLogWriterAdapter struct {
	logger *logger.Logger
}

func (a *iterationLogWriterAdapter) Write(log *logger.IterationLog) error {
	if a.logger == nil {
		return nil
	}
	return a.logger.LogIteration(log)
}

// scopeGateSubBead represents a single sub-bead from the LLM decomposition response.
type scopeGateSubBead struct {
	Title           string   `json:"title"`
	ExpectedOutputs []string `json:"expected_outputs"`
}

// decomposerAdapter uses provider routing to invoke LLM-powered decomposition of oversized beads.
type decomposerAdapter struct {
	beads  *bead.Client
	router *provider.Router
}

func (a *decomposerAdapter) Decompose(ctx context.Context, b *bead.Bead) error {
	if a.router == nil {
		return fmt.Errorf("decomposerAdapter: no router configured for LLM decomposition")
	}

	p, _ := a.router.Select("decompose", provider.TierMedium)
	if p == nil {
		return fmt.Errorf("decomposerAdapter: no provider available for decomposition")
	}

	promptText := buildScopeGateDecomposePrompt(b)
	result, err := p.Run(ctx, promptText, provider.TierMedium)
	if err != nil {
		return fmt.Errorf("decomposerAdapter: LLM invocation failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("decomposerAdapter: LLM returned failure (exit %d)", result.ExitCode)
	}

	var subBeads []scopeGateSubBead
	if err := jsonutil.ExtractJSON(strings.TrimSpace(result.Output), &subBeads); err != nil {
		return fmt.Errorf("decomposerAdapter: parsing LLM output: %w", err)
	}
	if len(subBeads) < 2 {
		return fmt.Errorf("decomposerAdapter: decomposition contract violation: expected 2-5 sub-beads, got %d", len(subBeads))
	}
	if len(subBeads) > 5 {
		return fmt.Errorf("decomposerAdapter: decomposition contract violation: expected 2-5 sub-beads, got %d", len(subBeads))
	}
	for i, sb := range subBeads {
		if strings.TrimSpace(sb.Title) == "" {
			return fmt.Errorf("decomposerAdapter: decomposition contract violation: sub-bead %d has empty title", i)
		}
		if len(sb.ExpectedOutputs) == 0 {
			return fmt.Errorf("decomposerAdapter: decomposition contract violation: sub-bead %d has no expected outputs", i)
		}
		if len(sb.ExpectedOutputs) > 5 {
			return fmt.Errorf("decomposerAdapter: decomposition contract violation: sub-bead %d has %d expected outputs (max 5)", i, len(sb.ExpectedOutputs))
		}
		seenOutputs := make(map[string]struct{}, len(sb.ExpectedOutputs))
		for j, output := range sb.ExpectedOutputs {
			if strings.TrimSpace(output) == "" {
				return fmt.Errorf("decomposerAdapter: decomposition contract violation: sub-bead %d has empty expected output at index %d", i, j)
			}
			if output == b.Title {
				return fmt.Errorf("decomposerAdapter: decomposition contract violation: sub-bead %d has expected output that echoes parent title", i)
			}
			if _, exists := seenOutputs[output]; exists {
				return fmt.Errorf("decomposerAdapter: decomposition contract violation: sub-bead %d has duplicate expected outputs", i)
			}
			seenOutputs[output] = struct{}{}
		}
	}

	labels := a.resolveBuildStrategyLabels(b)
	for _, sb := range subBeads {
		if _, err := a.beads.CreateWithParent(sb.Title, b.Priority, labels, sb.ExpectedOutputs, b.ID); err != nil {
			return fmt.Errorf("decomposerAdapter: creating child bead %q: %w", sb.Title, err)
		}
	}

	if err := a.beads.Close(b.ID); err != nil {
		return fmt.Errorf("decomposerAdapter: closing parent bead: %w", err)
	}
	return nil
}

func (a *decomposerAdapter) resolveBuildStrategyLabels(parent *bead.Bead) []string {
	const buildStrategyPrefix = "build_strategy:"

	if label := findLabelWithPrefix(parent.Labels, buildStrategyPrefix); label != "" {
		return []string{label}
	}

	if a.beads == nil || parent.ID == "" {
		return nil
	}
	fullParent, err := a.beads.Show(parent.ID)
	if err != nil || fullParent == nil {
		return nil
	}
	if label := findLabelWithPrefix(fullParent.Labels, buildStrategyPrefix); label != "" {
		return []string{label}
	}
	return nil
}

func findLabelWithPrefix(labels []string, prefix string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, prefix) {
			return label
		}
	}
	return ""
}

// buildScopeGateDecomposePrompt builds the LLM prompt for scope gate decomposition.
func buildScopeGateDecomposePrompt(b *bead.Bead) string {
	outputs := strings.Join(b.ExpectedOutputs, "\n- ")
	if outputs != "" {
		outputs = "- " + outputs
	}
	return fmt.Sprintf(`You are decomposing an oversized task into smaller sub-tasks that each touch 5 or fewer files.

## Oversized Task
Title: %s
Description: %s
Expected outputs (too many):
%s

## Instructions
Split this task into 2-5 sub-tasks. Each sub-task must:
- Have a clear, specific title
- Touch 5 or fewer files (expected_outputs list)
- Together cover all the work of the original task

## Output
Output ONLY a JSON array. No markdown, no explanation.
Each element: {"title": "...", "expected_outputs": ["file1", "file2", ...]}
`, b.Title, b.Description, outputs)
}

// failureLearnerAdapter wraps analyzer and related dependencies to satisfy epilogue.FailureLearner.
type failureLearnerAdapter struct {
	renderer *prompt.Renderer
	router   *provider.Router
	analyzer FailureAnalyzer
	logFn    func(string, ...interface{})
}

func (a *failureLearnerAdapter) ExtractFailureLearning(ctx context.Context, beadID, beadTitle, failureOutput string) error {
	if a.analyzer == nil {
		return nil
	}
	b := &bead.Bead{ID: beadID, Title: beadTitle}
	analysis, err := a.analyzer.Analyze(ctx, b, failureOutput)
	if err != nil {
		if a.logFn != nil {
			a.logFn("Warning: failure analysis error: %v", err)
		}
		return nil
	}
	if analysis == nil || analysis.Learning == nil {
		return nil
	}
	if a.renderer != nil {
		lf := a.renderer.GetLearningsFile()
		if lf != nil {
			_, _ = lf.Add(beadID, *analysis.Learning, analysis.LearningCategory())
		}
	}
	return nil
}
