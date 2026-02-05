package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/ralph-runner/internal/backlog"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <idea>",
	Short: "Add an idea to the backlog",
	Long: `Quickly capture ideas to the backlog without creating beads yet.

Examples:
  ralph add "Add cost tracking"
  ralph add "Fix the auth bug"
  ralph add "What if we had a web dashboard?"

The command will auto-categorize ideas when obvious (feature/bug/chore)
or ask for clarification when needed.`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	ideaText := args[0]

	// Get .ralph directory from config or default
	ralphDir := ".ralph"
	if cfg, err := loadConfig(); err == nil {
		if cfg.Paths.RalphDir != "" {
			ralphDir = cfg.Paths.RalphDir
		}
	} else if !os.IsNotExist(err) {
		// If config exists but can't be loaded, show error
		return fmt.Errorf("loading config: %w", err)
	}
	// If config doesn't exist, use default .ralph

	// Auto-categorize
	ideaType := backlog.CategorizeIdea(ideaText)

	// If unknown, ask user
	if ideaType == "unknown" {
		fmt.Println("What type of idea is this?")
		fmt.Println("  1. Feature - New functionality")
		fmt.Println("  2. Bug - Something broken to fix")
		fmt.Println("  3. Chore - Refactor, update, or maintenance")
		fmt.Print("\nChoice [1-3]: ")

		var choice string
		fmt.Scanln(&choice)
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			ideaType = "feature"
		case "2":
			ideaType = "bug"
		case "3":
			ideaType = "chore"
		default:
			fmt.Println("Invalid choice, defaulting to 'feature'")
			ideaType = "feature"
		}
	}

	// Ask for optional context
	fmt.Print("\nAny additional context? (optional, press Enter to skip): ")
	var context string
	fmt.Scanln(&context)
	context = strings.TrimSpace(context)

	// Create idea
	idea := &backlog.Idea{
		ID:        backlog.GenerateID(),
		Text:      ideaText,
		Type:      ideaType,
		Context:   context,
		CreatedAt: time.Now(),
	}

	// Save to backlog
	bf := backlog.NewFile(ralphDir)
	if err := bf.Add(idea); err != nil {
		return fmt.Errorf("saving to backlog: %w", err)
	}

	// Confirm
	fmt.Printf("\n✓ Added to backlog (%s): %s\n", ideaType, ideaText)
	if context != "" {
		fmt.Printf("  Context: %s\n", context)
	}
	fmt.Printf("  Saved to: %s\n", filepath.Join(ralphDir, "backlog.jsonl"))

	return nil
}
