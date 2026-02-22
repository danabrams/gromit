package runner

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/bead"
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

func (a *reviewRendererAdapter) RenderReview(beadTitle, diff string) (string, error) {
	ctx := &prompt.ReviewContext{
		Bead: &bead.Bead{
			Title: beadTitle,
		},
		Diff: diff,
	}
	return a.r.RenderReview(ctx)
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
	return a.mgr.PendingBranches()
}

func (a *worktreeMergerAdapter) MergeBack(branch string) error {
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

// decomposerAdapter wraps bead.Client to satisfy prepare.Decomposer for auto-decomposition of oversized beads.
type decomposerAdapter struct {
	beads *bead.Client
}

func (a *decomposerAdapter) Decompose(ctx context.Context, b *bead.Bead) error {
	_, err := a.beads.CreateWithParent(b.Title+" (decomposed)", b.Priority, b.Labels, b.ExpectedOutputs, b.ID)
	return err
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
