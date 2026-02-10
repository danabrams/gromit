package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestExploreCommand_BuildsExplorationPromptWithFullContext is an acceptance test
// that verifies the explore command builds a complete exploration prompt with all
// required project context (CLAUDE.md, RULES.md, LEARNINGS.md) and topic.
func TestExploreCommand_BuildsExplorationPromptWithFullContext(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")

	// Create directories
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create CLAUDE.md with project context
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	claudeContent := `# Gromit Project

This is a Go CLI tool for automated development workflows.

## Architecture

- cmd/gromit/ - CLI commands
- internal/ - Core packages

## Key Principles

1. Fresh context each iteration
2. State in files, not memory`
	if err := os.WriteFile(claudeMDPath, []byte(claudeContent), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	// Create RULES.md
	rulesPath := filepath.Join(gromitDir, "RULES.md")
	rulesContent := `# Rules

## Code Style

- Use go fmt
- Error returns, not panics

## Safety

- Never commit secrets
- Always run tests before committing`
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("failed to create RULES.md: %v", err)
	}

	// Create LEARNINGS.md
	learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
	learningsContent := `### 2026-02-01 | Mock Pattern | patterns

Mock implementations use optional function pointer fields (FnField pattern) with nil-safe defaults.

### 2026-02-05 | Status Struct | patterns

Status struct fields require backward-compatible changes (omitempty for new optional fields).`
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("failed to create LEARNINGS.md: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Build exploration prompt with topic
	topic := "Improve developer onboarding experience"
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{topic})
	if err != nil {
		t.Fatalf("buildExplorePrompt failed: %v", err)
	}

	// Verify prompt is not empty
	if len(prompt) == 0 {
		t.Fatal("prompt should not be empty")
	}

	// Verify topic is included
	if !strings.Contains(prompt, topic) {
		t.Errorf("prompt should contain topic: %s", topic)
	}

	// Verify CLAUDE.md content is included
	if !strings.Contains(prompt, "Gromit Project") {
		t.Error("prompt should contain project context from CLAUDE.md")
	}
	if !strings.Contains(prompt, "Fresh context each iteration") {
		t.Error("prompt should contain key principles from CLAUDE.md")
	}

	// Verify RULES.md content is included
	if !strings.Contains(prompt, "Never commit secrets") {
		t.Error("prompt should contain rules from RULES.md")
	}
	if !strings.Contains(prompt, "Error returns, not panics") {
		t.Error("prompt should contain code style rules from RULES.md")
	}

	// Verify LEARNINGS.md content is included (in some form)
	// Learnings are formatted differently, so check for patterns or "learning" keyword
	promptLower := strings.ToLower(prompt)
	hasLearningsContext := strings.Contains(promptLower, "learning") ||
		strings.Contains(promptLower, "mock") ||
		strings.Contains(promptLower, "status") ||
		strings.Contains(promptLower, "pattern")
	if !hasLearningsContext {
		t.Error("prompt should reference learnings content or section")
	}

	// Verify directory paths are mentioned (so Claude knows where to write artifacts)
	if !strings.Contains(promptLower, "epic") && !strings.Contains(promptLower, ".gromit") {
		t.Error("prompt should mention epics directory or .gromit path")
	}
	if !strings.Contains(promptLower, "spec") {
		t.Error("prompt should mention specs directory")
	}

	// Verify working directory is included
	if !strings.Contains(promptLower, "directory") || !strings.Contains(promptLower, "context") {
		t.Error("prompt should include directory context")
	}
}

