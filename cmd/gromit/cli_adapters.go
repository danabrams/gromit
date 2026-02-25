package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
)

// cliPromptRenderer adapts prompt.Renderer to pipeline.ReviewRenderer interface
// It loads ClaudeMD and Rules before rendering
type cliPromptRenderer struct {
	renderer *prompt.Renderer
}

var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)

func (r *cliPromptRenderer) RenderThoroughReview(input *pipeline.ThoroughReviewPromptInput) (string, error) {
	// Build ThoroughReviewContext from pipeline input
	reviewCtx := &prompt.ThoroughReviewContext{
		Diff: input.Diff,
	}

	// Load ClaudeMD and Rules (warnings only)
	var err error
	reviewCtx.ClaudeMD, err = r.renderer.LoadClaudeMD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load CLAUDE.md: %v\n", err)
	}
	reviewCtx.Rules, err = r.renderer.LoadRulesForPhase("thorough_review")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load thorough_review rules: %v\n", err)
	}

	return r.renderer.RenderThoroughReview(reviewCtx)
}

// explorePromptRenderer adapts prompt.Renderer to pipeline.ExploreRenderer
type explorePromptRenderer struct {
	renderer *prompt.Renderer
	// lastDiagnostics captures section-level prompt token estimates for explore prompts.
	lastDiagnostics *prompt.PromptDiagnostics
}

var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)

func (r *explorePromptRenderer) RenderExplore(input *pipeline.ExplorePromptInput) (string, error) {
	// Extract topic from typed input
	topic := input.Query

	// Load ClaudeMD and Rules
	claudeMD, err := r.renderer.LoadClaudeMD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load CLAUDE.md: %v\n", err)
	}

	rules, err := r.renderer.LoadRules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load RULES.md: %v\n", err)
	}

	// Load learnings
	lf := r.renderer.GetLearningsFile()
	confirmedLearnings := formatLearnings(lf, true)
	recentLearnings := formatLearnings(lf, false)

	// Get working directory and paths
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not determine working directory: %v\n", err)
		workDir = "."
	}
	gromitDir := r.renderer.GetGromitDir()
	specsDir := r.renderer.GetSpecsDir()

	learningsSection := fmt.Sprintf(`#### Confirmed Patterns

%s

#### Recent Learnings

%s`, confirmedLearnings, recentLearnings)
	instructionsSection := `You are running an exploration session. Your goal is to brainstorm a big idea or problem space and break it down into concrete, actionable artifacts.

### What To Do

1. **Understand the problem space** — If a topic was provided above, start there. Otherwise, ask the user what they want to explore. Read relevant code and docs to ground your understanding.

2. **Brainstorm broadly** — Think through the problem from multiple angles. Consider user needs, technical constraints, edge cases, and alternative approaches. Discuss ideas with the user — this is a collaborative session.

3. **Break it down** — As ideas crystallize, capture them as the appropriate artifact type:

   - **Backlog items** — Quick ideas, rough feature requests, bugs, or chores. Add these by running: gromit add "<idea>". Optionally provide context when prompted. These flow through the refine → plan → decompose pipeline later.
   - **Specs** — For ideas that are well-understood enough to specify, write a spec file to %s/<name>.md. A spec describes what to build, why, acceptance criteria, and key decisions. See existing specs in that directory for the format.
   - **Epics** — For large initiatives that span multiple specs, write an epic file to %s/epics/<name>.md with frontmatter containing epic_id and created fields.

4. **Prefer backlog items** — When in doubt, use gromit add. Backlog items are cheap and get refined later. Only write specs for ideas you've discussed enough to specify clearly. Only create epics when the scope genuinely spans multiple independent specs.

### What NOT To Do

- Do NOT implement features or write production code
- Do NOT create beads with bd create — that happens during decomposition
- Do NOT skip the conversation — explore with the user, don't just dump a list of ideas

### Session Flow

Start by understanding the topic, then alternate between discussing ideas and capturing them. End the session when the problem space feels well-mapped and the key ideas have been captured as artifacts.`
	renderedInstructions := fmt.Sprintf(instructionsSection, specsDir, gromitDir)
	sectionTokens := prompt.EstimateSectionTokens(map[string]string{
		"topic":          topic,
		prompt.SectionClaudeMD:     claudeMD,
		prompt.SectionRules:        rules,
		"learnings":      learningsSection,
		"instructions":   renderedInstructions,
	})
	r.lastDiagnostics = prompt.NewDiagnostics("explore", sectionTokens)

	// Build the system prompt
	var sb strings.Builder

	// Optional: pre-seeded topic
	if topic != "" {
		sb.WriteString(fmt.Sprintf(`Topic for exploration:

%s

`, topic))
	}

	// Context section
	sb.WriteString(fmt.Sprintf(`## Context

Working directory: %s
Epics directory: %s/epics
Specs directory: %s

`, workDir, gromitDir, specsDir))

	// Project context
	if claudeMD != "" {
		sb.WriteString(fmt.Sprintf(`### Project Context

%s

`, claudeMD))
	}

	if rules != "" {
		sb.WriteString(fmt.Sprintf(`### Rules (Non-Negotiable)

%s

`, rules))
	}

	// Learnings
	sb.WriteString(fmt.Sprintf(`### Learnings

%s

`, learningsSection))

	// Exploration instructions
	sb.WriteString("## Instructions\n\n")
	sb.WriteString(renderedInstructions)
	sb.WriteString("\n")

	return sb.String(), nil
}

func (r *explorePromptRenderer) LastDiagnostics() *prompt.PromptDiagnostics {
	if r == nil {
		return nil
	}
	return r.lastDiagnostics
}
