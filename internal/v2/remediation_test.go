package v2

import (
	"context"
	"errors"
	"testing"
)

func TestRemediationRunnerRun_requiresSpecID(t *testing.T) {
	runner := newRunnerForSpecValidation()

	if err := runner.Run(context.Background(), ""); !errors.Is(err, ErrSpecIDRequired) {
		t.Fatalf("expected ErrSpecIDRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresAcceptStage(t *testing.T) {
	runner := newRunnerWithAcceptStage(nil)

	if err := runner.Run(context.Background(), "spec-id"); !errors.Is(err, ErrAcceptStageRequired) {
		t.Fatalf("expected ErrAcceptStageRequired, got %v", err)
	}
}

func newRunnerForSpecValidation() *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{})
}
