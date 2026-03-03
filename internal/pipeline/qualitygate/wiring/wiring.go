package wiring

import (
    "context"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/pipeline"
)

// GitDiffFn returns the current git diff for wiring analysis.
type GitDiffFn func(ctx context.Context) (string, error)

// Gate enforces wiring checks for extracted symbols.
type Gate struct {
    gitDiff GitDiffFn
}

// Ensure Gate implements pipeline.Stage.
var _ pipeline.Stage = (*Gate)(nil)

// New creates a wiring gate stage. gitDiff may be nil when the gate is disabled.
func New(gitDiff GitDiffFn) *Gate {
    if gitDiff == nil {
        gitDiff = func(context.Context) (string, error) { return "", nil }
    }
    return &Gate{gitDiff: gitDiff}
}

// Run checks whether the wiring gate is enabled and skips when it's disabled.
func (g *Gate) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
    if !isEnabled(in.Config) {
        return pipeline.Output{Decision: pipeline.Proceed}, nil
    }
    return pipeline.Output{Decision: pipeline.Proceed}, nil
}

func isEnabled(cfg *config.Config) bool {
    if cfg == nil {
        return false
    }
    return cfg.WiringGate.Enabled
}
