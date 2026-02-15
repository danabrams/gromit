//go:build acceptance

package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
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

func TestRunnerInvocationTimeout_UsesClaudeTimeout(t *testing.T) {
	_ = config.DefaultInvocationTimeoutSeconds

	var observed time.Duration
	provider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				return nil, fmt.Errorf("missing invocation deadline")
			}
			observed = time.Until(deadline)
			return &provider.Result{Success: true, Model: "test-sonnet", Output: "ok"}, nil
		},
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Timeout:            2,
			StallTimeout:       1,
			StallTimeoutActive: 1,
			BeadTimeout:        30,
		},
		Validation: config.ValidationConfig{Enabled: false},
		Review:     config.ReviewConfig{Enabled: false},
	}

	r, _ := setupInvocationTimeoutRunner(t, cfg, provider)
	if err := r.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if observed == 0 {
		t.Fatal("expected StreamRun to observe an invocation deadline")
	}
	if observed < time.Second || observed > 5*time.Second {
		t.Fatalf("invocation deadline = %v, want ~2s", observed)
	}
}

func TestRunnerInvocationTimeout_RespectsModelOverride(t *testing.T) {
	_ = config.DefaultInvocationTimeoutSeconds

	var observed time.Duration
	provider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				return nil, fmt.Errorf("missing invocation deadline")
			}
			observed = time.Until(deadline)
			return &provider.Result{Success: true, Model: "test-sonnet", Output: "ok"}, nil
		},
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Timeout:            2,
			StallTimeout:       1,
			StallTimeoutActive: 1,
			BeadTimeout:        30,
			ModelTimeouts: map[string]config.ModelTimeoutOverrides{
				"test-sonnet": {Timeout: 4},
			},
		},
		Validation: config.ValidationConfig{Enabled: false},
		Review:     config.ReviewConfig{Enabled: false},
	}

	r, _ := setupInvocationTimeoutRunner(t, cfg, provider)
	if err := r.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if observed == 0 {
		t.Fatal("expected StreamRun to observe an invocation deadline")
	}
	if observed < 3*time.Second || observed > 6*time.Second {
		t.Fatalf("invocation deadline = %v, want ~4s override", observed)
	}
}

func TestRunnerLogsPhaseTimeoutWithElapsedDuration(t *testing.T) {
	_ = config.DefaultInvocationTimeoutSeconds

	provider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			<-ctx.Done()
			return &provider.Result{Success: false, Model: "test-sonnet", Output: "timeout"}, ctx.Err()
		},
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Timeout:            1,
			StallTimeout:       10,
			StallTimeoutActive: 10,
			BeadTimeout:        2,
		},
		Validation: config.ValidationConfig{Enabled: false},
		Review:     config.ReviewConfig{Enabled: false},
	}

	r, log := setupInvocationTimeoutRunner(t, cfg, provider)
	if err := r.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if len(log.Logs) != 1 {
		t.Fatalf("expected 1 iteration log, got %d", len(log.Logs))
	}
	entry := log.Logs[0]
	if entry.TimeoutType != runtypes.TimeoutTypePhase {
		t.Fatalf("TimeoutType = %q, want %q", entry.TimeoutType, runtypes.TimeoutTypePhase)
	}
	if entry.DurationMs <= 0 || entry.DurationMs > 1500 {
		t.Fatalf("DurationMs = %d, want elapsed <= 1500ms", entry.DurationMs)
	}
}

func TestRunnerInvocationTimeout_DefaultTimeoutApplied(t *testing.T) {
	var observed time.Duration
	provider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				return nil, fmt.Errorf("missing invocation deadline")
			}
			observed = time.Until(deadline)
			return &provider.Result{Success: true, Model: "test-sonnet", Output: "ok"}, nil
		},
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0644); err != nil {
		t.Fatalf("writing empty config: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	cfg.Validation = config.ValidationConfig{Enabled: false}
	cfg.Review = config.ReviewConfig{Enabled: false}

	r, _ := setupInvocationTimeoutRunner(t, cfg, provider)
	if err := r.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if observed == 0 {
		t.Fatal("expected StreamRun to observe an invocation deadline")
	}
	expected := time.Duration(config.DefaultInvocationTimeoutSeconds) * time.Second
	if observed < expected-30*time.Second || observed > expected+30*time.Second {
		t.Fatalf("invocation deadline = %v, want ~%v", observed, expected)
	}
}

func TestRunnerInvocationTimeout_DeadlineTriggersEscalation(t *testing.T) {
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
