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

	oldDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if err := os.Chdir(projectDir); err != nil {
		return "", err
	}
	defer os.Chdir(oldDir)

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

// testClaudeSettings is used in tests to parse mergeHookSettings output
type testClaudeSettings struct {
	Hooks map[string][]hookMatcher `json:"hooks,omitempty"`
}

func TestBuildSkillContent(t *testing.T) {
	// Create a minimal orchestrator template with all three placeholders
	template := `---
name: gromit
---

# Orchestrator

## Refine Skill

<!-- BEGIN GROMIT-REFINE-SKILL -->
[Content of skills/gromit-refine/SKILL.md will be inlined here]
<!-- END GROMIT-REFINE-SKILL -->

## Plan Skill

<!-- BEGIN GROMIT-PLAN-SKILL -->
[Content of skills/gromit-plan/SKILL.md will be inlined here]
<!-- END GROMIT-PLAN-SKILL -->

## Decompose Skill

<!-- BEGIN GROMIT-DECOMPOSE-SKILL -->
[Content of skills/gromit-decompose/SKILL.md will be inlined here]
<!-- END GROMIT-DECOMPOSE-SKILL -->
`

	result := buildSkillContent(template)

	// Verify that the refine skill content was inlined
	if !strings.Contains(result, "<!-- BEGIN GROMIT-REFINE-SKILL -->") {
		t.Error("Result missing refine skill begin marker")
	}
	if !strings.Contains(result, "<!-- END GROMIT-REFINE-SKILL -->") {
		t.Error("Result missing refine skill end marker")
	}
	if !strings.Contains(result, skills.RefineSkill) {
		t.Error("Result missing refine skill content")
	}

	// Verify that the plan skill content was inlined
	if !strings.Contains(result, "<!-- BEGIN GROMIT-PLAN-SKILL -->") {
		t.Error("Result missing plan skill begin marker")
	}
	if !strings.Contains(result, "<!-- END GROMIT-PLAN-SKILL -->") {
		t.Error("Result missing plan skill end marker")
	}
	if !strings.Contains(result, skills.PlanSkill) {
		t.Error("Result missing plan skill content")
	}

	// Verify that the decompose skill content was inlined
	if !strings.Contains(result, "<!-- BEGIN GROMIT-DECOMPOSE-SKILL -->") {
		t.Error("Result missing decompose skill begin marker")
	}
	if !strings.Contains(result, "<!-- END GROMIT-DECOMPOSE-SKILL -->") {
		t.Error("Result missing decompose skill end marker")
	}
	if !strings.Contains(result, skills.DecomposeSkill) {
		t.Error("Result missing decompose skill content")
	}

	// Verify that the placeholder text was replaced (not present in result)
	if strings.Contains(result, "[Content of skills/gromit-refine/SKILL.md will be inlined here]") {
		t.Error("Result still contains refine placeholder text")
	}
	if strings.Contains(result, "[Content of skills/gromit-plan/SKILL.md will be inlined here]") {
		t.Error("Result still contains plan placeholder text")
	}
	if strings.Contains(result, "[Content of skills/gromit-decompose/SKILL.md will be inlined here]") {
		t.Error("Result still contains decompose placeholder text")
	}
}

func TestBuildSkillContentWithRealOrchestratorTemplate(t *testing.T) {
	// Test with the actual embedded orchestrator skill
	result := buildSkillContent(skills.OrchestratorSkill)

	// Verify that all three skills were inlined
	if !strings.Contains(result, skills.RefineSkill) {
		t.Error("Result missing refine skill content when using real template")
	}
	if !strings.Contains(result, skills.PlanSkill) {
		t.Error("Result missing plan skill content when using real template")
	}
	if !strings.Contains(result, skills.DecomposeSkill) {
		t.Error("Result missing decompose skill content when using real template")
	}

	// Verify that markers are preserved
	if !strings.Contains(result, "<!-- BEGIN GROMIT-REFINE-SKILL -->") {
		t.Error("Result missing refine begin marker when using real template")
	}
	if !strings.Contains(result, "<!-- END GROMIT-REFINE-SKILL -->") {
		t.Error("Result missing refine end marker when using real template")
	}
}

