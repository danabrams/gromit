package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize ralph in the current project",
	Long: `Bootstrap ralph configuration and templates in the current directory.

Creates:
  ralph.yaml           - Configuration file
  .ralph/
    templates/         - Prompt templates
      PROMPT_build.md
      PROMPT_validate.md
    specs/             - Specification files (empty)`,
	RunE: runInit,
}

var forceInit bool

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Overwrite existing files")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	fmt.Printf("Initializing ralph in %s\n", cwd)

	// Create .ralph directory structure
	dirs := []string{
		".ralph/templates",
		".ralph/specs",
	}

	for _, dir := range dirs {
		path := filepath.Join(cwd, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		fmt.Printf("  Created %s/\n", dir)
	}

	// Write config file
	configPath := filepath.Join(cwd, "ralph.yaml")
	if err := writeFileIfNotExists(configPath, defaultConfig, forceInit); err != nil {
		return err
	}

	// Write templates
	buildPath := filepath.Join(cwd, ".ralph/templates/PROMPT_build.md")
	if err := writeFileIfNotExists(buildPath, defaultBuildTemplate, forceInit); err != nil {
		return err
	}

	validatePath := filepath.Join(cwd, ".ralph/templates/PROMPT_validate.md")
	if err := writeFileIfNotExists(validatePath, defaultValidateTemplate, forceInit); err != nil {
		return err
	}

	retroPath := filepath.Join(cwd, ".ralph/templates/PROMPT_retro.md")
	if err := writeFileIfNotExists(retroPath, defaultRetroTemplate, forceInit); err != nil {
		return err
	}

	// Write RULES.md
	rulesPath := filepath.Join(cwd, ".ralph/RULES.md")
	if err := writeFileIfNotExists(rulesPath, defaultRules, forceInit); err != nil {
		return err
	}

	// Write LEARNINGS.md
	learningsPath := filepath.Join(cwd, ".ralph/LEARNINGS.md")
	if err := writeFileIfNotExists(learningsPath, defaultLearnings, forceInit); err != nil {
		return err
	}

	// Add to .gitignore if it exists
	gitignorePath := filepath.Join(cwd, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		appendToGitignore(gitignorePath)
	}

	fmt.Println("\nDone! Next steps:")
	fmt.Println("  1. Edit ralph.yaml to customize validation commands")
	fmt.Println("  2. Edit .ralph/RULES.md to add project-specific rules")
	fmt.Println("  3. Create specs in .ralph/specs/ for complex features")
	fmt.Println("  4. Create beads with: bd create \"Task title\" --priority 1")
	fmt.Println("  5. Run: ralph run --dry-run")
	fmt.Println("\nPeriodically run 'ralph retro' to analyze and consolidate learnings.")

	return nil
}

func writeFileIfNotExists(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("  Skipped %s (already exists, use --force to overwrite)\n", filepath.Base(path))
			return nil
		}
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("  Created %s\n", filepath.Base(path))
	return nil
}

