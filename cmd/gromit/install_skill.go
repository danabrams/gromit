package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/skills"
)

// buildSkillContent takes the orchestrator skill template and inlines
// the refine, plan, and decompose skill content wrapped in delimiter comments.
// This creates a self-contained skill file that the SessionStart hook and
// Task subagents can use without invoking the gromit binary.
func buildSkillContent(orchestratorTemplate string) string {
	content := orchestratorTemplate

	// Replace the refine skill placeholder with the actual skill content
	refineMarker := "<!-- BEGIN GROMIT-REFINE-SKILL -->\n[Content of skills/gromit-refine/SKILL.md will be inlined here]\n<!-- END GROMIT-REFINE-SKILL -->"
	refineReplacement := "<!-- BEGIN GROMIT-REFINE-SKILL -->\n" + skills.RefineSkill + "\n<!-- END GROMIT-REFINE-SKILL -->"
	content = strings.Replace(content, refineMarker, refineReplacement, 1)

	// Replace the plan skill placeholder with the actual skill content
	planMarker := "<!-- BEGIN GROMIT-PLAN-SKILL -->\n[Content of skills/gromit-plan/SKILL.md will be inlined here]\n<!-- END GROMIT-PLAN-SKILL -->"
	planReplacement := "<!-- BEGIN GROMIT-PLAN-SKILL -->\n" + skills.PlanSkill + "\n<!-- END GROMIT-PLAN-SKILL -->"
	content = strings.Replace(content, planMarker, planReplacement, 1)

	// Replace the decompose skill placeholder with the actual skill content
	decomposeMarker := "<!-- BEGIN GROMIT-DECOMPOSE-SKILL -->\n[Content of skills/gromit-decompose/SKILL.md will be inlined here]\n<!-- END GROMIT-DECOMPOSE-SKILL -->"
	decomposeReplacement := "<!-- BEGIN GROMIT-DECOMPOSE-SKILL -->\n" + skills.DecomposeSkill + "\n<!-- END GROMIT-DECOMPOSE-SKILL -->"
	content = strings.Replace(content, decomposeMarker, decomposeReplacement, 1)

	return content
}

// hookEntry represents a single hook configuration
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookMatcher represents a hook matcher with its nested hooks
type hookMatcher struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookEntry `json:"hooks"`
}

// claudeSettings represents the structure of .claude/settings.json
type claudeSettings struct {
	Hooks map[string][]hookMatcher `json:"hooks,omitempty"`
}

// mergeHookSettings takes existing settings.json content (or empty slice if file doesn't exist)
// and adds the SessionStart hook for pipeline-resume.sh without duplicating.
// It preserves all existing settings and returns formatted JSON.
func mergeHookSettings(existingJSON []byte) ([]byte, error) {
	// Parse existing settings or start with empty object
	var settings claudeSettings
	if len(existingJSON) > 0 {
		if err := json.Unmarshal(existingJSON, &settings); err != nil {
			return nil, fmt.Errorf("parsing existing settings.json: %w", err)
		}
	}

	// Initialize hooks map if it doesn't exist
	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]hookMatcher)
	}

	// Define the hook we want to add
	targetHook := hookEntry{
		Type:    "command",
		Command: ".gromit/hooks/pipeline-resume.sh",
	}

	// Check if SessionStart hooks exist
	sessionStartHooks := settings.Hooks["SessionStart"]

	// Find or create the "clear" matcher
	var clearMatcher *hookMatcher
	for i := range sessionStartHooks {
		if sessionStartHooks[i].Matcher == "clear" {
			clearMatcher = &sessionStartHooks[i]
			break
		}
	}

	if clearMatcher == nil {
		// No "clear" matcher exists, add it with our hook
		newMatcher := hookMatcher{
			Matcher: "clear",
			Hooks:   []hookEntry{targetHook},
		}
		settings.Hooks["SessionStart"] = append(sessionStartHooks, newMatcher)
	} else {
		// "clear" matcher exists, check if our hook is already present
		hookExists := false
		for _, h := range clearMatcher.Hooks {
			if h.Type == targetHook.Type && h.Command == targetHook.Command {
				hookExists = true
				break
			}
		}

		// Add the hook if it doesn't exist
		if !hookExists {
			clearMatcher.Hooks = append(clearMatcher.Hooks, targetHook)
		}
	}

	// Marshal back to formatted JSON
	result, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling settings: %w", err)
	}

	return result, nil
}
