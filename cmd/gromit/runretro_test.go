package main

import (
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
	"github.com/spf13/cobra"
)

type retroTestAgent struct {
	launchInDirFn func(promptPath, dir string) error
}

func (a *retroTestAgent) Name() string { return "retro-test-agent" }

func (a *retroTestAgent) Launch(promptPath string) error {
	return a.LaunchInDir(promptPath, "")
}

func (a *retroTestAgent) LaunchInDir(promptPath, dir string) error {
	if a != nil && a.launchInDirFn != nil {
		return a.launchInDirFn(promptPath, dir)
	}
	return nil
}

func (a *retroTestAgent) Command(promptPath string) (*exec.Cmd, error) {
	return nil, errors.New("not implemented")
}

var _ agent.Agent = (*retroTestAgent)(nil)

func TestLaunchRetroInteractiveSession_UsesSessionWorktreeDir(t *testing.T) {
	origResolve := retroResolveAgentFn
	origLauncher := retroSessionLauncherFn
	origRecord := retroRecordStateFn
	t.Cleanup(func() {
		retroResolveAgentFn = origResolve
		retroSessionLauncherFn = origLauncher
		retroRecordStateFn = origRecord
	})

	cfg := &config.Config{}
	cfg.Paths.GromitDir = filepath.Join(t.TempDir(), ".gromit")
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	cmd.Flags().Bool("choose-agent", false, "")

	wantSessionDir := t.TempDir()
	gotLaunchDir := ""
	recordCalled := false

	retroResolveAgentFn = func(*config.Config, string, string, bool, io.Reader, io.Writer) (agent.Agent, error) {
		return &retroTestAgent{
			launchInDirFn: func(promptPath, dir string) error {
				gotLaunchDir = dir
				return nil
			},
		}, nil
	}
	retroSessionLauncherFn = func(
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		if command != "retro" {
			t.Fatalf("command = %q, want %q", command, "retro")
		}
		wantSettings := sessionConflictSettingsFromConfig(cfg)
		if conflictSettings.Policy != wantSettings.Policy || conflictSettings.RetryCap != wantSettings.RetryCap {
			t.Fatalf("conflict settings = %+v, want policy=%q retry_cap=%d", conflictSettings, wantSettings.Policy, wantSettings.RetryCap)
		}
		if err := callback(wantSessionDir); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{BranchName: "gromit/retro-test", WorktreeDir: wantSessionDir}, nil
	}
	retroRecordStateFn = func(gromitDir string) error {
		recordCalled = true
		return nil
	}

	if err := launchRetroInteractiveSession(cfg, cmd, cfg.Paths.GromitDir, "prompt.md"); err != nil {
		t.Fatalf("launchRetroInteractiveSession() error = %v", err)
	}
	if gotLaunchDir != wantSessionDir {
		t.Fatalf("launch dir = %q, want %q", gotLaunchDir, wantSessionDir)
	}
	if !recordCalled {
		t.Fatal("expected retro state to be recorded")
	}
}

func TestLaunchRetroInteractiveSession_ConflictHandoffPropagates(t *testing.T) {
	origResolve := retroResolveAgentFn
	origLauncher := retroSessionLauncherFn
	origRecord := retroRecordStateFn
	t.Cleanup(func() {
		retroResolveAgentFn = origResolve
		retroSessionLauncherFn = origLauncher
		retroRecordStateFn = origRecord
	})

	cfg := &config.Config{}
	cfg.Paths.GromitDir = filepath.Join(t.TempDir(), ".gromit")
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	cmd.Flags().Bool("choose-agent", false, "")

	recordCalled := false
	retroResolveAgentFn = func(*config.Config, string, string, bool, io.Reader, io.Writer) (agent.Agent, error) {
		return &retroTestAgent{}, nil
	}
	retroRecordStateFn = func(gromitDir string) error {
		recordCalled = true
		return nil
	}
	retroSessionLauncherFn = func(
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		return &worktree.SessionWorktree{
				BranchName:  "gromit/retro-conflict",
				WorktreeDir: "/tmp/session-retro",
			}, &mergeConflictHandoffError{
				Policy:     conflictPolicyManual,
				Branch:     "gromit/retro-conflict",
				SessionDir: "/tmp/session-retro",
				MergeErr:   errors.New("merge conflict"),
			}
	}

	err := launchRetroInteractiveSession(cfg, cmd, cfg.Paths.GromitDir, "prompt.md")
	if err == nil {
		t.Fatal("expected conflict handoff error, got nil")
	}
	if !isMergeConflictHandoffError(err) {
		t.Fatalf("expected merge conflict handoff error, got %T (%v)", err, err)
	}
	if recordCalled {
		t.Fatal("retro state should not be recorded when merge handoff occurs")
	}
}
