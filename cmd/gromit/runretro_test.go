package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
	"github.com/spf13/cobra"
)

type retroTestAgent = sessionTestAgent

func TestLaunchRetroInteractiveSession_UsesSessionWorktreeDir(t *testing.T) {
	t.Parallel()
	deps := newRunDeps()
	cfg := &config.Config{}
	cfg.Paths.GromitDir = filepath.Join(t.TempDir(), ".gromit")
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	cmd.Flags().Bool("choose-agent", false, "")

	wantSessionDir := t.TempDir()
	gotLaunchDir := ""
	recordCalled := false

	deps.retroResolveAgent = func(*config.Config, string, string, bool, io.Reader, io.Writer) (agent.Agent, error) {
		return &retroTestAgent{
			launchInDirFn: func(promptPath, dir string) error {
				gotLaunchDir = dir
				return nil
			},
		}, nil
	}
	deps.retroSessionLauncher = func(
		ctx context.Context,
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
	deps.retroRecordState = func(gromitDir string) error {
		recordCalled = true
		return nil
	}

	if err := deps.launchRetroInteractiveSession(cfg, cmd, cfg.Paths.GromitDir, "prompt.md"); err != nil {
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
	t.Parallel()
	deps := newRunDeps()
	cfg := &config.Config{}
	cfg.Paths.GromitDir = filepath.Join(t.TempDir(), ".gromit")
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	cmd.Flags().Bool("choose-agent", false, "")

	recordCalled := false
	deps.retroResolveAgent = func(*config.Config, string, string, bool, io.Reader, io.Writer) (agent.Agent, error) {
		return &retroTestAgent{}, nil
	}
	deps.retroRecordState = func(gromitDir string) error {
		recordCalled = true
		return nil
	}
	deps.retroSessionLauncher = func(
		ctx context.Context,
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

	err := deps.launchRetroInteractiveSession(cfg, cmd, cfg.Paths.GromitDir, "prompt.md")
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

func TestLaunchRetroInteractiveSession_ConvertsPromptPathToAbsolute(t *testing.T) {
	deps := newRunDeps()
	cfg := &config.Config{}
	cfg.Paths.GromitDir = filepath.Join(t.TempDir(), ".gromit")
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	cmd.Flags().Bool("choose-agent", false, "")

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	relativePromptPath := filepath.Join(".gromit", "tmp", "retro-prompt.md")
	if err := os.MkdirAll(filepath.Dir(relativePromptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	if err := os.WriteFile(relativePromptPath, []byte("prompt"), 0o644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	capturedPromptPath := ""
	deps.retroResolveAgent = func(*config.Config, string, string, bool, io.Reader, io.Writer) (agent.Agent, error) {
		return &retroTestAgent{
			launchInDirFn: func(promptPath, dir string) error {
				capturedPromptPath = promptPath
				return nil
			},
		}, nil
	}
	deps.retroSessionLauncher = func(
		ctx context.Context,
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		if err := callback(t.TempDir()); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{}, nil
	}
	deps.retroRecordState = func(gromitDir string) error { return nil }

	err := deps.launchRetroInteractiveSession(cfg, cmd, cfg.Paths.GromitDir, relativePromptPath)
	if err != nil {
		t.Fatalf("launchRetroInteractiveSession() error = %v", err)
	}
	if !filepath.IsAbs(capturedPromptPath) {
		t.Fatalf("prompt path = %q, want absolute path", capturedPromptPath)
	}
}
