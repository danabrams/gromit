package main

import (
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/spf13/cobra"
)

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Show processing queue with model assignments",
	Long: `Display the beads Gromit will process, in order, with their assigned models based on priority and labels.

Also shows any blocked beads and the reason they can't be processed yet.`,
	RunE: showQueue,
}

func init() {
	rootCmd.AddCommand(queueCmd)
}

func showQueue(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	bc, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}

	// Get ready beads (unblocked, type=task to exclude epics)
	readyBeads, err := getReadyBeads(bc)
	if err != nil {
		return fmt.Errorf("getting ready beads: %w", err)
	}

	// Get all open beads to identify blocked ones
	allBeads, err := bc.List()
	if err != nil {
		return fmt.Errorf("getting all beads: %w", err)
	}

	// Identify blocked beads (open but not ready)
	blockedBeads := findBlockedBeads(readyBeads, allBeads)

	// Display ready queue
	if len(readyBeads) > 0 {
		fmt.Println("Queue (" + fmt.Sprintf("%d", len(readyBeads)) + " beads ready):")
		for i, b := range readyBeads {
			model := cfg.SelectModel(b.Priority, b.Labels)
			priorityStr := fmt.Sprintf("P%d", b.Priority)
			fmt.Printf("  %d. [%s] %s  %s  → %s\n",
				i+1,
				priorityStr,
				b.ID,
				truncateTitle(b.Title, 30),
				model)
		}
	} else {
		fmt.Println("Queue: empty (no beads ready)")
	}

	// Display blocked beads
	if len(blockedBeads) > 0 {
		fmt.Println()
		fmt.Println("Blocked (" + fmt.Sprintf("%d", len(blockedBeads)) + "):")
		for _, b := range blockedBeads {
			reasonStr := getReason(b, allBeads)
			fmt.Printf("  [P%d] %s  %s  (%s)\n",
				b.Priority,
				b.ID,
				truncateTitle(b.Title, 30),
				reasonStr)
		}
	}

	return nil
}

// getReadyBeads fetches ready beads using bd ready --json
// This simulates getting multiple ready beads instead of just one
func getReadyBeads(bc *bead.Client) ([]*bead.Bead, error) {
	if bc == nil {
		return nil, fmt.Errorf("bead client is nil")
	}

	// Note: bd ready doesn't have a way to get all ready beads at once,
	// only the next one. We can use bd list with a filter to get all open
	// beads, then filter for ready ones by checking which don't have parent
	// relationships blocking them. For now, we'll just use the list and
	// identify the ones that would be ready.

	// Get all open beads sorted by priority
	allOpen, err := bc.List()
	if err != nil {
		return nil, fmt.Errorf("getting all open beads: %w", err)
	}

	readyBeads := []*bead.Bead{}
	parentMap := make(map[string]bool)

	// Build a map of beads that are parents (not yet closed)
	for _, b := range allOpen {
		if b.Parent != "" {
			parentMap[b.Parent] = true
		}
	}

	// Filter: a bead is ready if it has no parent, or its parent is closed
	for _, b := range allOpen {
		// If no parent, it's ready
		if b.Parent == "" {
			readyBeads = append(readyBeads, b)
			continue
		}

		// If parent exists in open beads, this is blocked
		isBlocked := false
		for _, openB := range allOpen {
			if openB.ID == b.Parent {
				isBlocked = true
				break
			}
		}

		if !isBlocked {
			// Parent is closed, so this is ready
			readyBeads = append(readyBeads, b)
		}
	}

	return readyBeads, nil
}

// findBlockedBeads returns beads that are open but not in the ready list
func findBlockedBeads(readyBeads, allBeads []*bead.Bead) []*bead.Bead {
	readyMap := make(map[string]bool)
	for _, b := range readyBeads {
		readyMap[b.ID] = true
	}

	blocked := []*bead.Bead{}
	for _, b := range allBeads {
		if !readyMap[b.ID] {
			blocked = append(blocked, b)
		}
	}
	return blocked
}

// getReason returns a human-readable reason why a bead is blocked
func getReason(b *bead.Bead, allBeads []*bead.Bead) string {
	if b == nil {
		return "unknown"
	}

	if b.Parent != "" {
		// Check if parent still exists in open beads
		for _, openB := range allBeads {
			if openB.ID == b.Parent {
				return fmt.Sprintf("blocked by: %s", b.Parent)
			}
		}
		// Parent doesn't exist in open beads, so it should be ready
		// (this shouldn't happen in normal flow)
		return "parent closed but still blocked"
	}

	return "unknown reason"
}

// truncateTitle returns a title truncated to maxLen characters with ellipsis if needed
func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	return title[:maxLen-3] + "..."
}
