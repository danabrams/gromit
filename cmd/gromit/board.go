package main

import (
	"fmt"
	"sort"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Show all beads grouped by status",
	Long: `Display all beads organized by status (open vs closed) with priority and title.

Open beads are sorted by priority (P0 first), while closed beads are listed without priority.`,
	RunE: runBoard,
}

func init() {
	rootCmd.AddCommand(boardCmd)
}

func runBoard(cmd *cobra.Command, args []string) error {
	client, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}
	open, closed, err := client.ListAll()
	if err != nil {
		return fmt.Errorf("listing beads: %w", err)
	}

	// Sort open beads by priority (lower number = higher priority)
	sort.Slice(open, func(i, j int) bool {
		return open[i].Priority < open[j].Priority
	})

	// Display open beads
	fmt.Printf("Open (%d)\n", len(open))
	if len(open) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, b := range open {
			priorityLabel := fmt.Sprintf("P%d", b.Priority)
			fmt.Printf("  %-3s %-20s %s\n", priorityLabel, b.ID, b.Title)
		}
	}

	fmt.Println()

	// Display closed beads
	fmt.Printf("Closed (%d)\n", len(closed))
	if len(closed) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, b := range closed {
			fmt.Printf("  %-20s %s\n", b.ID, b.Title)
		}
	}

	return nil
}
