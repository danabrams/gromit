//go:build acceptance

package runner

import (
	"context"
	"encoding/json"
	"io"
	"strings"
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
	// Expected failure: IterationResult.RateLimitRecoveryMs field does not exist yet
	// Expected failure: executeClaudeInvocation() does not capture the 6th return value from DiagnosticSnapshot()
	// Expected failure: DiagnosticSnapshot() does not yet return RateLimitRecoveryMs as the 6th return value

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
		output:       &strings.Builder{},
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
	// Expected failure: IterationResult.RateLimitRecoveryMs field does not exist yet
	// Expected failure: DiagnosticSnapshot() signature not yet updated

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
		output: &strings.Builder{},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:          "test-2",
			Title:       "Test bead no rate limit",
			Description: "Test description",
		},
		result:      &IterationResult{BeadID: "test-2", Model: "sonnet"},
		model:       "sonnet",
		buildPrompt: "test prompt",
	}

	ctx := context.Background()
	_, stats, _, err := r.executeClaudeInvocation(ctx, bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation failed: %v", err)
	}

	// Verify no recovery time when no rate limit
	_, _, _, _, _, recoveryMs := stats.DiagnosticSnapshot()
	if recoveryMs != 0 {
		t.Errorf("expected DiagnosticSnapshot to return 0 recovery time when no rate limit, got %d ms", recoveryMs)
	}

	// Verify IterationResult also has zero recovery time
	if bc.result.RateLimitRecoveryMs != 0 {
		t.Errorf("expected IterationResult.RateLimitRecoveryMs to be 0 when no rate limit, got %d ms", bc.result.RateLimitRecoveryMs)
	}
}

// TestWriteIterationLog_PropagatesRateLimitRecoveryMs verifies that
// writeIterationLog() copies RateLimitRecoveryMs from IterationResult to IterationLog.
func TestWriteIterationLog_PropagatesRateLimitRecoveryMs(t *testing.T) {
	// Expected failure: IterationLog.RateLimitRecoveryMs field does not exist yet
	// Expected failure: writeIterationLog() does not copy RateLimitRecoveryMs to the log entry

	mockLog := &mockIterationLogger{}

	r := &Runner{
		cfg:    &config.Config{},
		logger: mockLog,
		output: &strings.Builder{},
	}

	result := &IterationResult{
		BeadID:               "test-3",
		BeadTitle:            "Test with rate limit recovery",
		Model:                "sonnet",
		Success:              true,
		Validated:            true,
		Duration:             5 * time.Second,
		TimeoutType:          "",
		TimeToFirstEventMs:   450,
		ToolCallCount:        8,
		StallCount:           0,
		StallTier:            "",
		RateLimitHits:        2,
		RateLimitRecoveryMs:  235,
	}

	r.writeIterationLog(1, result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}

	log := mockLog.Logs[0]

	// Verify all diagnostic fields are propagated
	if log.TimeoutType != "" {
		t.Errorf("expected TimeoutType='', got %q", log.TimeoutType)
	}
	if log.TimeToFirstEventMs != 450 {
		t.Errorf("expected TimeToFirstEventMs=450, got %d", log.TimeToFirstEventMs)
	}
	if log.ToolCallCount != 8 {
		t.Errorf("expected ToolCallCount=8, got %d", log.ToolCallCount)
	}
	if log.StallCount != 0 {
		t.Errorf("expected StallCount=0, got %d", log.StallCount)
	}
	if log.StallTier != "" {
		t.Errorf("expected StallTier='', got %q", log.StallTier)
	}
	if log.RateLimitHits != 2 {
		t.Errorf("expected RateLimitHits=2, got %d", log.RateLimitHits)
	}

	// Verify the new RateLimitRecoveryMs field is propagated
	if log.RateLimitRecoveryMs != 235 {
		t.Errorf("expected RateLimitRecoveryMs=235, got %d", log.RateLimitRecoveryMs)
	}
}

// TestWriteIterationLog_ZeroRecoveryMsInLog verifies that when
// IterationResult.RateLimitRecoveryMs is zero, it's correctly propagated to the log.
func TestWriteIterationLog_ZeroRecoveryMsInLog(t *testing.T) {
	// Expected failure: IterationLog.RateLimitRecoveryMs field does not exist yet
	// Expected failure: writeIterationLog() does not set RateLimitRecoveryMs field

	mockLog := &mockIterationLogger{}

	r := &Runner{
		cfg:    &config.Config{},
		logger: mockLog,
		output: &strings.Builder{},
	}

	result := &IterationResult{
		BeadID:               "test-4",
		BeadTitle:            "Test without rate limit",
		Model:                "haiku",
		Success:              true,
		Validated:            true,
		Duration:             2 * time.Second,
		TimeoutType:          "",
		TimeToFirstEventMs:   150,
		ToolCallCount:        3,
		StallCount:           0,
		StallTier:            "",
		RateLimitHits:        0,
		RateLimitRecoveryMs:  0,
	}

	r.writeIterationLog(1, result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}

	log := mockLog.Logs[0]

	// Verify zero recovery time is correctly logged
	if log.RateLimitRecoveryMs != 0 {
		t.Errorf("expected RateLimitRecoveryMs=0, got %d", log.RateLimitRecoveryMs)
	}

	// Also verify other related fields
	if log.RateLimitHits != 0 {
		t.Errorf("expected RateLimitHits=0, got %d", log.RateLimitHits)
	}
}

