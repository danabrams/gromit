package procutil

import (
	"os/exec"
	"syscall"
)

// SetProcessGroupKill configures cmd to create a new process group and kill
// the entire group on context cancellation. This prevents orphaned child
// processes from accumulating when long-running subprocesses (LLM CLIs,
// validation commands) are cancelled.
func SetProcessGroupKill(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Negative PID targets the process group so child processes are terminated too.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
