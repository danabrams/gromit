package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
)

func makeBoolPtr(v bool) *bool { return &v }

func TestLaunchInSessionIfEnabled(t *testing.T) {
	launcherCalled := 0
	launcher := func(gromitDir string, command string, _ sessionConflictSettings, callback func(string) error) (*worktree.SessionWorktree, error) {
		launcherCalled++
		if err := callback("session-dir"); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{WorktreeDir: "session-dir"}, nil
	}

	fallbackCalled := false
	fallback := func() error {
		fallbackCalled = true
		return nil
	}

	cfg := &config.Config{Worktree: config.WorktreeConfig{Enabled: makeBoolPtr(false)}}

	if err := launchInSessionIfEnabled(cfg, "/tmp/gromit", "command", launcher, func(dir string) error {
		t.Fatalf("session callback should not be executed when worktree is disabled; got dir=%q", dir)
		return nil
	}, fallback); err != nil {
		t.Fatalf("helper returned error: %v", err)
	}

	if !fallbackCalled {
		t.Fatalf("expected fallback to run when worktree disabled")
	}
	if launcherCalled != 0 {
		t.Fatalf("launcher should not run when worktree disabled, got %d", launcherCalled)
	}

	fallbackCalled = false
	if err := launchInSessionIfEnabled(nil, "/tmp/gromit", "command", launcher, func(dir string) error {
		if dir != "session-dir" {
			t.Fatalf("expected runner to run inside the session dir, got %q", dir)
		}
		return nil
	}, fallback); err != nil {
		t.Fatalf("helper returned error: %v", err)
	}

	if fallbackCalled {
		t.Fatalf("fallback should not run when worktree enabled")
	}
	if launcherCalled != 1 {
		t.Fatalf("expected launcher to run exactly once, got %d", launcherCalled)
	}
}
