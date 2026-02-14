package runner

import (
	"context"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/worktree"
)

// Compile-time interface satisfaction checks.
var (
	_ BeadClient      = (*bead.Client)(nil)
	_ FailureAnalyzer = (*analyzer.Analyzer)(nil)
	_ PromptRenderer  = (*prompt.Renderer)(nil)
	_ IterationLogger = (*logger.Logger)(nil)
	_ WorktreeManager = (*worktree.Manager)(nil)
)

// BeadClient abstracts the bead (bd) CLI operations used by the runner.
type BeadClient interface {
	Ready() (*bead.Bead, error)
	ReadyWithLabel(label string) (*bead.Bead, error)
	ListWithLabel(label string) ([]*bead.Bead, error)
	Show(id string) (*bead.Bead, error)
	Close(id string) error
	Sync() error
	AddComment(id, comment string) error
	GetParent(b *bead.Bead) (*bead.Bead, error)
	CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error)
	CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error)
	HasOpenChildren(parentID string) (bool, error)
}

// FailureAnalyzer abstracts the failure analysis operations used by the runner.
type FailureAnalyzer interface {
	Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error)
}

// PromptRenderer abstracts the prompt rendering operations used by the runner.
type PromptRenderer interface {
	BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error)
	RenderBuild(ctx *prompt.Context) (string, error)
	RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error)
	RenderLearn(ctx *prompt.LearnContext) (string, error)
	RenderDecompose(ctx *prompt.DecomposeContext) (string, error)
	RenderScope(ctx *prompt.ScopeContext) (string, error)
	RenderPrecheck(ctx *prompt.PrecheckContext) (string, error)
	RenderReview(ctx *prompt.ReviewContext) (string, error)
	RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error)
	RenderAcceptanceTests(ctx *prompt.Context) (string, error)
	RenderATDDBuild(ctx *prompt.Context) (string, error)
	RenderTDDBuild(ctx *prompt.Context) (string, error)
	RenderRefactor(ctx *prompt.Context) (string, error)
	LoadSpec(name string) (string, error)
	LoadClaudeMD() (string, error)
	LoadRules() (string, error)
	LoadRulesForPhase(phase string) (string, error)
	GetLearningsFile() *learnings.File
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
	EnsureWorktree() (string, error)
	CreateBranch(command string) (string, error)
	MergeBack(branch string) error
	PendingBranches() ([]string, error)
	Cleanup() error
}
