package runner

import (
	"context"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/specbranch"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/worktree"
)

// Compile-time interface satisfaction checks.
var (
	_ BeadClient      = (*bead.Client)(nil)
	_ tracker.Client  = (*bead.BDAdapter)(nil)
	_ FailureAnalyzer = (*analyzer.Analyzer)(nil)
	_ PromptRenderer  = (*prompt.Renderer)(nil)
	_ IterationLogger = (*logger.Logger)(nil)
	_ WorktreeManager = (*worktree.Manager)(nil)
	_ BranchRouter    = (*specbranch.Router)(nil)
	_ GitCheckout       = (*specbranch.GitOps)(nil)
	_ PreflightChecker  = (*specbranch.GitOps)(nil)
)

// BeadClient abstracts the bead (bd) CLI operations used by the runner.
type BeadClient interface {
	Ready(ctx context.Context) (*bead.Bead, error)
	ReadyExcluding(ctx context.Context, excludeIDs map[string]bool) (*bead.Bead, error)
	ReadyWithLabel(ctx context.Context, label string) (*bead.Bead, error)
	ListWithLabel(ctx context.Context, label string) ([]*bead.Bead, error)
	Show(ctx context.Context, id string) (*bead.Bead, error)
	Close(ctx context.Context, id string) error
	Sync(ctx context.Context) error
	AddComment(ctx context.Context, id, comment string) error
	GetParent(ctx context.Context, b *bead.Bead) (*bead.Bead, error)
	Create(ctx context.Context, title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error)
	CreateWithParent(ctx context.Context, title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error)
	CreateWithParentAndDescription(ctx context.Context, title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error)
	HasOpenChildren(ctx context.Context, parentID string) (bool, error)
}

// FailureAnalyzer abstracts the failure analysis operations used by the runner.
type FailureAnalyzer interface {
	Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error)
}

// PromptRenderer abstracts the prompt rendering operations used by the runner.
type PromptRenderer interface {
	BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string, phase string) (*prompt.Context, error)
	RenderBuild(ctx *prompt.Context) (string, error)
	RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error)
	RenderLearn(ctx *prompt.LearnContext) (string, error)
	RenderDecompose(ctx *prompt.DecomposeContext) (string, error)
	RenderScope(ctx *prompt.ScopeContext) (string, error)
	RenderPrecheck(ctx *prompt.PrecheckContext) (string, error)
	RenderSpecAcceptance(ctx *prompt.SpecAcceptanceContext) (string, error)
	RenderSpecGate(ctx *prompt.SpecGateContext) (string, error)
	RenderReview(ctx *prompt.ReviewContext) (string, error)
	RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error)
	RenderAcceptanceTests(ctx *prompt.Context) (string, error)
	RenderATDDBuild(ctx *prompt.Context) (string, error)
	RenderATDDDiagnostic(ctx *prompt.DiagnosticContext) (string, error)
	RenderTDDBuild(ctx *prompt.Context) (string, error)
	RenderTDDRed(ctx *prompt.TDDRedContext) (string, error)
	RenderTDDGreen(ctx *prompt.TDDGreenContext) (string, error)
	RenderRefactor(ctx *prompt.Context) (string, error)
	RenderTestFix(ctx *prompt.TestFixContext) (string, error)
	RenderCoverageValidation(ctx *prompt.CoverageValidationContext) (string, error)
	LoadSpec(name string) (string, error)
	LoadClaudeMD() (string, error)
	LoadRules() (string, error)
	LoadRulesForPhase(phase string) (string, error)
	GetLearningsFile() *learnings.File
	SetSiblingTouchedPackagesResolver(resolver prompt.SiblingTouchedPackagesResolver)
	LastDiagnostics() *prompt.PromptDiagnostics
}

// IterationLogger abstracts the iteration log writing used by the runner.
type IterationLogger interface {
	LogIteration(log *logger.IterationLog) error
	LogReview(log *logger.ReviewLog) error
	Close() error
	FilePath() string
	RunID() string
}

// WorktreeManager abstracts the worktree lifecycle used by interactive commands.
type WorktreeManager interface {
	EnsureWorktree(ctx context.Context) (string, error)
	CreateBranch(ctx context.Context, command string) (string, error)
	MergeBack(ctx context.Context, branch string) error
	PendingBranches(ctx context.Context) ([]string, error)
	RemoveByPath(ctx context.Context, path string) error
	Cleanup(ctx context.Context) error
	PruneStaleSessionWorktrees(ctx context.Context) (int, error)
}

// BranchRouter abstracts branch selection based on bead labels.
type BranchRouter interface {
	BranchForLabels(labels []string) (string, error)
}

// GitCheckout abstracts git branch checkout operations.
type GitCheckout interface {
	CreateOrCheckoutSpecBranch(ctx context.Context, specBranchName string) error
	RevertAndReturnToBase(ctx context.Context) error
}

// PreflightChecker checks environment preconditions before the run loop starts.
type PreflightChecker interface {
	EnsureWorktreeClean(ctx context.Context) error
}

// IterationResult captures the outcome of one loop iteration.
// Type alias for backward compatibility — canonical definition is in runtypes.
type IterationResult = runtypes.IterationResult

// SubTask represents a single sub-task from task decomposition.
// Type alias for backward compatibility — canonical definition is in runtypes.
type SubTask = runtypes.SubTask

// ArgvRunnerFn runs an executable with argv and returns output plus exit metadata.
// Type alias for backward compatibility — canonical definition is in runtypes.
type ArgvRunnerFn = runtypes.ArgvRunnerFn
