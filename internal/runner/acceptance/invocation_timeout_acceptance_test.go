//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// smoke-matrix: keep | rationale: Retains high-value E2E failure-path coverage for timeout-triggered retry with tier escalation behavior. | destination: internal/runner/acceptance/invocation_timeout_acceptance_test.go:TestRunnerSmoke_ValidationFailureEscalatesTier
func TestRunnerSmoke_ValidationFailureEscalatesTier(t *testing.T) {
	callTiers := make(chan string, 3)
	callCount := 0
	mockProvider := &mockProviderWithRouterTracking{
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
		Claude: config.ClaudeConfig{
			Timeout:            config.DefaultInvocationTimeoutSeconds,
			StallTimeout:       1,
			StallTimeoutActive: 1,
			BeadTimeout:        30,
		},
		Escalation: config.EscalationConfig{
			Enabled: true,
			Chain:   []string{"sonnet", "opus"},
		},
		Validation: config.ValidationConfig{Enabled: false},
		Review:     config.ReviewConfig{Enabled: false},
	}

	r, _ := setupInvocationTimeoutRunner(t, cfg, mockProvider)
	if err := r.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
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