func appendToGitignore(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	// Check if already has ralph entries
	if contains(string(content), ".ralph/logs") {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.WriteString("\n# Ralph runner\n.ralph/logs/\n")
	fmt.Println("  Updated .gitignore")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsString(s, substr))
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

const defaultConfig = `# Ralph Runner Configuration
# See: https://github.com/danabrams/ralph-runner

# Model selection based on bead priority
models:
  # Priority-based defaults (P0=critical, P1=normal, P2=low)
  p0: opus
  p1: sonnet
  p2: haiku

  # Validation always uses haiku for cost efficiency
  validation: haiku

  # Label overrides (take precedence over priority)
  labels:
    "complexity:high": opus
    "complexity:low": haiku

# Escalation chain on failure (left to right)
escalation:
  enabled: true
  chain: [haiku, sonnet, opus]
  max_retries_per_model: 1

# Loop settings
loop:
  max_iterations: 0  # 0 = unlimited
  stop_on_failure: false  # Continue to next bead on failure

# Validation settings - customize for your project
validation:
  enabled: true
  commands:
    - "pnpm run test"
    - "pnpm run lint:check"
    - "pnpm run build"

# Claude CLI settings
claude:
  binary: "claude"
  timeout: 600  # seconds per invocation
  flags:
    - "--dangerously-skip-permissions"

# Paths (relative to project root)
paths:
  templates: ".ralph/templates"
  specs: ".ralph/specs"
  logs: ".ralph/logs"
  project_claude_md: "CLAUDE.md"  # Your project's CLAUDE.md
`

const defaultBuildTemplate = `# Task Execution

You are executing a single task from the work queue. Focus only on this task.

{{if .Rules}}
## Rules (Non-Negotiable)

{{.Rules}}
{{end}}

{{if .ConfirmedLearnings}}
## Learnings (Confirmed Patterns)

These patterns have been observed multiple times in this project:

{{formatLearnings .ConfirmedLearnings}}
{{end}}

{{if .RecentLearnings}}
## Recent Learnings

Recent observations that may be relevant:

{{formatLearnings .RecentLearnings}}
{{end}}

## Project Context

{{if .ClaudeMD}}
{{.ClaudeMD}}
{{end}}

## Current Task

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}
**Priority:** P{{.Bead.Priority}}
{{if .Bead.Labels}}**Labels:** {{join .Bead.Labels ", "}}{{end}}

{{if .Bead.Description}}
### Description
{{.Bead.Description}}
{{end}}

{{if .Spec}}
## Specification

The following specification provides detailed requirements for this task:

{{.Spec}}
{{end}}

{{if .ParentBead}}
## Parent Context

This task is part of: **{{.ParentBead.Title}}**
{{if .ParentBead.Description}}
{{.ParentBead.Description}}
{{end}}
{{end}}

{{if .IsRetry}}
## Previous Attempt Failed

{{if .FailureContext}}
Analysis suggests: {{.FailureContext}}
{{end}}

Previous output:
` + "```" + `
{{.PrevFailure}}
` + "```" + `

Please analyze the failure and try a different approach.
{{end}}

## Instructions

1. **Study the codebase** before making changes - don't assume code is missing
2. **Implement the task** following existing patterns in the codebase
3. **Write tests** if the task involves new functionality
4. **Commit your changes** with a clear commit message

## Completion

When the task is complete:
- All code changes are committed
- Tests pass (if applicable)
- The implementation matches the specification

Do NOT output any special completion markers - just complete the task and exit.
`

const defaultRules = `# Rules

These are non-negotiable constraints for this project. Ralph will always follow these.

## Code Style

<!-- Add project-specific rules here, for example: -->
<!-- - Always use TypeScript strict mode -->
<!-- - Never use 'any' type - use 'unknown' and narrow -->
<!-- - Use pnpm, never npm or yarn -->

## Safety

- Never commit secrets, API keys, or credentials
- Never delete data without explicit confirmation in the spec

## Process

- Always run tests before committing
- Follow existing patterns in the codebase
`

const defaultLearnings = `# Learnings

Accumulated operational knowledge from Ralph iterations.
This file is automatically updated. Review periodically with ` + "`ralph retro`" + `.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

*No confirmed learnings yet.*

---

## Provisional

*Seen once - may be specific to one task.*

*No provisional learnings yet.*
`

const defaultValidateTemplate = `# Validation Run

Run the following validation commands and report results.

## Working Directory
{{.WorkDir}}

## Commands to Execute

{{range .Commands}}
- ` + "`{{.}}`" + `
{{end}}

## Instructions

1. Execute each command in order
2. If any command fails, stop and report the failure
3. After all commands pass, output exactly: ` + "`VALIDATION_PASSED`" + `
4. If any command fails, output exactly: ` + "`VALIDATION_FAILED`" + ` followed by error details

Do not make any code changes during validation - only run the commands and report results.
`

const defaultRetroTemplate = `# Retrospective Analysis

You are analyzing accumulated learnings from ralph-runner iterations to identify patterns, consolidate knowledge, and recommend updates to project rules.

## Current Rules

{{.Rules}}

## Current Learnings

{{.Learnings}}

## Task

Analyze the learnings above and provide:

1. **Consolidation Opportunities**: Identify duplicate or related learnings that should be merged
2. **Patterns Worth Promoting**: Suggest learnings that should become rules in RULES.md
3. **Stale Learnings**: Identify learnings that may no longer be relevant
4. **Rule Updates**: Propose specific changes to RULES.md

## Output Format

Use the following format:

### Consolidation

For each set of related learnings:
- **Learnings to merge**: [List dates/IDs]
- **Consolidated version**: [Single clear statement]
- **Rationale**: [Why these should be merged]

### Promote to Rules

For learnings that should become rules:
- **Learning**: [Date | ID | Content]
- **Proposed rule**: [How it should appear in RULES.md]
- **Section**: [Which section of RULES.md: Code Style, Architecture, Safety, or Process]
- **Rationale**: [Why this should be a rule]

### Archive

For stale or obsolete learnings:
- **Learning**: [Date | ID | Content]
- **Rationale**: [Why this is no longer relevant]

### Rule Changes

For direct updates to existing rules:
- **Current rule**: [Exact text from RULES.md]
- **Proposed change**: [New text]
- **Rationale**: [Why this change is needed]

## Guidelines

- Be conservative - only promote patterns seen multiple times
- Focus on actionable, specific rules
- Ensure proposed rules align with Go idioms and project goals
- Consider whether a learning is truly a "rule" (constraint) or just good advice
`
