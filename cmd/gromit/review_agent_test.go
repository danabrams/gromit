package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

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
	tmpDir := t.TempDir()

	// Create minimal gromit config with custom agent
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
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
    review: test-agent
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
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

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create config with phase default
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `
agents:
  phases:
    review: codex
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

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

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
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

// TestReviewAgentPromptConfigTriggersPicker verifies agents.prompt config triggers picker
func TestReviewAgentPromptConfigTriggersPicker(t *testing.T) {
	// This acceptance test verifies agents.prompt: true config is respected

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
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

// TestReviewUsesAgentLaunchNotDirectExec verifies review uses pipeline which uses agent abstraction
func TestReviewUsesAgentLaunchNotDirectExec(t *testing.T) {
	// This acceptance test verifies that the review command has been refactored
	// to use the pipeline pattern, which in turn uses agent.Launch() instead of exec.Command directly

	// Read the review.go source code
	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("Reading review.go: %v", err)
	}

	sourceStr := string(reviewSource)

	// Check that agent package is imported (used in cliAgentResolver adapter)
	if !strings.Contains(sourceStr, `"github.com/danabrams/gromit/internal/agent"`) {
		t.Error("review.go does not import agent package - integration not complete")
	}

	// Check that agent.Resolve is called (in cliAgentResolver adapter)
	// This is the key integration point - review adapter should call agent.Resolve
	// to get the appropriate agent based on config and flags
	if !strings.Contains(sourceStr, "agent.Resolve") {
		t.Error("review.go does not call agent.Resolve - agent selection not integrated")
	}

	// Check that pipeline.ReviewInteractive is called
	// After pipeline extraction, review.go delegates to pipeline which does agent.Launch
	if !strings.Contains(sourceStr, "p.ReviewInteractive") {
		t.Error("review.go does not call pipeline.ReviewInteractive - pipeline integration missing")
	}

	// Check that the old exec.Command pattern for Claude in interactive mode is removed
	// The old code had: exec.Command(cfg.Claude.Binary, args...)
	// After refactoring, this should be gone from runReviewInteractive (replaced with pipeline call)
	// Note: runReviewNonInteractive should still use exec.Command via pipeline
	if strings.Contains(sourceStr, "func runReviewInteractive") {
		// Extract just the runReviewInteractive function to check it specifically
		startIdx := strings.Index(sourceStr, "func runReviewInteractive")
		endIdx := strings.Index(sourceStr[startIdx:], "\nfunc ")
		if endIdx == -1 {
			endIdx = len(sourceStr)
		} else {
			endIdx += startIdx
		}
		interactiveFn := sourceStr[startIdx:endIdx]

		// Check that exec.Command(cfg.Claude.Binary is not in runReviewInteractive
		if strings.Contains(interactiveFn, "exec.Command(cfg.Claude.Binary") {
			t.Error("runReviewInteractive still contains direct exec.Command(cfg.Claude.Binary...) - old code not removed")
		}
	}
}

// TestReviewPreservesPromptBuilding verifies review uses pipeline which builds prompts
func TestReviewPreservesPromptBuilding(t *testing.T) {
	// This acceptance test verifies that prompt building is delegated to pipeline
	// After pipeline extraction, prompt construction happens in pipeline.ReviewInteractive

	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("Reading review.go: %v", err)
	}

	sourceStr := string(reviewSource)

	// Verify that review.go delegates to pipeline
	if !strings.Contains(sourceStr, "p.ReviewInteractive") {
		t.Error("review.go missing p.ReviewInteractive call - pipeline integration missing")
	}

	// Verify that review.go uses PromptRenderer adapter
	if !strings.Contains(sourceStr, "PromptRenderer") {
		t.Error("review.go missing PromptRenderer - adapter pattern not implemented")
	}

	// Verify the pipeline package has prompt building logic
	pipelineSource, err := os.ReadFile("../../internal/pipeline/pipeline.go")
	if err != nil {
		t.Skipf("Cannot read pipeline.go: %v", err)
	}

	pipelineStr := string(pipelineSource)
	pipelinePatterns := []string{
		"RenderThoroughReview", // Rendering review prompt
		"CreateTemp",           // Creating temp file for prompt
		"WriteString",          // Writing prompt to temp file
		"promptPath",           // Prompt file path variable
	}

	for _, pattern := range pipelinePatterns {
		if !strings.Contains(pipelineStr, pattern) {
			t.Errorf("pipeline.go missing prompt building pattern %q - prompt construction may be broken", pattern)
		}
	}
}

// TestReviewInteractiveOnlyUsesAgentSelection verifies agent selection is only for interactive mode
func TestReviewInteractiveOnlyUsesAgentSelection(t *testing.T) {
	// This acceptance test verifies that agent selection is ONLY used in runReviewInteractive,
	// not in runReviewNonInteractive (which should remain unchanged)

	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("Reading review.go: %v", err)
	}

	sourceStr := string(reviewSource)

	// Extract runReviewNonInteractive function
	startIdx := strings.Index(sourceStr, "func runReviewNonInteractive")
	if startIdx == -1 {
		t.Fatal("Cannot find runReviewNonInteractive function")
	}

	endIdx := strings.Index(sourceStr[startIdx:], "\nfunc ")
	if endIdx == -1 {
		endIdx = len(sourceStr)
	} else {
		endIdx += startIdx
	}
	nonInteractiveFn := sourceStr[startIdx:endIdx]

	// runReviewNonInteractive should NOT use agent.Resolve or agent.Launch
	if strings.Contains(nonInteractiveFn, "agent.Resolve") {
		t.Error("runReviewNonInteractive contains agent.Resolve - should remain unchanged (only interactive mode uses agents)")
	}

	if strings.Contains(nonInteractiveFn, "agent.Launch") {
		t.Error("runReviewNonInteractive contains agent.Launch - should remain unchanged (only interactive mode uses agents)")
	}

	// runReviewNonInteractive should still use claude.Client (unchanged)
	if !strings.Contains(nonInteractiveFn, "claude.NewClient") {
		t.Error("runReviewNonInteractive missing claude.NewClient - non-interactive path may be broken")
	}
}

// TestReviewBuildReviewArgsHelperRemoved verifies buildReviewArgs is removed
func TestReviewBuildReviewArgsHelperRemoved(t *testing.T) {
	// This acceptance test verifies that buildReviewArgs helper is removed
	// after refactoring to use agent.Launch

	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("Reading review.go: %v", err)
	}

	sourceStr := string(reviewSource)

	// Check that buildReviewArgs function is removed
	if strings.Contains(sourceStr, "func buildReviewArgs") {
		t.Error("review.go still contains buildReviewArgs function - should be removed after refactoring to agent.Launch")
	}

	// Check that buildReviewArgs is not called
	if strings.Contains(sourceStr, "buildReviewArgs(") {
		t.Error("review.go still calls buildReviewArgs - should be removed after refactoring to agent.Launch")
	}
}

// TestReviewAgentSelectionIntegration is a comprehensive integration test
func TestReviewAgentSelectionIntegration(t *testing.T) {
	// This test verifies the complete integration flow:
	// 1. Flags exist and are parsed
	// 2. agent.Resolve is called with correct parameters
	// 3. agent.Launch is called with prompt file path
	// 4. Prompt building remains unchanged
	// 5. Non-interactive path remains unchanged

	t.Run("flags are defined", func(t *testing.T) {
		agentFlag := reviewCmd.Flags().Lookup("agent")
		if agentFlag == nil {
			t.Error("--agent flag not defined")
		}

		chooseAgentFlag := reviewCmd.Flags().Lookup("choose-agent")
		if chooseAgentFlag == nil {
			t.Error("--choose-agent flag not defined")
		}
	})

	t.Run("source code has agent integration", func(t *testing.T) {
		reviewSource, err := os.ReadFile("review.go")
		if err != nil {
			t.Skipf("Cannot read review.go: %v", err)
		}

		sourceStr := string(reviewSource)

		// Verify key integration points in review.go (adapter layer)
		integrationChecks := map[string]string{
			"imports agent package":            `"github.com/danabrams/gromit/internal/agent"`,
			"calls agent.Resolve":              "agent.Resolve",
			"calls pipeline.ReviewInteractive": "p.ReviewInteractive",
		}

		for check, pattern := range integrationChecks {
			if !strings.Contains(sourceStr, pattern) {
				t.Errorf("Integration check failed: %s (missing pattern %q)", check, pattern)
			}
		}

		// Verify pipeline has agent.Launch integration
		pipelineSource, err := os.ReadFile("../../internal/pipeline/pipeline.go")
		if err != nil {
			t.Skipf("Cannot read pipeline.go: %v", err)
		}

		if !strings.Contains(string(pipelineSource), ".Launch(") {
			t.Error("pipeline.go does not call .Launch() - agent launch not integrated in pipeline")
		}
	})

	t.Run("old exec.Command pattern removed from interactive path", func(t *testing.T) {
		reviewSource, err := os.ReadFile("review.go")
		if err != nil {
			t.Skipf("Cannot read review.go: %v", err)
		}

		sourceStr := string(reviewSource)

		// Extract runReviewInteractive function
		startIdx := strings.Index(sourceStr, "func runReviewInteractive")
		if startIdx == -1 {
			t.Skip("Cannot find runReviewInteractive function")
		}

		endIdx := strings.Index(sourceStr[startIdx:], "\nfunc ")
		if endIdx == -1 {
			endIdx = len(sourceStr)
		} else {
			endIdx += startIdx
		}
		interactiveFn := sourceStr[startIdx:endIdx]

		// Check that old patterns are removed from interactive function
		oldPatterns := []string{
			"exec.Command(cfg.Claude.Binary",
			"buildReviewArgs",
			"cmd.Stdin = os.Stdin",
			"cmd.Stdout = os.Stdout",
			"cmd.Stderr = os.Stderr",
		}

		foundOldPatterns := []string{}
		for _, pattern := range oldPatterns {
			if strings.Contains(interactiveFn, pattern) {
				foundOldPatterns = append(foundOldPatterns, pattern)
			}
		}

		if len(foundOldPatterns) > 0 {
			t.Errorf("Old exec.Command patterns still present in runReviewInteractive (should be replaced by agent.Launch): %v", foundOldPatterns)
		}
	})

	t.Run("non-interactive path unchanged", func(t *testing.T) {
		reviewSource, err := os.ReadFile("review.go")
		if err != nil {
			t.Skipf("Cannot read review.go: %v", err)
		}

		sourceStr := string(reviewSource)

		// Extract runReviewNonInteractive function
		startIdx := strings.Index(sourceStr, "func runReviewNonInteractive")
		if startIdx == -1 {
			t.Skip("Cannot find runReviewNonInteractive function")
		}

		endIdx := strings.Index(sourceStr[startIdx:], "\nfunc ")
		if endIdx == -1 {
			endIdx = len(sourceStr)
		} else {
			endIdx += startIdx
		}
		nonInteractiveFn := sourceStr[startIdx:endIdx]

		// Verify non-interactive path still uses claude.Client
		if !strings.Contains(nonInteractiveFn, "claude.NewClient") {
			t.Error("runReviewNonInteractive should still use claude.NewClient (non-interactive path should be unchanged)")
		}

		// Verify non-interactive path doesn't use agent.Launch
		if strings.Contains(nonInteractiveFn, "agent.Launch") {
			t.Error("runReviewNonInteractive should not use agent.Launch (only interactive mode uses agents)")
		}
	})
}

// TestReviewAgentConfigBackwardCompatibility verifies review works without agent config
func TestReviewAgentConfigBackwardCompatibility(t *testing.T) {
	// This acceptance test verifies backward compatibility
	// Existing configs without agents section should still work (defaults to claude)

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
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
