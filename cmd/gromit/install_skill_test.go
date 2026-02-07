package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/danabrams/gromit/skills"
)

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
	var settings claudeSettings
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
	var settings claudeSettings
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
	var settings1, settings2 claudeSettings
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
	var settings claudeSettings
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
	var settings claudeSettings
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
	var settings claudeSettings
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}
}
