package procutil

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// MaxGoParallelism controls the GOMAXPROCS value propagated to subprocesses.
// This limits how many threads/processes tools like `go test`, `go build`,
// and `go vet` spawn, preventing cgroup PID exhaustion during long runs.
const MaxGoParallelism = "4"

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

// ReapProcessGroup sends SIGKILL to the process group of cmd after it has
// exited. This cleans up orphaned child processes that survive the main
// process exit — for example, go test binaries or compilation processes
// spawned by an LLM CLI's Bash tool calls that created their own process
// groups. Safe to call after cmd.Wait() returns. Errors are silently
// ignored (the group may already be gone).
//
// Usage: defer procutil.ReapProcessGroup(cmd) after cmd.Start() succeeds.
func ReapProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Best-effort: the group may already be fully exited. ESRCH is expected.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// SubprocessEnv returns os.Environ() with resource-limiting variables applied.
// It sets GOMAXPROCS to bound the parallelism of Go toolchain commands
// (go test, go build, go vet) spawned by LLM agents and validation runners,
// preventing cgroup PID exhaustion on resource-constrained hosts.
func SubprocessEnv() []string {
	env := os.Environ()
	found := false
	for i, kv := range env {
		if strings.HasPrefix(kv, "GOMAXPROCS=") {
			env[i] = "GOMAXPROCS=" + MaxGoParallelism
			found = true
			break
		}
	}
	if !found {
		env = append(env, "GOMAXPROCS="+MaxGoParallelism)
	}
	return env
}
