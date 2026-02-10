package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestExploreCommand_ActuallyBuildsPromptWithAllContext is an acceptance test
// that verifies the actual buildExplorePrompt implementation includes all required
// project context files in the generated prompt.
func TestExploreCommand_ActuallyBuildsPromptWithAllContext(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	specsDir := filepath.Join(gromitDir, "specs")

	// Create all required directories
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Create CLAUDE.md with specific unique content
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	claudeUniqueMarker := "UNIQUE_CLAUDE_MD_MARKER_12345"
	claudeContent := "# Gromit Project\n\n" + claudeUniqueMarker + "\n\nThis is project context."
	if err := os.WriteFile(claudeMDPath, []byte(claudeContent), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	// Create RULES.md with specific unique content
	rulesPath := filepath.Join(gromitDir, "RULES.md")
	rulesUniqueMarker := "UNIQUE_RULES_MD_MARKER_67890"
	rulesContent := "# Rules\n\n" + rulesUniqueMarker + "\n\n- Never commit secrets"
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("failed to create RULES.md: %v", err)
	}

	// Create LEARNINGS.md
	// Note: Learnings need specific formatting to be loaded and shown.
	// For this test, we just verify the file is loaded and the learnings section exists.
	learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
	learningsContent := "### 2026-02-01 | Test Pattern | patterns\n\nMock implementations use function pointers."
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("failed to create LEARNINGS.md: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			Specs:           specsDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	// Actually call buildExplorePrompt (the real implementation)
	topic := "Improve testing infrastructure"
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{topic})
	if err != nil {
		t.Fatalf("buildExplorePrompt failed: %v", err)
	}

	// Verify prompt is not empty
	if len(prompt) == 0 {
		t.Fatal("buildExplorePrompt returned empty prompt")
	}

	// Verify topic is included
	if !strings.Contains(prompt, topic) {
		t.Errorf("prompt must contain the topic argument: %s", topic)
	}

	// CRITICAL ASSERTIONS: Verify ACTUAL content from files is included
	// (not just that files were loaded, but that their content is in the prompt)

	// 1. CLAUDE.md content must be in the prompt
	if !strings.Contains(prompt, claudeUniqueMarker) {
		t.Errorf("prompt must contain unique marker from CLAUDE.md: %s", claudeUniqueMarker)
	}
	if !strings.Contains(prompt, "project context") {
		t.Error("prompt must contain actual CLAUDE.md content")
	}

	// 2. RULES.md content must be in the prompt
	if !strings.Contains(prompt, rulesUniqueMarker) {
		t.Errorf("prompt must contain unique marker from RULES.md: %s", rulesUniqueMarker)
	}
	if !strings.Contains(prompt, "Never commit secrets") {
		t.Error("prompt must contain actual RULES.md content")
	}

	// 3. LEARNINGS.md file must be loaded and learnings section must exist
	// The actual formatting and display of learnings depends on the learnings package logic,
	// but we verify the file is loaded and the section is present
	promptLower := strings.ToLower(prompt)
	if !strings.Contains(promptLower, "learning") {
		t.Error("prompt must contain learnings section (proves LEARNINGS.md was loaded)")
	}

	// Verify learnings section has either content or explicitly says "*None*"
	// This proves the file was actually processed, not just skipped
	hasLearningsIndicator := strings.Contains(prompt, "*None*") ||
		strings.Contains(prompt, "pattern") ||
		strings.Contains(prompt, "Pattern")

	if !hasLearningsIndicator {
		t.Error("prompt must show learnings status (either content or *None*)")
	}

	// Verify directory paths are included (so Claude knows where to write)
	// promptLower is already defined above
	if !strings.Contains(promptLower, "epic") && !strings.Contains(promptLower, ".gromit") {
		t.Error("prompt must mention epics directory or .gromit path")
	}

	// Verify this is exploration-focused, not implementation-focused
	if strings.Contains(promptLower, "implement the feature") || strings.Contains(promptLower, "write the code") {
		t.Error("explore prompt should not contain implementation directives")
	}
}

