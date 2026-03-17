package stages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/next/contract"
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
	EvidenceDir   string
}

// ValidateStage runs final validation checks.
type ValidateStage struct {
	validator         FinalValidator
	contractEvaluator contract.ContractEvaluator
	cfg               ValidateStageConfig
	eventLog          *runstore.EventLog
}

// NewValidateStage creates a new ValidateStage. An optional ContractEvaluator may be
// provided; if nil, contract checking is skipped.
func NewValidateStage(v FinalValidator, cfg ValidateStageConfig, eventLog *runstore.EventLog, evaluator contract.ContractEvaluator) *ValidateStage {
	return &ValidateStage{validator: v, cfg: cfg, eventLog: eventLog, contractEvaluator: evaluator}
}

// Name returns the stage name.
func (s *ValidateStage) Name() string { return "validate" }

// Run executes final validation.
func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	workDir := s.cfg.WorkDir
	if rs.WorktreePath != "" {
		workDir = rs.WorktreePath
	}

	// Collect contract failures first (if EvidenceDir is configured and file exists).
	var failures []string
	if s.cfg.EvidenceDir != "" && s.contractEvaluator != nil {
		contractPath := filepath.Join(s.cfg.EvidenceDir, "scenario-contracts.yaml")
		data, err := os.ReadFile(contractPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return specloop.NextAction{}, fmt.Errorf("read scenario-contracts.yaml: %w", err)
		}
		if err == nil {
			sc, err := contract.ParseContractYAML(string(data))
			if err != nil {
				return specloop.NextAction{}, fmt.Errorf("parse scenario-contracts.yaml: %w", err)
			}
			contractFailures, err := s.contractEvaluator.Evaluate(ctx, &sc, workDir)
			if err != nil {
				return specloop.NextAction{}, fmt.Errorf("evaluate contracts: %w", err)
			}
			for _, f := range contractFailures {
				failures = append(failures, fmt.Sprintf("contract:%s — %s failed: %s", f.ScenarioName, f.AssertionType, f.Details))
			}
		}
	}

	// Run shell checks regardless of contract results.
	result, err := s.validator.RunFinal(ctx, s.cfg.AlwaysRun, s.cfg.ProjectChecks, workDir)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("final validation: %w", err)
	}

	// Collect shell check failures.
	for _, cr := range result.AlwaysRun.FailedChecks() {
		failures = append(failures, fmt.Sprintf("always-run check %q failed: %s", cr.Name, cr.Output))
	}
	for _, cr := range result.ProjectChecks.FailedChecks() {
		failures = append(failures, fmt.Sprintf("project check %q failed: %s", cr.Name, cr.Output))
	}

	// Determine final validation status after collecting ALL failures (contract + shell).
	finalPassed := len(failures) == 0 && result.Pass

	// Patch the result to reflect the actual final status (contract failures may
	// have turned a shell-passing run into an overall failure).
	result.Pass = finalPassed
	rs.LastFinalValidation = &result

	// Store validation result summary reflecting actual final status.
	validationSummary := fmt.Sprintf("pass=%v", finalPassed)
	rs.LastValidationResult = &validationSummary

	// Emit final_validation_result event after all failures are collected.
	if s.eventLog != nil {
		s.eventLog.Append(runstore.FinalValidationResultEvent{
			BaseEvent: runstore.BaseEvent{Type: "final_validation_result", Timestamp: time.Now()},
			Passed:    finalPassed,
		})
	}

	if finalPassed {
		rs.FinalValidationPassed = true
		return specloop.NextAction{Kind: specloop.Continue}, nil
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
