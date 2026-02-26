package provider

import (
	"context"
	"os/exec"
	"testing"
)

func TestCodexProviderRunSetsWaitDelay(t *testing.T) {
	mockBinary := testCreateBinaryWithETXTBSYProtection(t, `cat >/dev/null
`)

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

	if captured.WaitDelay != codexCommandWaitDelay {
		t.Fatalf("expected WaitDelay %v, got %v", codexCommandWaitDelay, captured.WaitDelay)
	}
}
