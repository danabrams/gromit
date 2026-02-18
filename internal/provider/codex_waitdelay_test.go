package provider

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexProviderRunSetsWaitDelay(t *testing.T) {
	tempDir := t.TempDir()
	mockBinary := filepath.Join(tempDir, "codex")
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

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	_, _ = cp.Run(context.Background(), "hello", TierMedium)

	if captured == nil {
		t.Fatal("expected command to be created")
	}

	if captured.WaitDelay != 100*time.Millisecond {
		t.Fatalf("expected WaitDelay 100ms, got %v", captured.WaitDelay)
	}
}
