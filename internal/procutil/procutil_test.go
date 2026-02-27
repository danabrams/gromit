package procutil

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
	"testing"
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
