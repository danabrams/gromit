//go:build smokecli

package provider

import (
	"context"
	"os"
	"testing"
)

// TestClaudeNonStreamSuccess exercises the Claude provider with a real CLI
// invocation in non-stream mode when CLAUDE_SMOKE=1.
func TestClaudeNonStreamSuccess(t *testing.T) {
	if os.Getenv("CLAUDE_SMOKE") != "1" {
		t.Skip("CLAUDE_SMOKE=1 not set")
	}

	// This will fail - we need to implement GetClaudeClient()
	client := GetClaudeClient(t)
	cp := NewClaudeProvider(client, map[string]string{
		TierLow: "haiku",
	})

	ctx := context.Background()
	result, err := cp.Run(ctx, "Say 'hello world' in exactly 2 words.", TierLow)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if !result.Success {
		t.Errorf("Run() Success = %v, want true", result.Success)
	}

	if result.Output == "" {
		t.Error("Run() Output is empty")
	}
}
