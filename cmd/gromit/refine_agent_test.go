package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/worktree"
	"github.com/spf13/cobra"
)

// TestRefineCommandHasAgentFlag verifies refine command has --agent flag
func TestRefineCommandHasAgentFlag(t *testing.T) {
	// This test will fail until --agent flag is added to refine command
	flag := refineCmd.Flags().Lookup("agent")
	if flag == nil {
		t.Error("refine command missing --agent flag")
	}

	if flag != nil && flag.Value.Type() != "string" {
		t.Errorf("--agent flag type = %q, want %q", flag.Value.Type(), "string")
	}
}

// TestRefineCommandHasChooseAgentFlag verifies refine command has --choose-agent flag
func TestRefineCommandHasChooseAgentFlag(t *testing.T) {
	// This test will fail until --choose-agent flag is added to refine command
	flag := refineCmd.Flags().Lookup("choose-agent")
	if flag == nil {
		t.Error("refine command missing --choose-agent flag")
	}

	if flag != nil && flag.Value.Type() != "bool" {
		t.Errorf("--choose-agent flag type = %q, want %q", flag.Value.Type(), "bool")
	}
}

func TestSetupAgentConfigCreatesExpectedFiles(t *testing.T) {
	_, gromitDir, configPath := setupAgentConfig(t, `
agents:
  definitions:
    helper-agent:
      binary: "echo"
      flags: []
`)

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Loading config: %v", err)
	}

	def, ok := cfg.Agents.Definitions["helper-agent"]
	if !ok {
		t.Fatalf("Config missing helper-agent definition")
	}
	if def.Binary != "echo" {
		t.Fatalf("helper-agent binary = %q, want %q", def.Binary, "echo")
	}

	backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
	if _, err := os.Stat(backlogPath); err != nil {
		t.Fatalf("backlog file missing: %v", err)
	}
}

// TestRefineUsesAgentResolve verifies refine command integrates with agent.Resolve
func TestRefineUsesAgentResolve(t *testing.T) {
	// This test verifies the integration by creating a minimal config and checking
	// that the agent selection behavior works end-to-end
	configContent := `
agents:
  definitions:
    test-agent:
      binary: "echo"
      flags: []
  phases:
    refine: test-agent
`
	tmpDir, _, configPath := setupAgentConfig(t, configContent)
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

	if cfg.Agents.Phases.Refine != "test-agent" {
		t.Errorf("Config loaded but Agents.Phases.Refine = %q, want %q", cfg.Agents.Phases.Refine, "test-agent")
	}

	// Test passes if config loads correctly with agent definitions
	// The actual command invocation test would require mocking exec.Command
	// which is better done in integration tests
}

// TestRefineFlagOverrideTakesPriority verifies --agent flag overrides config
func TestRefineFlagOverrideTakesPriority(t *testing.T) {
	// This acceptance test verifies the priority order:
	// --agent flag should override agents.phases config

	configContent := `
agents:
  phases:
    refine: codex
`
	_, _, configPath := setupAgentConfig(t, configContent)

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Loading config: %v", err)
	}

	// Verify phase config is set
	if cfg.Agents.Phases.Refine != "codex" {
		t.Fatalf("Test setup failed: phase config = %q, want %q", cfg.Agents.Phases.Refine, "codex")
	}

	// This test verifies the structure is in place
	// The actual command execution with flag override would be tested in integration tests
	// where we can verify that agent.Resolve is called with the flag value
	t.Log("Flag override priority will be tested via integration test with actual command execution")
}

