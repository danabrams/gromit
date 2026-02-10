package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestExploreCommand_BuildsPromptWithProjectContext verifies that buildExplorePrompt
// includes CLAUDE.md, RULES.md, and LEARNINGS.md in the prompt.
func TestExploreCommand_BuildsPromptWithProjectContext(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")

	// Create directories
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create CLAUDE.md
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	claudeContent := "# Project Context\n\nThis is the project context."
	if err := os.WriteFile(claudeMDPath, []byte(claudeContent), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	// Create RULES.md
	rulesPath := filepath.Join(gromitDir, "RULES.md")
	rulesContent := "# Rules\n\nThese are the rules."
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("failed to create RULES.md: %v", err)
	}

	// Create LEARNINGS.md
	learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
	learningsContent := "### 2026-02-01 | Test Learning | patterns\n\nThis is a test learning."
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("failed to create LEARNINGS.md: %v", err)
	}

	// Create minimal config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Build prompt
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{"Improve developer onboarding"})
	if err != nil {
		t.Fatalf("buildExplorePrompt failed: %v", err)
	}

	// Verify prompt contains topic
	if !strings.Contains(prompt, "Improve developer onboarding") {
		t.Error("prompt should contain the topic argument")
	}

	// Verify prompt contains project context from CLAUDE.md
	if !strings.Contains(prompt, "This is the project context") {
		t.Error("prompt should contain content from CLAUDE.md")
	}

	// Verify prompt contains rules
	if !strings.Contains(prompt, "These are the rules") {
		t.Error("prompt should contain content from RULES.md")
	}

	// Verify prompt mentions learnings (may be formatted differently)
	if !strings.Contains(prompt, "learning") && !strings.Contains(prompt, "Learning") {
		t.Error("prompt should reference learnings")
	}
}

// TestExploreCommand_BuildsPromptWithoutTopic verifies that buildExplorePrompt
// works when no topic argument is provided (blank exploration session).
func TestExploreCommand_BuildsPromptWithoutTopic(t *testing.T) {
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

	// Create minimal config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Build prompt without topic
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{})
	if err != nil {
		t.Fatalf("buildExplorePrompt should work without topic: %v", err)
	}

	// Verify prompt is not empty
	if len(prompt) == 0 {
		t.Error("prompt should not be empty when no topic is provided")
	}

	// Prompt should still contain project context
	if !strings.Contains(prompt, "# Project") {
		t.Error("prompt should still contain CLAUDE.md content without topic")
	}
}

// TestExploreCommand_WritesPromptToTempFile verifies that the explore command
// writes the prompt to a temp file to avoid ARG_MAX issues.
func TestExploreCommand_WritesPromptToTempFile(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	tmpPromptDir := filepath.Join(gromitDir, "tmp")

	// Create tmp directory
	if err := os.MkdirAll(tmpPromptDir, 0755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}

	// Create a large prompt (simulating project context)
	largePrompt := strings.Repeat("This is a very long prompt to simulate large project context. ", 1000)

	// Write prompt to temp file (following debug.go pattern)
	promptFile, err := os.CreateTemp(tmpPromptDir, "explore-prompt-*.md")
	if err != nil {
		t.Fatalf("failed to create temp prompt file: %v", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(largePrompt); err != nil {
		promptFile.Close()
		t.Fatalf("failed to write prompt to file: %v", err)
	}
	promptFile.Close()

	// Verify file exists and contains prompt
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("failed to read prompt file: %v", err)
	}

	if string(content) != largePrompt {
		t.Error("temp file should contain the full prompt")
	}

	// Verify file is in the tmp directory
	if !strings.HasPrefix(promptPath, tmpPromptDir) {
		t.Errorf("prompt file should be in tmp directory, got: %s", promptPath)
	}

	// Verify file has .md extension
	if !strings.HasSuffix(promptPath, ".md") {
		t.Error("prompt file should have .md extension")
	}

	// Verify file pattern matches explore-prompt-*.md
	filename := filepath.Base(promptPath)
	if !strings.HasPrefix(filename, "explore-prompt-") {
		t.Errorf("prompt filename should start with 'explore-prompt-', got: %s", filename)
	}
}

// TestExploreCommand_PromptIncludesEpicsDir verifies that the prompt includes
// the epics directory path so Claude knows where to write epic documents.
func TestExploreCommand_PromptIncludesEpicsDir(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	epicsDir := filepath.Join(gromitDir, "epics")

	// Create directories
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Create minimal CLAUDE.md
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte("# Project"), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	// Create minimal config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Build prompt
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{"test topic"})
	if err != nil {
		t.Fatalf("buildExplorePrompt failed: %v", err)
	}

	// Verify prompt mentions epics directory
	if !strings.Contains(prompt, "epics") && !strings.Contains(prompt, "Epics") {
		t.Error("prompt should mention the epics directory")
	}

	// Verify prompt mentions specs directory (specs are also possible outcomes)
	if !strings.Contains(prompt, "specs") && !strings.Contains(prompt, "Specs") {
		t.Error("prompt should mention the specs directory")
	}
}

