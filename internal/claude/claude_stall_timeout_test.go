//go:build !windows

package claude_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/policy"
)

// RED: test for stall timeout classification
func TestClaudeClient_StallTimeoutClassification(t *testing.T) {
	// A stall timeout occurs when the heartbeat detects no output for longer
	// than the stall timeout duration, causing the invocation context to be canceled.
	//
	// Scenario:
	// 1. Invocation context is created with a long timeout
	// 2. Heartbeat detects no output for longer than stall timeout
	// 3. Heartbeat calls onStall() which sets stallFired=true and cancels invocationCtx
	// 4. The invocation context is canceled, not deadline-exceeded
	//
	// For ClassifyTimeout classification inputs:
	// - stallFired = true (heartbeat detected the stall)
	// - ctxErr = nil (the invocation was canceled by heartbeat, not by deadline timeout)
	// - parentErr = nil (the parent context was not canceled)

	stallFired := true
	var ctxErr error = nil     // Stall cancellation, not deadline exceeded
	var parentErr error = nil  // Parent context not canceled

	// Classify using the policy
	cfg := &config.Config{}
	classification := policy.NewConfigEscalationPolicy(cfg).ClassifyTimeout(ctxErr, parentErr, stallFired)

	// Assert the timeout is classified as "stall"
	if classification.TimeoutType != "stall" {
		t.Fatalf("ClassifyTimeout() TimeoutType = %q, want %q", classification.TimeoutType, "stall")
	}
	if classification.ParentCanceled {
		t.Fatal("ClassifyTimeout() ParentCanceled = true, want false")
	}
}
