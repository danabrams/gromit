package procutil

import (
	"os/exec"
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
