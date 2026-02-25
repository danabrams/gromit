//go:build smokecli

package provider

import (
	"context"
	"os"
	"testing"

	"github.com/danabrams/gromit/internal/claude"
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

// TestClaudeStreamSuccess exercises the Claude provider with a real CLI
// invocation in stream mode when CLAUDE_SMOKE=1.
func TestClaudeStreamSuccess(t *testing.T) {
	if os.Getenv("CLAUDE_SMOKE") != "1" {
		t.Skip("CLAUDE_SMOKE=1 not set")
	}

	client := GetClaudeClient(t)
	cp := NewClaudeProvider(client, map[string]string{
		TierLow: "haiku",
	})

	ctx := context.Background()
	var eventCount int
	handler := func(line []byte) {
		eventCount++
	}

	result, err := cp.StreamRun(ctx, "Say 'hello' in exactly 1 word.", TierLow, nil, handler, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	if !result.Success {
		t.Errorf("StreamRun() Success = %v, want true", result.Success)
	}

	if result.Output == "" {
		t.Error("StreamRun() Output is empty")
	}
}

// TestCodexNonStreamSuccess exercises the Codex provider with a real CLI
// invocation in non-stream mode when CODEX_SMOKE=1.
func TestCodexNonStreamSuccess(t *testing.T) {
	if os.Getenv("CODEX_SMOKE") != "1" {
		t.Skip("CODEX_SMOKE=1 not set")
	}

	// This will fail - we need to implement GetCodexProvider()
	cp := GetCodexProvider(t)

	ctx := context.Background()
	result, err := cp.Run(ctx, "Say 'hello world' in exactly 2 words.", TierMedium)

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

// TestCodexStreamSuccess exercises the Codex provider with a real CLI
// invocation in stream mode when CODEX_SMOKE=1.
func TestCodexStreamSuccess(t *testing.T) {
	if os.Getenv("CODEX_SMOKE") != "1" {
		t.Skip("CODEX_SMOKE=1 not set")
	}

	cp := GetCodexProvider(t)

	ctx := context.Background()
	var eventCount int
	handler := func(line []byte) {
		eventCount++
	}

	result, err := cp.StreamRun(ctx, "Say 'hello' in exactly 1 word.", TierMedium, nil, handler, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	if !result.Success {
		t.Errorf("StreamRun() Success = %v, want true", result.Success)
	}

	if result.Output == "" {
		t.Error("StreamRun() Output is empty")
	}
}

// TestClaudeFailurePath exercises the Claude provider failure behavior
// when CLAUDE_SMOKE=1. Tests that failures are properly reported.
func TestClaudeFailurePath(t *testing.T) {
	if os.Getenv("CLAUDE_SMOKE") != "1" {
		t.Skip("CLAUDE_SMOKE=1 not set")
	}

	client := GetClaudeClient(t)
	cp := NewClaudeProvider(client, map[string]string{
		TierLow: "nonexistent-model",
	})

	ctx := context.Background()
	result, err := cp.Run(ctx, "Test prompt", TierLow)

	// We expect either an error or a failed result
	if err != nil && result != nil {
		// If there's an error, there shouldn't be a result
		t.Errorf("Run() returned both error and result: err=%v, result=%+v", err, result)
	}

	if err == nil && result != nil && !result.Success {
		// Verify that a failed result has exit code or diagnostics
		if result.ExitCode == 0 && result.Diagnostics == "" {
			t.Errorf("Run() failed but has no exit code or diagnostics: %+v", result)
		}
	}
}

// TestCodexFailurePath exercises the Codex provider failure behavior
// when CODEX_SMOKE=1. Tests that failures are properly reported.
func TestCodexFailurePath(t *testing.T) {
	if os.Getenv("CODEX_SMOKE") != "1" {
		t.Skip("CODEX_SMOKE=1 not set")
	}

	// Use a Codex provider with a nonexistent binary path
	tierMap := map[string]string{
		TierMedium: "gpt-5.3-codex",
	}
	cp := NewCodexProvider("/nonexistent/codex/binary", []string{}, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "Test prompt", TierMedium)

	// We expect either an error or a failed result
	if err != nil && result != nil {
		// If there's an error, there shouldn't be a result
		t.Errorf("Run() returned both error and result: err=%v, result=%+v", err, result)
	}

	if err == nil && result != nil && !result.Success {
		// Verify that a failed result has exit code or diagnostics
		if result.ExitCode == 0 && result.Diagnostics == "" {
			t.Errorf("Run() failed but has no exit code or diagnostics: %+v", result)
		}
	}
}

// GetClaudeClient creates a real Claude CLI client for smoke tests.
func GetClaudeClient(t *testing.T) *claude.Client {
	t.Helper()
	client, err := claude.NewClient("claude", []string{}, 30)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	return client
}

// GetCodexProvider creates a real Codex provider for smoke tests.
func GetCodexProvider(t *testing.T) *CodexProvider {
	t.Helper()
	tierMap := map[string]string{
		TierMedium: "gpt-5.3-codex",
		TierLow:    "gpt-5.1-codex-mini",
	}
	cp := NewCodexProvider("codex", []string{}, tierMap)
	if cp == nil {
		t.Fatal("NewCodexProvider() returned nil")
	}
	return cp
}
