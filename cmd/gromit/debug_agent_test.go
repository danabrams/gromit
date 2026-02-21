package main

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
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

func TestDebugCommandHasRestoreFlag(t *testing.T) {
	flag := debugCmd.Flags().Lookup("restore")
	if flag == nil {
		t.Fatal("debug command missing --restore flag")
	}
	if flag.Value.Type() != "bool" {
		t.Fatalf("--restore flag type = %q, want %q", flag.Value.Type(), "bool")
	}
}

func TestDebugUsesAgentLaunchNotDirectExec(t *testing.T) {
	cmd := debugCmd
	if err := cmd.Flags().Set(debugAgentFlag, "codex"); err != nil {
		t.Fatalf("setting %s flag: %v", debugAgentFlag, err)
	}
	t.Cleanup(func() {
		_ = cmd.Flags().Set(debugAgentFlag, "")
		_ = cmd.Flags().Set(debugChooseAgentFlag, "false")
	})

	cfg := &config.Config{}
	cfg.Agents.Definitions = map[string]config.AgentDefinition{
		"codex": {
			Binary: "echo",
			Flags:  []string{"--codex"},
		},
	}

	agentFlag, _ := cmd.Flags().GetString(debugAgentFlag)
	chooseAgent, _ := cmd.Flags().GetBool(debugChooseAgentFlag)
	selectedAgent, err := resolveDebugAgent(cfg, agentFlag, chooseAgent)
	if err != nil {
		t.Fatalf("resolveDebugAgent() error = %v", err)
	}

	if selectedAgent.Name() != "codex" {
		t.Fatalf("selected agent = %q, want %q", selectedAgent.Name(), "codex")
	}
}

func TestDebugHelpIncludesCodexExample(t *testing.T) {
	if !strings.Contains(debugCmd.Long, "--agent codex") {
		t.Fatalf("debug help missing codex example, got: %s", debugCmd.Long)
	}
}

func TestDebugChooseAgentUsesPicker_Reclassified(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agents.Phases.Debug = "claude"
	cfg.Agents.Definitions = map[string]config.AgentDefinition{
		"claude": {
			Binary: "echo",
			Flags:  []string{"--claude"},
		},
		"codex": {
			Binary: "echo",
			Flags:  []string{"--codex"},
		},
	}

	selectedAgent, err := resolveDebugAgent(cfg, "", false)
	if err != nil {
		t.Fatalf("resolveDebugAgent() error = %v", err)
	}

	if selectedAgent.Name() != "claude" {
		t.Fatalf("selected agent = %q, want %q", selectedAgent.Name(), "claude")
	}
}

func TestDebugPhaseConfigUsesAgent_Reclassified(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agents.Phases.Debug = "codex"
	cfg.Agents.Definitions = map[string]config.AgentDefinition{
		"claude": {
			Binary: "echo",
			Flags:  []string{"--claude"},
		},
		"codex": {
			Binary: "echo",
			Flags:  []string{"--codex"},
		},
	}

	selectedAgent, err := resolveDebugAgent(cfg, "", false)
	if err != nil {
		t.Fatalf("resolveDebugAgent() error = %v", err)
	}

	if selectedAgent.Name() != "codex" {
		t.Fatalf("selected agent = %q, want %q", selectedAgent.Name(), "codex")
	}
}

func TestShouldOverrideDebugModel_OnlyForClaudeWithChangedModelFlag(t *testing.T) {
	cmd := debugCmd
	_ = cmd.Flags().Set(debugModelFlag, "sonnet")
	t.Cleanup(func() { _ = cmd.Flags().Set(debugModelFlag, "opus") })

	cfg := &config.Config{}
	cfg.Agents.Phases.Debug = "codex"
	cfg.Agents.Definitions = map[string]config.AgentDefinition{
		"codex": {Binary: "echo"},
	}

	selectedAgent, err := resolveDebugAgent(cfg, "codex", false)
	if err != nil {
		t.Fatalf("resolveDebugAgent() error = %v", err)
	}

	if shouldOverrideDebugModel(cmd, selectedAgent) {
		t.Fatal("shouldOverrideDebugModel() = true, want false for non-claude agent")
	}
}
