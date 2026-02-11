package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/spf13/cobra"
)

var exploreCmd = &cobra.Command{
	Use:   "explore [topic]",
	Short: "Launch interactive exploration session",
	Long: `Launch an interactive brainstorming session for exploring a big idea or problem space.

The session receives full project context and guides collaborative exploration
to break down ideas into concrete artifacts: backlog items (via gromit add),
specs, or epics — whatever granularity makes sense.

Examples:
  gromit explore                              # Open-ended brainstorm
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

	// TODO: Implement post-session artifact detection
	// Should scan for new files in epicsDir/specsDir and new backlog items,
	// compare against pre-session snapshots, and create corresponding bd beads.
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

// formatLearnings formats learnings into a markdown string.
// If confirmed is true, returns confirmed learnings; otherwise returns recent learnings (last 24 months).
// Returns "*None*" if the file is nil or no learnings exist.
func formatLearnings(lf *learnings.File, confirmed bool) string {
	if lf == nil {
		return "*None*"
	}

	var items []learnings.Learning
	if confirmed {
		items = lf.GetConfirmed()
	} else {
		items = lf.GetRecent(24)
	}

	if len(items) == 0 {
		return "*None*"
	}

	var sb strings.Builder
	for _, l := range items {
		sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", l.Category, l.Content))
	}
	return sb.String()
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
	confirmedLearnings := formatLearnings(lf, true)
	recentLearnings := formatLearnings(lf, false)

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

	// Exploration instructions
	sb.WriteString("## Instructions\n\n")
	sb.WriteString("You are running an exploration session. Your goal is to brainstorm a big idea or problem space and break it down into concrete, actionable artifacts.\n\n")
	sb.WriteString("### What To Do\n\n")
	sb.WriteString("1. **Understand the problem space** — If a topic was provided above, start there. Otherwise, ask the user what they want to explore. Read relevant code and docs to ground your understanding.\n\n")
	sb.WriteString("2. **Brainstorm broadly** — Think through the problem from multiple angles. Consider user needs, technical constraints, edge cases, and alternative approaches. Discuss ideas with the user — this is a collaborative session.\n\n")
	sb.WriteString("3. **Break it down** — As ideas crystallize, capture them as the appropriate artifact type:\n\n")
	sb.WriteString(fmt.Sprintf("   - **Backlog items** — Quick ideas, rough feature requests, bugs, or chores. Add these by running: gromit add \"<idea>\". Optionally provide context when prompted. These flow through the refine → plan → decompose pipeline later.\n"))
	sb.WriteString(fmt.Sprintf("   - **Specs** — For ideas that are well-understood enough to specify, write a spec file to %s/<name>.md. A spec describes what to build, why, acceptance criteria, and key decisions. See existing specs in that directory for the format.\n", specsDir))
	sb.WriteString(fmt.Sprintf("   - **Epics** — For large initiatives that span multiple specs, write an epic file to %s/epics/<name>.md with frontmatter containing epic_id and created fields.\n\n", gromitDir))
	sb.WriteString("4. **Prefer backlog items** — When in doubt, use gromit add. Backlog items are cheap and get refined later. Only write specs for ideas you've discussed enough to specify clearly. Only create epics when the scope genuinely spans multiple independent specs.\n\n")
	sb.WriteString("### What NOT To Do\n\n")
	sb.WriteString("- Do NOT implement features or write production code\n")
	sb.WriteString("- Do NOT create beads with bd create — that happens during decomposition\n")
	sb.WriteString("- Do NOT skip the conversation — explore with the user, don't just dump a list of ideas\n\n")
	sb.WriteString("### Session Flow\n\n")
	sb.WriteString("Start by understanding the topic, then alternate between discussing ideas and capturing them. End the session when the problem space feels well-mapped and the key ideas have been captured as artifacts.\n")

	return sb.String(), nil
}
