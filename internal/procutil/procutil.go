package procutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// MaxGoParallelism controls the GOMAXPROCS value propagated to subprocesses.
// This limits how many threads/processes tools like `go test`, `go build`,
// and `go vet` spawn, preventing cgroup PID exhaustion during long runs.
const MaxGoParallelism = "4"

const (
	processPressureThreshold      = 0.90
	processPressurePollInterval   = 100 * time.Millisecond
	defaultProcessCapacityMaxWait = 1500 * time.Millisecond
)

var processCreationPressuredFn = processCreationPressured
var pidPressureFn = PIDPressure

// ProcessCapacityError indicates subprocess creation stayed PID-pressured
// for the full wait window.
type ProcessCapacityError struct {
	Current int
	Max     int
	Waited  time.Duration
}

func (e *ProcessCapacityError) Error() string {
	if e == nil {
		return "process capacity unavailable"
	}
	if e.Current > 0 && e.Max > 0 {
		pct := e.Current * 100 / e.Max
		return fmt.Sprintf("process capacity unavailable after %v: cgroup PID usage at %d%% (%d/%d)", e.Waited.Round(time.Millisecond), pct, e.Current, e.Max)
	}
	return fmt.Sprintf("process capacity unavailable after %v", e.Waited.Round(time.Millisecond))
}

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

// KillDescendantsOnCancel starts a goroutine that waits for ctx to be
// cancelled, then snapshots the live descendant tree of cmd's process and
// kills them deepest-first. This solves a timing race: if we wait until
// after cmd.Wait() returns, the parent's /proc entry is gone and
// collectDescendants can't find grandchildren. By listening on ctx.Done()
// we catch the cancellation while the process tree is still visible.
//
// Usage: call immediately after cmd.Start() succeeds.
func KillDescendantsOnCancel(ctx context.Context, cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	go func() {
		<-ctx.Done()
		descendants := collectDescendants(pid)
		// Kill deepest descendants first so parents don't respawn children.
		for i := len(descendants) - 1; i >= 0; i-- {
			_ = syscall.Kill(descendants[i], syscall.SIGKILL)
		}
		// Process-group sweep as fallback.
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}()
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

// WaitForProcessCapacity pauses briefly when the current cgroup is near its PID
// limit, reducing fork/exec EAGAIN spikes during process bursts. Best effort:
// if cgroup metrics are unavailable, it returns immediately.
func WaitForProcessCapacity(ctx context.Context, maxWait time.Duration) error {
	if maxWait <= 0 {
		maxWait = defaultProcessCapacityMaxWait
	}

	start := time.Now()
	deadline := time.Now().Add(maxWait)
	for {
		pressured, err := processCreationPressuredFn()
		if err != nil || !pressured {
			return nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			current, max, _ := pidPressureFn()
			return &ProcessCapacityError{
				Current: current,
				Max:     max,
				Waited:  time.Since(start),
			}
		}
		wait := processPressurePollInterval
		if remaining < wait {
			wait = remaining
		}

		if err := SleepWithContext(ctx, wait); err != nil {
			return err
		}
	}
}

// SleepWithContext pauses for the requested duration while honoring context cancellation.
func SleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func processCreationPressured() (bool, error) {
	current, max, err := readCgroupPIDUsage()
	if err != nil {
		return false, err
	}
	if max <= 0 || current < 0 {
		return false, nil
	}
	return float64(current)/float64(max) >= processPressureThreshold, nil
}

func readCgroupPIDUsage() (current int, max int, err error) {
	cgroupPath, err := currentCgroupPath()
	if err != nil {
		return 0, 0, err
	}
	root := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(filepath.Clean(cgroupPath), "/"))
	currentBytes, err := os.ReadFile(filepath.Join(root, "pids.current"))
	if err != nil {
		return 0, 0, err
	}
	maxBytes, err := os.ReadFile(filepath.Join(root, "pids.max"))
	if err != nil {
		return 0, 0, err
	}

	current, err = parsePIDCount(string(currentBytes))
	if err != nil {
		return 0, 0, err
	}
	max, unlimited, err := parsePIDLimit(string(maxBytes))
	if err != nil {
		return 0, 0, err
	}
	if unlimited {
		return current, 0, nil
	}
	return current, max, nil
}

func currentCgroupPath() (string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "::", 2)
		if len(parts) == 2 {
			return parts[1], nil
		}
	}
	return "", fmt.Errorf("unable to determine cgroup path")
}

func parsePIDCount(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	return strconv.Atoi(value)
}

func parsePIDLimit(raw string) (int, bool, error) {
	value := strings.TrimSpace(raw)
	if value == "max" {
		return 0, true, nil
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, false, err
	}
	return count, false, nil
}
