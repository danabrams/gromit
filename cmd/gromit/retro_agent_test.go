package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
)

func TestRetroCommandHasAgentFlag(t *testing.T) {
	flag := retroCmd.Flags().Lookup("agent")
	if flag == nil {
		t.Fatal("retro command missing --agent flag")
	}
	if flag.Value.Type() != "string" {
		t.Fatalf("--agent flag type = %q, want %q", flag.Value.Type(), "string")
	}
}

func TestRetroCommandHasChooseAgentFlag(t *testing.T) {
	flag := retroCmd.Flags().Lookup("choose-agent")
	if flag == nil {
		t.Fatal("retro command missing --choose-agent flag")
	}
	if flag.Value.Type() != "bool" {
		t.Fatalf("--choose-agent flag type = %q, want %q", flag.Value.Type(), "bool")
	}
}

func TestLaunchRetroInteractiveSessionUsesSessionLauncher(t *testing.T) {
	origResolve := retroResolveAgentFn
	origLauncher := retroSessionLauncherFn
	origRecord := retroRecordStateFn
	t.Cleanup(func() {
		retroResolveAgentFn = origResolve
		retroSessionLauncherFn = origLauncher
		retroRecordStateFn = origRecord
	})

	sessionDir := filepath.Join(t.TempDir(), "session")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}

	promptPath := filepath.Join(t.TempDir(), "retro-prompt.md")
	if err := os.WriteFile(promptPath, []byte("prompt"), 0644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	launchCalled := false
	selectedAgent := &testRetroAgent{}
	retroResolveAgentFn = func(cfg *config.Config, phase, flagOverride string, chooseAgent bool, r io.Reader, w io.Writer) (agent.Agent, error) {
		return selectedAgent, nil
	}
	retroSessionLauncherFn = func(gromitDir string, command string, conflict sessionConflictSettings, callback func(sessionDir string) error) (*worktree.SessionWorktree, error) {
		if command != retroSessionCommand {
			t.Fatalf("command = %q, want %q", command, retroSessionCommand)
		}
		launchCalled = true
		if err := callback(sessionDir); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{WorktreeDir: sessionDir}, nil
	}
	recordCalled := false
	retroRecordStateFn = func(gromitDir string) error {
		recordCalled = true
		return nil
	}

	if err := launchRetroInteractiveSession(&config.Config{}, retroCmd, t.TempDir(), promptPath); err != nil {
		t.Fatalf("launchRetroInteractiveSession error = %v", err)
	}

	if !launchCalled {
		t.Fatal("expected session launcher to be called")
	}
	if selectedAgent.launchDir != sessionDir {
		t.Fatalf("launch dir = %q, want %q", selectedAgent.launchDir, sessionDir)
	}
	if !recordCalled {
		t.Fatal("expected retro state to be recorded")
	}
}

func TestLaunchRetroInteractiveSessionResolvesAgentWithRetroCommand(t *testing.T) {
	origResolve := retroResolveAgentFn
	origLauncher := retroSessionLauncherFn
	origRecord := retroRecordStateFn
	t.Cleanup(func() {
		retroResolveAgentFn = origResolve
		retroSessionLauncherFn = origLauncher
		retroRecordStateFn = origRecord
	})

	promptPath := filepath.Join(t.TempDir(), "retro-prompt.md")
	if err := os.WriteFile(promptPath, []byte("prompt"), 0644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	gotPhase := ""
	gotChoose := true
	retroResolveAgentFn = func(cfg *config.Config, phase, flagOverride string, chooseAgent bool, r io.Reader, w io.Writer) (agent.Agent, error) {
		gotPhase = phase
		gotChoose = chooseAgent
		return &testRetroAgent{}, nil
	}
	retroSessionLauncherFn = func(gromitDir string, command string, conflict sessionConflictSettings, callback func(sessionDir string) error) (*worktree.SessionWorktree, error) {
		if err := callback(""); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{WorktreeDir: ""}, nil
	}
	retroRecordStateFn = func(gromitDir string) error { return nil }

	if err := launchRetroInteractiveSession(&config.Config{}, retroCmd, t.TempDir(), promptPath); err != nil {
		t.Fatalf("launchRetroInteractiveSession error = %v", err)
	}

	if gotPhase != retroSessionCommand {
		t.Fatalf("phase = %q, want %q", gotPhase, retroSessionCommand)
	}
	if gotChoose {
		t.Fatalf("chooseAgent = %v, want false", gotChoose)
	}
}

type testRetroAgent struct {
	launchDir  string
	promptPath string
}

func (t *testRetroAgent) Name() string { return "retro-test" }
func (t *testRetroAgent) Launch(promptPath string) error {
	t.promptPath = promptPath
	t.launchDir = ""
	return nil
}
func (t *testRetroAgent) LaunchInDir(promptPath, dir string) error {
	t.promptPath = promptPath
	t.launchDir = dir
	return nil
}
func (t *testRetroAgent) Command(promptPath string) (*exec.Cmd, error) {
	t.promptPath = promptPath
	return nil, nil
}
