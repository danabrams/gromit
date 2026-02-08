package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/skills"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug [description]",
	Short: "Launch interactive bug investigation session",
	Long: `Launch an interactive Claude Code session for investigating bugs.

The session receives full project context (CLAUDE.md, RULES.md, LEARNINGS.md)
and guides free-form investigation to identify root cause, triage severity, and
produce appropriate outcomes:
  - Trivial fix: Apply directly and validate
  - Clear fix: Create investigation report + plan
  - Needs investigation: Create report + backlog item

Examples:
  gromit debug                              # Blank session
  gromit debug "login fails with + in email" # Pre-seeded description
  gromit debug --model sonnet "API returns 500" # Override model`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDebug,
}

var debugModel string

func init() {
	debugCmd.Flags().StringVar(&debugModel, "model", "opus", "Claude model to use (opus, sonnet, haiku)")
	rootCmd.AddCommand(debugCmd)
}

func runDebug(cmd *cobra.Command, args []string) error {
	// Get config and directories
	cfg, err := loadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = nil
	}
	gromitDir := resolveGromitDir(cfg)

	// Build system prompt with full project context
	systemPrompt, err := buildDebugPrompt(cfg, gromitDir, args)
	if err != nil {
		return fmt.Errorf("building debug prompt: %w", err)
	}

	// Determine binary and flags from config
	claudeBinary := "claude"
	var claudeFlags []string
	if cfg != nil {
		claudeBinary = cfg.Claude.Binary
		claudeFlags = cfg.Claude.Flags
	}

	// Write system prompt to a temp file to avoid "argument list too long" errors
	tmpDir := filepath.Join(gromitDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("creating tmp dir: %w", err)
	}

	promptFile, err := os.CreateTemp(tmpDir, "debug-prompt-*.md")
	if err != nil {
		return fmt.Errorf("creating temp prompt file: %w", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(systemPrompt); err != nil {
		promptFile.Close()
		return fmt.Errorf("writing prompt file: %w", err)
	}
	promptFile.Close()

	// Launch Claude Code with a short initial prompt that references the temp file
	initialPrompt := fmt.Sprintf("Read and follow the debug instructions in %s", promptPath)

	// Build command args: flags + initial message
	cmdArgs := append([]string{}, claudeFlags...)
	cmdArgs = append(cmdArgs, initialPrompt)

	claudeCmd := exec.Command(claudeBinary, cmdArgs...)
	claudeCmd.Stdin = os.Stdin
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr

	if err := claudeCmd.Run(); err != nil {
		// Don't treat Claude exit code as an error - it's normal when user exits
		if _, ok := err.(*exec.ExitError); ok {
			// User exited gracefully, not an error
			return nil
		}
		return fmt.Errorf("launching Claude Code: %w", err)
	}

	return nil
}

// buildDebugPrompt constructs the system prompt for the debug session
func buildDebugPrompt(cfg *config.Config, gromitDir string, args []string) (string, error) {
	// Load project context
	templatesDir := resolveTemplatesDir(cfg)
	specsDir := resolveSpecsDir(cfg)
	claudeMDPath := resolveProjectClaudeMD(cfg)

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir)
	if err != nil {
		return "", fmt.Errorf("creating prompt renderer: %w", err)
	}

	claudeMD, err := renderer.LoadClaudeMD()
	if err != nil {
		return "", fmt.Errorf("loading CLAUDE.md: %w", err)
	}

	rules, err := renderer.LoadRules()
	if err != nil {
		return "", fmt.Errorf("loading RULES.md: %w", err)
	}

	// Load learnings
	lf := renderer.GetLearningsFile()
	var confirmedLearnings, recentLearnings string
	if lf != nil {
		confirmed := lf.GetConfirmed()
		recent := lf.GetRecent(24)

		if len(confirmed) > 0 {
			var sb strings.Builder
			for _, l := range confirmed {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", l.Category, l.Content))
			}
			confirmedLearnings = sb.String()
		} else {
			confirmedLearnings = "*None*"
		}

		if len(recent) > 0 {
			var sb strings.Builder
			for _, l := range recent {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", l.Category, l.Content))
			}
			recentLearnings = sb.String()
		} else {
			recentLearnings = "*None*"
		}
	} else {
		confirmedLearnings = "*None*"
		recentLearnings = "*None*"
	}

	// Get working directory
	workDir, _ := os.Getwd()

	// Extract validation commands
	var validationCommands string
	if cfg != nil && cfg.Validation.Enabled && len(cfg.Validation.Commands) > 0 {
		var sb strings.Builder
		for _, cmd := range cfg.Validation.Commands {
			sb.WriteString(fmt.Sprintf("   %s\n", cmd))
		}
		validationCommands = strings.TrimRight(sb.String(), "\n")
	} else {
		validationCommands = "   # No validation commands configured"
	}

	// Determine reports directory
	reportsDir := filepath.Join(gromitDir, "reports")

	// Build the system prompt
	var sb strings.Builder

	// Optional: pre-seeded bug description
	if len(args) > 0 {
		sb.WriteString(fmt.Sprintf(`Bug description:

%s

`, args[0]))
	}

	// Context section
	sb.WriteString(fmt.Sprintf(`## Context

Working directory: %s
Reports directory: %s

### Validation Commands

When applying direct fixes (outcome 1), run these commands:

%s

`, workDir, reportsDir, validationCommands))

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

#### Confirmed Patterns

%s

#### Recent Learnings

%s

`, confirmedLearnings, recentLearnings))

	// Embedded skill instructions
	sb.WriteString(fmt.Sprintf(`## Instructions

%s`, skills.DebugSkill))

	return sb.String(), nil
}