func TestBuildSkillContentPreservesOtherContent(t *testing.T) {
	// Ensure that content outside the placeholders is preserved
	template := `Header content

<!-- BEGIN GROMIT-REFINE-SKILL -->
[Content of skills/gromit-refine/SKILL.md will be inlined here]
<!-- END GROMIT-REFINE-SKILL -->

Middle content

<!-- BEGIN GROMIT-PLAN-SKILL -->
[Content of skills/gromit-plan/SKILL.md will be inlined here]
<!-- END GROMIT-PLAN-SKILL -->

More middle content

<!-- BEGIN GROMIT-DECOMPOSE-SKILL -->
[Content of skills/gromit-decompose/SKILL.md will be inlined here]
<!-- END GROMIT-DECOMPOSE-SKILL -->

Footer content`

	result := buildSkillContent(template)

	// Verify that non-placeholder content is preserved
	if !strings.Contains(result, "Header content") {
		t.Error("Result missing header content")
	}
	if !strings.Contains(result, "Middle content") {
		t.Error("Result missing middle content")
	}
	if !strings.Contains(result, "More middle content") {
		t.Error("Result missing more middle content")
	}
	if !strings.Contains(result, "Footer content") {
		t.Error("Result missing footer content")
	}
}

func TestMergeHookSettingsEmptyInput(t *testing.T) {
	// Test with empty input (no existing settings.json)
	result, err := mergeHookSettings([]byte{})
	if err != nil {
		t.Fatalf("mergeHookSettings failed: %v", err)
	}

	// Parse the result to verify structure
	var settings testClaudeSettings
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify that SessionStart hooks were created
	if settings.Hooks == nil {
		t.Fatal("Hooks map is nil")
	}
	sessionStart, exists := settings.Hooks["SessionStart"]
	if !exists {
		t.Fatal("SessionStart key not found in hooks")
	}

	// Verify that there's exactly one matcher
	if len(sessionStart) != 1 {
		t.Fatalf("Expected 1 matcher, got %d", len(sessionStart))
	}

	// Verify that the matcher is "clear"
	if sessionStart[0].Matcher != "clear" {
		t.Errorf("Expected matcher 'clear', got %q", sessionStart[0].Matcher)
	}

	// Verify that there's exactly one hook
	if len(sessionStart[0].Hooks) != 1 {
		t.Fatalf("Expected 1 hook, got %d", len(sessionStart[0].Hooks))
	}

	// Verify hook details
	hook := sessionStart[0].Hooks[0]
	if hook.Type != "command" {
		t.Errorf("Expected hook type 'command', got %q", hook.Type)
	}
	if hook.Command != ".gromit/hooks/pipeline-resume.sh" {
		t.Errorf("Expected hook command '.gromit/hooks/pipeline-resume.sh', got %q", hook.Command)
	}
}

func TestMergeHookSettingsExistingHooksPreserved(t *testing.T) {
	// Test that existing hooks are preserved when adding our hook
	existingJSON := `{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": "some-other-hook.sh"
          }
        ]
      }
    ]
  }
}`

	result, err := mergeHookSettings([]byte(existingJSON))
	if err != nil {
		t.Fatalf("mergeHookSettings failed: %v", err)
	}

	// Parse the result
	var settings testClaudeSettings
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify that SessionStart still has exactly one matcher
	sessionStart := settings.Hooks["SessionStart"]
	if len(sessionStart) != 1 {
		t.Fatalf("Expected 1 matcher, got %d", len(sessionStart))
	}

	// Verify that the matcher is "clear"
	if sessionStart[0].Matcher != "clear" {
		t.Errorf("Expected matcher 'clear', got %q", sessionStart[0].Matcher)
	}

	// Verify that there are now TWO hooks (existing + new)
	if len(sessionStart[0].Hooks) != 2 {
		t.Fatalf("Expected 2 hooks, got %d", len(sessionStart[0].Hooks))
	}

	// Verify that the existing hook is still present
	existingHookFound := false
	for _, hook := range sessionStart[0].Hooks {
		if hook.Command == "some-other-hook.sh" {
			existingHookFound = true
			if hook.Type != "command" {
				t.Errorf("Existing hook type changed to %q", hook.Type)
			}
		}
	}
	if !existingHookFound {
		t.Error("Existing hook was not preserved")
	}

	// Verify that our hook was added
	gromitHookFound := false
	for _, hook := range sessionStart[0].Hooks {
		if hook.Command == ".gromit/hooks/pipeline-resume.sh" {
			gromitHookFound = true
			if hook.Type != "command" {
				t.Errorf("Gromit hook type is %q, want 'command'", hook.Type)
			}
		}
	}
	if !gromitHookFound {
		t.Error("Gromit hook was not added")
	}
}

