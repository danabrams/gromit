//go:build acceptance

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/skills"
)

func runInstallSkillInDir(t *testing.T, projectDir string, force bool) (string, error) {
	t.Helper()

	t.Chdir(projectDir)

	oldForce := forceInstallSkill
	forceInstallSkill = force
	defer func() { forceInstallSkill = oldForce }()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	runErr := runInstallSkill(nil, nil)
	_ = w.Close()

	out, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		return "", readErr
	}
	return string(out), runErr
}

// TestInstallSkillIntegrationFullCommand tests running the full install-skill command
// in a temp directory and verifying all artifacts are created correctly.
func TestInstallSkillIntegrationFullCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test project directory
	projectDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Run install-skill in the test project directory
	output, err := runInstallSkillInDir(t, projectDir, false)
	if err != nil {
		t.Fatalf("install-skill command failed: %v\nOutput: %s", err, output)
	}

	// Verify the hook script was created
	hookScriptPath := filepath.Join(projectDir, ".gromit", "hooks", "pipeline-resume.sh")
	hookInfo, err := os.Stat(hookScriptPath)
	if err != nil {
		t.Errorf("hook script not created: %v", err)
	} else {
		// Verify it's executable
		if hookInfo.Mode()&0111 == 0 {
			t.Error("hook script is not executable")
		}

		// Verify content is non-empty
		hookContent, err := os.ReadFile(hookScriptPath)
		if err != nil {
			t.Errorf("failed to read hook script: %v", err)
		} else if len(hookContent) == 0 {
			t.Error("hook script is empty")
		}
	}

	// Verify the skill file was created
	skillPath := filepath.Join(projectDir, ".claude", "skills", "gromit.md")
	skillInfo, err := os.Stat(skillPath)
	if err != nil {
		t.Errorf("skill file not created: %v", err)
	} else {
		// Verify it's readable
		if skillInfo.Mode()&0400 == 0 {
			t.Error("skill file is not readable")
		}

		// Verify content contains embedded skills
		skillContent, err := os.ReadFile(skillPath)
		if err != nil {
			t.Errorf("failed to read skill file: %v", err)
		} else {
			skillStr := string(skillContent)
			if !strings.Contains(skillStr, "<!-- BEGIN GROMIT-REFINE-SKILL -->") {
				t.Error("skill file missing refine skill marker")
			}
			if !strings.Contains(skillStr, "<!-- BEGIN GROMIT-PLAN-SKILL -->") {
				t.Error("skill file missing plan skill marker")
			}
			if !strings.Contains(skillStr, "<!-- BEGIN GROMIT-DECOMPOSE-SKILL -->") {
				t.Error("skill file missing decompose skill marker")
			}
			// Verify actual skill content is present (not just markers)
			if !strings.Contains(skillStr, skills.RefineSkill[:100]) {
				t.Error("skill file missing refine skill content")
			}
		}
	}

	// Verify settings.json was created and contains the hook
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	settingsInfo, err := os.Stat(settingsPath)
	if err != nil {
		t.Errorf("settings.json not created: %v", err)
	} else {
		// Verify it's readable
		if settingsInfo.Mode()&0400 == 0 {
			t.Error("settings.json is not readable")
		}

		// Verify hook is registered
		settingsContent, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Errorf("failed to read settings.json: %v", err)
		} else {
			var settings testClaudeSettings
			if err := json.Unmarshal(settingsContent, &settings); err != nil {
				t.Errorf("settings.json is not valid JSON: %v", err)
			} else {
				// Verify SessionStart hook exists
				sessionStart, exists := settings.Hooks["SessionStart"]
				if !exists {
					t.Error("SessionStart hook not found in settings.json")
				} else {
					// Find clear matcher
					clearMatcherFound := false
					for _, matcher := range sessionStart {
						if matcher.Matcher == "clear" {
							clearMatcherFound = true
							// Verify our hook is present
							hookFound := false
							for _, hook := range matcher.Hooks {
								if hook.Command == ".gromit/hooks/pipeline-resume.sh" {
									hookFound = true
									break
								}
							}
							if !hookFound {
								t.Error("pipeline-resume.sh hook not found in clear matcher")
							}
							break
						}
					}
					if !clearMatcherFound {
						t.Error("clear matcher not found in SessionStart hooks")
					}
				}
			}
		}
	}

	// Verify output contains success message
	outputStr := output
	if !strings.Contains(outputStr, "Installation complete") {
		t.Error("output missing success message")
	}
}