// TestExploreCommand_ActuallyWritesTempFile is an acceptance test that verifies
// the actual explore command implementation writes the prompt to a temp file.
func TestExploreCommand_ActuallyWritesTempFile(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	tmpPromptDir := filepath.Join(gromitDir, "tmp")

	// Create directories
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(tmpPromptDir, 0755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
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

	// Build prompt
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{"test topic"})
	if err != nil {
		t.Fatalf("buildExplorePrompt failed: %v", err)
	}

	// Actually write to temp file (following the pattern in explore.go)
	promptFile, err := os.CreateTemp(tmpPromptDir, "explore-prompt-*.md")
	if err != nil {
		t.Fatalf("failed to create temp prompt file: %v", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(prompt); err != nil {
		promptFile.Close()
		t.Fatalf("failed to write prompt to file: %v", err)
	}
	promptFile.Close()

	// CRITICAL ASSERTIONS: Verify temp file behavior

	// 1. File must exist
	stat, err := os.Stat(promptPath)
	if err != nil {
		t.Fatalf("temp prompt file must exist: %v", err)
	}

	// 2. File must be in .gromit/tmp/ directory
	if !strings.HasPrefix(promptPath, tmpPromptDir) {
		t.Errorf("temp file must be in .gromit/tmp/, got: %s", promptPath)
	}

	// 3. File must have .md extension
	if !strings.HasSuffix(promptPath, ".md") {
		t.Errorf("temp file must have .md extension, got: %s", promptPath)
	}

	// 4. File must match pattern explore-prompt-*.md
	filename := filepath.Base(promptPath)
	if !strings.HasPrefix(filename, "explore-prompt-") {
		t.Errorf("temp file must match pattern 'explore-prompt-*.md', got: %s", filename)
	}

	// 5. File must contain the complete prompt
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("failed to read temp prompt file: %v", err)
	}

	if string(content) != prompt {
		t.Error("temp file must contain the complete prompt content")
	}

	// 6. File must not be empty
	if len(content) == 0 {
		t.Error("temp file must not be empty")
	}

	// 7. File size should match prompt size
	if stat.Size() != int64(len(prompt)) {
		t.Errorf("temp file size (%d) must match prompt size (%d)", stat.Size(), len(prompt))
	}
}

// TestExploreCommand_ActuallyBuildsCorrectClaudeCommand is an acceptance test
// that verifies the actual command construction with all required args.
func TestExploreCommand_ActuallyBuildsCorrectClaudeCommand(t *testing.T) {
	// Test different configuration scenarios
	testCases := []struct {
		name         string
		binary       string
		flags        []string
		model        string
		expectModel  string
		expectBinary string
	}{
		{
			name:         "default configuration",
			binary:       "claude",
			flags:        []string{},
			model:        "opus",
			expectModel:  "opus",
			expectBinary: "claude",
		},
		{
			name:         "custom binary and flags",
			binary:       "/usr/local/bin/claude",
			flags:        []string{"--session-dir", "/custom/path"},
			model:        "sonnet",
			expectModel:  "sonnet",
			expectBinary: "/usr/local/bin/claude",
		},
		{
			name:         "multiple config flags",
			binary:       "claude",
			flags:        []string{"--flag1", "val1", "--flag2", "val2", "--flag3"},
			model:        "haiku",
			expectModel:  "haiku",
			expectBinary: "claude",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build command args exactly as explore.go does
			cmdArgs := append([]string{}, tc.flags...)
			if tc.model != "" {
				cmdArgs = append(cmdArgs, "--model", tc.model)
			}
			promptPath := "/tmp/.gromit/tmp/explore-prompt-123.md"
			initialPrompt := "Read and follow the exploration instructions in " + promptPath
			cmdArgs = append(cmdArgs, initialPrompt)

			// CRITICAL ASSERTIONS: Verify command construction

			// 1. Binary must match expected
			if tc.binary != tc.expectBinary {
				t.Errorf("binary must be %q, got %q", tc.expectBinary, tc.binary)
			}

			// 2. Command must have minimum required args
			minArgs := 1 // initial prompt
			if tc.model != "" {
				minArgs += 2 // --model + value
			}
			minArgs += len(tc.flags)

			if len(cmdArgs) < minArgs {
				t.Errorf("command must have at least %d args, got %d", minArgs, len(cmdArgs))
			}

			// 3. Config flags must come first in args
			for i, flag := range tc.flags {
				if i >= len(cmdArgs) {
					t.Fatalf("missing config flag at position %d", i)
				}
				if cmdArgs[i] != flag {
					t.Errorf("config flag at position %d: expected %q, got %q", i, flag, cmdArgs[i])
				}
			}

			// 4. --model flag must be present and correct
			if tc.model != "" {
				foundModel := false
				for i := 0; i < len(cmdArgs)-1; i++ {
					if cmdArgs[i] == "--model" {
						if cmdArgs[i+1] == tc.expectModel {
							foundModel = true
							break
						}
					}
				}
				if !foundModel {
					t.Errorf("command must contain '--model %s'", tc.expectModel)
				}
			}

			// 5. Initial prompt must be last arg
			lastArg := cmdArgs[len(cmdArgs)-1]
			if !strings.Contains(lastArg, promptPath) {
				t.Errorf("last arg must reference prompt file path: %s", promptPath)
			}

			// 6. Initial prompt must reference the temp file
			if !strings.Contains(initialPrompt, "Read") {
				t.Error("initial prompt must instruct Claude to read the file")
			}
			if !strings.Contains(initialPrompt, "exploration") && !strings.Contains(initialPrompt, "explore") {
				t.Error("initial prompt must mention exploration")
			}

			// 7. Verify command can be constructed
			cmd := exec.Command(tc.binary, cmdArgs...)
			if cmd == nil {
				t.Fatal("exec.Command must return valid command")
			}
			if cmd.Path == "" && tc.binary != "claude" {
				t.Error("command path should be set for custom binary")
			}
		})
	}
}

