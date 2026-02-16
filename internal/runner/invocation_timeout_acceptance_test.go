//go:build acceptance

package runner

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

func TestRunnerSmoke_ValidationFailureEscalatesTier(t *testing.T) {
	var tiers []string
	callCount := 0
	mockProvider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			tiers = append(tiers, tier)
			callCount++
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
			ModelTimeouts: map[string]config.ModelTimeoutOverrides{
				"test-sonnet": {Timeout: 1},
			},
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
	if len(tiers) < 2 {
		t.Fatalf("expected retry after phase timeout, got %d invocation(s)", len(tiers))
	}
	if tiers[0] != provider.TierMedium || tiers[1] != provider.TierHigh {
		t.Fatalf("invocation tiers = %v, want [%s %s]", tiers, provider.TierMedium, provider.TierHigh)
	}
}