// TestInstallSkillIntegrationIdempotency verifies that running install-skill
// multiple times doesn't duplicate hooks and preserves existing files.
func TestInstallSkillIntegrationIdempotency(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test project directory
	projectDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Run install-skill first time
	output1, err := runInstallSkillInDir(t, projectDir, false)
	if err != nil {
		t.Fatalf("first install-skill failed: %v\nOutput: %s", err, output1)
	}

	// Read settings.json after first run
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	settings1, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json after first run: %v", err)
	}

	// Parse first settings
	var firstSettings testClaudeSettings
	if err := json.Unmarshal(settings1, &firstSettings); err != nil {
		t.Fatalf("failed to parse first settings: %v", err)
	}

	// Count hooks in first run
	firstHookCount := 0
	for _, matcher := range firstSettings.Hooks["SessionStart"] {
		if matcher.Matcher == "clear" {
			firstHookCount = len(matcher.Hooks)
			break
		}
	}

	if firstHookCount != 1 {
		t.Fatalf("expected 1 hook after first run, got %d", firstHookCount)
	}

	// Modify the hook script slightly to verify it's not overwritten
	hookScriptPath := filepath.Join(projectDir, ".gromit", "hooks", "pipeline-resume.sh")
	originalHookContent, err := os.ReadFile(hookScriptPath)
	if err != nil {
		t.Fatalf("failed to read hook script: %v", err)
	}
	modifiedContent := append([]byte("# Modified\n"), originalHookContent...)
	if err := os.WriteFile(hookScriptPath, modifiedContent, 0755); err != nil {
		t.Fatalf("failed to modify hook script: %v", err)
	}

	// Run install-skill second time
	output2, err := runInstallSkillInDir(t, projectDir, false)
	if err != nil {
		t.Fatalf("second install-skill failed: %v\nOutput: %s", err, output2)
	}

	// Verify hook script was not overwritten (should see "Skipped" message)
	output2Str := output2
	if !strings.Contains(output2Str, "Skipped") {
		t.Error("expected 'Skipped' message for existing hook script")
	}

	// Verify modification is preserved
	hookContentAfter, err := os.ReadFile(hookScriptPath)
	if err != nil {
		t.Fatalf("failed to read hook script after second run: %v", err)
	}
	if !strings.HasPrefix(string(hookContentAfter), "# Modified\n") {
		t.Error("hook script was overwritten (modification lost)")
	}

	// Read settings.json after second run
	settings2, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json after second run: %v", err)
	}

	// Parse second settings
	var secondSettings testClaudeSettings
	if err := json.Unmarshal(settings2, &secondSettings); err != nil {
		t.Fatalf("failed to parse second settings: %v", err)
	}

	// Count hooks in second run
	secondHookCount := 0
	for _, matcher := range secondSettings.Hooks["SessionStart"] {
		if matcher.Matcher == "clear" {
			secondHookCount = len(matcher.Hooks)
			break
		}
	}

	// Verify hook count is still 1 (not duplicated)
	if secondHookCount != 1 {
		t.Errorf("expected 1 hook after second run (idempotency), got %d", secondHookCount)
	}
}

// TestInstallSkillIntegrationForceFlag verifies that the --force flag
// overwrites existing files when specified.
func TestInstallSkillIntegrationForceFlag(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test project directory
	projectDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Run install-skill first time
	if output, err := runInstallSkillInDir(t, projectDir, false); err != nil {
		t.Fatalf("first install-skill failed: %v\nOutput: %s", err, output)
	}

	// Modify the hook script
	hookScriptPath := filepath.Join(projectDir, ".gromit", "hooks", "pipeline-resume.sh")
	originalContent, err := os.ReadFile(hookScriptPath)
	if err != nil {
		t.Fatalf("failed to read hook script: %v", err)
	}
	modifiedContent := []byte("# Custom hook\necho 'custom'")
	if err := os.WriteFile(hookScriptPath, modifiedContent, 0755); err != nil {
		t.Fatalf("failed to modify hook script: %v", err)
	}

	// Verify modification was applied
	afterMod, err := os.ReadFile(hookScriptPath)
	if err != nil {
		t.Fatalf("failed to read modified hook script: %v", err)
	}
	if string(afterMod) != string(modifiedContent) {
		t.Fatal("modification was not applied")
	}

	// Modify the skill file
	skillPath := filepath.Join(projectDir, ".claude", "skills", "gromit.md")
	modifiedSkillContent := []byte("# Custom skill")
	if err := os.WriteFile(skillPath, modifiedSkillContent, 0644); err != nil {
		t.Fatalf("failed to modify skill file: %v", err)
	}

	// Run install-skill with --force flag
	output2, err := runInstallSkillInDir(t, projectDir, true)
	if err != nil {
		t.Fatalf("install-skill --force failed: %v\nOutput: %s", err, output2)
	}

	// Verify hook script was overwritten (should NOT see "Skipped" message)
	output2Str := output2
	if strings.Contains(output2Str, "Skipped pipeline-resume.sh") {
		t.Error("expected hook script to be overwritten with --force, but it was skipped")
	}

	// Verify hook script content is restored to original
	hookContentAfter, err := os.ReadFile(hookScriptPath)
	if err != nil {
		t.Fatalf("failed to read hook script after --force: %v", err)
	}
	if string(hookContentAfter) == string(modifiedContent) {
		t.Error("hook script was not overwritten by --force (modification still present)")
	}
	if string(hookContentAfter) != string(originalContent) {
		// Content should match the original embedded content
		if !strings.Contains(string(hookContentAfter), "#!/bin/bash") {
			t.Error("hook script content doesn't look like pipeline-resume.sh")
		}
	}

	// Verify skill file was overwritten
	skillContentAfter, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read skill file after --force: %v", err)
	}
	if string(skillContentAfter) == string(modifiedSkillContent) {
		t.Error("skill file was not overwritten by --force (modification still present)")
	}
	if !strings.Contains(string(skillContentAfter), "<!-- BEGIN GROMIT-REFINE-SKILL -->") {
		t.Error("skill file doesn't contain expected content after --force")
	}
}

