package main

import (
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
