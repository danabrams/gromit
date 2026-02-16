package main

import (
	"os"
	"strings"
	"testing"
)

func TestDebugCommandHasAgentFlag(t *testing.T) {
	flag := debugCmd.Flags().Lookup("agent")
	if flag == nil {
		t.Fatal("debug command missing --agent flag")
	}
	if flag.Value.Type() != "string" {
		t.Fatalf("--agent flag type = %q, want %q", flag.Value.Type(), "string")
	}
}

func TestDebugCommandHasChooseAgentFlag(t *testing.T) {
	flag := debugCmd.Flags().Lookup("choose-agent")
	if flag == nil {
		t.Fatal("debug command missing --choose-agent flag")
	}
	if flag.Value.Type() != "bool" {
		t.Fatalf("--choose-agent flag type = %q, want %q", flag.Value.Type(), "bool")
	}
}

func TestDebugUsesAgentLaunchNotDirectExec(t *testing.T) {
	debugSource, err := os.ReadFile("debug.go")
	if err != nil {
		t.Fatalf("Reading debug.go: %v", err)
	}

	sourceStr := string(debugSource)

	if !strings.Contains(sourceStr, `"github.com/danabrams/gromit/internal/agent"`) {
		t.Error("debug.go does not import agent package - integration not complete")
	}

	if !strings.Contains(sourceStr, "agent.Resolve") {
		t.Error("debug.go does not call agent.Resolve - agent selection not integrated")
	}

	if !strings.Contains(sourceStr, ".Launch(") {
		t.Error("debug.go does not call .Launch() - agent launch not integrated")
	}

	if strings.Contains(sourceStr, "exec.Command(claudeBinary") {
		t.Error("debug.go still contains direct exec.Command(claudeBinary...) - old code not removed")
	}
}

func TestDebugHelpIncludesCodexExample(t *testing.T) {
	if !strings.Contains(debugCmd.Long, "--agent codex") {
		t.Fatalf("debug help missing codex example, got: %s", debugCmd.Long)
	}
}

func TestDebugChooseAgentUsesPicker_Reclassified(t *testing.T) {
	debugSource, err := os.ReadFile("debug.go")
	if err != nil {
		t.Fatalf("Reading debug.go: %v", err)
	}

	src := string(debugSource)
	if !strings.Contains(src, `GetBool("choose-agent")`) {
		t.Fatal("debug.go missing choose-agent flag read")
	}
	if !strings.Contains(src, `agent.Resolve(cfg, "debug", agentFlag, chooseAgent`) {
		t.Fatal("debug.go missing agent.Resolve call with chooseAgent")
	}
}

func TestDebugPhaseConfigUsesAgent_Reclassified(t *testing.T) {
	debugSource, err := os.ReadFile("debug.go")
	if err != nil {
		t.Fatalf("Reading debug.go: %v", err)
	}

	if !strings.Contains(string(debugSource), `agent.Resolve(cfg, "debug", agentFlag, chooseAgent`) {
		t.Fatal("debug.go must resolve with debug phase to honor agents.phases.debug")
	}
}
