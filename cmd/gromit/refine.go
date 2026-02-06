package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/danabrams/ralph-runner/internal/backlog"
	"github.com/spf13/cobra"
)

var refineCmd = &cobra.Command{
	Use:   "refine",
	Short: "Launch Claude Code session for backlog refinement",
	Long: `Start an interactive Claude Code session with backlog context pre-loaded.

The command launches Claude with:
- Backlog ideas as context for refinement
- References the ralph-refine skill for brainstorming and breaking down ideas

Example:
  ralph refine`,
	Args: cobra.NoArgs,
	RunE: runRefine,
}

func init() {
	rootCmd.AddCommand(refineCmd)
}

func runRefine(cmd *cobra.Command, args []string) error {
	// Get .ralph directory from config or default
	ralphDir := ".ralph"
	if cfg, err := loadConfig(); err == nil {
		if cfg.Paths.RalphDir != "" {
			ralphDir = cfg.Paths.RalphDir
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("loading config: %w", err)
	}

	// Load backlog items
	bf, err := backlog.NewFile(ralphDir)
	if err != nil {
		return fmt.Errorf("creating backlog file: %w", err)
	}
	ideas, err := bf.List()
	if err != nil {
		return fmt.Errorf("loading backlog: %w", err)
	}

	// Build context string with backlog items
	var contextBuilder strings.Builder
	contextBuilder.WriteString("## Backlog Items to Refine\n")
	if len(ideas) == 0 {
		contextBuilder.WriteString("(Backlog is empty)\n")
	} else {
		for _, idea := range ideas {
			typeLabel := idea.Type
			contextBuilder.WriteString(fmt.Sprintf("- [%s] %s (%s)\n", typeLabel, idea.Text, idea.ID))
			if idea.Context != "" {
				contextBuilder.WriteString(fmt.Sprintf("  Context: %s\n", idea.Context))
			}
		}
	}

	// Build system prompt with backlog and instructions
	systemPrompt := fmt.Sprintf(`Backlog to refine: %d items

%s

## Instructions
Use the ralph-refine skill to help brainstorm and refine these backlog items into properly-sized beads (tasks).
The ralph-refine skill is designed for backlog refinement and will guide you through:
- Clarifying vague or ambiguous ideas
- Breaking down large ideas into smaller tasks
- Creating acceptance criteria
- Determining dependencies and complexity
- Promoting ideas to beads with bd create`, len(ideas), contextBuilder.String())

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
