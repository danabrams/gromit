package procutil

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSetProcessGroupKill(t *testing.T) {
	cmd := exec.Command("echo", "test")
	SetProcessGroupKill(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatal("expected Setpgid to be true")
	}
	if cmd.Cancel == nil {
		t.Fatal("expected Cancel to be set")
	}
}

func TestSetProcessGroupKillCancelNilProcess(t *testing.T) {
	cmd := exec.Command("echo", "test")
	SetProcessGroupKill(cmd)

	// Cancel with nil Process should not panic
	err := cmd.Cancel()
	if err != nil {
		t.Fatalf("expected nil error for nil process, got %v", err)
	}
}

func TestSetProcessGroupKillSysProcAttrValues(t *testing.T) {
	cmd := exec.Command("echo", "test")
	SetProcessGroupKill(cmd)

	want := &syscall.SysProcAttr{Setpgid: true}
	if cmd.SysProcAttr.Setpgid != want.Setpgid {
		t.Fatalf("SysProcAttr.Setpgid = %v, want %v", cmd.SysProcAttr.Setpgid, want.Setpgid)
	}
}

func TestSubprocessEnvSetsGOMAXPROCS(t *testing.T) {
	env := SubprocessEnv()
	found := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "GOMAXPROCS=") {
			if kv != "GOMAXPROCS="+MaxGoParallelism {
				t.Fatalf("GOMAXPROCS = %q, want %q", kv, "GOMAXPROCS="+MaxGoParallelism)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("GOMAXPROCS not found in SubprocessEnv()")
	}
}

func TestSubprocessEnvOverridesExistingGOMAXPROCS(t *testing.T) {
	t.Setenv("GOMAXPROCS", "99")
	env := SubprocessEnv()
	for _, kv := range env {
		if kv == "GOMAXPROCS=99" {
			t.Fatal("SubprocessEnv() did not override existing GOMAXPROCS=99")
		}
	}
}

func TestReapProcessGroupNilProcess(t *testing.T) {
	cmd := exec.Command("echo", "test")
	// Should not panic when Process is nil
	ReapProcessGroup(cmd)
}

func TestReapProcessGroupAfterExit(t *testing.T) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "true")
	SetProcessGroupKill(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	// Should not panic on already-exited process group (ESRCH expected)
	ReapProcessGroup(cmd)
}

func TestReapProcessTreeNilProcess(t *testing.T) {
	cmd := exec.Command("echo", "test")
	// Should not panic when Process is nil
	ReapProcessTree(cmd)
}

func TestReapProcessTreeAfterExit(t *testing.T) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "true")
	SetProcessGroupKill(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	// Should not panic on already-exited process (ESRCH expected)
	ReapProcessTree(cmd)
}

func TestReapProcessTreeKillsChildren(t *testing.T) {
	// Start a shell that spawns a backgrounded sleep, writes its PID to a
	// temp file, then exits. We use a temp file because the backgrounded
	// child inherits stdout, which would cause cmd.Output() to hang.
	pidFile := t.TempDir() + "/child.pid"
	ctx := context.Background()
	script := "sleep 300 </dev/null >/dev/null 2>&1 & echo $! > " + pidFile
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	childPidStr := strings.TrimSpace(string(data))

	// Verify the child is still alive.
	childProc, err := os.FindProcess(mustAtoi(t, childPidStr))
	if err != nil {
		t.Fatalf("FindProcess: %v", err)
	}
	// Signal 0 checks existence without killing.
	if err := childProc.Signal(syscall.Signal(0)); err != nil {
		t.Skipf("child process already gone before reap, skipping: %v", err)
	}

	ReapProcessTree(cmd)

	// Give the kernel a moment to deliver the signal.
	time.Sleep(50 * time.Millisecond)

	// The child should now be dead.
	if err := childProc.Signal(syscall.Signal(0)); err == nil {
		t.Error("child process still alive after ReapProcessTree")
		// Clean up.
		_ = childProc.Kill()
	}
}

func TestCollectDescendantsCurrentProcess(t *testing.T) {
	// The current process has at least one thread; collectDescendants
	// should return without error (though it likely finds no children).
	pids := collectDescendants(os.Getpid())
	// Just verify it doesn't panic; pids may be empty.
	_ = pids
}

func TestCollectDescendantsInvalidPID(t *testing.T) {
	// A non-existent PID should return nil, not panic.
	pids := collectDescendants(999999999)
	if pids != nil {
		t.Fatalf("expected nil for invalid PID, got %v", pids)
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("mustAtoi: non-digit in %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}
