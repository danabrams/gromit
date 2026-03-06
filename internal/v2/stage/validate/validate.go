package validate

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagesvalidate "github.com/danabrams/gromit/internal/v2/stages/validate"
)

// ValidationRunner executes a single validation command.
type ValidationRunner interface {
	Run(ctx context.Context, command string) error
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
	return &Stage{name: stagesvalidate.Describe(cfg), runner: runner, cfg: cfg}, nil
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

	commands := cfg.EffectiveValidationCommands()
	snapshot := append([]string(nil), commands...)
	for _, cmd := range commands {
		if err := s.runner.Run(ctx, cmd); err != nil {
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