func TestMergeHookSettingsIdempotent(t *testing.T) {
	// Test that running mergeHookSettings multiple times doesn't duplicate the hook
	emptyInput := []byte{}

	// First call
	result1, err := mergeHookSettings(emptyInput)
	if err != nil {
		t.Fatalf("First mergeHookSettings call failed: %v", err)
	}

	// Second call with the result of the first call
	result2, err := mergeHookSettings(result1)
	if err != nil {
		t.Fatalf("Second mergeHookSettings call failed: %v", err)
	}

	// Parse both results
	var settings1, settings2 testClaudeSettings
	if err := json.Unmarshal(result1, &settings1); err != nil {
		t.Fatalf("Failed to parse result1: %v", err)
	}
	if err := json.Unmarshal(result2, &settings2); err != nil {
		t.Fatalf("Failed to parse result2: %v", err)
	}

	// Verify that both have the same structure
	if len(settings1.Hooks["SessionStart"]) != len(settings2.Hooks["SessionStart"]) {
		t.Errorf("Matcher count changed: %d vs %d", len(settings1.Hooks["SessionStart"]), len(settings2.Hooks["SessionStart"]))
	}

	hooks1 := settings1.Hooks["SessionStart"][0].Hooks
	hooks2 := settings2.Hooks["SessionStart"][0].Hooks

	if len(hooks1) != len(hooks2) {
		t.Fatalf("Hook count changed: %d vs %d (not idempotent)", len(hooks1), len(hooks2))
	}

	// Verify that there's still only one gromit hook
	gromitHookCount := 0
	for _, hook := range hooks2 {
		if hook.Command == ".gromit/hooks/pipeline-resume.sh" {
			gromitHookCount++
		}
	}
	if gromitHookCount != 1 {
		t.Errorf("Expected exactly 1 gromit hook after second merge, got %d", gromitHookCount)
	}
}

func TestMergeHookSettingsDifferentMatcher(t *testing.T) {
	// Test that a different SessionStart matcher is preserved
	existingJSON := `{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "other-matcher",
        "hooks": [
          {
            "type": "command",
            "command": "other-command.sh"
          }
        ]
      }
    ]
  }
}`

	result, err := mergeHookSettings([]byte(existingJSON))
	if err != nil {
		t.Fatalf("mergeHookSettings failed: %v", err)
	}

	// Parse the result
	var settings testClaudeSettings
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify that there are now TWO matchers
	sessionStart := settings.Hooks["SessionStart"]
	if len(sessionStart) != 2 {
		t.Fatalf("Expected 2 matchers, got %d", len(sessionStart))
	}

	// Verify that both matchers exist
	otherMatcherFound := false
	clearMatcherFound := false
	for _, matcher := range sessionStart {
		if matcher.Matcher == "other-matcher" {
			otherMatcherFound = true
			if len(matcher.Hooks) != 1 || matcher.Hooks[0].Command != "other-command.sh" {
				t.Error("other-matcher hooks were modified")
			}
		}
		if matcher.Matcher == "clear" {
			clearMatcherFound = true
			if len(matcher.Hooks) != 1 || matcher.Hooks[0].Command != ".gromit/hooks/pipeline-resume.sh" {
				t.Error("clear matcher hooks are incorrect")
			}
		}
	}

	if !otherMatcherFound {
		t.Error("Existing other-matcher was not preserved")
	}
	if !clearMatcherFound {
		t.Error("New clear matcher was not added")
	}
}

