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
  CLAUDE.md             - Project documentation (with Gromit patterns)
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
      PROMPT_learn.md
      PROMPT_acceptance_tests.md
      PROMPT_atdd_build.md
      PROMPT_tdd_build.md
      PROMPT_refactor.md
      PROMPT_precheck.md
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

	// Write CLAUDE.md
	claudeMDPath := filepath.Join(cwd, "CLAUDE.md")
	if err := writeFileIfNotExists(claudeMDPath, defaultClaudeMD, forceInit); err != nil {
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

	learnPath := filepath.Join(cwd, ".gromit/templates/PROMPT_learn.md")
	if err := writeFileIfNotExists(learnPath, defaultLearnTemplate, forceInit); err != nil {
		return err
	}

	acceptanceTestsPath := filepath.Join(cwd, ".gromit/templates/PROMPT_acceptance_tests.md")
	if err := writeFileIfNotExists(acceptanceTestsPath, defaultAcceptanceTestsTemplate, forceInit); err != nil {
		return err
	}

	atddBuildPath := filepath.Join(cwd, ".gromit/templates/PROMPT_atdd_build.md")
	if err := writeFileIfNotExists(atddBuildPath, defaultAtddBuildTemplate, forceInit); err != nil {
		return err
	}

	refactorPath := filepath.Join(cwd, ".gromit/templates/PROMPT_refactor.md")
	if err := writeFileIfNotExists(refactorPath, defaultRefactorTemplate, forceInit); err != nil {
		return err
	}

	tddBuildPath := filepath.Join(cwd, ".gromit/templates/PROMPT_tdd_build.md")
	if err := writeFileIfNotExists(tddBuildPath, defaultTDDBuildTemplate, forceInit); err != nil {
		return err
	}

	precheckPath := filepath.Join(cwd, ".gromit/templates/PROMPT_precheck.md")
	if err := writeFileIfNotExists(precheckPath, defaultPrecheckTemplate, forceInit); err != nil {
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

# Methodology settings - ATDD workflow phases
# Uncomment to enable Acceptance Test-Driven Development (ATDD) workflow phases:
#   - acceptance_tests: Write tests before implementation
#   - atdd_build: Implement to make tests pass
#   - refactor: Improve code quality after tests pass
# methodology:
#   enabled: false
#   phases:
#     - acceptance_tests
#     - atdd_build
#     - refactor

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

const defaultLearnTemplate = `# Success Learning Extraction

A task just succeeded. Extract any codebase patterns, conventions, or gotchas that would help future tasks.

## Task

**ID:** {{.BeadID}}
**Title:** {{.BeadTitle}}

{{.BeadDescription}}

## Summary of Work Done

{{.Summary}}

## Your Job

Extract ONE generalizable learning from this successful iteration:

1. **Focus on codebase insights:**
   - Patterns: How things are structured or organized in this codebase
   - Conventions: Naming, formatting, or architectural choices
   - Gotchas: Surprising behavior, edge cases, or things to watch out for

2. **Make it actionable:**
   - Should tell what to do or avoid
   - Should be useful for similar future tasks
   - Should be concise (1-2 sentences)

3. **Skip if no learning:**
   - If the task was straightforward and revealed nothing new, return null
   - Don't force a learning from routine work

## Output Format

Respond with ONLY a JSON object (no markdown, no explanation):

{"learning": "The insight or null", "category": "conventions | gotchas | patterns"}

Examples:
- {"learning": "Config validation always happens in setDefaults() method, not in Load()", "category": "conventions"}
- {"learning": "Test files use table-driven tests with t.Run for each case", "category": "patterns"}
- {"learning": null, "category": "patterns"}
`

const defaultAcceptanceTestsTemplate = `# Acceptance Test Writing

You are writing acceptance tests for a task before implementation begins. This is the ATDD (Acceptance Test-Driven Development) workflow.

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

Your job is to write acceptance tests that verify the acceptance criteria for this task. **You must NOT write any implementation code.** Only create or modify test files.

1. **Explore the codebase** to understand:
   - What test framework and conventions are used (e.g., Go's testing package, table-driven tests)
   - Where test files are located (e.g., ` + "`*_test.go` files alongside implementation)" + `)
   - How existing tests are structured and named
   - What helper functions or test utilities exist
   - What shared setup helpers already exist — reuse them instead of writing new setup code

2. **Write acceptance tests** that:
   - Cover each acceptance criterion with at least one test
   - Follow existing test patterns and naming conventions in the project
   - Test behavior through the public API or command surface, not internal helper functions
   - Will fail until the feature is implemented (test for the new behavior)
   - Are clear, readable, and maintainable

3. **Only modify test files** - do not write any implementation code:
   - Create new test files following the project's naming conventions
   - Add test cases to existing test files if appropriate
   - Do NOT modify implementation files (e.g., non-test .go files)
   - Do NOT stub out implementations to make tests pass

4. **Commit your changes** with a clear commit message like "test: add acceptance tests for [task title]"

## What Makes a Good Acceptance Test

Acceptance tests verify **user-visible behavior**, not implementation details. Apply these principles:

- **Test through the public surface.** Call the function, method, or command that a user/caller would use. Do NOT test internal helpers directly — if a helper is private, test the command that calls it instead.
- **Test behavior, not mechanics.** Assert on outcomes ("the output contains X", "the file was created", "the error message says Y"), not on how the code achieves them. Never test that stdlib functions work (e.g., don't verify that ` + "`os.MkdirAll`" + ` creates directories or that ` + "`os.WriteFile`" + ` writes files).
- **One acceptance criterion per test case, not per test function.** Use table-driven tests or subtests (` + "`t.Run`" + `) to cover multiple scenarios. Multiple test functions that share identical setup are a sign you need a table-driven test instead.
- **Extract shared setup into helpers.** If two or more tests create the same directory structure, config, or mock setup, extract it into a ` + "`setupXxx(t *testing.T)`" + ` helper. Keep helpers in the same package as the tests.
- **Keep tests concise.** A 100-line test function with 30 lines of setup, 5 lines of action, and 65 lines of assertions is too long. Extract setup, use helpers, and trust that stdlib works.
- **No skipped tests.** Do not write tests with ` + "`t.Skip()`" + ` for features that don't exist yet or can't run in the test environment. Every test you write must be runnable and must fail for the right reason (missing implementation, not missing infrastructure).
- **Use build tags for true acceptance tests.** If the test requires external dependencies (real binaries, network, etc.), use ` + "`//go:build acceptance`" + ` so it runs separately from unit tests.

## Anti-Patterns to Avoid

Do NOT write tests that:
- Test Go standard library behavior (file creation, temp files, JSON marshaling)
- Call private/internal helper functions directly instead of the public API
- Duplicate 10+ lines of identical setup across multiple test functions
- Have ` + "`t.Skip()`" + ` because the test can't actually run
- Verify implementation details like file permissions, temp file naming patterns, or internal data structures that aren't part of the acceptance criteria
- Are actually unit tests labeled as "acceptance" — if it tests a single function with mocked dependencies, it's a unit test

## Completion

When complete:
- Acceptance test files are created/modified
- Each acceptance criterion is covered by at least one test
- Tests are concise — shared setup is extracted, related cases use table-driven tests
- No implementation code has been written
- All changes are committed

Do NOT output any special completion markers - just complete the task and exit.
`

const defaultAtddBuildTemplate = `# Task Execution

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

**Acceptance tests have been written and committed.** Your job is to make them pass.

1. **Study the failing tests** - understand what behavior they're testing and why they fail
2. **Study the codebase** - understand existing patterns and where your implementation should fit
3. **Implement the functionality** - write the minimal code needed to make the tests pass
4. **Do NOT modify the test files** - the tests define the behavioral contract; only change implementation code
5. **Commit your changes** with a clear commit message

## Completion

When the task is complete:
- All code changes are committed
- The acceptance tests now pass
- You have NOT modified any test files
- The implementation matches the specification

Do NOT output any special completion markers - just complete the task and exit.
`

const defaultRefactorTemplate = `# Refactoring Phase

You are refactoring the implementation after tests pass. Your goal is to improve code quality without changing behavior.

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

**All tests are passing.** Your job is to review the implementation for quality improvements without changing behavior.

1. **Review the implementation** looking for:
   - Code duplication or unnecessary complexity
   - Unclear variable or function names
   - Missing or misleading comments/documentation
   - Opportunities to follow established project patterns better
   - Error handling that could be clearer
   - Long functions that could be decomposed
   - Magic numbers or strings that should be constants

2. **Make refactoring changes** that:
   - Improve readability and maintainability
   - Do NOT change external behavior (tests must still pass)
   - Follow existing project conventions and patterns
   - Are small, focused improvements (not large rewrites)
   - Make the code clearer, not just different

3. **Do NOT refactor if the code is already clear and follows project conventions** - refactoring for its own sake adds no value

4. **Commit your changes** with a clear commit message like "refactor: improve clarity in [component/function]" - keep refactoring commits separate from implementation commits

## Important Notes

- Refactoring must preserve all existing behavior - tests must still pass
- Only refactor the code touched by this task, not the entire codebase
- If the implementation is already clear and well-structured, say so and make no changes
- Focus on readability and maintainability, not premature optimization
- Follow the project's existing patterns - don't introduce new styles or conventions

## Completion

When complete:
- Code quality improvements are committed (if any were needed)
- All tests still pass (behavior unchanged)
- Changes follow project conventions

Do NOT output any special completion markers - just complete the task and exit.
`

const defaultPrecheckTemplate = `# Pre-Check: Acceptance Criteria Already Met?

You are performing a lightweight pre-check to determine whether the acceptance criteria for this task are already satisfied by the current codebase.

## Task Details

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}
**Priority:** P{{.Bead.Priority}}
{{if .Bead.Labels}}**Labels:** {{join .Bead.Labels ", "}}{{end}}

{{if .Bead.Description}}
### Description
{{.Bead.Description}}
{{end}}

{{if .ParentBead}}
## Parent Context

This task is part of: **{{.ParentBead.Title}}**
{{if .ParentBead.Description}}
{{.ParentBead.Description}}
{{end}}
{{end}}

## Your Job

**Read the codebase** and determine whether ALL acceptance criteria for this task are already met.

### Instructions

1. **Examine relevant files** in the codebase to check each acceptance criterion
2. **Do NOT make any changes** — this is a read-only inspection
3. **Be conservative** — when uncertain, err on the side of NOT_MET
4. **Output your verdict clearly:**
   - If ALL criteria are already satisfied: output exactly ` + "`PRECHECK_PASSED`" + `
   - If ANY criterion is not yet satisfied: output exactly ` + "`PRECHECK_NOT_MET`" + `

### Why Be Conservative?

- A false negative (saying NOT_MET when criteria are met) just means we run the normal iteration — no harm done
- A false positive (saying PASSED when criteria aren't met) would skip needed work — this is bad
- When in doubt, choose ` + "`PRECHECK_NOT_MET`" + `

## Important

- This is a quick check, not a full code review
- Focus on whether the acceptance criteria are met, not code quality
- Do NOT write or modify any code
- Do NOT run any commands or tests
- Just read files and report your verdict

## Output Format

After your analysis, output ONE of these exact strings on a line by itself:
- ` + "`PRECHECK_PASSED`" + ` (if all acceptance criteria are satisfied)
- ` + "`PRECHECK_NOT_MET`" + ` (if any criterion is not satisfied or you're uncertain)
`

const defaultTDDBuildTemplate = `# Task Execution

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

You are following Test-Driven Development (TDD) with strict red-green-refactor cycles. Your implementation must proceed incrementally through repeated cycles.

### TDD Discipline: Red-Green-Refactor

Work in small increments, following this cycle for each piece of functionality:

1. **Red — Write ONE failing test:**
   - Focus on a single behavior or requirement piece
   - Write a test that expresses what the code should do
   - The test must fail because the behavior doesn't exist yet
   - Do NOT write multiple tests before implementing

2. **Green — Write minimal code to pass:**
   - Write the simplest code that makes this one test pass
   - Do NOT write "proper" or "complete" implementations yet
   - Do NOT write code for functionality you haven't tested
   - It's okay if the code feels hacky or incomplete — that's the point

3. **Commit — Record the cycle:**
   - Commit the test and implementation together
   - Use a clear message describing what was tested and implemented
   - The commit trail creates accountability for the TDD discipline

4. **Repeat — Move to the next piece:**
   - Continue with the next behavior or requirement piece
   - Each cycle should be small — aim for 5-15 minutes of work
   - Do NOT skip ahead to write multiple tests or "complete" implementations

### Key Principles

- **One test at a time** — Resist the urge to write multiple tests upfront
- **Minimum to pass** — Don't write more implementation than needed for the current test
- **Small steps** — Break requirements into the smallest testable pieces
- **Commit each cycle** — Your git log should show the progression of red-green cycles
- **Stop when done** — When all requirements are covered by tests, stop — refactoring will happen in a separate phase

### What About Refactoring?

Do NOT refactor during this phase. Focus only on the red-green cycles. A separate refactoring phase will run after validation passes, where you can:
- Improve names and structure
- Remove duplication
- Extract abstractions
- Clean up "minimum to pass" roughness

### Completion

When the task is complete:
- Each requirement piece is covered by at least one test
- All tests pass
- Each red-green cycle is recorded in a separate commit
- All changes are committed

Do NOT output any special completion markers - just complete the task and exit.
`

const defaultClaudeMD = "# Your Project\n\n" +
	"<!-- Update this with your project's name and description -->\n\n" +
	"## Quick Start\n\n" +
	"```bash\n" +
	"bd create \"Task title\" --priority 1\n" +
	"gromit run                         # Run until no work\n" +
	"gromit run -n 5 --time-budget 30   # Max 5 beads, 30-min budget\n" +
	"gromit status                      # Show next bead + model\n" +
	"```\n\n" +
	"## Bead Sizing\n\n" +
	"- **One concern per bead** — a single file or two tightly coupled files\n" +
	"- **1-3 acceptance criteria** — concrete, testable criteria only; split if more than 3\n" +
	"- **Self-contained** — understandable without reading other beads\n" +
	"- **No ambiguity** — Claude implements without making design decisions\n" +
	"- **Max 2 files touched** — if more, consider splitting the bead\n" +
	"- **Clear definition of done** — each criterion has an obvious pass/fail test\n\n" +
	"## Capturing Ideas vs Creating Beads\n\n" +
	"When asked to add something to the backlog, use `gromit add \"<idea>\"` — not `bd create`. " +
	"The backlog is for rough ideas that flow through the refine → plan → decompose pipeline. " +
	"Only use `bd create` when you have a fully scoped, ready-to-implement task with clear acceptance criteria.\n\n" +
	"## Rules\n\n" +
	"See `.gromit/RULES.md` for project-specific constraints and best practices.\n\n" +
	"## Learnings\n\n" +
	"See `.gromit/LEARNINGS.md` for accumulated patterns and conventions from this project's iterations.\n"
