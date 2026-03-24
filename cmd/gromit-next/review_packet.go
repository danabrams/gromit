package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/reviewsession"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/workspace"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review command group for running and recording reviews",
}

// newReviewRecordCmd creates the `review record` command.
func newReviewRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record [run-id]",
		Short: "Record a review outcome for a run with --outcome and --summary flags",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outcome, _ := cmd.Flags().GetString("outcome")
			summary, _ := cmd.Flags().GetString("summary")
			override, _ := cmd.Flags().GetString("override")
			storeDir, _ := cmd.Flags().GetString("store-dir")
			runFlag, _ := cmd.Flags().GetString("run")
			specsDir, _ := cmd.Flags().GetString("specs-dir")
			project, _ := cmd.Flags().GetString("project")

			if outcome == "" {
				return fmt.Errorf("--outcome flag is required")
			}

			var runID string
			// --run flag takes precedence over positional arg
			if runFlag != "" {
				runID = runFlag
			} else if len(args) > 0 {
				runID = args[0]
			} else {
				return fmt.Errorf("run ID is required (provide via --run flag or positional argument)")
			}

			// Resolve specsDir from project config if not explicitly provided
			// (same pattern as exec_complete.go lines 38-56)
			if specsDir == "" && project != "" {
				resolver := workspace.NewEnvResolver()
				root, _ := resolver.Resolve()
				if root != "" {
					projectDir, _ := ResolveProjectConfigPath(root, project)
					if cfg, err := LoadProjectConfig(projectDir); err == nil {
						specsDir = cfg.SpecsDir
						if specsDir == "" && cfg.RepoPath != "" {
							specsDir = filepath.Join(cfg.RepoPath, "docs", "specs")
						}
					}
				}
			}

			err := reviewRecord(runID, storeDir, outcome, summary, override)
			if err != nil {
				return err
			}

			// After reviewRecord succeeds, handle remediation spec if accepted
			if outcome == "accepted" {
				if specsDir == "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping remediation spec generation (specs-dir not configured)\n")
				} else {
					specPath, err := maybeGenerateRemediationSpec(runID, storeDir, specsDir)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to generate remediation spec: %v\n", err)
					} else if specPath != "" {
						fmt.Fprintln(cmd.OutOrStdout(), specPath)
					}
				}
			}

			return nil
		},
	}
	cmd.Flags().String("outcome", "", "The review outcome (accepted, rework_implementation_gap, rework_vision_change)")
	cmd.Flags().String("summary", "", "Summary of the review")
	cmd.Flags().String("override", "", "Override reason for accepting a run with unsure items")
	cmd.Flags().String("store-dir", "", "Run store directory (default: .gromit-next)")
	cmd.Flags().String("run", "", "Run ID to record (if not specified, uses positional argument)")
	cmd.Flags().String("specs-dir", "", "Override specs directory (for testing)")
	cmd.Flags().String("project", "", "Project name for resolving specsDir from config")
	return cmd
}

func init() {
	reviewCmd.AddCommand(newReviewRecordCmd())
}

