package accept

import (
    "context"
    "fmt"

    "github.com/danabrams/gromit/internal/config"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    "github.com/danabrams/gromit/internal/v2/adapter"
    "github.com/danabrams/gromit/internal/v2/adapter/llm"
    stagesdesc "github.com/danabrams/gromit/internal/v2/stages/accept"
)

// Stage evaluates acceptance criteria against the current worktree.
type Stage struct {
    name     string
    cfg      *config.Config
    git      adapter.GitAdapter
    llm      llm.LLMProvider
    base     string
    project  string
    fragment string
}

// New constructs an accept stage with the provided dependencies.
func New(cfg *config.Config, git adapter.GitAdapter, provider llm.LLMProvider, base, project, fragment string) (*Stage, error) {
    if cfg == nil {
        return nil, fmt.Errorf("config required")
    }
    if git == nil {
        return nil, fmt.Errorf("git adapter required")
    }
    if provider == nil {
        return nil, fmt.Errorf("llm provider required")
    }
    return &Stage{
        name:     stagesdesc.Describe(cfg),
        cfg:      cfg,
        git:      git,
        llm:      provider,
        base:     base,
        project:  project,
        fragment: fragment,
    }, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the canonical accept stage identifier.
func (s *Stage) Name() string {
    if s == nil {
        return ""
    }
    return s.name
}

// Run implements the acceptance evaluation stage (not yet implemented).
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
    return nil, fmt.Errorf("accept stage not implemented")
}
