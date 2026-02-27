package procutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// ReapProcessTree sends SIGKILL to every descendant of cmd's process by
// recursively walking /proc/<pid>/task/*/children, then falls back to the
// process-group kill used by ReapProcessGroup. This catches orphaned
// processes that escaped the original process group (e.g., double-forked
// daemons or processes that called setpgid). Safe to call after cmd.Wait()
// returns. Errors are silently ignored (processes may already be gone).
//
// Usage: defer procutil.ReapProcessTree(cmd) after cmd.Start() succeeds.
func ReapProcessTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	descendants := collectDescendants(pid)
	if len(descendants) == 0 {
		// /proc walk found nothing (or failed); fall back to group kill.
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		return
	}
	// Kill deepest descendants first (reverse order) so parents don't
	// respawn children, then kill the root's group as a final sweep.
	for i := len(descendants) - 1; i >= 0; i-- {
		_ = syscall.Kill(descendants[i], syscall.SIGKILL)
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

// collectDescendants recursively walks /proc/<pid>/task/*/children to
// discover all descendant PIDs of the given process.
func collectDescendants(pid int) []int {
	var pids []int
	taskDir := fmt.Sprintf("/proc/%d/task", pid)
	tasks, err := os.ReadDir(taskDir)
	if err != nil {
		return nil
	}
	for _, task := range tasks {
		childrenFile := filepath.Join(taskDir, task.Name(), "children")
		data, err := os.ReadFile(childrenFile)
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(string(data)) {
			childPid, err := strconv.Atoi(field)
			if err != nil {
				continue
			}
			pids = append(pids, childPid)
			pids = append(pids, collectDescendants(childPid)...)
		}
	}
	return pids
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