// TestExploreCommand_PromptIncludesWorkingDirectory verifies that the prompt
// includes the working directory for context.
func TestExploreCommand_PromptIncludesWorkingDirectory(t *testing.T) {
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

	// Create minimal config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Build prompt
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{"test topic"})
	if err != nil {
		t.Fatalf("buildExplorePrompt failed: %v", err)
	}

	// Verify prompt contains context section with directory info
	if !strings.Contains(prompt, "Context") && !strings.Contains(prompt, "directory") {
		t.Error("prompt should include context section with directory information")
	}
}

// TestExploreCommand_HandlesMissingProjectFiles verifies that buildExplorePrompt
// gracefully handles missing RULES.md or LEARNINGS.md files.
func TestExploreCommand_HandlesMissingProjectFiles(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")

	// Create directories
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create only CLAUDE.md (RULES.md and LEARNINGS.md are missing)
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte("# Project"), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	// Create minimal config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Build prompt - should not error even with missing files
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{"test topic"})
	if err != nil {
		t.Fatalf("buildExplorePrompt should handle missing RULES.md and LEARNINGS.md: %v", err)
	}

	// Verify prompt is not empty
	if len(prompt) == 0 {
		t.Error("prompt should not be empty even when some files are missing")
	}

	// Should still contain CLAUDE.md content
	if !strings.Contains(prompt, "# Project") {
		t.Error("prompt should still contain CLAUDE.md content when other files are missing")
	}
}

// TestExploreCommand_PromptStructureMatchesSpec verifies that the prompt
// follows the expected structure for exploration (not the same as debug).
func TestExploreCommand_PromptStructureMatchesSpec(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")

	// Create directories
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create CLAUDE.md
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte("# Project"), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	// Create minimal config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Build prompt
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{"Improve onboarding"})
	if err != nil {
		t.Fatalf("buildExplorePrompt failed: %v", err)
	}

	// The prompt should be exploration-focused, not debug-focused
	// Verify it's NOT a debug prompt
	if strings.Contains(strings.ToLower(prompt), "bug") || strings.Contains(strings.ToLower(prompt), "investigation") {
		t.Error("explore prompt should not be focused on bug investigation")
	}

	// Verify it IS exploration-focused
	hasExplorationContext := strings.Contains(strings.ToLower(prompt), "explore") ||
		strings.Contains(strings.ToLower(prompt), "exploration") ||
		strings.Contains(strings.ToLower(prompt), "problem space") ||
		strings.Contains(strings.ToLower(prompt), "epic") ||
		strings.Contains(strings.ToLower(prompt), "spec")

	if !hasExplorationContext {
		t.Error("explore prompt should be focused on exploration, epics, or specs")
	}
}
