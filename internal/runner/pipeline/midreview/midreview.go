package midreview

import (
    "context"
    "io"

    "github.com/danabrams/gromit/internal/events"
    "github.com/danabrams/gromit/internal/pipeline"
)

// Stage implements the mid-review pipeline stage that runs between build and validate.
type Stage struct {
    events.EmitterMixin
    renderer PromptRenderer
    invoker  Invoker
    gitDiff  GitDiffFn
    output   io.Writer
}

// PromptRenderer renders the mid-build review prompt.
type PromptRenderer interface {
    RenderMidBuildReview(ctx *MidBuildReviewContext) (string, error)
}

// MidBuildReviewContext represents the data required to render the mid-review prompt.
type MidBuildReviewContext struct{}

// Invoker executes provider invocations for mid-review.
type Invoker interface {
    StreamRun(ctx context.Context, prompt, tier string, w io.Writer, handler interface{}, onToolCall interface{}) (interface{}, error)
}

// GitDiffFn returns the current git diff used by the mid-review prompt.
type GitDiffFn func(ctx context.Context) (string, error)

// NewStage creates a new mid-review Stage.
func NewStage(renderer PromptRenderer, invoker Invoker, gitDiff GitDiffFn, output io.Writer) *Stage {
    return &Stage{
        renderer: renderer,
        invoker:  invoker,
        gitDiff:  gitDiff,
        output:   output,
    }
}

// Run executes the mid-review stage. It currently only checks whether the stage is enabled.
func (s *Stage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
    cfg := in.Config
    if cfg == nil || !cfg.MidBuildReview.Enabled {
        return pipeline.Output{Decision: pipeline.Proceed}, nil
    }
    return pipeline.Output{Decision: pipeline.Proceed}, nil
}
