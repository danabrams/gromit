package runner

import (
	"context"
	"io"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
)

// Compile-time interface satisfaction checks.
var (
	_ BeadClient      = (*bead.Client)(nil)
	_ ClaudeClient    = (*claude.Client)(nil)
	_ FailureAnalyzer = (*analyzer.Analyzer)(nil)
	_ PromptRenderer  = (*prompt.Renderer)(nil)
	_ IterationLogger = (*logger.Logger)(nil)
)

// BeadClient abstracts the bead (bd) CLI operations used by the runner.
type BeadClient interface {
	Ready() (*bead.Bead, error)
	Show(id string) (*bead.Bead, error)
	Close(id string) error
	Sync() error
	AddComment(id, comment string) error
	GetParent(b *bead.Bead) (*bead.Bead, error)
	CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error)
	CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error)
	HasOpenChildren(parentID string) (bool, error)
}

// ClaudeClient abstracts the Claude CLI operations used by the runner.
type ClaudeClient interface {
	Run(ctx context.Context, prompt string, model string) (*claude.Result, error)
	StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error)
	RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error)
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
	RenderReview(ctx *prompt.ReviewContext) (string, error)
	RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error)
	RenderAcceptanceTests(ctx *prompt.Context) (string, error)
	RenderATDDBuild(ctx *prompt.Context) (string, error)
	RenderTDDBuild(ctx *prompt.Context) (string, error)
	RenderRefactor(ctx *prompt.Context) (string, error)
	LoadSpec(name string) (string, error)
	LoadClaudeMD() (string, error)
	LoadRules() (string, error)
	GetLearningsFile() *learnings.File
}

// IterationLogger abstracts the iteration log writing used by the runner.
type IterationLogger interface {
	LogIteration(log *logger.IterationLog) error
	LogReview(log *logger.ReviewLog) error
	Close() error
	FilePath() string
}
