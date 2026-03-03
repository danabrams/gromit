package bead

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

type beadProcutilFnsSnapshot struct {
	wait    func(context.Context, time.Duration) error
	kill    func(context.Context, *exec.Cmd)
	reap    func(*exec.Cmd)
	env     func() []string
	resolve func(context.Context, string) string
}

func restoreBeadProcutilFns(t *testing.T) func() {
	t.Helper()

	snapshot := beadProcutilFnsSnapshot{
		wait:    waitForProcessCapacityFn,
		kill:    killDescendantsOnCancelFn,
		reap:    reapProcessTreeFn,
		env:     subprocessEnvFn,
		resolve: resolveBeadsDirFn,
	}

	restore := func() {
		waitForProcessCapacityFn = snapshot.wait
		killDescendantsOnCancelFn = snapshot.kill
		reapProcessTreeFn = snapshot.reap
		subprocessEnvFn = snapshot.env
		resolveBeadsDirFn = snapshot.resolve
	}

	t.Cleanup(restore)
	return restore
}
