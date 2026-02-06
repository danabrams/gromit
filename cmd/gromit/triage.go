package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/spf13/cobra"
)

var triageCmd = &cobra.Command{
	Use:   "triage",
	Short: "Interactively triage open beads",
	Long: `Walk through open beads one at a time for quick triage decisions.

For each bead, you can:
  (k)eep       - No change, move to next
  (r)eprioritize - Change priority (P0-P4)
  (c)lose      - Mark as done/won't do
  (s)kip       - Skip for now, continue to next
  (q)uit       - Exit triage mode

Beads are shown in priority order (P0 first).`,
	RunE: runTriage,
}

func init() {
	rootCmd.AddCommand(triageCmd)
}

func runTriage(cmd *cobra.Command, args []string) error {
	client, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}

	// List all open beads in priority order
	beads, err := client.List()
	if err != nil {
		return fmt.Errorf("listing beads: %w", err)
	}

	if len(beads) == 0 {
		fmt.Println("No open beads to triage.")
		return nil
	}

	fmt.Printf("Triaging %d open beads\n\n", len(beads))

	reader := bufio.NewReader(os.Stdin)

	for i, b := range beads {
		// Display bead info
		fmt.Printf("[%d/%d] %s: %s (P%d)\n", i+1, len(beads), b.ID, b.Title, b.Priority)

		// Show creation time
		createdAt := ""
		if b.ID != "" {
			// Try to get full details to show created_at
			if detailed, err := client.Show(b.ID); err == nil && detailed != nil {
				b = detailed
			}
		}

		if createdAt != "" {
			fmt.Printf("  Created: %s\n", createdAt)
		}

		// Show labels if any
		if len(b.Labels) > 0 {
			fmt.Printf("  Labels: %s\n", strings.Join(b.Labels, ", "))
		}

		// Show description snippet if not empty
		if b.Description != "" {
			desc := b.Description
			if len(desc) > 100 {
				desc = desc[:97] + "..."
			}
			// Replace newlines with spaces for single-line display
			desc = strings.ReplaceAll(desc, "\n", " ")
			fmt.Printf("  Description: %s\n", desc)
		}

		fmt.Println()

		// Prompt for action
		for {
			fmt.Print("  (k)eep  (r)eprioritize  (c)lose  (s)kip  (q)uit: ")
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}

			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "k", "keep":
				fmt.Println("  ✓ Kept")
				fmt.Println()
				goto nextBead

			case "r", "reprioritize":
				newPriority, err := promptPriority(reader)
				if err != nil {
					fmt.Printf("  Error: %v\n", err)
					continue
				}

				if err := client.UpdatePriority(b.ID, newPriority); err != nil {
					fmt.Printf("  Error updating priority: %v\n", err)
					continue
				}

				fmt.Printf("  ✓ Changed priority to P%d\n", newPriority)
				fmt.Println()
				goto nextBead

			case "c", "close":
				reason, err := promptCloseReason(reader)
				if err != nil {
					fmt.Printf("  Error: %v\n", err)
					continue
				}

				if reason != "" {
					// Add comment with reason before closing
					if err := client.AddComment(b.ID, fmt.Sprintf("Closed during triage: %s", reason)); err != nil {
						fmt.Printf("  Warning: could not add comment: %v\n", err)
					}
				}

				if err := client.Close(b.ID); err != nil {
					fmt.Printf("  Error closing bead: %v\n", err)
					continue
				}

				fmt.Println("  ✓ Closed")
				fmt.Println()
				goto nextBead

			case "s", "skip":
				fmt.Println("  → Skipped")
				fmt.Println()
				goto nextBead

			case "q", "quit":
				fmt.Printf("\nTriage stopped at [%d/%d]\n", i, len(beads))
				return nil

			default:
				fmt.Println("  Invalid input. Use k, r, c, s, or q.")
			}
		}

	nextBead:
	}

	fmt.Println("✓ Triage complete!")
	return nil
}

func promptPriority(reader *bufio.Reader) (int, error) {
	fmt.Print("  New priority (0-4 or P0-P4): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("reading input: %w", err)
	}

	input = strings.TrimSpace(strings.ToUpper(input))

	// Handle P0-P4 format
	if strings.HasPrefix(input, "P") {
		input = strings.TrimPrefix(input, "P")
	}

	priority, err := strconv.Atoi(input)
	if err != nil || priority < 0 || priority > 4 {
		return 0, fmt.Errorf("invalid priority (must be 0-4)")
	}

	return priority, nil
}

func promptCloseReason(reader *bufio.Reader) (string, error) {
	fmt.Print("  Reason (optional, press Enter to skip): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}

	return strings.TrimSpace(input), nil
}
