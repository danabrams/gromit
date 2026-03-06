package decompose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagesdesc "github.com/danabrams/gromit/internal/v2/stages/decompose"
)

const (
	defaultGromitDir = ".gromit"
	v2DirName        = "v2"
	planFileName     = "plan.md"
)

// Stage implements the decompose stage of the spec loop.
type Stage struct {
	name    string
	cfg     *config.Config
	llm     llm.LLMProvider
	tracker tasktracker.TaskTracker
}

// New constructs a decompose stage backed by the provided dependencies.
func New(cfg *config.Config, provider llm.LLMProvider, tracker tasktracker.TaskTracker) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	if tracker == nil {
		return nil, fmt.Errorf("task tracker required")
	}
	return &Stage{
		name:    stagesdesc.Describe(cfg),
		cfg:     cfg,
		llm:     provider,
		tracker: tracker,
	}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the stage identifier consumed by the loop.
func (s *Stage) Name() string {
	return s.name
}

// Run executes the decompose stage.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	if req.Bead.ID == "" {
		return nil, fmt.Errorf("spec ID required")
	}
	planPath, err := s.planPath(req)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(planPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plan not found: %s", planPath)
		}
		return nil, fmt.Errorf("stat plan file: %w", err)
	}
	return nil, fmt.Errorf("decompose stage not implemented")
}

func (s *Stage) planPath(req *stagepkg.Request) (string, error) {
	cfg := req.Config
	if cfg == nil {
		cfg = s.cfg
	}
	if cfg == nil {
		return "", fmt.Errorf("config required")
	}
	root := cfg.ProjectRoot
	if root == "" {
		root = "."
	}
	gromitDir := cfg.Paths.GromitDir
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	return filepath.Join(root, gromitDir, v2DirName, planFileName), nil
}