// TestIterationLogJSON_IncludesRateLimitRecoveryMs verifies that when
// IterationLog is marshaled to JSON, the RateLimitRecoveryMs field is included.
func TestIterationLogJSON_IncludesRateLimitRecoveryMs(t *testing.T) {
	// Expected failure: IterationLog.RateLimitRecoveryMs field does not exist yet
	// Expected failure: JSON struct tag for rate_limit_recovery_ms does not exist

	log := &logger.IterationLog{
		Timestamp:            time.Now(),
		Iteration:            1,
		BeadID:               "test-5",
		BeadTitle:            "Test JSON serialization",
		Model:                "sonnet",
		Success:              true,
		Validated:            true,
		DurationMs:           3500,
		CostUSD:              0.025,
		InputTokens:          1200,
		OutputTokens:         800,
		TimeoutType:          "",
		TimeToFirstEventMs:   320,
		ToolCallCount:        6,
		StallCount:           0,
		StallTier:            "",
		RateLimitHits:        1,
		RateLimitRecoveryMs:  180,
	}

	// Marshal to JSON using encoding/json
	jsonBytes, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("failed to marshal IterationLog: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Verify the rate_limit_recovery_ms field is present in JSON
	if !strings.Contains(jsonStr, "rate_limit_recovery_ms") {
		t.Error("expected JSON to contain 'rate_limit_recovery_ms' field")
	}

	// Verify the value is correct
	if !strings.Contains(jsonStr, `"rate_limit_recovery_ms":180`) {
		t.Errorf("expected JSON to contain rate_limit_recovery_ms value 180, got: %s", jsonStr)
	}

	// Verify other diagnostic fields are also present
	if !strings.Contains(jsonStr, "rate_limit_hits") {
		t.Error("expected JSON to contain 'rate_limit_hits' field")
	}
	if !strings.Contains(jsonStr, "time_to_first_event_ms") {
		t.Error("expected JSON to contain 'time_to_first_event_ms' field")
	}
}

// TestIterationLogJSON_OmitsZeroRecoveryMs verifies that when RateLimitRecoveryMs
// is zero, it's either included as 0 or omitted (depending on omitempty tag).
func TestIterationLogJSON_OmitsZeroRecoveryMs(t *testing.T) {
	// Expected failure: IterationLog.RateLimitRecoveryMs field does not exist yet
	// Expected failure: JSON struct tag needs to be configured with appropriate omitempty setting

	log := &logger.IterationLog{
		Timestamp:            time.Now(),
		Iteration:            1,
		BeadID:               "test-6",
		BeadTitle:            "Test zero recovery",
		Model:                "haiku",
		Success:              true,
		Validated:            true,
		DurationMs:           1500,
		CostUSD:              0.005,
		InputTokens:          400,
		OutputTokens:         200,
		TimeoutType:          "",
		TimeToFirstEventMs:   100,
		ToolCallCount:        2,
		StallCount:           0,
		StallTier:            "",
		RateLimitHits:        0,
		RateLimitRecoveryMs:  0,
	}

	// Marshal to JSON using encoding/json
	jsonBytes, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("failed to marshal IterationLog: %v", err)
	}

	jsonStr := string(jsonBytes)

	// The field should either be present with value 0, or omitted if tagged with omitempty
	// Since other diagnostic fields use omitempty, we expect consistency
	// If rate_limit_recovery_ms is present, it should be 0
	if strings.Contains(jsonStr, "rate_limit_recovery_ms") {
		if !strings.Contains(jsonStr, `"rate_limit_recovery_ms":0`) {
			t.Errorf("expected rate_limit_recovery_ms to be 0 when present, got: %s", jsonStr)
		}
	}
	// If it's omitted, that's also acceptable for omitempty fields with zero values
}

