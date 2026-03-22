package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/reviewsession"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
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
			runFlag, _ := cmd.Flags().GetString("run")

			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			store := runstore.NewStore(storeDir)

			var runID string
			// --run flag takes precedence over positional arg
			if runFlag != "" {
				runID = runFlag
			} else if len(args) > 0 && args[0] != "latest" {
				runID = args[0]
			} else {
				id, err := findLatestRunID(storeDir, "", store)
				if err != nil {
					return err
				}
				runID = id
			}

			// Use stdin for interactive input
			return reviewGuided(runID, storeDir, os.Stdin, cmd.OutOrStdout())
		},
	}
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	cmd.Flags().String("run", "", "Run ID to review (if not specified, uses latest or positional argument)")
	return cmd
}

// reviewGuided runs an interactive guided review for a run.
// Input is read from the provided reader (for testing, this can be a strings.Reader; in normal use, it's os.Stdin).
func reviewGuided(runID string, storeDir string, input io.Reader, out io.Writer) error {
	// Load run and ensure packet exists
	_, _, evidenceDir, err := loadRunAndEnsurePacket(runID, storeDir)
	if err != nil {
		return err
	}

	// Load review packet outputs
	outputs, err := loadPacketOutputs(evidenceDir)
	if err != nil {
		return fmt.Errorf("load packet outputs: %w", err)
	}

	// Render and display product review
	rendered := reviewpacket.RenderProductReview(outputs.ProductReview)
	fmt.Fprintln(out, rendered)

	// Create session and start interactive review
	session := reviewsession.Start(*outputs)
	scanner := bufio.NewScanner(input)

	// Process checklist items
	fmt.Fprintln(out, "\n=== Manual Review Checklist ===")
	for session.CurrentItem() != nil {
		item := session.CurrentItem()
		fmt.Fprintf(out, "[%d/%d] %s\n", session.CurrentStep+1, len(session.Checklist), item.Item.Title)
		if item.Item.Instructions != "" {
			fmt.Fprintf(out, "  Instructions: %s\n", item.Item.Instructions)
		}

		// Prompt for result
		fmt.Fprint(out, "\nResult (pass/fail/unsure/skipped): ")
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
			fmt.Fprintf(out, "Invalid result %q. Please use: pass, fail, unsure, or skipped.\n\n", result)
			continue
		}

		// Prompt for notes (optional)
		fmt.Fprint(out, "Notes (optional, press Enter to skip): ")
		var notes string
		if scanner.Scan() {
			notes = strings.TrimSpace(scanner.Text())
		}

		// Record the result
		if err := session.RecordItemResult(result, notes); err != nil {
			return fmt.Errorf("record item result: %w", err)
		}
		fmt.Fprintln(out)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	// All items completed, now prompt for outcome
	fmt.Fprintln(out, "=== Review Outcome ===")
	fmt.Fprintln(out, "Choose the final outcome:")
	fmt.Fprintln(out, "1. accepted (or accept) - Accept the work as-is")
	fmt.Fprintln(out, "2. rework_implementation_gap - Work needs fixes (implementation issue)")
	fmt.Fprintln(out, "3. rework_vision_change     - Work direction needs to change")

	var outcome string
	for {
		fmt.Fprint(out, "Outcome (accepted/rework_implementation_gap/rework_vision_change): ")
		if !scanner.Scan() {
			return fmt.Errorf("failed to read outcome")
		}
		outcome = strings.TrimSpace(scanner.Text())

		// Check for acceptance-specific validation first
		if outcome == "accept" || outcome == reviewsession.OutcomeAccepted {
			outcome = reviewsession.OutcomeAccepted
			canAccept, reason := session.CanAccept()
			if !canAccept {
				fmt.Fprintf(out, "Cannot accept: %s\n\n", reason)
				fmt.Fprintln(out, "Please choose a rework outcome instead.")
				continue
			}
			break
		}

		validOutcomes := map[string]bool{
			reviewsession.OutcomeAccepted:                true,
			reviewsession.OutcomeReworkImplementationGap: true,
			reviewsession.OutcomeReworkVisionChange:      true,
		}
		if validOutcomes[outcome] {
			break
		}

		fmt.Fprintf(out, "Invalid outcome %q.\n", outcome)
	}

	// Prompt for summary
	fmt.Fprint(out, "Summary of the review: ")
	var summary string
	if scanner.Scan() {
		summary = strings.TrimSpace(scanner.Text())
	}

	// If accepting with unsure items, require override reason
	var overrideReason string
	if outcome == reviewsession.OutcomeAccepted && session.NeedsOverride() {
		fmt.Fprintln(out, "Note: Unsure items found in the checklist.")
		fmt.Fprint(out, "Override reason (why accepting despite unsure items): ")
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

	if err := os.WriteFile(filepath.Join(evidenceDir, "review-outcome.json"), outcomeData, 0o644); err != nil {
		return fmt.Errorf("write review-outcome.json: %w", err)
	}

	// Load project config to get configured distiller tier (non-blocking, defaults to TierMedium)
	distillerTier := getDistillerTier(storeDir)

	// Attempt automatic distillation (non-blocking)
	const defaultClaudeBinary = "claude"
	defaultPolicy := execpolicy.DefaultPolicy()
	client, err := claude.NewClient(defaultClaudeBinary, []string{"--dangerously-skip-permissions"}, defaultPolicy.Budgets.MaxTaskDurationSeconds)
	if err != nil {
		log.Printf("distillation skipped: failed to create claude client: %v", err)
	} else {
		prov := provider.NewClaudeProvider(client, provider.DefaultTierToModelMap)
		adapter := llmadapter.New(prov, llmadapter.Config{
			Phase: "review",
			Tier:  string(distillerTier),
		})
		completer := NewInvokerAdapter(adapter)
		if err := attemptDistillation(runID, storeDir, distillerTier, completer); err != nil {
			log.Printf("distillation failed (non-blocking): %v", err)
		}
	}

	// Print summary
	fmt.Fprintln(out, "\n=== Review Complete ===")
	fmt.Fprintf(out, "Outcome: %s\n", outcome)
	if summary != "" {
		fmt.Fprintf(out, "Summary: %s\n", summary)
	}
	if overrideReason != "" {
		fmt.Fprintf(out, "Override: %s\n", overrideReason)
	}
	fmt.Fprintln(out, "Review saved to review-outcome.json")

	return nil
}

// getDistillerTier loads the configured distiller tier from project config,
// defaulting to TierMedium and logging if the config load fails (non-blocking).
func getDistillerTier(storeDir string) reviewdistiller.Tier {
	tier := reviewdistiller.TierMedium
	cfg, err := LoadProjectConfig(storeDir)
	if err != nil {
		log.Printf("load project config failed, using default tier: %v", err)
		return tier
	}
	return reviewdistiller.Tier(cfg.DistillerTier)
}

func init() {
	reviewCmd.AddCommand(newReviewGuidedCmd())
}