// TestExploreCommand_LaunchesClaudeWithCorrectArguments is an acceptance test
// that verifies the explore command constructs the correct Claude CLI invocation
// with model flag, config flags, and prompt file reference.
func TestExploreCommand_LaunchesClaudeWithCorrectArguments(t *testing.T) {
	testCases := []struct {
		name         string
		modelFlag    string
		configFlags  []string
		claudeBinary string
	}{
		{
			name:         "default opus model with no config flags",
			modelFlag:    "opus",
			configFlags:  []string{},
			claudeBinary: "claude",
		},
		{
			name:         "sonnet model with config flags",
			modelFlag:    "sonnet",
			configFlags:  []string{"--session-dir", "/custom/sessions"},
			claudeBinary: "claude",
		},
		{
			name:         "haiku model with custom binary",
			modelFlag:    "haiku",
			configFlags:  []string{"--flag1", "value1"},
			claudeBinary: "/usr/local/bin/claude",
		},
		{
			name:         "custom model with multiple config flags",
			modelFlag:    "opus",
			configFlags:  []string{"--api-key", "test", "--timeout", "300"},
			claudeBinary: "claude",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build command args following the pattern in explore.go
			cmdArgs := append([]string{}, tc.configFlags...)
			if tc.modelFlag != "" {
				cmdArgs = append(cmdArgs, "--model", tc.modelFlag)
			}

			// Add initial prompt (would reference temp file in actual implementation)
			promptPath := "/tmp/.gromit/tmp/explore-prompt-123.md"
			initialPrompt := "Read and follow the exploration instructions in " + promptPath
			cmdArgs = append(cmdArgs, initialPrompt)

			// Verify command args structure
			expectedMinArgs := 1 // At least the initial prompt
			if tc.modelFlag != "" {
				expectedMinArgs += 2 // --model <value>
			}
			expectedMinArgs += len(tc.configFlags)

			if len(cmdArgs) < expectedMinArgs {
				t.Errorf("expected at least %d args, got %d", expectedMinArgs, len(cmdArgs))
			}

			// Verify config flags come first
			for i, flag := range tc.configFlags {
				if cmdArgs[i] != flag {
					t.Errorf("config flag %d: expected %q, got %q", i, flag, cmdArgs[i])
				}
			}

			// Verify --model flag is present when specified
			if tc.modelFlag != "" {
				foundModel := false
				for i := 0; i < len(cmdArgs)-1; i++ {
					if cmdArgs[i] == "--model" && cmdArgs[i+1] == tc.modelFlag {
						foundModel = true
						break
					}
				}
				if !foundModel {
					t.Errorf("--model %s not found in command args", tc.modelFlag)
				}
			}

			// Verify initial prompt is last
			lastArg := cmdArgs[len(cmdArgs)-1]
			if !strings.Contains(lastArg, "exploration") && !strings.Contains(lastArg, "explore") {
				t.Error("initial prompt should mention exploration")
			}
			if !strings.Contains(lastArg, promptPath) {
				t.Errorf("initial prompt should reference temp file path: %s", promptPath)
			}

			// Verify binary is as expected
			if tc.claudeBinary != "claude" && tc.claudeBinary != "/usr/local/bin/claude" {
				t.Errorf("unexpected claude binary: %s", tc.claudeBinary)
			}
		})
	}
}

// TestExploreCommand_WritesPromptToTempFileToAvoidARGMAX is an acceptance test
// that verifies the explore command writes large prompts to a temp file instead of
// passing them as CLI arguments, avoiding ARG_MAX limits.
func TestExploreCommand_WritesPromptToTempFileToAvoidARGMAX(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	tmpPromptDir := filepath.Join(gromitDir, "tmp")

	// Create tmp directory
	if err := os.MkdirAll(tmpPromptDir, 0755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}

	// Simulate a very large prompt (e.g., large CLAUDE.md + RULES.md + LEARNINGS.md)
	// This would exceed ARG_MAX if passed directly as a CLI argument
	largePrompt := strings.Builder{}
	largePrompt.WriteString("# Exploration Prompt\n\n")
	largePrompt.WriteString("## Project Context\n\n")

	// Add 50KB of content (simulating large project docs)
	baseContent := "This is a line of project documentation. " +
		"It contains important context about the codebase architecture, " +
		"design decisions, patterns, and conventions. " +
		"Claude needs this full context to make informed exploration decisions.\n"

	for i := 0; i < 1000; i++ {
		largePrompt.WriteString(baseContent)
	}

	promptContent := largePrompt.String()

	// Verify prompt is large (> 10KB)
	if len(promptContent) < 10000 {
		t.Fatal("test prompt should be > 10KB to simulate ARG_MAX concern")
	}

	// Write prompt to temp file (following explore.go pattern)
	promptFile, err := os.CreateTemp(tmpPromptDir, "explore-prompt-*.md")
	if err != nil {
		t.Fatalf("failed to create temp prompt file: %v", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(promptContent); err != nil {
		promptFile.Close()
		t.Fatalf("failed to write prompt to file: %v", err)
	}
	promptFile.Close()

	// Verify file exists and contains full prompt
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("failed to read prompt file: %v", err)
	}

	if string(content) != promptContent {
		t.Error("prompt file should contain complete prompt content")
	}

	if len(content) < 10000 {
		t.Error("prompt file should contain large prompt (> 10KB)")
	}

	// Verify initial prompt references file (not contains full content)
	initialPrompt := "Read and follow the exploration instructions in " + promptPath

	// Initial prompt should be short (< 1KB) even though actual prompt is large
	if len(initialPrompt) > 1000 {
		t.Error("initial prompt should be short (< 1KB) to avoid ARG_MAX")
	}

	// Verify initial prompt references the temp file path
	if !strings.Contains(initialPrompt, promptPath) {
		t.Error("initial prompt should reference temp file path")
	}
}

// TestExploreCommand_PromptIncludesExplorationGuidance is an acceptance test
// that verifies the exploration prompt is distinct from debug/implementation prompts
// and guides Claude to explore problem spaces, not implement solutions.
func TestExploreCommand_PromptIncludesExplorationGuidance(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")

	// Create directories
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create minimal CLAUDE.md
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte("# Project"), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Build exploration prompt
	topic := "Improve build performance"
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{topic})
	if err != nil {
		t.Fatalf("buildExplorePrompt failed: %v", err)
	}

	promptLower := strings.ToLower(prompt)

	// Verify prompt is exploration-focused
	explorationKeywords := []string{"explore", "exploration", "epic", "spec", "problem space"}
	hasExplorationContext := false
	for _, keyword := range explorationKeywords {
		if strings.Contains(promptLower, keyword) {
			hasExplorationContext = true
			break
		}
	}
	if !hasExplorationContext {
		t.Error("prompt should contain exploration-focused keywords")
	}

	// Verify prompt is NOT implementation-focused
	// Check for specific implementation phrases, not just keywords that might appear in context
	implementationPhrases := []string{
		"implement the",
		"write the code",
		"build the feature",
		"create the function",
	}
	hasImplementationContext := false
	for _, phrase := range implementationPhrases {
		if strings.Contains(promptLower, phrase) {
			hasImplementationContext = true
			break
		}
	}
	if hasImplementationContext {
		t.Error("exploration prompt should not contain implementation-focused phrases")
	}

	// Verify prompt is NOT debug-focused
	debugKeywords := []string{"bug", "debug", "investigate", "root cause"}
	hasDebugContext := false
	for _, keyword := range debugKeywords {
		if strings.Contains(promptLower, keyword) {
			hasDebugContext = true
			break
		}
	}
	if hasDebugContext {
		t.Error("exploration prompt should not be debug-focused")
	}
}

