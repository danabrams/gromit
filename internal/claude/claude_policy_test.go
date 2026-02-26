//go:build !windows

package claude_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/policy"
)

func TestClaudeClient_InvocationTimeoutClassification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell to run the fake Claude binary")
	}

	binary := createHangBeforeOutputBinary(t)

	client, err := claude.NewClient(binary, nil, 1)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	_, err = client.Run(context.Background(), "timeout prompt", "sonnet")
	if err == nil {
		t.Fatal("Run() should error when invocation exceeds timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}

	cfg := &config.Config{}
	classification := policy.NewConfigEscalationPolicy(cfg).ClassifyTimeout(err, nil, false)

	if classification.TimeoutType != "invocation" {
		t.Fatalf("ClassifyTimeout() TimeoutType = %q, want %q", classification.TimeoutType, "invocation")
	}
	if classification.ParentCanceled {
		t.Fatal("ClassifyTimeout() ParentCanceled = true, want false")
	}
}

// createHangBeforeOutputBinary creates a Claude-like binary that hangs before producing output.
func createHangBeforeOutputBinary(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "claude")
	script := "#!/bin/sh\nsleep 3600\n"
	if err := os.WriteFile(binary, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake claude hang-before-output binary: %v", err)
	}
	return binary
}