func TestMergeHookSettingsOtherHookTypes(t *testing.T) {
	// Test that other hook types (beyond SessionStart) are preserved
	existingJSON := `{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": "existing.sh"
          }
        ]
      }
    ],
    "PreCommand": [
      {
        "matcher": "some-pattern",
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

	result, err := mergeHookSettings([]byte(existingJSON))
	if err != nil {
		t.Fatalf("mergeHookSettings failed: %v", err)
	}

	// Parse the result
	var settings testClaudeSettings
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify that PreCommand hooks are preserved
	preCommand, exists := settings.Hooks["PreCommand"]
	if !exists {
		t.Fatal("PreCommand hooks were not preserved")
	}
	if len(preCommand) != 1 {
		t.Fatalf("PreCommand matcher count changed: expected 1, got %d", len(preCommand))
	}
	if preCommand[0].Matcher != "some-pattern" {
		t.Errorf("PreCommand matcher changed: expected 'some-pattern', got %q", preCommand[0].Matcher)
	}
	if len(preCommand[0].Hooks) != 1 || preCommand[0].Hooks[0].Command != "pre-command.sh" {
		t.Error("PreCommand hooks were modified")
	}

	// Verify that SessionStart was still updated correctly
	sessionStart := settings.Hooks["SessionStart"]
	if len(sessionStart) != 1 {
		t.Fatalf("SessionStart matcher count incorrect: expected 1, got %d", len(sessionStart))
	}
	if len(sessionStart[0].Hooks) != 2 {
		t.Fatalf("SessionStart hook count incorrect: expected 2, got %d", len(sessionStart[0].Hooks))
	}
}

func TestMergeHookSettingsPreservesNonHookFields(t *testing.T) {
	// Test that non-hook fields (permissions, allowedTools, model, etc.) are preserved
	existingJSON := `{
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
            "command": "existing.sh"
          }
        ]
      }
    ]
  },
  "customField": "custom-value"
}`

	result, err := mergeHookSettings([]byte(existingJSON))
	if err != nil {
		t.Fatalf("mergeHookSettings failed: %v", err)
	}

	// Parse the result as a generic map to check all fields
	var resultMap map[string]json.RawMessage
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify non-hook fields are preserved
	if _, exists := resultMap["permissions"]; !exists {
		t.Error("permissions field was not preserved")
	}
	if _, exists := resultMap["allowedTools"]; !exists {
		t.Error("allowedTools field was not preserved")
	}
	if _, exists := resultMap["model"]; !exists {
		t.Error("model field was not preserved")
	}
	if _, exists := resultMap["customField"]; !exists {
		t.Error("customField was not preserved")
	}

	// Verify the model value is correct
	var model string
	if err := json.Unmarshal(resultMap["model"], &model); err != nil {
		t.Fatalf("Failed to parse model field: %v", err)
	}
	if model != "claude-sonnet-4-5-20250929" {
		t.Errorf("model field changed: expected 'claude-sonnet-4-5-20250929', got %q", model)
	}

	// Verify permissions structure is preserved
	var permissions map[string][]string
	if err := json.Unmarshal(resultMap["permissions"], &permissions); err != nil {
		t.Fatalf("Failed to parse permissions field: %v", err)
	}
	if len(permissions["allow"]) != 2 || permissions["allow"][0] != "Read" {
		t.Error("permissions.allow was modified")
	}

	// Verify hooks were still updated correctly
	var hooks map[string][]hookMatcher
	if err := json.Unmarshal(resultMap["hooks"], &hooks); err != nil {
		t.Fatalf("Failed to parse hooks field: %v", err)
	}
	sessionStart := hooks["SessionStart"]
	if len(sessionStart) != 1 {
		t.Fatalf("Expected 1 matcher, got %d", len(sessionStart))
	}
	if len(sessionStart[0].Hooks) != 2 {
		t.Fatalf("Expected 2 hooks (existing + gromit), got %d", len(sessionStart[0].Hooks))
	}
}

func TestMergeHookSettingsInvalidJSON(t *testing.T) {
	// Test that invalid JSON returns an error
	invalidJSON := []byte(`{"hooks": invalid}`)

	_, err := mergeHookSettings(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parsing existing settings.json") {
		t.Errorf("Expected error message about parsing, got: %v", err)
	}
}

func TestMergeHookSettingsJSONFormatting(t *testing.T) {
	// Test that the output is properly formatted JSON
	result, err := mergeHookSettings([]byte{})
	if err != nil {
		t.Fatalf("mergeHookSettings failed: %v", err)
	}

	// Verify that the JSON is indented (contains newlines and spaces)
	resultStr := string(result)
	if !strings.Contains(resultStr, "\n") {
		t.Error("Result JSON is not formatted (no newlines)")
	}
	if !strings.Contains(resultStr, "  ") {
		t.Error("Result JSON is not indented (no spaces)")
	}

	// Verify that it's valid JSON by parsing it
	var settings testClaudeSettings
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}
}

// TestInstallSkillIntegrationFullCommand tests running the full install-skill command
// in a temp directory and verifying all artifacts are created correctly.
func TestInstallSkillIntegrationFullCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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
