package stages

import (
	"context"
	"fmt"
	"time"

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
	eventLog  *runstore.EventLog
}

// NewValidateStage creates a new ValidateStage.
func NewValidateStage(v FinalValidator, cfg ValidateStageConfig, eventLog *runstore.EventLog) *ValidateStage {
	return &ValidateStage{validator: v, cfg: cfg, eventLog: eventLog}
}

// Name returns the stage name.
func (s *ValidateStage) Name() string { return "validate" }

// Run executes final validation.
func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	result, err := s.validator.RunFinal(ctx, s.cfg.AlwaysRun, s.cfg.ProjectChecks, s.cfg.WorkDir)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("final validation: %w", err)
	}

	// Store validation result summary in RunState for EvidenceStage (L3)
	validationSummary := fmt.Sprintf("pass=%v", result.Pass)
	rs.LastValidationResult = &validationSummary
	rs.LastFinalValidation = &result

	// Emit final_validation_result event
	if s.eventLog != nil {
		s.eventLog.Append(runstore.FinalValidationResultEvent{
			BaseEvent: runstore.BaseEvent{Type: "final_validation_result", Timestamp: time.Now()},
			Passed:    result.Pass,
		})
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
