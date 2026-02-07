package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/skills"
	"github.com/spf13/cobra"
)

var refineCmd = &cobra.Command{
	Use:   "refine [backlog-id or idea text]",
	Short: "Refine ideas into structured specs",
	Long: `Start an interactive Claude Code session to refine ideas into structured specifications.

Three input modes:
  gromit refine                    # Interactive picker for unrefined backlog items
  gromit refine <backlog-id>       # Refine a specific backlog item
  gromit refine "some idea text"   # Refine an ad-hoc idea (not in backlog)

The command launches Claude with:
- The idea text as context
- Specs directory path for output
- References the gromit-refine skill for conversational refinement

After Claude exits, scans for new spec files and marks backlog items as refined.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRefine,
}

func init() {
	rootCmd.AddCommand(refineCmd)
}

func runRefine(cmd *cobra.Command, args []string) error {
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

	// Load backlog
	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		return fmt.Errorf("creating backlog file: %w", err)
	}

	// Determine input mode and get idea text
	var ideaText string
	var backlogID string
	var fromBacklog bool

	if len(args) == 0 {
		// Mode 1: Interactive picker for unrefined backlog items
		ideas, err := bf.List()
		if err != nil {
			return fmt.Errorf("loading backlog: %w", err)
		}

		// Filter to unrefined items
		unrefined := []*backlog.Idea{}
		for _, idea := range ideas {
			if idea.Status != "refined" {
				unrefined = append(unrefined, idea)
			}
		}

		if len(unrefined) == 0 {
			fmt.Println("No unrefined backlog items found.")
			fmt.Println("\nUse 'gromit add <idea>' to add new ideas, or")
			fmt.Println("use 'gromit refine \"idea text\"' to refine an ad-hoc idea.")
			return nil
		}

		// Display picker
		fmt.Println("Select an idea to refine:")
		fmt.Println()
		for i, idea := range unrefined {
			typeLabel := formatTypeLabel(idea.Type)
			fmt.Printf("  %d. %s %s\n", i+1, typeLabel, idea.Text)
			if idea.Context != "" {
				fmt.Printf("     Context: %s\n", idea.Context)
			}
		}

		fmt.Print("\nChoice [1-", len(unrefined), "]: ")
		reader := bufio.NewReader(os.Stdin)
		choiceStr, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading choice: %w", err)
		}

		var choice int
		if _, err := fmt.Sscanf(strings.TrimSpace(choiceStr), "%d", &choice); err != nil || choice < 1 || choice > len(unrefined) {
			return fmt.Errorf("invalid choice")
		}

		selectedIdea := unrefined[choice-1]
		ideaText = selectedIdea.Text
		if selectedIdea.Context != "" {
			ideaText = fmt.Sprintf("%s\n\nContext: %s", selectedIdea.Text, selectedIdea.Context)
		}
		backlogID = selectedIdea.ID
		fromBacklog = true

		fmt.Printf("\nRefining: %s\n\n", selectedIdea.Text)

	} else {
		// Check if arg is a backlog ID or ad-hoc text
		arg := args[0]
		if strings.HasPrefix(arg, "idea-") {
			// Mode 2: Specific backlog ID
			idea, err := bf.Get(arg)
			if err != nil {
				return fmt.Errorf("loading backlog item: %w", err)
			}
			if idea == nil {
				return fmt.Errorf("backlog item not found: %s", arg)
			}

			ideaText = idea.Text
			if idea.Context != "" {
				ideaText = fmt.Sprintf("%s\n\nContext: %s", idea.Text, idea.Context)
			}
			backlogID = idea.ID
			fromBacklog = true

		} else {
			// Mode 3: Ad-hoc idea text
			ideaText = arg
			fromBacklog = false
		}
	}

	// Record existing spec files before running Claude
	existingSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		return fmt.Errorf("scanning specs directory: %w", err)
	}

	// Build system prompt with embedded skill content
	systemPrompt := fmt.Sprintf(`Idea to refine:

%s

## Context

Specs directory: %s

## Instructions

%s`, ideaText, specsDir, skills.RefineSkill)

	// Determine binary and flags from config
	claudeBinary := "claude"
	var claudeFlags []string
	if cfg != nil {
		claudeBinary = cfg.Claude.Binary
		claudeFlags = cfg.Claude.Flags
	}

	// Build command args: flags + --append-system-prompt + system prompt + initial message
	cmdArgs := append([]string{}, claudeFlags...)
	cmdArgs = append(cmdArgs, "--append-system-prompt", systemPrompt, "Begin refining this idea into a structured spec following the instructions above.")

	// Launch Claude Code with system prompt and initial message
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

	// Scan for new spec files
	newSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		return fmt.Errorf("scanning specs directory after Claude: %w", err)
	}

	// Find newly created spec(s)
	createdSpecs := []string{}
	for _, spec := range newSpecs {
		if !containsSpec(existingSpecs, spec) {
			createdSpecs = append(createdSpecs, spec)
		}
	}

	// If from backlog and a spec was created, mark as refined
	if fromBacklog && len(createdSpecs) > 0 {
		// Use the first new spec (should typically be only one)
		specPath := createdSpecs[0]
		specName := strings.TrimSuffix(filepath.Base(specPath), ".md")

		// Update backlog item status
		err := bf.Update(backlogID, func(idea *backlog.Idea) {
			idea.Status = "refined"
			idea.SpecName = specName
		})
		if err != nil {
			// Don't fail the whole command if update fails
			fmt.Fprintf(os.Stderr, "Warning: failed to update backlog item status: %v\n", err)
		} else {
			fmt.Printf("\n✓ Marked backlog item %s as refined (spec: %s)\n", backlogID, specName)
		}
	}

	if len(createdSpecs) > 0 {
		fmt.Printf("\nSpec files created:\n")
		for _, spec := range createdSpecs {
			fmt.Printf("  - %s\n", spec)
		}
	}

	return nil
}

// getSpecFiles returns a list of .md files in the specs directory
func getSpecFiles(specsDir string) ([]string, error) {
	// Ensure directory exists
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, err
	}

	specs := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			specs = append(specs, filepath.Join(specsDir, entry.Name()))
		}
	}

	return specs, nil
}

// containsSpec checks if a string slice contains a value
func containsSpec(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// formatTypeLabel formats type as colored label
func formatTypeLabel(ideaType string) string {
	typeMap := map[string]string{
		"feature": "[feature]",
		"bug":     "[bug]    ",
		"chore":   "[chore]  ",
		"unknown": "[unknown]",
	}

	if label, ok := typeMap[ideaType]; ok {
		return label
	}
	return fmt.Sprintf("[%-7s]", ideaType)
}