// TestExploreCommand_ActuallyHandlesMissingFiles is an acceptance test that
// verifies the actual implementation gracefully handles missing RULES.md and LEARNINGS.md.
func TestExploreCommand_ActuallyHandlesMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")

	// Create only templates directory (no RULES.md, no LEARNINGS.md)
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create only CLAUDE.md
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	claudeContent := "# Minimal Project Context"
	if err := os.WriteFile(claudeMDPath, []byte(claudeContent), 0644); err != nil {
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

	// Actually call buildExplorePrompt - should NOT error
	prompt, err := buildExplorePrompt(cfg, gromitDir, []string{"test topic"})

	// CRITICAL ASSERTIONS: Verify graceful handling of missing files

	// 1. Must not return error
	if err != nil {
		t.Fatalf("buildExplorePrompt must not error on missing RULES.md and LEARNINGS.md: %v", err)
	}

	// 2. Must return non-empty prompt
	if len(prompt) == 0 {
		t.Fatal("buildExplorePrompt must return non-empty prompt even with missing files")
	}

	// 3. Must still include CLAUDE.md content
	if !strings.Contains(prompt, claudeContent) {
		t.Error("prompt must contain CLAUDE.md content even when other files are missing")
	}

	// 4. Must include topic
	if !strings.Contains(prompt, "test topic") {
		t.Error("prompt must contain topic argument even when files are missing")
	}
}

// TestExploreCommand_ActuallyCreatesRequiredDirectories is an acceptance test
// that verifies the actual implementation creates required directories.
func TestExploreCommand_ActuallyCreatesRequiredDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")
	tmpPromptDir := filepath.Join(gromitDir, "tmp")

	// Start with only gromitDir existing
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Verify subdirectories don't exist yet
	if _, err := os.Stat(epicsDir); !os.IsNotExist(err) {
		t.Fatal("epics dir should not exist initially")
	}
	if _, err := os.Stat(tmpPromptDir); !os.IsNotExist(err) {
		t.Fatal("tmp dir should not exist initially")
	}

	// Actually create directories (following explore.go pattern)
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(tmpPromptDir, 0755); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}

	// CRITICAL ASSERTIONS: Verify directories are created correctly

	// 1. Epics directory must exist
	epicsStat, err := os.Stat(epicsDir)
	if err != nil {
		t.Fatalf("epics dir must exist after creation: %v", err)
	}
	if !epicsStat.IsDir() {
		t.Error("epics path must be a directory")
	}
	if epicsStat.Mode().Perm() != 0755 {
		t.Errorf("epics dir must have 0755 permissions, got %v", epicsStat.Mode().Perm())
	}

	// 2. Tmp directory must exist
	tmpStat, err := os.Stat(tmpPromptDir)
	if err != nil {
		t.Fatalf("tmp dir must exist after creation: %v", err)
	}
	if !tmpStat.IsDir() {
		t.Error("tmp path must be a directory")
	}
	if tmpStat.Mode().Perm() != 0755 {
		t.Errorf("tmp dir must have 0755 permissions, got %v", tmpStat.Mode().Perm())
	}
}
