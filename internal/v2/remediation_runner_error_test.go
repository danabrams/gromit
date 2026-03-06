package v2

import (
    "context"
    "errors"
    "testing"
)

func TestRemediationRunnerRunSpecIDRequired(t *testing.T) {
	t.Parallel()

	runner := NewRemediationRunner(RemediationRunnerConfig{})
	err := runner.Run(context.Background(), "")
	if !errors.Is(err, ErrSpecIDRequired) {
		t.Fatalf("expected ErrSpecIDRequired, got %v", err)
	}
}

func TestRemediationRunnerRunAcceptStageRequired(t *testing.T) {
	t.Parallel()

	runner := NewRemediationRunner(RemediationRunnerConfig{})
	err := runner.Run(context.Background(), "spec-id")
	if !errors.Is(err, ErrAcceptStageRequired) {
		t.Fatalf("expected ErrAcceptStageRequired, got %v", err)
	}
}
