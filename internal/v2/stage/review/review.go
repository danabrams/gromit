package review

import (
    "context"
    "fmt"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/events"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    stagesreview "github.com/danabrams/gromit/internal/v2/stages/review"
)

// GitDiffFn returns the git diff for the current worktree.
type GitDiffFn func(ctx context.Context, worktree string) (string, error)

// Stage implements the review stage of the v2 loop.
type Stage struct {
    name     string
    cfg      *config.Config
    gitDiff  GitDiffFn
    llm      interface{}
    tracker  interface{}
    base     string
    project  string
    fragment string
    events.EmitterMixin
}

// New constructs a review stage backed by the provided configuration and helpers.
func New(cfg *config.Config, gitDiff GitDiffFn, llm interface{}, tracker interface{}, base, project, fragment string) (*Stage, error) {
    if cfg == nil {
        return nil, fmt.Errorf("config required")
    }
    if gitDiff == nil {
        return nil, fmt.Errorf("git diff function required")
    }
    name := stagesreview.Describe(cfg)
    return &Stage{name: name, cfg: cfg, gitDiff: gitDiff, llm: llm, tracker: tracker, base: base, project: project, fragment: fragment}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the canonical stage identifier.
func (s *Stage) Name() string {
    return s.name
}

// Run executes the review stage.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
    if req == nil {
        return nil, fmt.Errorf("request required")
    }
    cfg := s.cfg
    if req.Config != nil {
        cfg = req.Config
    }
    if cfg == nil {
        return nil, fmt.Errorf("config required")
    }
    if !cfg.Review.Enabled {
        return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
    }
    return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}
