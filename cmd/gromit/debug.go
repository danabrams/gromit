package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
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
	debugCmd.Flags().String("agent", "", "Override the default agent for this debug session")
	debugCmd.Flags().Bool("choose-agent", false, "Show interactive picker to choose agent")
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
	reportsDir := filepath.Join(gromitDir, "reports")
	plansDir := resolvePlansDir(cfg)

	// Ensure reports directory exists
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return fmt.Errorf("creating reports dir: %w", err)
	}

	// Record existing artifacts before session
	existingReports, err := getReportFiles(reportsDir)
	if err != nil {
		return fmt.Errorf("scanning reports directory: %w", err)
	}

	existingPlans, err := getPlanFiles(plansDir)
	if err != nil {
		return fmt.Errorf("scanning plans directory: %w", err)
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

	// Build command args: flags + model + initial message
	cmdArgs := append([]string{}, claudeFlags...)
	if debugModel != "" {
		cmdArgs = append(cmdArgs, "--model", debugModel)
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

	// Post-session artifact detection
	return detectAndReportArtifacts(reportsDir, plansDir, existingReports, existingPlans, existingBacklogItems, bf, cfg)
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

// getMDFiles returns a list of .md files in the given directory
func getMDFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	mdFiles := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			mdFiles = append(mdFiles, filepath.Join(dir, entry.Name()))
		}
	}

	return mdFiles, nil
}

// getReportFiles returns a list of .md files in the reports directory
func getReportFiles(reportsDir string) ([]string, error) {
	return getMDFiles(reportsDir)
}

// getPlanFiles returns a list of .md files in the plans directory
func getPlanFiles(plansDir string) ([]string, error) {
	return getMDFiles(plansDir)
}

// getNewBacklogItems returns backlog items that are not in the existing list
func getNewBacklogItems(existing []*backlog.Idea, bf *backlog.File) ([]*backlog.Idea, error) {
	current, err := bf.List()
	if err != nil {
		return nil, err
	}

	// Build map of existing IDs
	existingIDs := make(map[string]bool)
	for _, item := range existing {
		existingIDs[item.ID] = true
	}

	// Find new items
	newItems := []*backlog.Idea{}
	for _, item := range current {
		if !existingIDs[item.ID] {
			newItems = append(newItems, item)
		}
	}

	return newItems, nil
}

// detectAndReportArtifacts scans for new reports, plans, and backlog items,
// then displays a summary and offers chaining options
func detectAndReportArtifacts(reportsDir, plansDir string, existingReports, existingPlans []string, existingBacklogItems []*backlog.Idea, bf *backlog.File, cfg *config.Config) error {
	// Scan for new artifacts
	newReports, err := getReportFiles(reportsDir)
	if err != nil {
		return fmt.Errorf("scanning reports directory: %w", err)
	}

	newPlans, err := getPlanFiles(plansDir)
	if err != nil {
		return fmt.Errorf("scanning plans directory: %w", err)
	}

	newBacklogItems, err := getNewBacklogItems(existingBacklogItems, bf)
	if err != nil {
		return fmt.Errorf("scanning backlog: %w", err)
	}

	// Find newly created artifacts
	createdReports := []string{}
	for _, report := range newReports {
		if !slices.Contains(existingReports, report) {
			createdReports = append(createdReports, report)
		}
	}

	createdPlans := []string{}
	for _, plan := range newPlans {
		if !slices.Contains(existingPlans, plan) {
			createdPlans = append(createdPlans, plan)
		}
	}

	// Display summary
	hasArtifacts := len(createdReports) > 0 || len(createdPlans) > 0 || len(newBacklogItems) > 0

	if hasArtifacts {
		fmt.Println()
	}

	if len(createdReports) > 0 {
		fmt.Println("Investigation reports created:")
		for _, report := range createdReports {
			fmt.Printf("  - %s\n", report)
		}
		fmt.Println()
	}

	if len(newBacklogItems) > 0 {
		fmt.Println("Backlog items created:")
		for _, item := range newBacklogItems {
			fmt.Printf("  - %s: %s\n", item.ID, item.Text)
		}
		fmt.Println()
	}

	// Offer chaining if plans were created
	if len(createdPlans) > 0 {
		fmt.Println("Plan files created:")
		for _, plan := range createdPlans {
			fmt.Printf("  - %s\n", plan)
		}
		fmt.Println()

		// Extract plan names for chaining
		planNames := make([]string, 0, len(createdPlans))
		for _, planPath := range createdPlans {
			planName := strings.TrimSuffix(filepath.Base(planPath), ".md")
			planNames = append(planNames, planName)
		}

		reader := bufio.NewReader(os.Stdin)
		chainAfterRefine(planNames, plansDir, func(prompt string, defaultYes bool) bool {
			return confirmPrompt(reader, prompt, defaultYes)
		}, execGromit)
	}

	return nil
}
