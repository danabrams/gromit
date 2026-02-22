//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner"
)

// smoke-matrix: keep | rationale: Retains high-value E2E failure-path coverage for timeout-triggered retry with tier escalation behavior. | destination: internal/runner/acceptance/invocation_timeout_acceptance_test.go:TestRunnerSmoke_ValidationFailureEscalatesTier
func TestRunnerSmoke_ValidationFailureEscalatesTier(t *testing.T) {
	callTiers := make(chan string, 3)
	callCount := 0
	mockInvoker := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			callCount++
			callTiers <- tier
			if callCount == 1 {
				return &provider.Result{Success: false, Model: "test-sonnet", Output: "timeout"}, context.DeadlineExceeded
			}
			return &provider.Result{Success: true, Model: "test-opus", Output: "ok"}, nil
		},
	}

	cfg := &config.Config{
		Models: config.ModelsConfig{
			P1: "sonnet",
		},
		Escalation: config.EscalationConfig{
			Enabled: true,
			Chain:   []string{"sonnet", "opus"},
		},
		Validation: config.ValidationConfig{Enabled: false},
		Review:     config.ReviewConfig{Enabled: false},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	beadReady := false
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadReady {
				return nil, nil
			}
			beadReady = true
			return &bead.Bead{
				ID:       "escalation-test-1",
				Title:    "Escalation test bead",
				Priority: 1,
			}, nil
		},
	}

	buildStage := execute.New(mockInvoker, &mockExecuteRenderer{}, io.Discard)
	orch := runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate:     &noopStage{},
		Build:    buildStage,
		Validate: &noopStage{},
		Epilogue: &noopStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return beads.Ready()
		},
		Config: cfg,
		Output: io.Discard,
	})

	if err := orch.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	close(callTiers)

	var tiers []string
	for tier := range callTiers {
		tiers = append(tiers, tier)
	}

	if callCount != 2 {
		t.Fatalf("expected exactly 2 invocations (initial + retry), got %d", callCount)
	}
	if len(tiers) != 2 {
		t.Fatalf("expected exactly 2 recorded tiers, got %d (%v)", len(tiers), tiers)
	}
	if tiers[0] != provider.TierMedium || tiers[1] != provider.TierHigh {
		t.Fatalf("invocation tiers = %v, want [%s %s]", tiers, provider.TierMedium, provider.TierHigh)
	}
}

// mockExecuteRenderer is a minimal execute.PromptRenderer for testing.
type mockExecuteRenderer struct{}

func (m *mockExecuteRenderer) RenderBuild(title, description string, validationFailures []string) (string, error) {
	return "test prompt", nil
}

func (m *mockExecuteRenderer) RenderTDDBuild(title, description string, validationFailures []string) (string, error) {
	return "test tdd prompt", nil
}

func (m *mockExecuteRenderer) RenderRefactorBuild(title, description string, validationFailures []string) (string, error) {
	return "test refactor prompt", nil
}
