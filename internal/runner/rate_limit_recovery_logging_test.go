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
)

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

				// Wait to simulate recovery time
				time.Sleep(120 * time.Millisecond)

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

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Timeout:            600,
			StallTimeout:       120,
			StallTimeoutActive: 300,
			BeadTimeout:        1200,
			AnalysisTimeout:    30,
		},
	}

	// Create StreamLogger to enable handler in executeClaudeInvocation
	dir := t.TempDir()
	sl, err := logger.NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("failed to create StreamLogger: %v", err)
	}
	defer sl.Close()

	r := &Runner{
		cfg:          cfg,
		claude:       mockClaude,
		streamLogger: sl,
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:          "test-1",
			Title:       "Test bead",
			Description: "Test description",
		},
		result:      &IterationResult{BeadID: "test-1", Model: "sonnet"},
		model:       "sonnet",
		buildPrompt: "test prompt",
	}

	ctx := context.Background()
	_, stats, _, err := r.executeClaudeInvocation(ctx, bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation failed: %v", err)
	}

	// Verify stats captured rate limit recovery
	if stats == nil {
		t.Fatal("expected stats to be non-nil")
	}

	// Verify DiagnosticSnapshot returns recovery time
	_, _, _, _, _, recoveryMs := stats.DiagnosticSnapshot()
	if recoveryMs == 0 {
		t.Error("expected DiagnosticSnapshot to return non-zero recovery time after rate limit")
	}

	// Verify IterationResult captured the recovery time
	if bc.result.RateLimitRecoveryMs == 0 {
		t.Error("expected IterationResult.RateLimitRecoveryMs to be set from DiagnosticSnapshot()")
	}

	if bc.result.RateLimitRecoveryMs < 100 {
		t.Errorf("expected RateLimitRecoveryMs >= 100ms based on sleep, got %d ms", bc.result.RateLimitRecoveryMs)
	}

	// Verify it matches what DiagnosticSnapshot returned
	if bc.result.RateLimitRecoveryMs != recoveryMs {
		t.Errorf("expected IterationResult.RateLimitRecoveryMs (%d) to match DiagnosticSnapshot return value (%d)",
			bc.result.RateLimitRecoveryMs, recoveryMs)
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

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Timeout:            600,
			StallTimeout:       120,
			StallTimeoutActive: 300,
			BeadTimeout:        1200,
			AnalysisTimeout:    30,
		},
	}

	r := &Runner{
		cfg:    cfg,
		claude: mockClaude,
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:          "test-2",
			Title:       "Test bead",
			Description: "Test description",
		},
		result:      &IterationResult{BeadID: "test-2", Model: "sonnet"},
		model:       "sonnet",
		buildPrompt: "test prompt",
	}

	ctx := context.Background()
	_, _, _, err := r.executeClaudeInvocation(ctx, bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation failed: %v", err)
	}

	// Verify RateLimitRecoveryMs is zero when no rate limit occurred
	if bc.result.RateLimitRecoveryMs != 0 {
		t.Errorf("expected RateLimitRecoveryMs=0 when no rate limit, got %d ms", bc.result.RateLimitRecoveryMs)
	}
}
