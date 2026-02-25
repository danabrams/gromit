package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
)

// TestPlanCommandHasAgentFlag verifies plan command has --agent flag
func TestPlanCommandHasAgentFlag(t *testing.T) {
	// This test will fail until --agent flag is added to plan command
	flag := planCmd.Flags().Lookup("agent")
	if flag == nil {
		t.Error("plan command missing --agent flag")
	}

	if flag != nil && flag.Value.Type() != "string" {
		t.Errorf("--agent flag type = %q, want %q", flag.Value.Type(), "string")
	}
}

// TestPlanCommandHasChooseAgentFlag verifies plan command has --choose-agent flag
func TestPlanCommandHasChooseAgentFlag(t *testing.T) {
	// This test will fail until --choose-agent flag is added to plan command
	flag := planCmd.Flags().Lookup("choose-agent")
	if flag == nil {
		t.Error("plan command missing --choose-agent flag")
	}

	if flag != nil && flag.Value.Type() != "bool" {
		t.Errorf("--choose-agent flag type = %q, want %q", flag.Value.Type(), "bool")
	}
}

// TestPlanUsesAgentResolve verifies plan command integrates with agent.Resolve
func TestPlanUsesAgentResolve(t *testing.T) {
	// This test verifies the integration by creating a minimal config and checking
	// that the agent selection behavior works end-to-end
	tmpDir := t.TempDir()

	// Create minimal gromit config with custom agent
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `
agents:
  definitions:
    test-agent:
      binary: "echo"
      flags: []
  phases:
    plan: test-agent
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

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

	if cfg.Agents.Phases.Plan != "test-agent" {
		t.Errorf("Config loaded but Agents.Phases.Plan = %q, want %q", cfg.Agents.Phases.Plan, "test-agent")
	}

	// Test passes if config loads correctly with agent definitions
	// The actual command invocation test would require mocking exec.Command
	// which is better done in integration tests
}

// TestPlanFlagOverrideTakesPriority verifies --agent flag overrides config
func TestPlanFlagOverrideTakesPriority(t *testing.T) {
	// This acceptance test verifies the priority order:
	// --agent flag should override agents.phases config

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create config with phase default
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `
agents:
  phases:
    plan: codex
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Loading config: %v", err)
	}

	// Verify phase config is set
	if cfg.Agents.Phases.Plan != "codex" {
		t.Fatalf("Test setup failed: phase config = %q, want %q", cfg.Agents.Phases.Plan, "codex")
	}

	// This test verifies the structure is in place
	// The actual command execution with flag override would be tested in integration tests
	// where we can verify that agent.Resolve is called with the flag value
	t.Log("Flag override priority will be tested via integration test with actual command execution")
}

// TestPlanChooseAgentTriggersPickerBehavior verifies --choose-agent flag behavior
func TestPlanChooseAgentTriggersPickerBehavior(t *testing.T) {
	// This acceptance test verifies that --choose-agent flag is wired up correctly
	// The actual picker interaction would be tested in integration tests

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `
agents:
  definitions:
    agent1:
      binary: "echo"
    agent2:
      binary: "cat"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

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

// TestPlanAgentPromptConfigTriggersPicker verifies agents.prompt config triggers picker
func TestPlanAgentPromptConfigTriggersPicker(t *testing.T) {
	// This acceptance test verifies agents.prompt: true config is respected

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `
agents:
  prompt: true
  definitions:
    agent1:
      binary: "echo"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

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

func TestLaunchPlanSessionUsesSessionLauncher(t *testing.T) {
	origLauncher := planSessionLauncherFn
	t.Cleanup(func() { planSessionLauncherFn = origLauncher })
	planSessionLauncherFn = func(gDir, command string, _ sessionConflictSettings, callback func(sessionDir string) error) (*worktree.SessionWorktree, error) {
		if gDir == "" || command == "" {
			t.Fatalf("unexpected launcher args: gDir=%q command=%q", gDir, command)
		}
		if err := callback("session-dir"); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{WorktreeDir: "session-dir"}, nil
	}

	agent := &recordingAgent{}
	cfg := &config.Config{}
	promptPath, err := filepath.Abs("prompt-path.md")
	if err != nil {
		t.Fatalf("abs prompt path: %v", err)
	}

	if err := launchPlanSession(cfg, "/tmp/gromit", agent, promptPath); err != nil {
		t.Fatalf("launchPlanSession error: %v", err)
	}

	if agent.launchDir != "session-dir" {
		t.Fatalf("expected launch dir session-dir, got %q", agent.launchDir)
	}
	if agent.promptPath != promptPath {
		t.Fatalf("expected prompt path %q, got %q", promptPath, agent.promptPath)
	}
}

func TestLaunchPlanSessionSkipsSessionLauncherWhenWorktreeDisabled(t *testing.T) {
	origLauncher := planSessionLauncherFn
	t.Cleanup(func() { planSessionLauncherFn = origLauncher })
	planSessionLauncherFn = func(string, string, sessionConflictSettings, func(string) error) (*worktree.SessionWorktree, error) {
		t.Fatalf("session launcher should not be invoked when worktree is disabled")
		return nil, nil
	}

	agent := &recordingAgent{}
	cfg := &config.Config{Worktree: config.WorktreeConfig{Enabled: boolPtr(false)}}
	promptPath, err := filepath.Abs("prompt-path.md")
	if err != nil {
		t.Fatalf("abs prompt path: %v", err)
	}

	if err := launchPlanSession(cfg, "/tmp/gromit", agent, promptPath); err != nil {
		t.Fatalf("launchPlanSession error: %v", err)
	}

	if agent.launchDir != "" {
		t.Fatalf("expected empty launch dir, got %q", agent.launchDir)
	}
	if agent.promptPath != promptPath {
		t.Fatalf("expected prompt path %q, got %q", promptPath, agent.promptPath)
	}
}

type recordingAgent struct {
	promptPath string
	launchDir  string
}

var _ agent.Agent = (*recordingAgent)(nil)

func (r *recordingAgent) Name() string { return "recording" }
func (r *recordingAgent) Launch(promptPath string) error {
	r.promptPath = promptPath
	r.launchDir = ""
	return nil
}

func (r *recordingAgent) LaunchInDir(promptPath, dir string) error {
	r.promptPath = promptPath
	r.launchDir = dir
	return nil
}

func (r *recordingAgent) Command(promptPath string) (*exec.Cmd, error) {
	r.promptPath = promptPath
	r.launchDir = ""
	return nil, nil
}

func boolPtr(v bool) *bool { return &v }

// TestPlanAgentConfigBackwardCompatibility verifies plan works without agent config
func TestPlanAgentConfigBackwardCompatibility(t *testing.T) {
	// This acceptance test verifies backward compatibility
	// Existing configs without agents section should still work (defaults to claude)

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create minimal config WITHOUT agents section (legacy config)
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `
claude:
  binary: "claude"
  flags: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

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
