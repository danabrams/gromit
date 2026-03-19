package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/next/reviewsession"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/spf13/cobra"
)

// newReviewGuidedCmd creates the `review guided` command.
func newReviewGuidedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guided [run-id]",
		Short: "Interactively review a run with guided prompts",
		Long: `Interactively review a run by stepping through each manual checklist item.
If no run-id is given, the latest run is reviewed (by modification time of .gromit-next/runs/).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			storeDir, _ := cmd.Flags().GetString("store-dir")

			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			var runID string
			if len(args) == 0 || args[0] == "latest" {
				id, err := findLatestRunID(storeDir, "", nil)
				if err != nil {
					return err
				}
				runID = id
			} else {
				runID = args[0]
			}

			// Use stdin for interactive input
			return reviewGuided(runID, storeDir, os.Stdin)
		},
	}
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	return cmd
}

// reviewGuided runs an interactive guided review for a run.
// Input is read from the provided reader (for testing, this can be a strings.Reader; in normal use, it's os.Stdin).
func reviewGuided(runID string, storeDir string, input io.Reader) error {
	// Load run and ensure packet exists
	_, err := loadRunAndEnsurePacket(runID, storeDir)
	if err != nil {
		return err
	}

	// Initialize run store
	if storeDir == "" {
		storeDir = ".gromit-next"
	}
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(runID)

	// Load review packet outputs
	outputs, err := loadPacketOutputs(evidenceDir)
	if err != nil {
		return fmt.Errorf("load packet outputs: %w", err)
	}

	// Create session and start interactive review
	session := reviewsession.Start(*outputs)
	scanner := bufio.NewScanner(input)

	// Process checklist items
	fmt.Println("\n=== Manual Review Checklist ===")
	for session.CurrentItem() != nil {
		item := session.CurrentItem()
		fmt.Printf("[%d/%d] %s\n", session.CurrentStep+1, len(session.Checklist), item.Item.Title)
		if item.Item.Instructions != "" {
			fmt.Printf("  Instructions: %s\n", item.Item.Instructions)
		}

		// Prompt for result
		fmt.Print("\nResult (pass/fail/unsure/skip): ")
		if !scanner.Scan() {
			return fmt.Errorf("failed to read result")
		}
		result := strings.TrimSpace(scanner.Text())

		// Validate result
		validResults := map[string]bool{
			reviewsession.ResultPass:    true,
			reviewsession.ResultFail:    true,
			reviewsession.ResultUnsure:  true,
			reviewsession.ResultSkipped: true,
		}
		if !validResults[result] {
			fmt.Printf("Invalid result %q. Please use: pass, fail, unsure, or skip.\n\n", result)
			continue
		}

		// Prompt for notes (optional)
		fmt.Print("Notes (optional, press Enter to skip): ")
		var notes string
		if scanner.Scan() {
			notes = strings.TrimSpace(scanner.Text())
		}

		// Record the result
		if err := session.RecordItemResult(result, notes); err != nil {
			return fmt.Errorf("record item result: %w", err)
		}
		fmt.Println()
	}

	// All items completed, now prompt for outcome
	fmt.Println("=== Review Outcome ===")
	fmt.Println("Choose the final outcome:")
	fmt.Println("1. accept         - Accept the work as-is")
	fmt.Println("2. rework_implementation_gap - Work needs fixes (implementation issue)")
	fmt.Println("3. rework_vision_change     - Work direction needs to change")

	var outcome string
	for {
		fmt.Print("Outcome (accept/rework_implementation_gap/rework_vision_change): ")
		if !scanner.Scan() {
			return fmt.Errorf("failed to read outcome")
		}
		outcome = strings.TrimSpace(scanner.Text())

		validOutcomes := map[string]bool{
			reviewsession.OutcomeAccepted:                true,
			reviewsession.OutcomeReworkImplementationGap: true,
			reviewsession.OutcomeReworkVisionChange:      true,
		}
		if validOutcomes[outcome] {
			break
		}

		// Check for acceptance-specific validation
		if outcome == "accept" || outcome == reviewsession.OutcomeAccepted {
			outcome = reviewsession.OutcomeAccepted
			canAccept, reason := session.CanAccept()
			if !canAccept {
				fmt.Printf("Cannot accept: %s\n\n", reason)
				fmt.Println("Please choose a rework outcome instead.")
				continue
			}
			break
		}

		fmt.Printf("Invalid outcome %q.\n", outcome)
	}

	// Prompt for summary
	fmt.Print("Summary of the review: ")
	var summary string
	if scanner.Scan() {
		summary = strings.TrimSpace(scanner.Text())
	}

	// If accepting with unsure items, require override reason
	var overrideReason string
	if outcome == reviewsession.OutcomeAccepted && session.NeedsOverride() {
		fmt.Println("Note: Unsure items found in the checklist.")
		fmt.Print("Override reason (why accepting despite unsure items): ")
		if scanner.Scan() {
			overrideReason = strings.TrimSpace(scanner.Text())
		}
	}

	// Record the outcome
	reviewOutcome, err := session.RecordOutcome(outcome, summary, overrideReason)
	if err != nil {
		return fmt.Errorf("record outcome: %w", err)
	}

	// Write review-outcome.json
	reviewOutcome.NormalizeNilFields()
	outcomeData, err := json.MarshalIndent(reviewOutcome, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal review outcome: %w", err)
	}

	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if err := os.WriteFile(outcomeFile, outcomeData, 0o644); err != nil {
		return fmt.Errorf("write review-outcome.json: %w", err)
	}

	// Print summary
	fmt.Println("\n=== Review Complete ===")
	fmt.Printf("Outcome: %s\n", outcome)
	if summary != "" {
		fmt.Printf("Summary: %s\n", summary)
	}
	if overrideReason != "" {
		fmt.Printf("Override: %s\n", overrideReason)
	}
	fmt.Println("Review saved to review-outcome.json")

	return nil
}

func init() {
	reviewCmd.AddCommand(newReviewGuidedCmd())
}
