package v2

import (
    "context"
    "errors"
    "testing"
)

func TestRemediationRunnerRunMissingSpecID(t *testing.T) {
	runner := runnerForSpecIDValidation()
	if err := runner.Run(context.Background(), ""); !errors.Is(err, ErrSpecIDRequired) {
		t.Fatalf("expected ErrSpecIDRequired, got %v", err)
	}
}

func TestRemediationRunnerRunMissingAcceptStage(t *testing.T) {
	runner := runnerForAcceptStageValidation()
	if err := runner.Run(context.Background(), "spec"); !errors.Is(err, ErrAcceptStageRequired) {
		t.Fatalf("expected ErrAcceptStageRequired, got %v", err)
	}
}

func runnerForSpecIDValidation() *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{})
}
