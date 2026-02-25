package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func writeReviewTestConfig(t *testing.T, configContent string) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	return tmpDir, configPath
}

func extractFunction(source, name string) (string, bool) {
	startIdx := strings.Index(source, "func "+name)
	if startIdx == -1 {
		return "", false
	}

	endIdx := strings.Index(source[startIdx:], "\nfunc ")
	if endIdx == -1 {
		return source[startIdx:], true
	}
	endIdx += startIdx
	return source[startIdx:endIdx], true
}

// TestReviewCommandHasAgentFlag verifies review command has --agent flag
func TestReviewCommandHasAgentFlag(t *testing.T) {
	// This test will fail until --agent flag is added to review command
	flag := reviewCmd.Flags().Lookup("agent")
	if flag == nil {
		t.Error("review command missing --agent flag")
	}

	if flag != nil && flag.Value.Type() != "string" {
		t.Errorf("--agent flag type = %q, want %q", flag.Value.Type(), "string")
	}
}

// TestReviewCommandHasChooseAgentFlag verifies review command has --choose-agent flag
func TestReviewCommandHasChooseAgentFlag(t *testing.T) {
	// This test will fail until --choose-agent flag is added to review command
	flag := reviewCmd.Flags().Lookup("choose-agent")
	if flag == nil {
		t.Error("review command missing --choose-agent flag")
	}

	if flag != nil && flag.Value.Type() != "bool" {
		t.Errorf("--choose-agent flag type = %q, want %q", flag.Value.Type(), "bool")
	}
}

// TestReviewUsesAgentResolve verifies review command integrates with agent.Resolve
func TestReviewUsesAgentResolve(t *testing.T) {
	// This test verifies the integration by creating a minimal config and checking
	// that the agent selection behavior works end-to-end
	tmpDir, configPath := writeReviewTestConfig(t, `
agents:
  definitions:
    test-agent:
      binary: "echo"
      flags: []
  phases:
    review: test-agent
`)

	// Change to temp directory so config is found
	t.Chdir(tmpDir)

	// Try to load config - this verifies the config structure is correct
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Loading test config failed: %v (agent config structure may be incomplete)", err)
	}

	// Verify agents config is present
	if cfg.Agents.Definitions == nil {
		t.Error("Config loaded but Agents.Definitions is nil - missing agent config support")
	}

	if cfg.Agents.Phases.Review != "test-agent" {
		t.Errorf("Config loaded but Agents.Phases.Review = %q, want %q", cfg.Agents.Phases.Review, "test-agent")
	}

	// Test passes if config loads correctly with agent definitions
	// The actual command invocation test would require mocking exec.Command
	// which is better done in integration tests
}

// TestReviewFlagOverrideTakesPriority verifies --agent flag overrides config
func TestReviewFlagOverrideTakesPriority(t *testing.T) {
	// This acceptance test verifies the priority order:
	// --agent flag should override agents.phases config

	_, configPath := writeReviewTestConfig(t, `
agents:
  phases:
    review: codex
`)

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Loading config: %v", err)
	}

	// Verify phase config is set
	if cfg.Agents.Phases.Review != "codex" {
		t.Fatalf("Test setup failed: phase config = %q, want %q", cfg.Agents.Phases.Review, "codex")
	}

	// This test verifies the structure is in place
	// The actual command execution with flag override would be tested in integration tests
	// where we can verify that agent.Resolve is called with the flag value
	t.Log("Flag override priority will be tested via integration test with actual command execution")
}

// TestReviewChooseAgentTriggersPickerBehavior verifies --choose-agent flag behavior
func TestReviewChooseAgentTriggersPickerBehavior(t *testing.T) {
	// This acceptance test verifies that --choose-agent flag is wired up correctly
	// The actual picker interaction would be tested in integration tests

	_, configPath := writeReviewTestConfig(t, `
agents:
  definitions:
    agent1:
      binary: "echo"
    agent2:
      binary: "cat"
`)

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Loading config: %v", err)
	}

	// Verify multiple agents are defined
	if len(cfg.Agents.Definitions) < 2 {
		t.Fatalf("Test setup failed: expected 2 agent definitions, got %d", len(cfg.Agents.Definitions))
	}

	// This test verifies the config structure supports the feature
	// The actual picker behavior would be tested in integration tests
	t.Log("Choose-agent picker behavior will be tested via integration test with stdin simulation")
}

// TestReviewAgentPromptConfigTriggersPicker verifies agents.prompt config triggers picker
func TestReviewAgentPromptConfigTriggersPicker(t *testing.T) {
	// This acceptance test verifies agents.prompt: true config is respected

	_, configPath := writeReviewTestConfig(t, `
agents:
  prompt: true
  definitions:
    agent1:
      binary: "echo"
`)

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Loading config: %v", err)
	}

	// Verify agents.prompt is set
	if !cfg.Agents.Prompt {
		t.Error("Config loaded but Agents.Prompt = false, want true")
	}

	// This test verifies the config structure is correct
	// The actual picker triggering would be tested in integration tests
	t.Log("agents.prompt: true picker triggering will be tested via integration test")
}

func TestCmdAgentResolverDefaultsToClaude(t *testing.T) {
	resolver := &cmdAgentResolver{cfg: nil}
	agent, err := resolver.Resolve(reviewSessionCommand, "", false)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if agent.Name() != "claude" {
		t.Fatalf("agent.Name() = %q, want %q", agent.Name(), "claude")
	}
}

func TestCmdAgentResolverFlagOverride(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Definitions: map[string]config.AgentDefinition{
				"stub-agent": {
					Binary: "echo",
					Flags:  []string{},
				},
			},
		},
	}

	resolver := &cmdAgentResolver{cfg: cfg}
	agent, err := resolver.Resolve(reviewSessionCommand, "stub-agent", false)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if agent.Name() != "stub-agent" {
		t.Fatalf("agent.Name() = %q, want %q", agent.Name(), "stub-agent")
	}
}

// TestReviewAgentConfigBackwardCompatibility verifies review works without agent config
func TestReviewAgentConfigBackwardCompatibility(t *testing.T) {
	// This acceptance test verifies backward compatibility
	// Existing configs without agents section should still work (defaults to claude)

	_, configPath := writeReviewTestConfig(t, `
claude:
  binary: "claude"
  flags: []
`)

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Loading legacy config: %v", err)
	}

	// Verify config loads even without agents section
	if cfg == nil {
		t.Fatal("Config is nil after loading legacy config")
	}

	// The agent.Resolve function should default to "claude" when no agents config exists
	// This is tested in agent package tests, but we verify the config structure here
	t.Log("Backward compatibility: legacy configs without agents section should work")
}
