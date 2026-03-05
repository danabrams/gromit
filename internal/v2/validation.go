package v2

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
)

// ValidationRunner executes a configured validation command.
type ValidationRunner interface {
	Run(ctx context.Context, command string) error
}

// ValidationStage runs the project's validation commands.
type ValidationStage struct {
	runner ValidationRunner
	cfg    *config.Config
}

// NewValidationStage constructs a stage that uses the provided runner/config.
func NewValidationStage(runner ValidationRunner, cfg *config.Config) *ValidationStage {
	return &ValidationStage{runner: runner, cfg: cfg}
}

// Run executes the configured validation commands sequentially.
func (v *ValidationStage) Run(ctx context.Context) error {
	if v.runner == nil {
		return fmt.Errorf("validation runner required")
	}
	if v.cfg == nil {
		return fmt.Errorf("validation config required")
	}

	commands := v.cfg.EffectiveValidationCommands()
	if len(commands) == 0 {
		return fmt.Errorf("no validation commands configured")
	}

	for _, cmd := range commands {
		if err := v.runner.Run(ctx, cmd); err != nil {
			return err
		}
	}

	return nil
}
