package main

import (
	"os"
	"strings"
	"testing"
)

func readMainSource(t *testing.T) string {
	t.Helper()

	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("Reading main.go: %v", err)
	}

	return string(mainSource)
}

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

func TestRetroUsesAgentLaunchNotDirectExec(t *testing.T) {
	sourceStr := readMainSource(t)

	if !strings.Contains(sourceStr, `"github.com/danabrams/gromit/internal/agent"`) {
		t.Error("main.go does not import agent package - retro migration not complete")
	}

	if !strings.Contains(sourceStr, "agent.Resolve") {
		t.Error("main.go does not call agent.Resolve - agent selection not integrated for retro")
	}

	if !strings.Contains(sourceStr, "runWithSessionWorktreeWithConflictSettings") {
		t.Error("main.go does not use session worktree launcher for retro interactive flow")
	}

	if !strings.Contains(sourceStr, ".LaunchInDir(") {
		t.Error("main.go does not call .LaunchInDir() - agent launch not integrated for retro")
	}

	if !strings.Contains(sourceStr, ".LaunchInDir(promptPath, sessionDir)") {
		t.Error("main.go does not call .LaunchInDir(promptPath, sessionDir) in session callback")
	}

	if strings.Contains(sourceStr, "selectedAgent.Launch(promptPath)") {
		t.Error("main.go still calls selectedAgent.Launch(promptPath) - should use LaunchInDir with launchDir")
	}

	if strings.Contains(sourceStr, "retro.LaunchClaudeCode") {
		t.Error("main.go still contains retro.LaunchClaudeCode - migration not complete")
	}
}

func TestRetroUsesResolveSpecsDir(t *testing.T) {
	sourceStr := readMainSource(t)

	if strings.Contains(sourceStr, `filepath.Join(gromitDir, "specs")`) {
		t.Error(`main.go contains hardcoded filepath.Join(gromitDir, "specs") - should use resolveSpecsDir(cfg)`)
	}
}

func TestRetroWritesPromptToTempFile(t *testing.T) {
	sourceStr := readMainSource(t)

	if !strings.Contains(sourceStr, "retro.BuildClaudeCodePrompt") {
		t.Error("main.go does not call retro.BuildClaudeCodePrompt - prompt building not separated from launch")
	}

	if !strings.Contains(sourceStr, "os.CreateTemp") {
		t.Error("main.go does not write retro prompt to a temp file via os.CreateTemp")
	}
}
