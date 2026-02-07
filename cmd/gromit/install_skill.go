package main

import (
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
