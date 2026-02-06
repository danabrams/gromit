package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan <feature>",
	Short: "Launch Claude Code session for feature planning",
	Long: `Start an interactive Claude Code session with feature context pre-loaded.

The command launches Claude with:
- Feature description as context
- Open beads (current tasks) as project context
- References the ralph-plan skill for feature decomposition

Example:
  ralph plan "Add tmux integration"
  ralph plan "Implement OAuth authentication"`,
	Args: cobra.ExactArgs(1),
	RunE: runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
	featureDescription := args[0]

	// Verify config exists (ralph must be initialized)
	if _, err := loadConfig(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading config: %w", err)
	}

	// Gather context from beads
	beadClient := bead.NewClient()
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

	// Build system prompt with feature and context
	systemPrompt := fmt.Sprintf(`Feature to plan: %s

%s

## Instructions
Use the ralph-plan skill to help decompose this feature into properly-sized beads (tasks).
The ralph-plan skill is designed for feature decomposition and will guide you through:
- Breaking down the feature into atomic work items
- Creating beads with proper acceptance criteria
- Considering dependencies and complexity
- Sizing beads appropriately for the Ralph loop`, featureDescription, contextBuilder.String())

	// Launch Claude Code with system prompt
	claudeCmd := exec.Command("claude", "--append-system-prompt", systemPrompt)
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
