//go:build acceptance

package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

func setupInvocationTimeoutRunner(t *testing.T, cfg *config.Config, p provider.Provider) (*Runner, *mockIterationLogger) {
	t.Helper()

	beadReady := false
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadReady {
				return nil, nil
			}
			beadReady = true
			return &bead.Bead{ID: "bead-timeout-1", Title: "Timeout bead", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	mockLog := &mockIterationLogger{}
	var buf strings.Builder
	router := provider.NewSingleProviderRouter(p)
	r, err := NewRunnerWithDeps(
		cfg,
		&buf,
		t.TempDir(),
		Deps{
			Beads:    beads,
			Router:   router,
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   mockLog,
		},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	return r, mockLog
}

// smoke-matrix: keep | rationale: Retains high-value E2E failure-path coverage for timeout-triggered retry with tier escalation behavior. | destination: internal/runner/invocation_timeout_acceptance_test.go:TestRunnerSmoke_ValidationFailureEscalatesTier
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
