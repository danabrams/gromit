package specmerge

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestDefaultGHRunner_UsesProcessCapacityAndProcessGroupKill(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	calledCapacity := false
	oldCapacityFn := ghWaitForProcessCapacityFn
	t.Cleanup(func() { ghWaitForProcessCapacityFn = oldCapacityFn })
	ghWaitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		calledCapacity = true
		return nil
	}

	calledGroupKill := false
	oldGroupKillFn := ghSetProcessGroupKillFn
	t.Cleanup(func() { ghSetProcessGroupKillFn = oldGroupKillFn })
	ghSetProcessGroupKillFn = func(cmd *exec.Cmd) {
		calledGroupKill = true
	}

	runner := &defaultGHRunner{}
	_, _ = runner.Run(ctx, "version")

	if !calledCapacity {
		t.Fatal("WaitForProcessCapacity was not invoked")
	}
	if !calledGroupKill {
		t.Fatal("SetProcessGroupKill was not invoked")
	}
}