// loadRunAndEnsurePacket loads the run from store, checks IsTerminal,
// verifies review packet artifacts exist, and regenerates them if missing.
// Returns the loaded run state, the store, the evidence directory, and any error.
func loadRunAndEnsurePacket(runID, storeDir string) (*runstore.RunState, *runstore.Store, string, error) {
	// Default store directory
	if storeDir == "" {
		storeDir = ".gromit-next"
	}

	// Initialize run store
	store := runstore.NewStore(storeDir)

	// Load the run
	run, err := store.Get(runID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("load run %q: %w", runID, err)
	}

	// Check if run is terminal
	if !run.IsTerminal() {
		return nil, nil, "", fmt.Errorf("cannot review non-terminal run (status: %s) - run must be in ready_for_review, needs_human, blocked, or completed state", run.Status)
	}

	// Check for review packet artifacts in run evidence directory
	runEvidenceDir := store.RunEvidenceDir(runID)
	productReviewPath := filepath.Join(runEvidenceDir, "product-review.json")
	processReviewPath := filepath.Join(runEvidenceDir, "process-review.json")
	manualChecklistPath := filepath.Join(runEvidenceDir, "manual-checklist.json")

	// Check if artifacts exist
	_, errProd := os.Stat(productReviewPath)
	_, errProc := os.Stat(processReviewPath)
	_, errManual := os.Stat(manualChecklistPath)

	// If all artifacts exist, return early
	if errProd == nil && errProc == nil && errManual == nil {
		return run, store, runEvidenceDir, nil
	}

	// Regenerate artifacts: load inputs from evidence, generate outputs, write artifacts
	specPath := filepath.Join(store.RunDir(runID), "spec.md")
	inputs, err := reviewpacket.InputsFromEvidence(runEvidenceDir, specPath, run)
	if err != nil {
		return nil, nil, "", fmt.Errorf("load inputs from evidence: %w", err)
	}

	// Generate review packet outputs
	gen := &reviewpacket.Generator{}
	outputs, err := gen.Generate(inputs)
	if err != nil {
		return nil, nil, "", fmt.Errorf("generate review packet: %w", err)
	}

	// Write artifacts to run evidence directory
	if err := os.MkdirAll(runEvidenceDir, 0o755); err != nil {
		return nil, nil, "", fmt.Errorf("create evidence directory: %w", err)
	}

	// Write product review
	outputs.ProductReview.NormalizeNilFields()
	prodData, err := json.MarshalIndent(outputs.ProductReview, "", "  ")
	if err != nil {
		return nil, nil, "", fmt.Errorf("marshal product review: %w", err)
	}
	if err := os.WriteFile(productReviewPath, prodData, 0o644); err != nil {
		return nil, nil, "", fmt.Errorf("write product review: %w", err)
	}

	// Write process review
	outputs.ProcessReview.NormalizeNilFields()
	procData, err := json.MarshalIndent(outputs.ProcessReview, "", "  ")
	if err != nil {
		return nil, nil, "", fmt.Errorf("marshal process review: %w", err)
	}
	if err := os.WriteFile(processReviewPath, procData, 0o644); err != nil {
		return nil, nil, "", fmt.Errorf("write process review: %w", err)
	}

	// Write manual checklist
	outputs.ManualChecklist.NormalizeNilFields()
	manualData, err := json.MarshalIndent(outputs.ManualChecklist, "", "  ")
	if err != nil {
		return nil, nil, "", fmt.Errorf("marshal manual checklist: %w", err)
	}
	if err := os.WriteFile(manualChecklistPath, manualData, 0o644); err != nil {
		return nil, nil, "", fmt.Errorf("write manual checklist: %w", err)
	}

	return run, store, runEvidenceDir, nil
}

// reviewRecord records a review outcome and writes review-outcome.json to the run's evidence directory.
// All unwalked checklist items default to skipped.
func reviewRecord(runID string, storeDir string, outcome string, summary string, overrideReason string) error {
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

	// Create session and skip all remaining items (non-interactive mode)
	session := reviewsession.Start(*outputs)
	session.SkipRemaining()

	// Record the outcome with validation
	reviewOutcome, err := session.RecordOutcome(outcome, summary, overrideReason)
	if err != nil {
		return fmt.Errorf("record outcome: %w", err)
	}

	// Normalize nil fields and write review-outcome.json
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

	// Attempt automatic distillation (non-blocking on error)
	const defaultClaudeBinary = "claude"
	defaultPolicy := execpolicy.DefaultPolicy()

	// Check if claude binary is available before attempting distillation
	claudePath, err := exec.LookPath(defaultClaudeBinary)
	if err != nil {
		log.Printf("distillation skipped: %q not found in PATH", defaultClaudeBinary)
	} else {
		client, err := claude.NewClient(claudePath, []string{"--dangerously-skip-permissions"}, defaultPolicy.Budgets.MaxTaskDurationSeconds)
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
	}

	return nil
}

// loadPacketOutputs loads the review packet outputs from evidence directory.
func loadPacketOutputs(evidenceDir string) (*reviewpacket.Outputs, error) {
	// Read product-review.json
	productPath := filepath.Join(evidenceDir, "product-review.json")
	productData, err := os.ReadFile(productPath)
	if err != nil {
		return nil, fmt.Errorf("read product-review.json: %w", err)
	}
	var productReview reviewpacket.ProductReview
	if err := json.Unmarshal(productData, &productReview); err != nil {
		return nil, fmt.Errorf("parse product-review.json: %w", err)
	}

	// Read process-review.json
	processPath := filepath.Join(evidenceDir, "process-review.json")
	processData, err := os.ReadFile(processPath)
	if err != nil {
		return nil, fmt.Errorf("read process-review.json: %w", err)
	}
	var processReview reviewpacket.ProcessReview
	if err := json.Unmarshal(processData, &processReview); err != nil {
		return nil, fmt.Errorf("parse process-review.json: %w", err)
	}

	// Read manual-checklist.json
	manualPath := filepath.Join(evidenceDir, "manual-checklist.json")
	manualData, err := os.ReadFile(manualPath)
	if err != nil {
		return nil, fmt.Errorf("read manual-checklist.json: %w", err)
	}
	var manualChecklist reviewpacket.ManualChecklist
	if err := json.Unmarshal(manualData, &manualChecklist); err != nil {
		return nil, fmt.Errorf("parse manual-checklist.json: %w", err)
	}

	outputs := &reviewpacket.Outputs{
		ProductReview:   productReview,
		ProcessReview:   processReview,
		ManualChecklist: manualChecklist,
	}
	return outputs, nil
}
