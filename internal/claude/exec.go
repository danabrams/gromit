package claude

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
