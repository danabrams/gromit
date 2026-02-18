package runner

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func spinForAtLeast(d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
	}
}

func newTestClaudeConfig() *config.Config {
	return &config.Config{
		Claude: config.ClaudeConfig{
			Timeout:            600,
			StallTimeout:       120,
			StallTimeoutActive: 300,
			BeadTimeout:        1200,
			AnalysisTimeout:    30,
		},
	}
}

func newRunnerForRateLimitTest(cfg *config.Config, mockClaude *mockClaudeClient, streamLogger *logger.StreamLogger) *Runner {
	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	return &Runner{
		cfg:          cfg,
		router:       mockRouter,
		invoker:      newInvokerForTest(mockRouter, nil, streamLogger),
		streamLogger: streamLogger,
	}
}

func newRateLimitBeadContext(id string) *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          id,
			Title:       "Test bead",
			Description: "Test description",
		},
		Result:      &IterationResult{BeadID: id, Model: "sonnet"},
		Model:       "sonnet",
		BuildPrompt: "test prompt",
	}
}

// TestExecuteClaudeInvocation_CapturesRateLimitRecoveryMs verifies that
// executeClaudeInvocation() captures RateLimitRecoveryMs from DiagnosticSnapshot()
// and stores it in the IterationResult.
func TestExecuteClaudeInvocation_CapturesRateLimitRecoveryMs(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			// Simulate rate limit recovery by calling the handler with events
			if handler != nil {
				// Simulate rate limit error event
				rateLimitEvent := []byte(`{"type":"error","subtype":"overloaded"}`)
				handler(rateLimitEvent)

				// Ensure recovery is measured across multiple millisecond boundaries.
				spinForAtLeast(5 * time.Millisecond)

				// Simulate recovery event
				normalEvent := []byte(`{"type":"system","subtype":"init"}`)
				handler(normalEvent)
			}

			return &claude.Result{
				Success: true,
				Output:  "implementation complete",
			}, nil
		},
	}

	cfg := newTestClaudeConfig()

	// Create StreamLogger to enable handler in executeClaudeInvocation
	dir := t.TempDir()
	sl, err := logger.NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("failed to create StreamLogger: %v", err)
	}
	defer func() {
		if err := sl.Close(); err != nil {
			t.Fatalf("failed to close StreamLogger: %v", err)
		}
	}()

	r := newRunnerForRateLimitTest(cfg, mockClaude, sl)
	bc := newRateLimitBeadContext("test-1")

	ctx := context.Background()
	invResult, err := r.executeClaudeInvocation(ctx, bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation failed: %v", err)
	}

	// Verify stats captured rate limit recovery
	if invResult == nil || invResult.Stats == nil {
		t.Fatal("expected invResult.Stats to be non-nil")
	}
	stats := invResult.Stats

	// Verify DiagnosticSnapshot returns recovery time
	_, _, _, _, _, recoveryMs := stats.DiagnosticSnapshot()
	if recoveryMs == 0 {
		t.Error("expected DiagnosticSnapshot to return non-zero recovery time after rate limit")
	}

	// Verify IterationResult captured the recovery time
	if bc.Result.RateLimitRecoveryMs == 0 {
		t.Error("expected IterationResult.RateLimitRecoveryMs to be set from DiagnosticSnapshot()")
	}

	if bc.Result.RateLimitRecoveryMs < 1 {
		t.Errorf("expected RateLimitRecoveryMs >= 1ms after boundary crossing, got %d ms", bc.Result.RateLimitRecoveryMs)
	}

	// Verify it matches what DiagnosticSnapshot returned
	if bc.Result.RateLimitRecoveryMs != recoveryMs {
		t.Errorf("expected IterationResult.RateLimitRecoveryMs (%d) to match DiagnosticSnapshot return value (%d)",
			bc.Result.RateLimitRecoveryMs, recoveryMs)
	}
}

// TestExecuteClaudeInvocation_ZeroRecoveryMsWhenNoRateLimit verifies that
// RateLimitRecoveryMs is zero when no rate limit occurs during execution.
func TestExecuteClaudeInvocation_ZeroRecoveryMsWhenNoRateLimit(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			// Simulate normal events without rate limiting
			if handler != nil {
				normalEvent := []byte(`{"type":"system","subtype":"init"}`)
				handler(normalEvent)
			}

			return &claude.Result{
				Success: true,
				Output:  "implementation complete",
			}, nil
		},
	}

	cfg := newTestClaudeConfig()
	r := newRunnerForRateLimitTest(cfg, mockClaude, nil)
	bc := newRateLimitBeadContext("test-2")

	ctx := context.Background()
	_, err := r.executeClaudeInvocation(ctx, bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation failed: %v", err)
	}

	// Verify RateLimitRecoveryMs is zero when no rate limit occurred
	if bc.Result.RateLimitRecoveryMs != 0 {
		t.Errorf("expected RateLimitRecoveryMs=0 when no rate limit, got %d ms", bc.Result.RateLimitRecoveryMs)
	}
}