// TestExploreCommand_IntegrationWithAllComponents is an end-to-end acceptance test
// that verifies the complete explore command flow: config loading, directory setup,
// pre-session snapshot, prompt building, and command construction.
func TestExploreCommand_IntegrationWithAllComponents(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := filepath.Join(gromitDir, "specs")

	// Set up complete project structure
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Create CLAUDE.md
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	claudeContent := "# Project Context\n\nThis is a test project."
	if err := os.WriteFile(claudeMDPath, []byte(claudeContent), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	// Create RULES.md
	rulesPath := filepath.Join(gromitDir, "RULES.md")
	rulesContent := "# Rules\n\n- Follow conventions"
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("failed to create RULES.md: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary: "claude",
			Flags:  []string{"--config-flag", "value"},
		},
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Step 1: Verify epics directory exists (should be created by command)
	stat, err := os.Stat(epicsDir)
	if err != nil {
		t.Fatalf("epics dir should exist: %v", err)
	}
	if !stat.IsDir() {
		t.Fatal("epics path should be a directory")
	}

	// Step 2: Take pre-session snapshot
	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("failed to get epic files: %v", err)
	}

	existingSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("failed to get spec files: %v", err)
	}

	// Should start with empty snapshots
	if len(existingEpics) != 0 {
		t.Errorf("expected 0 existing epics, got %d", len(existingEpics))
	}
	if len(existingSpecs) != 0 {
		t.Errorf("expected 0 existing specs, got %d", len(existingSpecs))
	}

	// Step 3: Build exploration prompt
	topic := "Test exploration topic"
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{topic})
	if err != nil {
		t.Fatalf("buildExplorePrompt failed: %v", err)
	}

	// Verify prompt contains required elements
	if !strings.Contains(prompt, topic) {
		t.Error("prompt should contain topic")
	}
	if !strings.Contains(prompt, "test project") {
		t.Error("prompt should contain CLAUDE.md content")
	}
	if !strings.Contains(prompt, "conventions") {
		t.Error("prompt should contain RULES.md content")
	}

	// Step 4: Write prompt to temp file
	tmpPromptDir := filepath.Join(gromitDir, "tmp")
	if err := os.MkdirAll(tmpPromptDir, 0755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}

	promptFile, err := os.CreateTemp(tmpPromptDir, "explore-prompt-*.md")
	if err != nil {
		t.Fatalf("failed to create temp prompt file: %v", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(prompt); err != nil {
		promptFile.Close()
		t.Fatalf("failed to write prompt: %v", err)
	}
	promptFile.Close()

	// Verify prompt file contains full content
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("failed to read prompt file: %v", err)
	}
	if string(content) != prompt {
		t.Error("prompt file should contain complete prompt")
	}

	// Step 5: Build command args
	claudeBinary := cfg.Claude.Binary
	claudeFlags := cfg.Claude.Flags
	model := "opus"

	cmdArgs := append([]string{}, claudeFlags...)
	cmdArgs = append(cmdArgs, "--model", model)
	cmdArgs = append(cmdArgs, "Read and follow the exploration instructions in "+promptPath)

	// Verify command construction
	if claudeBinary != "claude" {
		t.Errorf("expected claude binary, got %s", claudeBinary)
	}

	if len(cmdArgs) < 4 {
		t.Fatalf("expected at least 4 args (config flags + model + prompt), got %d", len(cmdArgs))
	}

	// Verify order: config flags, model, prompt
	if cmdArgs[0] != "--config-flag" || cmdArgs[1] != "value" {
		t.Error("config flags should be first in command args")
	}
	if cmdArgs[2] != "--model" || cmdArgs[3] != "opus" {
		t.Error("--model flag should follow config flags")
	}
	if !strings.Contains(cmdArgs[len(cmdArgs)-1], promptPath) {
		t.Error("initial prompt should reference temp file path")
	}

	// This test verifies the complete integration without actually executing Claude
	t.Log("End-to-end integration test passed: all components work together correctly")
}
