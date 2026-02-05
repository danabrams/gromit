package main

import (
	"fmt"
	"os"
	"time"

	"github.com/danabrams/ralph-runner/internal/backlog"
	"github.com/spf13/cobra"
)

var (
	filterType   string
	recentDays   int
	deleteIdeaID string
)

var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "View and manage the idea backlog",
	Long: `List ideas in the backlog that haven't been refined into beads yet.

Examples:
  ralph backlog              # List all ideas
  ralph backlog --type bug   # Filter by type
  ralph backlog --recent 7   # Ideas from last 7 days
  ralph backlog delete <id>  # Remove an idea`,
	RunE: runBacklog,
}

var deleteBacklogCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an idea from the backlog",
	Args:  cobra.ExactArgs(1),
	RunE:  runBacklogDelete,
}

func init() {
	backlogCmd.Flags().StringVar(&filterType, "type", "", "Filter by type (feature, bug, chore)")
	backlogCmd.Flags().IntVar(&recentDays, "recent", 0, "Show only ideas from last N days")

	backlogCmd.AddCommand(deleteBacklogCmd)
	rootCmd.AddCommand(backlogCmd)
}

func runBacklog(cmd *cobra.Command, args []string) error {
	// Get .ralph directory from config or default
	ralphDir := ".ralph"
	if cfg, err := loadConfig(); err == nil {
		if cfg.Paths.RalphDir != "" {
			ralphDir = cfg.Paths.RalphDir
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("loading config: %w", err)
	}

	// Load backlog
	bf := backlog.NewFile(ralphDir)
	ideas, err := bf.List()
	if err != nil {
		return fmt.Errorf("loading backlog: %w", err)
	}

	if len(ideas) == 0 {
		fmt.Println("Backlog is empty.")
		fmt.Println("\nUse 'ralph add <idea>' to capture ideas quickly.")
		return nil
	}

	// Apply filters
	filtered := filterIdeas(ideas, filterType, recentDays)

	if len(filtered) == 0 {
		fmt.Printf("No ideas found matching filters (type=%s, recent=%d days).\n", filterType, recentDays)
		return nil
	}

	// Display header
	if filterType != "" || recentDays > 0 {
		fmt.Printf("Backlog (%d of %d ideas", len(filtered), len(ideas))
		if filterType != "" {
			fmt.Printf(", type=%s", filterType)
		}
		if recentDays > 0 {
			fmt.Printf(", recent=%d days", recentDays)
		}
		fmt.Println(")")
	} else {
		fmt.Printf("Backlog (%d ideas)\n", len(ideas))
	}
	fmt.Println()

	// Display ideas
	for _, idea := range filtered {
		// Format: idea-001 [feature] Add cost tracking
		typeLabel := formatType(idea.Type)
		fmt.Printf("  %s %s %s\n", idea.ID, typeLabel, idea.Text)
	}

	fmt.Println()
	fmt.Println("Run 'ralph refine' to review and promote to tasks.")

	return nil
}

func runBacklogDelete(cmd *cobra.Command, args []string) error {
	ideaID := args[0]

	// Get .ralph directory from config or default
	ralphDir := ".ralph"
	if cfg, err := loadConfig(); err == nil {
		if cfg.Paths.RalphDir != "" {
			ralphDir = cfg.Paths.RalphDir
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("loading config: %w", err)
	}

	// Delete idea
	bf := backlog.NewFile(ralphDir)
	if err := bf.Delete(ideaID); err != nil {
		return fmt.Errorf("deleting idea: %w", err)
	}

	fmt.Printf("✓ Deleted idea: %s\n", ideaID)
	return nil
}

func filterIdeas(ideas []*backlog.Idea, typeFilter string, recentDays int) []*backlog.Idea {
	var filtered []*backlog.Idea

	cutoff := time.Time{}
	if recentDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -recentDays)
	}

	for _, idea := range ideas {
		// Filter by type
		if typeFilter != "" && idea.Type != typeFilter {
			continue
		}

		// Filter by recency
		if recentDays > 0 && idea.CreatedAt.Before(cutoff) {
			continue
		}

		filtered = append(filtered, idea)
	}

	return filtered
}

func formatType(ideaType string) string {
	// Format as colored label: [feature], [bug], [chore]
	// Using fixed-width for alignment
	typeMap := map[string]string{
		"feature": "[feature]",
		"bug":     "[bug]    ",
		"chore":   "[chore]  ",
		"unknown": "[unknown]",
	}

	if label, ok := typeMap[ideaType]; ok {
		return label
	}
	// Fallback for any unexpected types
	return fmt.Sprintf("[%-7s]", ideaType)
}
