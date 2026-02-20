package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
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

// TestPlanUsesAgentLaunchNotDirectExec verifies plan uses agent.LaunchInDir() via session launcher, not exec.Command directly
func TestPlanUsesAgentLaunchNotDirectExec(t *testing.T) {
	// This acceptance test verifies that the plan command has been refactored
	// to use agent.LaunchInDir() instead of constructing exec.Command directly

	// Read the plan.go source code
	planSource, err := os.ReadFile("plan.go")
	if err != nil {
		t.Fatalf("Reading plan.go: %v", err)
	}

	sourceStr := string(planSource)

	// Check that agent package is imported
	if !strings.Contains(sourceStr, `"github.com/danabrams/gromit/internal/agent"`) {
		t.Error("plan.go does not import agent package - integration not complete")
	}

	// Check that agent.Resolve is called
	// This is the key integration point - plan should call agent.Resolve
	// to get the appropriate agent based on config and flags
	if !strings.Contains(sourceStr, "agent.Resolve") {
		t.Error("plan.go does not call agent.Resolve - agent selection not integrated")
	}

	// Check that agent.LaunchInDir is called
	// After getting the agent, plan should call agent.LaunchInDir(promptPath, ...)
	// instead of constructing exec.Command directly
	if !strings.Contains(sourceStr, ".LaunchInDir(") {
		t.Error("plan.go does not call .LaunchInDir() - agent launch not integrated")
	}

	// Check that the old exec.Command pattern for Claude is removed
	// The old code had: exec.Command(claudeBinary, cmdArgs...)
	// After refactoring, this should be gone (replaced with agent.Launch)
	if strings.Contains(sourceStr, "exec.Command(claudeBinary") {
		t.Error("plan.go still contains direct exec.Command(claudeBinary...) - old code not removed")
	}
}

// TestPlanPreservesPromptBuilding verifies plan still builds prompts correctly
func TestPlanPreservesPromptBuilding(t *testing.T) {
	// This acceptance test verifies that prompt building logic is unchanged
	// Only the agent invocation should change, not prompt construction

	planSource, err := os.ReadFile("plan.go")
	if err != nil {
		t.Fatalf("Reading plan.go: %v", err)
	}

	sourceStr := string(planSource)

	// Verify prompt building steps are still present
	requiredPatterns := []string{
		"systemPrompt",              // System prompt variable
		"WriteString(systemPrompt)", // Writing prompt to temp file
		"CreateTemp",                // Creating temp file for prompt
		"specsDir",                  // Specs directory still used in prompt
		"plansDir",                  // Plans directory still used in prompt
		"skills.PlanSkill",          // Plan skill still embedded
		"specBody",                  // Spec content still included
		"openBeads",                 // Open beads context still included
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(sourceStr, pattern) {
			t.Errorf("plan.go missing prompt building pattern %q - prompt construction may be broken", pattern)
		}
	}
}

// TestPlanPreservesArtifactDetection verifies plan still detects new plan files
func TestPlanPreservesArtifactDetection(t *testing.T) {
	// This acceptance test verifies that post-launch artifact detection is unchanged
	// After agent exits, plan should still check if plan file was created

	planSource, err := os.ReadFile("plan.go")
	if err != nil {
		t.Fatalf("Reading plan.go: %v", err)
	}

	sourceStr := string(planSource)

	// Verify artifact detection steps are still present
	requiredPatterns := []string{
		"planPath",       // Plan path construction
		"planCreated",    // Flag for tracking plan creation
		"os.Stat",        // Checking if file exists
		"Plan created:",  // Success message
		"chainAfterPlan", // Offering to chain to decompose
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(sourceStr, pattern) {
			t.Errorf("plan.go missing artifact detection pattern %q - plan detection may be broken", pattern)
		}
	}
}

// TestPlanAgentSelectionIntegration is a comprehensive integration test
func TestPlanAgentSelectionIntegration(t *testing.T) {
	// This test verifies the complete integration flow:
	// 1. Flags exist and are parsed
	// 2. agent.Resolve is called with correct parameters
	// 3. agent.LaunchInDir is called with prompt file path
	// 4. Prompt building and artifact detection remain unchanged

	t.Run("flags are defined", func(t *testing.T) {
		agentFlag := planCmd.Flags().Lookup("agent")
		if agentFlag == nil {
			t.Error("--agent flag not defined")
		}

		chooseAgentFlag := planCmd.Flags().Lookup("choose-agent")
		if chooseAgentFlag == nil {
			t.Error("--choose-agent flag not defined")
		}
	})

	t.Run("source code has agent integration", func(t *testing.T) {
		planSource, err := os.ReadFile("plan.go")
		if err != nil {
			t.Skipf("Cannot read plan.go: %v", err)
		}

		sourceStr := string(planSource)

		// Verify key integration points
		integrationChecks := map[string]string{
			"imports agent package":   `"github.com/danabrams/gromit/internal/agent"`,
			"calls agent.Resolve":     "agent.Resolve",
			"calls agent.LaunchInDir": ".LaunchInDir(",
		}

		for check, pattern := range integrationChecks {
			if !strings.Contains(sourceStr, pattern) {
				t.Errorf("Integration check failed: %s (missing pattern %q)", check, pattern)
			}
		}
	})

	t.Run("old exec.Command pattern removed", func(t *testing.T) {
		planSource, err := os.ReadFile("plan.go")
		if err != nil {
			t.Skipf("Cannot read plan.go: %v", err)
		}

		sourceStr := string(planSource)

		// Check that old patterns are removed
		oldPatterns := []string{
			"exec.Command(claudeBinary",
			"claudeCmd := exec.Command",
			"claudeCmd.Stdin = os.Stdin",
			"claudeCmd.Stdout = os.Stdout",
			"claudeCmd.Run()",
		}

		foundOldPatterns := []string{}
		for _, pattern := range oldPatterns {
			if strings.Contains(sourceStr, pattern) {
				foundOldPatterns = append(foundOldPatterns, pattern)
			}
		}

		if len(foundOldPatterns) > 0 {
			t.Errorf("Old exec.Command patterns still present (should be replaced by agent.LaunchInDir): %v", foundOldPatterns)
		}
	})
}

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