// TestEndToEndRateLimitRecoveryLogging verifies the complete flow from
// executeClaudeInvocation through writeIterationLog to the logged entry.
func TestEndToEndRateLimitRecoveryLogging(t *testing.T) {
	// Expected failure: Complete chain not yet implemented
	// Expected failure: IterationResult.RateLimitRecoveryMs field does not exist
	// Expected failure: IterationLog.RateLimitRecoveryMs field does not exist
	// Expected failure: DiagnosticSnapshot() does not return recovery time
	// Expected failure: executeClaudeInvocation() does not capture recovery time
	// Expected failure: writeIterationLog() does not propagate recovery time

	mockLog := &mockIterationLogger{}

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			if handler != nil {
				// Simulate rate limit
				rateLimitEvent := []byte(`{"type":"error","subtype":"rate_limit"}`)
				handler(rateLimitEvent)

				// Simulate recovery delay
				time.Sleep(200 * time.Millisecond)

				// Recovery event
				handler([]byte(`{"type":"system","subtype":"init"}`))
			}

			return &claude.Result{
				Success: true,
				Output:  "complete",
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
		logger:       mockLog,
		output:       &strings.Builder{},
		streamLogger: sl,
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:          "test-7",
			Title:       "End-to-end test",
			Description: "Test complete flow",
		},
		result:      &IterationResult{BeadID: "test-7", Model: "sonnet"},
		model:       "sonnet",
		buildPrompt: "test prompt",
	}

	// Execute Claude invocation (captures recovery time)
	ctx := context.Background()
	_, _, _, err = r.executeClaudeInvocation(ctx, bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation failed: %v", err)
	}

	// Verify result has recovery time
	if bc.result.RateLimitRecoveryMs == 0 {
		t.Error("expected IterationResult to capture rate limit recovery time")
	}
	if bc.result.RateLimitRecoveryMs < 200 {
		t.Errorf("expected recovery time >= 200ms, got %d ms", bc.result.RateLimitRecoveryMs)
	}

	// Write to log
	r.writeIterationLog(1, bc.result)

	// Verify log entry
	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}

	logEntry := mockLog.Logs[0]

	// Verify recovery time propagated to log
	if logEntry.RateLimitRecoveryMs == 0 {
		t.Error("expected IterationLog to contain rate limit recovery time")
	}
	if logEntry.RateLimitRecoveryMs != bc.result.RateLimitRecoveryMs {
		t.Errorf("expected log entry recovery time (%d) to match result recovery time (%d)",
			logEntry.RateLimitRecoveryMs, bc.result.RateLimitRecoveryMs)
	}

	// Verify rate limit hits also logged
	if logEntry.RateLimitHits == 0 {
		t.Error("expected log entry to record rate limit hits")
	}
}

// TestMultipleRateLimits_LogsMostRecentRecovery verifies that when multiple
// rate limit events occur during an invocation, the most recent recovery time
// is captured and logged.
func TestMultipleRateLimits_LogsMostRecentRecovery(t *testing.T) {
	// Expected failure: Complete flow not yet implemented for multiple rate limits
	// Expected failure: StreamStats.RateLimitRecoveryMs should track most recent recovery

	mockLog := &mockIterationLogger{}

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			if handler != nil {
				// First rate limit
				handler([]byte(`{"type":"error","subtype":"overloaded"}`))
				time.Sleep(50 * time.Millisecond)
				handler([]byte(`{"type":"system","subtype":"init"}`))

				// Second rate limit (most recent)
				time.Sleep(30 * time.Millisecond)
				handler([]byte(`{"type":"error","subtype":"rate_limit"}`))
				time.Sleep(150 * time.Millisecond)
				handler([]byte(`{"type":"assistant","message":{"content":[]}}`))
			}

			return &claude.Result{
				Success: true,
				Output:  "complete",
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
		logger:       mockLog,
		output:       &strings.Builder{},
		streamLogger: sl,
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:          "test-8",
			Title:       "Multiple rate limits",
			Description: "Test most recent recovery",
		},
		result:      &IterationResult{BeadID: "test-8", Model: "sonnet"},
		model:       "sonnet",
		buildPrompt: "test prompt",
	}

	ctx := context.Background()
	_, _, _, err = r.executeClaudeInvocation(ctx, bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation failed: %v", err)
	}

	// Write to log
	r.writeIterationLog(1, bc.result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}

	logEntry := mockLog.Logs[0]

	// Verify recovery time is the most recent one (~150ms, not ~50ms)
	if logEntry.RateLimitRecoveryMs < 140 {
		t.Errorf("expected most recent recovery time ~150ms, got %d ms", logEntry.RateLimitRecoveryMs)
	}

	// Verify rate limit hits count includes both
	if logEntry.RateLimitHits < 2 {
		t.Errorf("expected at least 2 rate limit hits, got %d", logEntry.RateLimitHits)
	}
}
