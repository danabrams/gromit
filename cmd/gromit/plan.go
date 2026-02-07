package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/skills"
	"github.com/spf13/cobra"
)

var (
	planForce bool
)

var planCmd = &cobra.Command{
	Use:   "plan [spec-name]",
	Short: "Create an implementation plan from a spec",
	Long: `Start an interactive Claude Code session to create an implementation plan from a spec.

Two input modes:
  gromit plan                    # Interactive picker for available specs
  gromit plan <spec-name>        # Plan a specific spec

The command launches Claude with:
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

		if len(specs) == 0 {
			fmt.Println("No specs found in", specsDir)
			fmt.Println("\nUse 'gromit refine' to create a spec first.")
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

	// Launch Claude Code with system prompt and initial message
	claudeCmd := exec.Command("claude", "--append-system-prompt", systemPrompt, "Begin creating an implementation plan for this spec following the instructions above.")
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

	// Check if plan was created
	if _, err := os.Stat(planPath); err == nil {
		fmt.Printf("\n✓ Plan created: %s\n", planPath)
	}

	return nil
}
