package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/skills"
	"github.com/spf13/cobra"
)

var (
	planForce   bool
	planNoChain bool
)

const planSessionCommand = "plan"

var planSessionLauncherFn = runWithSessionWorktreeWithConflictSettings

var planCmd = &cobra.Command{
	Use:   "plan [spec-name]",
	Short: "Create an implementation plan from a spec",
	Long: `Start an interactive agent session to create an implementation plan from a spec.

Two input modes:
  gromit plan                    # Interactive picker for available specs
  gromit plan <spec-name>        # Plan a specific spec

The command launches the selected agent with:
- Full spec content as context
- Plans directory path for output
- Spec name for naming the plan file
- Open beads (current tasks) as project context
- References the gromit-plan skill for architecture and test planning

Plan refuses to run if a plan already exists for that spec unless --force is passed.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPlan,
}

func init() {
	planCmd.Flags().BoolVar(&planForce, "force", false, "Regenerate plan even if it already exists")
	planCmd.Flags().BoolVar(&planNoChain, "no-chain", false, "Skip offering to run next command in pipeline")
	planCmd.Flags().MarkHidden("no-chain")
	planCmd.Flags().String("agent", "", "Override the default agent for this plan session")
	planCmd.Flags().Bool("choose-agent", false, "Show interactive picker to choose agent")
	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
	// Get config and directories
	cfg, err := loadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = nil
	}

	gromitDir := resolveGromitDir(cfg)
	specsDir := resolveSpecsDir(cfg)
	plansDir := resolvePlansDir(cfg)

	// Ensure plans directory exists
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		return fmt.Errorf("creating plans directory: %w", err)
	}

	// Determine input mode and get spec name
	var specName string
	if len(args) == 0 {
		// Mode 1: Interactive picker for available specs
		specs, err := getSpecFiles(specsDir)
		if err != nil {
			return fmt.Errorf("scanning specs directory: %w", err)
		}

		// Filter to unplanned specs
		specs = filterUnplannedSpecs(specs, plansDir)

		if len(specs) == 0 {
			fmt.Println("No unplanned specs found.")
			fmt.Println("\nAll specs already have plans. Use 'gromit plan <spec-name> --force' to re-plan an existing spec.")
			return nil
		}

		// Display picker
		fmt.Println("Select a spec to plan:")
		fmt.Println()
		specNames := []string{}
		for i, specPath := range specs {
			name := strings.TrimSuffix(filepath.Base(specPath), ".md")
			specNames = append(specNames, name)
			fmt.Printf("  %d. %s\n", i+1, name)
		}

		fmt.Print("\nChoice [1-", len(specs), "]: ")
		reader := bufio.NewReader(os.Stdin)
		choiceStr, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading choice: %w", err)
		}

		var choice int
		if _, err := fmt.Sscanf(strings.TrimSpace(choiceStr), "%d", &choice); err != nil || choice < 1 || choice > len(specs) {
			return fmt.Errorf("invalid choice")
		}

		specName = specNames[choice-1]
		fmt.Printf("\nPlanning: %s\n\n", specName)

	} else {
		// Mode 2: Specific spec name
		specName = args[0]
		// Remove .md suffix if provided
		specName = strings.TrimSuffix(specName, ".md")
	}

	// Check if spec file exists
	specPath := filepath.Join(specsDir, specName+".md")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("spec not found: %s\nLooking for: %s", specName, specPath)
	}

	// Check if plan already exists
	planPath := filepath.Join(plansDir, specName+".md")
	if _, err := os.Stat(planPath); err == nil && !planForce {
		return fmt.Errorf("plan already exists: %s\nUse --force to regenerate", planPath)
	}

	// Load spec content
	_, specBody, err := frontmatter.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading spec file: %w", err)
	}

	// Gather context from beads
	beadClient, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}
	openBeads, err := beadClient.List()
	if err != nil {
		return fmt.Errorf("listing open beads: %w", err)
	}

	// Build context string with open beads
	var contextBuilder strings.Builder
	contextBuilder.WriteString("## Current Open Beads\n")
	if len(openBeads) == 0 {
		contextBuilder.WriteString("(None)\n")
	} else {
		for _, b := range openBeads {
			priority := fmt.Sprintf("P%d", b.Priority)
			contextBuilder.WriteString(fmt.Sprintf("- [%s] %s (%s)\n", priority, b.Title, b.ID))
			if b.Description != "" {
				// Indent description lines
				desc := strings.TrimSpace(b.Description)
				lines := strings.Split(desc, "\n")
				for _, line := range lines {
					contextBuilder.WriteString(fmt.Sprintf("  %s\n", line))
				}
			}
		}
	}

	// Build system prompt with spec content and embedded skill
	systemPrompt := fmt.Sprintf(`# Spec to Plan

Spec name: %s

## Spec Content

%s

## Context

%s

Plans directory: %s
Plan output path: %s

## Instructions

%s`, specName, specBody, contextBuilder.String(), plansDir, planPath, skills.PlanSkill)

	// Write system prompt to a temp file to avoid "argument list too long" errors
	// when the spec or open beads list is large
	tmpDir := filepath.Join(gromitDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("creating tmp dir: %w", err)
	}

	promptFile, err := os.CreateTemp(tmpDir, "plan-prompt-*.md")
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

	// Get flag values
	agentFlag, _ := cmd.Flags().GetString("agent")
	chooseAgent, _ := cmd.Flags().GetBool("choose-agent")

	// Resolve which agent to use
	selectedAgent, err := agent.Resolve(cfg, "plan", agentFlag, chooseAgent, os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("resolving agent: %w", err)
	}

	// Launch the agent with the prompt file
	if err := launchPlanSession(cfg, gromitDir, selectedAgent, promptPath); err != nil {
		return fmt.Errorf("launching agent: %w", err)
	}

	// Check if plan was created
	planCreated := false
	if _, err := os.Stat(planPath); err == nil {
		fmt.Printf("\n✓ Plan created: %s\n", planPath)
		planCreated = true
	}

	// Offer to chain to 'gromit decompose' if chaining is enabled and plan was created
	if !planNoChain && planCreated {
		chainAfterPlan(specName, plansDir)
	}

	return nil
}

func launchPlanSession(cfg *config.Config, gromitDir string, selectedAgent agent.Agent, promptPath string) error {
	if selectedAgent == nil {
		return fmt.Errorf("selected agent is nil")
	}

	if cfg != nil && !cfg.Worktree.IsEnabled() {
		return selectedAgent.LaunchInDir(promptPath, "")
	}

	conflictSettings := sessionConflictSettingsFromConfig(cfg)
	_, err := planSessionLauncherFn(gromitDir, planSessionCommand, conflictSettings, func(sessionDir string) error {
		return selectedAgent.LaunchInDir(promptPath, sessionDir)
	})
	return err
}

// filterUnplannedSpecs returns only specs that don't have a corresponding plan file
func filterUnplannedSpecs(specs []string, plansDir string) []string {
	unplanned := []string{}
	for _, specPath := range specs {
		specName := strings.TrimSuffix(filepath.Base(specPath), ".md")
		planPath := filepath.Join(plansDir, specName+".md")
		if _, err := os.Stat(planPath); os.IsNotExist(err) {
			unplanned = append(unplanned, specPath)
		}
	}
	return unplanned
}

// chainAfterPlan offers to run 'gromit decompose' after plan is created.
// Default is yes [Y/n] because decompose is a natural continuation of the pipeline.
func chainAfterPlan(planName string, plansDir string) {
	if isPlanDecomposed(plansDir, planName) {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	prompt := fmt.Sprintf("Run 'gromit decompose %s'?", planName)
	if confirmPrompt(reader, prompt, true) {
		if err := execGromit("decompose", planName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to execute decompose: %v\n", err)
		}
	}
}
