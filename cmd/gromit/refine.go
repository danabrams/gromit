package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/agent"
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
	refineCmd.Flags().String("agent", "", "Override the default agent for this refine session")
	refineCmd.Flags().Bool("choose-agent", false, "Show interactive picker to choose agent")
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
	var isBlankSession bool

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
			// Empty backlog - skip picker and launch blank session
			fmt.Println("No unrefined backlog items. Starting a blank refinement session...")
			fmt.Println()
			isBlankSession = true
		} else {
			// Display picker with "Something new..." option
			fmt.Println("Select an idea to refine:")
			fmt.Println()
			for i, idea := range unrefined {
				typeLabel := formatTypeLabel(idea.Type)
				fmt.Printf("  %d. %s %s\n", i+1, typeLabel, idea.Text)
				if idea.Context != "" {
					fmt.Printf("     Context: %s\n", idea.Context)
				}
			}
			fmt.Printf("  %d. [new]     Something new...\n", len(unrefined)+1)

			fmt.Printf("\nChoice [1-%d]: ", len(unrefined)+1)
			reader := bufio.NewReader(os.Stdin)
			choiceStr, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reading choice: %w", err)
			}

			var choice int
			if _, err := fmt.Sscanf(strings.TrimSpace(choiceStr), "%d", &choice); err != nil || choice < 1 || choice > len(unrefined)+1 {
				return fmt.Errorf("invalid choice")
			}

			if choice == len(unrefined)+1 {
				// User selected "Something new..."
				fmt.Println("\nStarting a blank refinement session...")
				fmt.Println()
				isBlankSession = true
			} else {
				// User selected an existing unrefined item
				selectedIdea := unrefined[choice-1]
				ideaText = selectedIdea.Text
				if selectedIdea.Context != "" {
					ideaText = fmt.Sprintf("%s\n\nContext: %s", selectedIdea.Text, selectedIdea.Context)
				}
				backlogID = selectedIdea.ID
				fromBacklog = true

				fmt.Printf("\nRefining: %s\n\n", selectedIdea.Text)
			}
		}

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
	var systemPrompt string
	if isBlankSession {
		// Blank session - no pre-set idea text
		systemPrompt = fmt.Sprintf(`## Context

Specs directory: %s

## Instructions

%s`, specsDir, skills.RefineSkill)
	} else {
		// Normal session with pre-set idea text
		systemPrompt = fmt.Sprintf(`Idea to refine:

%s

## Context

Specs directory: %s

## Instructions

%s`, ideaText, specsDir, skills.RefineSkill)
	}

	// Write system prompt to a temp file to avoid "argument list too long" errors
	// when the idea text or context is large
	tmpDir := filepath.Join(gromitDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("creating tmp dir: %w", err)
	}

	promptFile, err := os.CreateTemp(tmpDir, "refine-prompt-*.md")
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
	selectedAgent, err := agent.Resolve(cfg, "refine", agentFlag, chooseAgent, os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("resolving agent: %w", err)
	}

	// Launch the agent with the prompt file
	if err := selectedAgent.Launch(promptPath); err != nil {
		return fmt.Errorf("launching agent: %w", err)
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

	// If blank session and a spec was created, auto-create a backlog item
	if isBlankSession && len(createdSpecs) > 0 {
		// Use the first new spec (should typically be only one)
		specPath := createdSpecs[0]
		specName := strings.TrimSuffix(filepath.Base(specPath), ".md")

		// Extract title from spec file
		specTitle := extractSpecTitle(specPath)
		if specTitle == "" {
			specTitle = specName // Fallback to filename if no title found
		}

		// Create backlog item
		idea := &backlog.Idea{
			ID:        backlog.GenerateID(),
			Text:      specTitle,
			Type:      "feature",
			Status:    "refined",
			SpecName:  specName,
			CreatedAt: time.Now(),
		}

		err := bf.Add(idea)
		if err != nil {
			// Don't fail the whole command if adding fails
			fmt.Fprintf(os.Stderr, "Warning: failed to create backlog item: %v\n", err)
		} else {
			fmt.Printf("\n✓ Created backlog item %s: %s (spec: %s)\n", idea.ID, specTitle, specName)
		}
	}

	if len(createdSpecs) > 0 {
		fmt.Printf("\nSpec files created:\n")
		for _, spec := range createdSpecs {
			fmt.Printf("  - %s\n", spec)
		}

		// Extract spec names and chain to next stages
		specNames := make([]string, 0, len(createdSpecs))
		for _, specPath := range createdSpecs {
			specName := strings.TrimSuffix(filepath.Base(specPath), ".md")
			specNames = append(specNames, specName)
		}

		plansDir := resolvePlansDir(cfg)
		reader := bufio.NewReader(os.Stdin)
		chainAfterRefine(specNames, plansDir, func(prompt string, defaultYes bool) bool {
			return confirmPrompt(reader, prompt, defaultYes)
		}, execGromit)
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

// extractSpecTitle reads a spec file and returns the first level-1 markdown heading text.
// Returns empty string if file is missing, empty, or has no level-1 heading.
// Handles frontmatter blocks (YAML between --- markers).
func extractSpecTitle(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	firstLine := true

	for scanner.Scan() {
		line := scanner.Text()

		// Check for frontmatter start/end on first line or after frontmatter start
		if firstLine && line == "---" {
			inFrontmatter = true
			firstLine = false
			continue
		}
		firstLine = false

		if inFrontmatter {
			if line == "---" {
				inFrontmatter = false
			}
			continue
		}

		// Look for level-1 heading
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}

	if err := scanner.Err(); err != nil {
		return ""
	}

	return ""
}
