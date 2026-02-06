package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gromit in the current project",
	Long: `Bootstrap gromit configuration and templates in the current directory.

Creates:
  gromit.yaml           - Configuration file
  .gromit/
    templates/         - Prompt templates
      PROMPT_build.md
      PROMPT_validate.md
      PROMPT_analyze.md
      PROMPT_retro.md
      PROMPT_scope.md
      PROMPT_decompose.md
      PROMPT_review.md
      PROMPT_thorough_review.md
    specs/             - Specification files (empty)
    plans/             - Plan files (empty)`,
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

	fmt.Printf("Initializing gromit in %s\n", cwd)

	// Create .gromit directory structure
	dirs := []string{
		".gromit/templates",
		".gromit/specs",
		".gromit/plans",
	}

	for _, dir := range dirs {
		path := filepath.Join(cwd, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		fmt.Printf("  Created %s/\n", dir)
	}

	// Write config file
	configPath := filepath.Join(cwd, "gromit.yaml")
	if err := writeFileIfNotExists(configPath, defaultConfig, forceInit); err != nil {
		return err
	}

	// Write templates
	buildPath := filepath.Join(cwd, ".gromit/templates/PROMPT_build.md")
	if err := writeFileIfNotExists(buildPath, defaultBuildTemplate, forceInit); err != nil {
		return err
	}

	validatePath := filepath.Join(cwd, ".gromit/templates/PROMPT_validate.md")
	if err := writeFileIfNotExists(validatePath, defaultValidateTemplate, forceInit); err != nil {
		return err
	}

	retroPath := filepath.Join(cwd, ".gromit/templates/PROMPT_retro.md")
	if err := writeFileIfNotExists(retroPath, defaultRetroTemplate, forceInit); err != nil {
		return err
	}

	analyzePath := filepath.Join(cwd, ".gromit/templates/PROMPT_analyze.md")
	if err := writeFileIfNotExists(analyzePath, defaultAnalyzeTemplate, forceInit); err != nil {
		return err
	}

	scopePath := filepath.Join(cwd, ".gromit/templates/PROMPT_scope.md")
	if err := writeFileIfNotExists(scopePath, defaultScopeTemplate, forceInit); err != nil {
		return err
	}

	decomposePath := filepath.Join(cwd, ".gromit/templates/PROMPT_decompose.md")
	if err := writeFileIfNotExists(decomposePath, defaultDecomposeTemplate, forceInit); err != nil {
		return err
	}

	reviewPath := filepath.Join(cwd, ".gromit/templates/PROMPT_review.md")
	if err := writeFileIfNotExists(reviewPath, defaultReviewTemplate, forceInit); err != nil {
		return err
	}

	thoroughReviewPath := filepath.Join(cwd, ".gromit/templates/PROMPT_thorough_review.md")
	if err := writeFileIfNotExists(thoroughReviewPath, defaultThoroughReviewTemplate, forceInit); err != nil {
		return err
	}

	// Write RULES.md
	rulesPath := filepath.Join(cwd, ".gromit/RULES.md")
	if err := writeFileIfNotExists(rulesPath, defaultRules, forceInit); err != nil {
		return err
	}

	// Write LEARNINGS.md
	learningsPath := filepath.Join(cwd, ".gromit/LEARNINGS.md")
	if err := writeFileIfNotExists(learningsPath, defaultLearnings, forceInit); err != nil {
		return err
	}

	// Add to .gitignore if it exists
	gitignorePath := filepath.Join(cwd, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		appendToGitignore(gitignorePath)
	}

	fmt.Println("\nDone! Next steps:")
	fmt.Println("  1. Edit gromit.yaml to customize validation commands")
	fmt.Println("  2. Edit .gromit/RULES.md to add project-specific rules")
	fmt.Println("  3. Create specs in .gromit/specs/ and plans in .gromit/plans/")
	fmt.Println("  4. Create beads with: bd create \"Task title\" --priority 1")
	fmt.Println("  5. Run: gromit run --dry-run")
	fmt.Println("\nPeriodically run 'gromit retro' to analyze and consolidate learnings.")

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

	// Check if already has gromit entries
	if contains(string(content), ".gromit/logs") {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.WriteString("\n# Gromit runner\n.gromit/logs/\n")
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

const defaultConfig = `# Gromit Configuration
# See: https://github.com/danabrams/gromit

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
  stuck_bead_threshold: 3  # Skip bead if it fails this many times across runs

# Scope checking - estimate task complexity before work
scope_check:
  enabled: true
  model: haiku  # haiku is fast and cheap for estimation

# Validation settings - customize for your project
validation:
  enabled: true
  commands:
    - "pnpm run test"
    - "pnpm run lint:check"
    - "pnpm run build"

# Pre-flight checks - verify required tools before validation
preflight:
  auto_install: ask  # ask | always | never

# Review settings - post-iteration and thorough reviews
review:
  enabled: false               # set to true to enable post-iteration review
  model: sonnet
  match_build_model: true      # use opus if build used opus
  timeout: 120

  thorough:
    enabled: false             # set to true to enable periodic thorough reviews
    every_n_iterations: 5
    on_epic_complete: true
    model: opus
    timeout: 900

# Claude CLI settings
claude:
  binary: "claude"
  timeout: 600           # seconds per Claude invocation
  stall_timeout: 120     # seconds with no output before auto-retry (initial, pre-activity)
  stall_timeout_active: 300  # seconds with no output after tool activity (longer, allows thinking)
  bead_timeout: 1200     # seconds max per bead (all retries + analysis + validation)
  analysis_timeout: 120  # seconds max per failure analysis invocation
  flags:
    - "--dangerously-skip-permissions"

# Paths (relative to project root)
paths:
  templates: ".gromit/templates"
  specs: ".gromit/specs"
  logs: ".gromit/logs"
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

These are non-negotiable constraints for this project. Gromit will always follow these.

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

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with ` + "`gromit retro`" + `.

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

You are analyzing accumulated learnings from gromit iterations to identify patterns, consolidate knowledge, and recommend updates to project rules.

## Current Rules

{{.Rules}}

## Current Learnings

{{.Learnings}}

## Run Statistics

{{- if .RunStats.Total }}
### Aggregate Statistics
- **Total iterations**: {{ .RunStats.Total }}
- **Succeeded**: {{ .RunStats.Succeeded }}
- **Failed**: {{ .RunStats.Failed }}
- **Failure rate**: {{ printf "%.1f%%" (mul .RunStats.FailureRate 100) }}
{{- else }}
*No iteration data available yet.*
{{- end }}

{{- if .BeadStats }}
### Stuck Beads (2+ failures)
| Bead ID | Title | Total Runs | Failures | Failure Rate |
|---------|-------|-----------|----------|--------------|
{{- range $id, $stats := .BeadStats }}
| {{ $stats.BeadID }} | {{ $stats.BeadTitle }} | {{ $stats.TotalRuns }} | {{ $stats.Failures }} | {{ printf "%.1f%%" (mul $stats.FailureRate 100) }} |
{{- end }}
{{- else if .RunStats.Total }}
*No stuck beads identified (fewer than 2 failures each).*
{{- end }}

## Task

Analyze the learnings above and provide:

1. **Consolidation Opportunities**: Identify duplicate or related learnings that should be merged
2. **Patterns Worth Promoting**: Suggest learnings that should become rules in RULES.md
3. **Stale Learnings**: Identify learnings that may no longer be relevant
4. **Rule Updates**: Propose specific changes to RULES.md

{{- if .BeadStats }}

5. **Stuck Beads Analysis**: For each stuck bead (with 2+ failures) above, suggest:
   - Root cause hypothesis (based on the failures and learnings)
   - Recommended decomposition strategy (how to break it into smaller tasks)
   - Specific next steps to unblock it
{{- end }}

## Output Format

Provide your analysis in two parts:

1. **Freeform Analysis**: Write a narrative summary of your findings, patterns you've noticed, and reasoning behind your recommendations. Use markdown formatting.

2. **Structured Proposals**: After your analysis, include a JSON code block with structured proposals using this schema:

` + "```json" + `
{
  "consolidations": [
    {
      "learning_hashes": ["hash1", "hash2"],
      "consolidated_text": "Merged learning content",
      "rationale": "Why these should be merged"
    }
  ],
  "promotions": [
    {
      "learning_hash": "hash",
      "proposed_rule": "How it should appear in RULES.md",
      "section": "Code Style | Architecture | Safety | Process",
      "rationale": "Why this should be a rule"
    }
  ],
  "archives": [
    {
      "learning_hash": "hash",
      "rationale": "Why this is no longer relevant"
    }
  ],
  "rule_changes": [
    {
      "current_rule": "Exact text from RULES.md",
      "proposed_rule": "New text",
      "rationale": "Why this change is needed"
    }
  ]
}
` + "```" + `

**Important**: Use the learning hashes (shown as ` + "`Hash: xxxx`" + ` in the learnings above) to reference specific learnings in your proposals. This ensures the correct learnings are updated.

## Guidelines

- Be conservative - only promote patterns seen multiple times
- Focus on actionable, specific rules
- Ensure proposed rules align with Go idioms and project goals
- Consider whether a learning is truly a "rule" (constraint) or just good advice
- Use the learning hashes from above to reference learnings in your JSON proposals
`

const defaultAnalyzeTemplate = `# Failure Analysis

A task just failed. Analyze what went wrong and extract any learnings.

## Task

**ID:** {{.BeadID}}
**Title:** {{.BeadTitle}}

{{.BeadDescription}}

## Error Output

{{.FailureOutput}}

## Your Job

1. **Categorize** the failure:
   - syntax: Typo, missing import, wrong API usage
   - logic: Algorithm wrong, edge case missed
   - environment: Wrong tool version, missing dependency, config issue
   - unclear_spec: The specification is ambiguous or contradictory
   - missing_context: Didn't know about existing code/patterns in the codebase
   - test_flake: Non-deterministic test failure (timing, random, external)

2. **Determine if recoverable** without escalating to a stronger model:
   - true: Can fix with more context or a simple retry
   - false: Needs deeper reasoning or human intervention

3. **Extract a learning** if this insight would help future tasks:
   - Should be generalizable (not specific to this one task)
   - Should be actionable (tells what to do or avoid)
   - Should be concise (1-2 sentences)
   - Set to null if no generalizable learning

4. **Suggest** what to try next

## Output Format

Respond with ONLY a JSON object (no markdown, no explanation):

{"category": "missing_context", "recoverable": true, "root_cause": "Brief description", "learning": "The insight or null", "suggestion": "What to try next"}
`

const defaultScopeTemplate = `# Task Scope Estimation

You are reviewing a task to quickly estimate its complexity and whether it can be completed in a single iteration.

## Task Details

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}
**Priority:** P{{.Bead.Priority}}

{{if .Bead.Labels}}**Labels:** {{join .Bead.Labels ", "}}{{end}}

### Description

{{.Bead.Description}}

{{if .ParentBead}}## Parent Context

This task is part of: **{{.ParentBead.Title}}**{{if .ParentBead.Description}}

{{.ParentBead.Description}}{{end}}{{end}}

## Your Job

Estimate the scope of this task and determine if it can be completed in a single Claude iteration. Consider:

1. **Codebase familiarity** - How much existing code needs to be understood?
2. **Number of files** - How many files will likely need changes?
3. **Complexity** - Are there intricate algorithms, architectural changes, or cross-system dependencies?
4. **Testing** - How thorough must testing be to validate completion?
5. **Unknowns** - Are there architectural decisions or dependencies that are unclear?

## Output Format

Respond with ONLY a JSON object (no markdown, no explanation):

` + "```json" + `
{
  "complexity": "low|medium|high",
  "estimated_iterations": 1,
  "rationale": "Brief explanation of scope assessment",
  "can_complete_in_single_iteration": true,
  "blockers": ["List of", "potential blockers if any"]
}
` + "```" + `

### Complexity Levels

- **low**: Straightforward changes to 1-2 files, minimal testing, clear requirements
- **medium**: Changes to 3-5 files, moderate testing, some architectural consideration
- **high**: Changes to 6+ files, extensive testing, complex architecture, unclear requirements, or cross-system dependencies

### Iteration Estimates

- 1-2 iterations: Task is achievable with current context
- 3+ iterations: Task likely too large and should be decomposed

Tasks that cannot be completed in a single iteration should return ` + "`can_complete_in_single_iteration: false`" + ` and recommend breakdown in blockers.
`

const defaultDecomposeTemplate = `# Task Decomposition

A task has been identified as too large or complex to complete in a single iteration. Your job is to break it down into 2-4 smaller, more manageable sub-tasks.

## Original Task

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}
**Priority:** P{{.Bead.Priority}}

### Description

{{.Bead.Description}}

{{if .ParentBead}}
### Parent Context

This is part of: **{{.ParentBead.Title}}**
{{end}}

## Your Job

Break this task into 2-4 smaller sub-tasks that:
- Each can be completed independently (or with minimal ordering constraints)
- Are smaller in scope but maintain the original goal
- Can be executed sequentially through the Gromit loop
- Each have clear acceptance criteria

## Output Format

Respond with a JSON array containing your proposed sub-tasks. Each sub-task should have:
- ` + "`title`" + `: Brief title (max 60 chars)
- ` + "`description`" + `: What needs to be done
- ` + "`depends_on`" + `: Index of previous task if dependent (null if independent)
- ` + "`acceptance_criteria`" + `: 2-3 bullet points

Example format:
` + "```json" + `
[
  {
    "title": "Set up database migrations",
    "description": "Create the initial migration files and schema...",
    "depends_on": null,
    "acceptance_criteria": ["Migration files created", "Schema matches spec"]
  },
  {
    "title": "Implement user model",
    "description": "Add User model with validation...",
    "depends_on": 0,
    "acceptance_criteria": ["Model created", "Tests pass", "Validation works"]
  }
]
` + "```" + `

## Guidelines

- Keep tasks focused on a single concern
- If a task has a natural prerequisite, indicate the dependency
- Avoid tasks that are just "refactoring" or "cleanup" - focus on functionality
- Each task should be demonstrable with commits/tests
- Consider what files will likely be touched by each task

Respond with ONLY the JSON array (no markdown, no explanation).
`

const defaultReviewTemplate = `# Post-Iteration Review

You are reviewing code changes from a single iteration to catch issues early.

{{if .Rules}}
## Project Rules

{{.Rules}}
{{end}}

## Task Context

**Bead:** {{.Bead.Title}}
{{if .Bead.Description}}
**Description:** {{.Bead.Description}}
{{end}}

{{if .Spec}}
## Specification

{{.Spec}}
{{end}}

## Changes This Iteration

` + "```diff" + `
{{.Diff}}
` + "```" + `

{{if .ValidationCommands}}
## Validation Already Run

These commands passed:
{{range .ValidationCommands}}
- ` + "`{{.}}`" + `
{{end}}
{{end}}

## Your Job

Review the changes above across **6 dimensions**:

### 1. Intent & Spec Drift
- Do changes fulfill the bead's intent, not just pass tests?
- Does the implementation match what was actually requested?
- Are there unnecessary scope additions?

### 2. Correctness
- Does the code work beyond the test coverage?
- Are there edge cases not handled?
- Are error conditions properly handled?

### 3. Security
- SQL injection, XSS, command injection risks?
- Authentication/authorization bypass?
- Data exposure or logging of secrets?
- OWASP top 10 concerns?

### 4. Test Gaps
- Are there untested code paths?
- Missing edge case tests?
- Are tests actually validating behavior or just passing?

### 5. Consistency
- Does new code match existing patterns in the project?
- Naming conventions followed?
- File structure and organization consistent?

### 6. Code Quality
- Dead code or unused imports?
- Poor variable/function naming?
- Missing or incorrect error handling?
- Overly complex logic that should be simplified?

## Issue Triage

Categorize each issue you find:

**Fix immediately** (trivial issues you can fix right now):
- Missing error checks
- Poor naming
- Dead code removal
- Simple refactoring

**Create bead** (significant work needing dedicated iteration):
- New functionality to add
- Complex refactoring
- Multiple files or systems involved
- Provide: title, description, priority (0-2), labels

**Backlog** (needs design discussion or product owner input):
- Architectural decisions
- Unclear requirements
- Cross-system impacts
- Provide: title, description, reason (why it's blocked)

## Output Format

Return a JSON object with this exact structure:

` + "```json" + `
{
  "passed": true,
  "fixes_applied": [
    "Added nil check in handler.go line 45",
    "Removed unused import from service.go"
  ],
  "beads_to_create": [
    {
      "title": "Add input validation for email field",
      "description": "Email field accepts invalid formats. Need regex validation and error messages.",
      "priority": 1,
      "labels": ["validation", "from-review"]
    }
  ],
  "backlog_items": [
    {
      "title": "Consider rate limiting for auth endpoints",
      "description": "No rate limiting on login/signup. Vulnerable to brute force.",
      "reason": "Needs infra decision on rate limit backend (Redis vs in-memory)"
    }
  ],
  "summary": "Implementation matches spec. Fixed 2 minor issues. Created 1 bead for validation gap."
}
` + "```" + `

**Notes:**
- ` + "`passed`" + `: true if no blocking issues found, false if major problems exist
- ` + "`fixes_applied`" + `: List of fixes you made directly (empty array if none)
- ` + "`beads_to_create`" + `: Issues that need dedicated work (empty array if none)
- ` + "`backlog_items`" + `: Issues needing discussion/decision (empty array if none)
- ` + "`summary`" + `: 1-2 sentence summary of your review

**Important:**
- Fix trivial issues directly. If you fix something, re-validation will run automatically.
- Only create beads for issues requiring significant work.
- Use backlog for issues blocked on decisions or unclear requirements.
- All review-created beads automatically get a ` + "`from-review`" + ` label added.
- Be concise but specific. Each finding should be actionable.

Respond with ONLY the JSON object. No markdown, no explanation, just the JSON.
`

const defaultThoroughReviewTemplate = `# Thorough Code Review

You are performing a comprehensive review of multiple completed iterations to assess architectural quality, cross-cutting concerns, and accumulated technical debt.

{{if .Rules}}
## Project Rules

{{.Rules}}
{{end}}

## Review Scope

This review covers changes from {{len .CompletedBeads}} completed beads:

{{range .CompletedBeads}}
### {{.Title}} ({{.ID}})
{{if .Description}}
{{.Description}}
{{end}}

{{end}}

## All Changes

` + "```diff" + `
{{.Diff}}
` + "```" + `

## Your Job

Review the cumulative changes above across **9 dimensions**:

### 1. Intent & Spec Drift
- Do changes fulfill the original intents across all beads?
- Is there scope creep or unnecessary additions?
- Do the beads work coherently together?

### 2. Correctness
- Does the code work beyond the test coverage?
- Are there edge cases not handled?
- Are error conditions properly handled?

### 3. Security
- SQL injection, XSS, command injection risks?
- Authentication/authorization bypass?
- Data exposure or logging of secrets?
- OWASP top 10 concerns?

### 4. Test Gaps
- Are there untested code paths?
- Missing edge case tests?
- Are tests actually validating behavior or just passing?

### 5. Consistency
- Does new code match existing patterns in the project?
- Naming conventions followed?
- File structure and organization consistent?

### 6. Code Quality
- Dead code or unused imports?
- Poor variable/function naming?
- Missing or incorrect error handling?
- Overly complex logic that should be simplified?

### 7. Architectural Assessment
- Do the changes maintain good separation of concerns?
- Are there new coupling issues introduced?
- Are abstractions appropriate or over/under-engineered?
- Does the architecture still make sense with these additions?

### 8. Cross-Cutting Concerns
- Are there patterns that should be extracted into shared utilities?
- Are there inconsistent approaches to similar problems?
- Are configuration, logging, and error handling uniform?

### 9. Pattern Detection
- Are there repeated code patterns that suggest missing abstractions?
- Are there anti-patterns emerging across changes?
- Are there opportunities for refactoring that span multiple beads?

## Issue Triage

Categorize each issue you find:

**Fix immediately** (trivial issues you can fix right now):
- Missing error checks
- Poor naming
- Dead code removal
- Simple refactoring

**Create bead** (significant work needing dedicated iteration):
- New functionality to add
- Complex refactoring
- Multiple files or systems involved
- Provide: title, description, priority (0-2), labels

**Backlog** (needs design discussion or product owner input):
- Architectural decisions
- Unclear requirements
- Cross-system impacts
- Provide: title, description, reason (why it's blocked)

## Output Format

Return a JSON object with this exact structure:

` + "```json" + `
{
  "passed": true,
  "fixes_applied": [
    "Added nil check in handler.go line 45",
    "Removed unused import from service.go"
  ],
  "beads_to_create": [
    {
      "title": "Refactor auth middleware for consistency",
      "description": "Auth checks are implemented differently in handlers A, B, and C. Extract common pattern.",
      "priority": 1,
      "labels": ["refactor", "from-review"]
    }
  ],
  "backlog_items": [
    {
      "title": "Consider event sourcing for audit trail",
      "description": "Current audit approach is fragmented across beads. Event sourcing could unify it.",
      "reason": "Needs architectural discussion and product owner buy-in"
    }
  ],
  "summary": "Architecture is sound. Found opportunities for extracting shared patterns and identified minor security gap."
}
` + "```" + `

**Notes:**
- ` + "`passed`" + `: true if no blocking issues found, false if major problems exist
- ` + "`fixes_applied`" + `: List of fixes you made directly (empty array if none)
- ` + "`beads_to_create`" + `: Issues that need dedicated work (empty array if none)
- ` + "`backlog_items`" + `: Issues needing discussion/decision (empty array if none)
- ` + "`summary`" + `: 1-2 sentence summary of your review

**Important:**
- Fix trivial issues directly. If you fix something, re-validation will run automatically.
- Only create beads for issues requiring significant work.
- Use backlog for issues blocked on decisions or unclear requirements.
- All review-created beads automatically get a ` + "`from-review`" + ` label added.
- Focus on cross-cutting patterns and architectural concerns — this is your chance to see the bigger picture.
- Be concise but specific. Each finding should be actionable.

Respond with ONLY the JSON object. No markdown, no explanation, just the JSON.
`
