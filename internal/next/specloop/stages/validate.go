package stages

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// FinalValidator abstracts final validation for testability.
type FinalValidator interface {
	RunFinal(ctx context.Context, alwaysRun []validator.Check, projectChecks []validator.Check, workDir string) (validator.FinalResult, error)
}

// ValidateStageConfig configures the ValidateStage.
type ValidateStageConfig struct {
	AlwaysRun     []validator.Check
	ProjectChecks []validator.Check
	WorkDir       string
}

// ValidateStage runs final validation checks.
type ValidateStage struct {
	validator FinalValidator
	cfg       ValidateStageConfig
}

// NewValidateStage creates a new ValidateStage.
func NewValidateStage(v FinalValidator, cfg ValidateStageConfig) *ValidateStage {
	return &ValidateStage{validator: v, cfg: cfg}
}

// Name returns the stage name.
func (s *ValidateStage) Name() string { return "validate" }

// Run executes final validation.
func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	result, err := s.validator.RunFinal(ctx, s.cfg.AlwaysRun, s.cfg.ProjectChecks, s.cfg.WorkDir)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("final validation: %w", err)
	}

	if result.Pass {
		rs.FinalValidationPassed = true
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}

	// Collect failure details
	var failures []string
	for _, cr := range result.AlwaysRun.FailedChecks() {
		failures = append(failures, fmt.Sprintf("always-run check %q failed: %s", cr.Name, cr.Output))
	}
	for _, cr := range result.ProjectChecks.FailedChecks() {
		failures = append(failures, fmt.Sprintf("project check %q failed: %s", cr.Name, cr.Output))
	}
	if len(failures) == 0 {
		failures = []string{"validation failed"}
	}

	return specloop.NextAction{
		Kind: specloop.ReplanFrom,
		Context: &specloop.FailureContext{
			Failures: failures,
			Cycle:    rs.Cycle,
		},
	}, nil
}
