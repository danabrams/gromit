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
