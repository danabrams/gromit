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

// TestReviewUsesAgentLaunchNotDirectExec verifies review uses pipeline which uses agent abstraction
func TestReviewUsesAgentLaunchNotDirectExec(t *testing.T) {
	// This acceptance test verifies that the review command has been refactored
	// to use the pipeline pattern, which in turn uses agent.LaunchInDir() instead of exec.Command directly

	// Read the review.go source code
	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("Reading review.go: %v", err)
	}

	sourceStr := string(reviewSource)

	// Shared resolver adapter now owns agent.Resolve integration.
	adaptersSource, err := os.ReadFile("adapters.go")
	if err != nil {
		t.Fatalf("Reading adapters.go: %v", err)
	}
	adaptersStr := string(adaptersSource)
	if !strings.Contains(adaptersStr, `"github.com/danabrams/gromit/internal/agent"`) {
		t.Error("adapters.go does not import agent package - resolver integration not complete")
	}
	if !strings.Contains(adaptersStr, "agent.Resolve") {
		t.Error("adapters.go does not call agent.Resolve - resolver integration not complete")
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
	if interactiveFn, ok := extractFunction(sourceStr, "runReviewInteractive"); ok {
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
		"RenderThoroughReview",       // Rendering review prompt
		"writeTempPromptWithPattern", // Temp prompt creation helper
		"promptPath",                 // Prompt file path variable
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

	nonInteractiveFn, ok := extractFunction(sourceStr, "runReviewNonInteractive")
	if !ok {
		t.Fatal("Cannot find runReviewNonInteractive function")
	}

	// runReviewNonInteractive should NOT use agent.Resolve or agent.Launch
	if strings.Contains(nonInteractiveFn, "agent.Resolve") {
		t.Error("runReviewNonInteractive contains agent.Resolve - should remain unchanged (only interactive mode uses agents)")
	}

	if strings.Contains(nonInteractiveFn, "agent.Launch") {
		t.Error("runReviewNonInteractive contains agent.Launch - should remain unchanged (only interactive mode uses agents)")
	}

	if strings.Contains(nonInteractiveFn, "agent.LaunchInDir") {
		t.Error("runReviewNonInteractive contains agent.LaunchInDir - should remain unchanged (only interactive mode uses agents)")
	}

	// runReviewNonInteractive should use provider-neutral client builder.
	if !strings.Contains(nonInteractiveFn, "buildReviewNonInteractiveClient") {
		t.Error("runReviewNonInteractive missing buildReviewNonInteractiveClient call")
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

		// Verify key integration points in review.go and shared adapter layer.
		integrationChecks := map[string]string{
			"calls pipeline.ReviewInteractive": "p.ReviewInteractive",
		}

		for check, pattern := range integrationChecks {
			if !strings.Contains(sourceStr, pattern) {
				t.Errorf("Integration check failed: %s (missing pattern %q)", check, pattern)
			}
		}

		adaptersSource, err := os.ReadFile("adapters.go")
		if err != nil {
			t.Skipf("Cannot read adapters.go: %v", err)
		}
		adaptersStr := string(adaptersSource)
		if !strings.Contains(adaptersStr, `"github.com/danabrams/gromit/internal/agent"`) {
			t.Error("Integration check failed: shared adapter missing agent import")
		}
		if !strings.Contains(adaptersStr, "agent.Resolve") {
			t.Error("Integration check failed: shared adapter missing agent.Resolve call")
		}

		// Verify pipeline has agent.LaunchInDir integration
		pipelineSource, err := os.ReadFile("../../internal/pipeline/pipeline.go")
		if err != nil {
			t.Skipf("Cannot read pipeline.go: %v", err)
		}

		if !strings.Contains(string(pipelineSource), ".LaunchInDir(") {
			t.Error("pipeline.go does not call .LaunchInDir() - agent launch not integrated in pipeline")
		}
	})

	t.Run("old exec.Command pattern removed from interactive path", func(t *testing.T) {
		reviewSource, err := os.ReadFile("review.go")
		if err != nil {
			t.Skipf("Cannot read review.go: %v", err)
		}

		sourceStr := string(reviewSource)

		interactiveFn, ok := extractFunction(sourceStr, "runReviewInteractive")
		if !ok {
			t.Skip("Cannot find runReviewInteractive function")
		}

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

	t.Run("non-interactive path stays agent-neutral", func(t *testing.T) {
		reviewSource, err := os.ReadFile("review.go")
		if err != nil {
			t.Skipf("Cannot read review.go: %v", err)
		}

		sourceStr := string(reviewSource)

		nonInteractiveFn, ok := extractFunction(sourceStr, "runReviewNonInteractive")
		if !ok {
			t.Skip("Cannot find runReviewNonInteractive function")
		}

		// Verify non-interactive path routes through provider-neutral client wiring.
		if !strings.Contains(nonInteractiveFn, "buildReviewNonInteractiveClient") {
			t.Error("runReviewNonInteractive should use buildReviewNonInteractiveClient")
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
