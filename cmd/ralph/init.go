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

	// Add to .gitignore if it exists
	gitignorePath := filepath.Join(cwd, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		appendToGitignore(gitignorePath)
	}

	fmt.Println("\nDone! Next steps:")
	fmt.Println("  1. Edit ralph.yaml to customize validation commands")
	fmt.Println("  2. Create specs in .ralph/specs/ for complex features")
	fmt.Println("  3. Create beads with: bd create \"Task title\" --priority 1")
	fmt.Println("  4. Run: ralph run --dry-run")

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

The previous attempt with a different model failed. Here's what happened:

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
