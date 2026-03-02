package runner

import (
    "testing"

    "github.com/danabrams/gromit/internal/config"
)

func TestNewIntegrationQueueScopedGateEvaluator_ReturnsNilWhenValidationDisabled(t *testing.T) {
    cfg := &config.Config{}
    cfg.SetDefaults()
    cfg.Validation.Enabled = false

    evaluator := newIntegrationQueueScopedGateEvaluator(cfg, "/repo", nil)
    if evaluator != nil {
        t.Fatalf("expected nil evaluator when validation disabled, got %T", evaluator)
    }
}
