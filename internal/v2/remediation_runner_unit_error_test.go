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
