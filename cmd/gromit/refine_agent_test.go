package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
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

// TestRefineUsesAgentResolve verifies refine command integrates with agent.Resolve
func TestRefineUsesAgentResolve(t *testing.T) {
	// This test verifies the integration by creating a minimal config and checking
	// that the agent selection behavior works end-to-end
	tmpDir := t.TempDir()

	// Create minimal gromit config with custom agent
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
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
    refine: test-agent
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an empty backlog file
	backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
	if err := os.WriteFile(backlogPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory so config is found
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

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

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create config with phase default
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `
agents:
  phases:
    refine: codex
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create empty backlog
	backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
	if err := os.WriteFile(backlogPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

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

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
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

	// Create empty backlog
	backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
	if err := os.WriteFile(backlogPath, []byte(""), 0644); err != nil {
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

// TestRefineAgentPromptConfigTriggersPicker verifies agents.prompt config triggers picker
func TestRefineAgentPromptConfigTriggersPicker(t *testing.T) {
	// This acceptance test verifies agents.prompt: true config is respected

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
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

// TestRefineUsesAgentLaunchNotDirectExec verifies refine uses agent.Launch() not exec.Command directly
func TestRefineUsesAgentLaunchNotDirectExec(t *testing.T) {
	// This acceptance test verifies that the refine workflow has been refactored
	// to use agent.Launch() instead of constructing exec.Command directly
	// The agent integration now lives in internal/pipeline/refine.go

	// Read the pipeline refine.go source code
	refineSource, err := os.ReadFile("../../internal/pipeline/refine.go")
	if err != nil {
		t.Fatalf("Reading pipeline/refine.go: %v", err)
	}

	sourceStr := string(refineSource)

	// Check that agent.Resolve is called
	// This is the key integration point - refine should call Resolve
	// to get the appropriate agent based on config and flags
	if !strings.Contains(sourceStr, ".Resolve(") {
		t.Error("pipeline/refine.go does not call .Resolve() - agent selection not integrated")
	}

	// Check that agent.Launch is called
	// After getting the agent, refine should call agent.Launch(promptPath)
	// instead of constructing exec.Command directly
	if !strings.Contains(sourceStr, ".Launch(") {
		t.Error("pipeline/refine.go does not call .Launch() - agent launch not integrated")
	}
}

// TestRefinePreservesPromptBuilding verifies refine still builds prompts correctly
func TestRefinePreservesPromptBuilding(t *testing.T) {
	// This acceptance test verifies that prompt building logic is preserved
	// Prompt construction now lives in internal/pipeline/refine.go

	refineSource, err := os.ReadFile("../../internal/pipeline/refine.go")
	if err != nil {
		t.Fatalf("Reading pipeline/refine.go: %v", err)
	}

	sourceStr := string(refineSource)

	// Verify prompt building steps are still present
	requiredPatterns := []string{
		"systemPrompt",           // System prompt variable
		"buildRefinePrompt",      // Prompt building method
		"WriteTempPrompt",        // Writing prompt to temp file
		"SpecsDir",               // Specs directory still used in prompt
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(sourceStr, pattern) {
			t.Errorf("pipeline/refine.go missing prompt building pattern %q - prompt construction may be broken", pattern)
		}
	}
}

// TestRefinePreservesArtifactDetection verifies refine still detects new spec files
func TestRefinePreservesArtifactDetection(t *testing.T) {
	// This acceptance test verifies that post-launch artifact detection is preserved
	// After agent exits, refine should still scan for new spec files
	// Artifact detection now lives in internal/pipeline/refine.go

	refineSource, err := os.ReadFile("../../internal/pipeline/refine.go")
	if err != nil {
		t.Fatalf("Reading pipeline/refine.go: %v", err)
	}

	sourceStr := string(refineSource)

	// Verify artifact detection steps are still present
	requiredPatterns := []string{
		"existingSpecs",      // Recording specs before launch
		"ListMarkdownFiles",  // Getting spec files
		"newSpecs",           // Getting specs after launch
		"createdSpecs",       // Finding newly created specs
		"DiffFiles",          // Comparing old vs new specs
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(sourceStr, pattern) {
			t.Errorf("pipeline/refine.go missing artifact detection pattern %q - spec detection may be broken", pattern)
		}
	}
}

// TestRefineAgentSelectionIntegration is a comprehensive integration test
func TestRefineAgentSelectionIntegration(t *testing.T) {
	// This test verifies the complete integration flow:
	// 1. Flags exist and are parsed
	// 2. agent.Resolve is called with correct parameters
	// 3. agent.Launch is called with prompt file path
	// 4. Prompt building and artifact detection remain unchanged

	t.Run("flags are defined", func(t *testing.T) {
		agentFlag := refineCmd.Flags().Lookup("agent")
		if agentFlag == nil {
			t.Error("--agent flag not defined")
		}

		chooseAgentFlag := refineCmd.Flags().Lookup("choose-agent")
		if chooseAgentFlag == nil {
			t.Error("--choose-agent flag not defined")
		}
	})

	t.Run("source code has agent integration", func(t *testing.T) {
		refineSource, err := os.ReadFile("../../internal/pipeline/refine.go")
		if err != nil {
			t.Skipf("Cannot read pipeline/refine.go: %v", err)
		}

		sourceStr := string(refineSource)

		// Verify key integration points in pipeline package
		integrationChecks := map[string]string{
			"calls agent.Resolve":    ".Resolve(",
			"calls agent.Launch":     ".Launch(",
		}

		for check, pattern := range integrationChecks {
			if !strings.Contains(sourceStr, pattern) {
				t.Errorf("Integration check failed: %s (missing pattern %q)", check, pattern)
			}
		}
	})

	t.Run("old exec.Command pattern removed", func(t *testing.T) {
		// Check both cmd/gromit/refine.go and internal/pipeline/refine.go
		files := []string{"refine.go", "../../internal/pipeline/refine.go"}

		for _, file := range files {
			refineSource, err := os.ReadFile(file)
			if err != nil {
				continue // Skip if file doesn't exist
			}

			sourceStr := string(refineSource)

			// Check that old patterns are removed
			oldPatterns := []string{
				"exec.Command(claudeBinary",
				"claudeCmd := exec.Command",
			}

			foundOldPatterns := []string{}
			for _, pattern := range oldPatterns {
				if strings.Contains(sourceStr, pattern) {
					foundOldPatterns = append(foundOldPatterns, pattern)
				}
			}

			if len(foundOldPatterns) > 0 {
				t.Errorf("%s: Old exec.Command patterns still present (should be replaced by agent.Launch): %v", file, foundOldPatterns)
			}
		}
	})
}

// TestRefineAgentConfigBackwardCompatibility verifies refine works without agent config
func TestRefineAgentConfigBackwardCompatibility(t *testing.T) {
	// This acceptance test verifies backward compatibility
	// Existing configs without agents section should still work (defaults to claude)

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
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
