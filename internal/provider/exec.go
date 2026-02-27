package provider

import (
	"context"
	"os/exec"

	"github.com/danabrams/gromit/internal/procutil"
)

var execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	procutil.SetProcessGroupKill(cmd)
	return cmd
}

// subprocessEnvFn returns environment variables for LLM CLI subprocesses.
// It includes GOMAXPROCS to limit Go toolchain parallelism in tool calls.
var subprocessEnvFn = procutil.SubprocessEnv

// reapProcessGroupFn cleans up orphaned child processes after a CLI exit.
var reapProcessGroupFn = procutil.ReapProcessTree
