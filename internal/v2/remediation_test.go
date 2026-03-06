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
