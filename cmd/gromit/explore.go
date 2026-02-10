package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/spf13/cobra"
)

var exploreCmd = &cobra.Command{
	Use:   "explore [topic]",
	Short: "Launch interactive exploration session",
	Long: `Launch an interactive Claude Code session for exploring problem spaces.

The session receives full project context (CLAUDE.md, RULES.md, LEARNINGS.md)
and guides free-form exploration to understand problems, research approaches,
and identify scope before committing to artifacts.

Examples:
  gromit explore                              # Blank exploration
  gromit explore "Improve developer onboarding" # Pre-seeded topic
  gromit explore --model sonnet "Add dark mode" # Override model`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExplore,
}

var exploreModel string

func init() {
	exploreCmd.Flags().StringVar(&exploreModel, "model", "opus", "Claude model to use (opus, sonnet, haiku)")
	rootCmd.AddCommand(exploreCmd)
}

func runExplore(cmd *cobra.Command, args []string) error {
	// Get config and directories
	cfg, err := loadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = nil
	}
	gromitDir := resolveGromitDir(cfg)
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := resolveSpecsDir(cfg)

	// Ensure epics directory exists
	if err := os.MkdirAll(epicsDir, 0o755); err != nil {
		return fmt.Errorf("creating epics dir: %w", err)
	}

	// Record existing artifacts before session
	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		return fmt.Errorf("scanning epics directory: %w", err)
	}

	existingSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		return fmt.Errorf("scanning specs directory: %w", err)
	}

	// Load backlog to track existing items
	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		return fmt.Errorf("creating backlog file: %w", err)
	}

	existingBacklogItems, err := bf.List()
	if err != nil {
		return fmt.Errorf("loading backlog: %w", err)
	}

	// Build system prompt with full project context
	systemPrompt, err := buildExplorePrompt(cfg, gromitDir, args)
	if err != nil {
		return fmt.Errorf("building explore prompt: %w", err)
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

	promptFile, err := os.CreateTemp(tmpDir, "explore-prompt-*.md")
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
	initialPrompt := fmt.Sprintf("Read and follow the exploration instructions in %s", promptPath)

	// Build command args: flags + model + initial message
	cmdArgs := append([]string{}, claudeFlags...)
	if exploreModel != "" {
		cmdArgs = append(cmdArgs, "--model", exploreModel)
	}
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

	// Post-session artifact detection would go here
	// For now, we just note that existing artifacts were captured
	_ = existingEpics
	_ = existingSpecs
	_ = existingBacklogItems

	return nil
}

// getEpicFiles returns a list of .md files in the epics directory.
// Creates the directory if it doesn't exist.
func getEpicFiles(epicsDir string) ([]string, error) {
	return listMarkdownFiles(epicsDir)
}

// buildExplorePrompt constructs the system prompt for the exploration session
func buildExplorePrompt(cfg *config.Config, gromitDir string, args []string) (string, error) {
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

	// Build the system prompt
	var sb strings.Builder

	// Optional: pre-seeded topic
	if len(args) > 0 {
		sb.WriteString(fmt.Sprintf(`Topic for exploration:

%s

`, args[0]))
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

#### Confirmed Patterns

%s

#### Recent Learnings

%s

`, confirmedLearnings, recentLearnings))

	return sb.String(), nil
}
