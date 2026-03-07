package validate

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

// ValidationRunner executes a single validation command.
type ValidationRunner interface {
	Run(ctx context.Context, command, worktree string) error
}

// ValidateArtifacts capture failure details when validation commands do not all succeed.
type ValidateArtifacts struct {
	Commands      []string
	FailedCommand string
	FailureError  error
}

// Stage enforces project validation commands before proceeding.
type Stage struct {
	name   string
	runner ValidationRunner
	cfg    *config.Config
}

// New creates a validation stage backed by the provided config and runner.
func New(cfg *config.Config, runner ValidationRunner) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if runner == nil {
		return nil, fmt.Errorf("validation runner required")
	}
	return &Stage{name: stagedesc.Describe("validate", cfg), runner: runner, cfg: cfg}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the canonical stage identifier.
func (s *Stage) Name() string {
	return s.name
}

// Run executes the configured validation commands sequentially.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}

	cfg, err := s.config(req)
	if err != nil {
		return nil, err
	}

	commands := cfg.Validation.FastCommandsOrDefault()
	if len(commands) == 0 {
		commands = cfg.EffectiveValidationCommands()
	}
	snapshot := append([]string(nil), commands...)
	worktree := strings.TrimSpace(req.Worktree)
	for _, cmd := range commands {
		if err := s.runner.Run(ctx, cmd, worktree); err != nil {
			return &stagepkg.Result{
				Decision: stagepkg.DecisionFail,
				Artifacts: &ValidateArtifacts{
					Commands:      snapshot,
					FailedCommand: cmd,
					FailureError:  err,
				},
			}, nil
		}
	}

	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

func (s *Stage) config(req *stagepkg.Request) (*config.Config, error) {
	cfg := s.cfg
	if req != nil && req.Config != nil {
		cfg = req.Config
	}
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return cfg, nil
}

func (s *Stage) RetryConfig() stagepkg.RetryConfig {
	if s == nil {
		return stagepkg.RetryConfig{}
	}
	maxRetries := 0
	if s.cfg != nil {
		maxRetries = s.cfg.Validation.MaxValidationRetries
		if maxRetries < 0 {
			maxRetries = 0
		}
	}
	return stagepkg.RetryConfig{
		MaxRetries: maxRetries,
		RetryWith:  []string{stagedesc.Describe("build", s.cfg)},
	}
}
