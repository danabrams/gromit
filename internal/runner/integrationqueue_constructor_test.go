package runner

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/integrationqueue"
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

func TestNewIntegrationQueueScopedGateEvaluator_RunsValidationCommands(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Validation.Enabled = true
	cfg.Validation.FastCommands = []string{"run"}

	var seenWorkDir string
	successRunner := func(ctx context.Context, command, workDir string) (string, string, int, error) {
		seenWorkDir = workDir
		if command != "run" {
			t.Fatalf("command = %q, want %q", command, "run")
		}
		return "ok", "", 0, nil
	}

	evaluator := newIntegrationQueueScopedGateEvaluator(cfg, "/repo", successRunner)
	if evaluator == nil {
		t.Fatal("expected evaluator when validation is configured")
	}

	entry := integrationqueue.Entry{Branch: "feature/gate"}
	if err := evaluator(context.Background(), entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenWorkDir != "/repo" {
		t.Fatalf("workDir = %q, want %q", seenWorkDir, "/repo")
	}

	failureRunner := func(ctx context.Context, command, workDir string) (string, string, int, error) {
		return "", "fail", 1, nil
	}
	failureEvaluator := newIntegrationQueueScopedGateEvaluator(cfg, "/repo", failureRunner)
	if failureEvaluator == nil {
		t.Fatal("expected evaluator for failure branch")
	}
	if err := failureEvaluator(context.Background(), entry); err == nil {
		t.Fatal("expected error when validation command fails")
	}
}
