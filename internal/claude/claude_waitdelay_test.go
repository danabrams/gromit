package claude

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestClientRunSetsWaitDelay(t *testing.T) {
	tempDir := t.TempDir()
	mockBinary := filepath.Join(tempDir, "claude")
	mockScript := "#!/bin/sh\ncat >/dev/null\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	original := execCommandContext
	var captured *exec.Cmd
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, name, args...)
		captured = cmd
		return cmd
	}
	t.Cleanup(func() {
		execCommandContext = original
	})

	client, err := NewClient(mockBinary, nil, 5)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, _ = client.Run(context.Background(), "hello", "model")

	if captured == nil {
		t.Fatal("expected command to be created")
	}

	if captured.WaitDelay != 100*time.Millisecond {
		t.Fatalf("expected WaitDelay 100ms, got %v", captured.WaitDelay)
	}
}

func TestClientStreamRunSetsWaitDelay(t *testing.T) {
	tempDir := t.TempDir()
	mockBinary := filepath.Join(tempDir, "claude")
	mockScript := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	original := execCommandContext
	var captured *exec.Cmd
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, name, args...)
		captured = cmd
		return cmd
	}
	t.Cleanup(func() {
		execCommandContext = original
	})

	client, err := NewClient(mockBinary, nil, 5)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, _ = client.StreamRun(context.Background(), "hello", "model", io.Discard, nil, nil)

	if captured == nil {
		t.Fatal("expected command to be created")
	}

	if captured.WaitDelay != 100*time.Millisecond {
		t.Fatalf("expected WaitDelay 100ms, got %v", captured.WaitDelay)
	}
}