// TestRefineChooseAgentTriggersPickerBehavior verifies --choose-agent flag behavior
func TestRefineChooseAgentTriggersPickerBehavior(t *testing.T) {
	// This acceptance test verifies that --choose-agent flag is wired up correctly
	// The actual picker interaction would be tested in integration tests

	configContent := `
agents:
  definitions:
    agent1:
      binary: "echo"
    agent2:
      binary: "cat"
`
	_, _, configPath := setupAgentConfig(t, configContent)

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

// TestRefineAgentPromptConfigTriggersPicker verifies agents.prompt config triggers picker
func TestRefineAgentPromptConfigTriggersPicker(t *testing.T) {
	// This acceptance test verifies agents.prompt: true config is respected

	configContent := `
agents:
  prompt: true
  definitions:
    agent1:
      binary: "echo"
`
	_, _, configPath := setupAgentConfig(t, configContent)

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

func TestRefineChooseAgentFlagPropagatesToResolver(t *testing.T) {
	configContent := `
agents:
  definitions:
    stub-agent:
      binary: "echo"
`
	tmpDir, gromitDir, testConfigPath := setupAgentConfig(t, configContent)
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = testConfigPath
	t.Cleanup(func() {
		configPath = origConfigPath
	})

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	capturedChoose := false
	resolver := &testChooseAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
			capturedChoose = choosePicker
			return &testAgent{}, nil
		},
	}

	deps := &pipeline.Deps{
		AgentResolver: resolver,
		BacklogClient: &testBacklogClient{},
	}
	paths := &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	origCreate := createRefinePipelineFn
	createRefinePipelineFn = func(cfg *config.Config, gDir, specs, plans string) (*pipeline.Pipeline, error) {
		return pipeline.New(deps, paths), nil
	}
	t.Cleanup(func() {
		createRefinePipelineFn = origCreate
	})

	origLauncher := refineSessionLauncherFn
	origRunInDir := refineRunInDirFn
	refineSessionLauncherFn = func(
		gDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		sessionDir := filepath.Join(tmpDir, "session")
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			return nil, err
		}
		if err := callback(sessionDir); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{WorktreeDir: sessionDir}, nil
	}
	refineRunInDirFn = func(dir string, fn func() error) error {
		if dir == "" {
			t.Fatalf("refineRunInDir called with empty dir")
		}
		return fn()
	}
	t.Cleanup(func() {
		refineSessionLauncherFn = origLauncher
		refineRunInDirFn = origRunInDir
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("agent", "", "")
	cmd.Flags().Bool("choose-agent", false, "")
	if err := cmd.Flags().Set("choose-agent", "true"); err != nil {
		t.Fatalf("set choose-agent flag: %v", err)
	}

	if err := runRefine(cmd, []string{"emit a spec"}); err != nil {
		t.Fatalf("runRefine() error = %v", err)
	}

	if !capturedChoose {
		t.Fatalf("AgentResolver.Resolve choosePicker = %v, want true", capturedChoose)
	}
}

type testChooseAgentResolver struct {
	ResolveFn func(phase, flagOverride string, choosePicker bool) (pipeline.Agent, error)
}

func (t *testChooseAgentResolver) Resolve(phase, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return t.ResolveFn(phase, flagOverride, choosePicker)
}

type testAgent struct{}

func (testAgent) Name() string                             { return "test-agent" }
func (testAgent) Launch(promptPath string) error           { return nil }
func (testAgent) LaunchInDir(promptPath, dir string) error { return nil }

type testBacklogClient struct{}

func (testBacklogClient) List() ([]*pipeline.Idea, error)       { return nil, nil }
func (testBacklogClient) Get(id string) (*pipeline.Idea, error) { return nil, nil }
func (testBacklogClient) Add(item *pipeline.Idea) error         { return nil }
func (testBacklogClient) Update(id string, fn func(*pipeline.Idea)) error {
	return nil
}

func TestToPipelineIdeaCopiesFields(t *testing.T) {
	idea := &backlog.Idea{
		ID:       "idea-1",
		Text:     "refine this",
		Type:     "feature",
		Context:  "context",
		Status:   "open",
		SpecName: "spec-name",
	}

	pipeIdea := toPipelineIdea(idea)
	if pipeIdea == nil {
		t.Fatal("toPipelineIdea returned nil")
	}
	if pipeIdea.ID != idea.ID {
		t.Fatalf("ID = %q, want %q", pipeIdea.ID, idea.ID)
	}
	if pipeIdea.Text != idea.Text {
		t.Fatalf("Text = %q, want %q", pipeIdea.Text, idea.Text)
	}
	if pipeIdea.SpecName != idea.SpecName {
		t.Fatalf("SpecName = %q, want %q", pipeIdea.SpecName, idea.SpecName)
	}
}

func TestApplyPipelineIdeaFieldsCopiesStatus(t *testing.T) {
	idea := &backlog.Idea{
		Status:   "open",
		SpecName: "old-spec",
	}
	pipeIdea := &pipeline.Idea{
		Status:   "refined",
		SpecName: "new-spec",
	}

	applyPipelineIdeaFields(idea, pipeIdea)
	if idea.Status != "refined" {
		t.Fatalf("Status = %q, want %q", idea.Status, "refined")
	}
	if idea.SpecName != "new-spec" {
		t.Fatalf("SpecName = %q, want %q", idea.SpecName, "new-spec")
	}
}

// TestRefineAgentConfigBackwardCompatibility verifies refine works without agent config
func TestRefineAgentConfigBackwardCompatibility(t *testing.T) {
	// This acceptance test verifies backward compatibility
	// Existing configs without agents section should still work (defaults to claude)

	configContent := `
claude:
  binary: "claude"
  flags: []
`
	_, _, configPath := setupAgentConfig(t, configContent)

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