// TestInstallSkillIntegrationPreservesExistingHooks verifies that install-skill
// preserves existing hooks in settings.json when adding the gromit hook.
func TestInstallSkillIntegrationPreservesExistingHooks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test project directory
	projectDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Create .claude directory with existing settings.json
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	// Create settings.json with existing hooks AND non-hook fields
	existingSettings := `{
  "permissions": {
    "allow": ["Read", "Write"],
    "deny": ["Bash"]
  },
  "allowedTools": ["grep", "find"],
  "model": "claude-sonnet-4-5-20250929",
  "hooks": {
    "SessionStart": [
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": "existing-hook.sh"
          }
        ]
      }
    ],
    "PreCommand": [
      {
        "matcher": "test",
        "hooks": [
          {
            "type": "command",
            "command": "pre-command.sh"
          }
        ]
      }
    ]
  }
}`
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(existingSettings), 0644); err != nil {
		t.Fatalf("failed to create existing settings.json: %v", err)
	}

	// Run install-skill
	output, err := runInstallSkillInDir(t, projectDir, false)
	if err != nil {
		t.Fatalf("install-skill failed: %v\nOutput: %s", err, output)
	}

	// Read settings.json after install-skill
	settingsContent, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	// Parse settings as generic map to verify all fields
	var rawSettings map[string]json.RawMessage
	if err := json.Unmarshal(settingsContent, &rawSettings); err != nil {
		t.Fatalf("failed to parse settings.json: %v", err)
	}

	// Verify non-hook fields are preserved
	if _, exists := rawSettings["permissions"]; !exists {
		t.Error("permissions field was not preserved")
	}
	if _, exists := rawSettings["allowedTools"]; !exists {
		t.Error("allowedTools field was not preserved")
	}
	if _, exists := rawSettings["model"]; !exists {
		t.Error("model field was not preserved")
	}

	// Verify model value
	var model string
	if err := json.Unmarshal(rawSettings["model"], &model); err != nil {
		t.Fatalf("failed to parse model field: %v", err)
	}
	if model != "claude-sonnet-4-5-20250929" {
		t.Errorf("model field changed: expected 'claude-sonnet-4-5-20250929', got %q", model)
	}

	// Parse hooks to verify hook-specific behavior
	var settings testClaudeSettings
	if err := json.Unmarshal(settingsContent, &settings); err != nil {
		t.Fatalf("failed to parse settings.json into testClaudeSettings: %v", err)
	}

	// Verify PreCommand hooks are preserved
	preCommand, exists := settings.Hooks["PreCommand"]
	if !exists {
		t.Error("PreCommand hooks were not preserved")
	} else {
		if len(preCommand) != 1 || preCommand[0].Matcher != "test" {
			t.Error("PreCommand hooks were modified")
		}
		if len(preCommand[0].Hooks) != 1 || preCommand[0].Hooks[0].Command != "pre-command.sh" {
			t.Error("PreCommand hook content was modified")
		}
	}

	// Verify SessionStart hooks contain both existing and new hooks
	sessionStart, exists := settings.Hooks["SessionStart"]
	if !exists {
		t.Fatal("SessionStart hooks not found")
	}

	// Find clear matcher
	var clearMatcher *hookMatcher
	for i := range sessionStart {
		if sessionStart[i].Matcher == "clear" {
			clearMatcher = &sessionStart[i]
			break
		}
	}
	if clearMatcher == nil {
		t.Fatal("clear matcher not found in SessionStart")
	}

	// Verify both hooks are present
	if len(clearMatcher.Hooks) != 2 {
		t.Fatalf("expected 2 hooks in clear matcher, got %d", len(clearMatcher.Hooks))
	}

	// Verify existing hook is preserved
	existingHookFound := false
	gromitHookFound := false
	for _, hook := range clearMatcher.Hooks {
		if hook.Command == "existing-hook.sh" {
			existingHookFound = true
		}
		if hook.Command == ".gromit/hooks/pipeline-resume.sh" {
			gromitHookFound = true
		}
	}

	if !existingHookFound {
		t.Error("existing hook 'existing-hook.sh' was not preserved")
	}
	if !gromitHookFound {
		t.Error("gromit hook was not added")
	}
}
