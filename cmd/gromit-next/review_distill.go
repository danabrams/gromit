package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/spf13/cobra"
)

// newReviewDistillCmd creates the `review distill` command.
func newReviewDistillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "distill [run-id]",
		Short: "Distill review outcomes into improvement proposals",
		Long: `Distill a run's review outcomes into structured improvement proposals.
If no run-id is given, the latest run is distilled (by modification time of .gromit-next/runs/).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			storeDir, _ := cmd.Flags().GetString("store-dir")
			tierStr, _ := cmd.Flags().GetString("tier")
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

			// Determine tier (CLI override takes precedence)
			tier := reviewdistiller.TierMedium
			if tierStr != "" {
				tier = reviewdistiller.Tier(tierStr)
				if tier != reviewdistiller.TierLow && tier != reviewdistiller.TierMedium && tier != reviewdistiller.TierHigh {
					return fmt.Errorf("invalid tier: %q (must be low, medium, or high)", tierStr)
				}
			}

			return reviewDistill(runID, storeDir, tier)
		},
	}
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	cmd.Flags().String("run", "", "Run ID to distill (if not specified, uses latest or positional argument)")
	cmd.Flags().String("tier", "", "Override configured model tier (low, medium, high)")
	return cmd
}

// reviewDistill runs distillation for a run.
func reviewDistill(runID string, storeDir string, tier reviewdistiller.Tier) error {
	// Use stub completer for now
	completer := &stubLLMCompleter{}

	if err := attemptDistillation(runID, storeDir, tier, completer); err != nil {
		return err
	}

	// Load result for display
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(runID)
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")

	proposalsData, err := os.ReadFile(proposalsPath)
	if err != nil {
		return fmt.Errorf("read distillation-proposals.json: %w", err)
	}

	var result reviewdistiller.DistillationResult
	if err := json.Unmarshal(proposalsData, &result); err != nil {
		return fmt.Errorf("parse distillation-proposals.json: %w", err)
	}

	fmt.Printf("Distillation complete for run %q\n", runID)
	fmt.Printf("  - Outcome: %s\n", result.Outcome)
	fmt.Printf("  - Model tier: %s\n", result.ModelTier)
	fmt.Printf("  - Proposals: %d\n", len(result.Proposals))
	fmt.Printf("  - Written to: %s and %s\n", proposalsPath, filepath.Join(evidenceDir, "distillation-proposals.md"))

	return nil
}

// attemptDistillation loads artifacts, invokes distiller, and writes outputs.
// Returns error (non-blocking — callers log the error).
func attemptDistillation(runID string, storeDir string, tier reviewdistiller.Tier, completer reviewdistiller.LLMCompleter) error {
	// Load run
	store := runstore.NewStore(storeDir)
	run, err := store.Get(runID)
	if err != nil {
		return fmt.Errorf("load run %q: %w", runID, err)
	}

	evidenceDir := store.RunEvidenceDir(runID)

	// Check if review-outcome.json exists
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err != nil {
		return fmt.Errorf("no review outcome has been recorded for run %q (review-outcome.json not found)", runID)
	}

	// Ensure review packet exists (regenerate if missing)
	_, _, _, err = loadRunAndEnsurePacket(runID, storeDir)
	if err != nil {
		return fmt.Errorf("ensure review packet: %w", err)
	}

	// Load run spec
	runDir := store.RunDir(runID)
	specPath := filepath.Join(runDir, "spec.md")
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("read spec: %w", err)
	}

	// Load all artifact files into DistillerInputs
	inputs := &reviewdistiller.DistillerInputs{
		RunID:       runID,
		SpecID:      run.SpecID,
		SpecContent: string(specContent),
	}

	// Load review-outcome.json
	outcomeData, err := os.ReadFile(outcomeFile)
	if err != nil {
		return fmt.Errorf("read review-outcome.json: %w", err)
	}
	inputs.ReviewOutcome = json.RawMessage(outcomeData)

	// Load product-review.json
	productFile := filepath.Join(evidenceDir, "product-review.json")
	if productData, err := os.ReadFile(productFile); err == nil {
		inputs.ProductReview = json.RawMessage(productData)
	}

	// Load process-review.json
	processFile := filepath.Join(evidenceDir, "process-review.json")
	if processData, err := os.ReadFile(processFile); err == nil {
		inputs.ProcessReview = json.RawMessage(processData)
	}

	// Load manual-checklist.json
	checklistFile := filepath.Join(evidenceDir, "manual-checklist.json")
	if checklistData, err := os.ReadFile(checklistFile); err == nil {
		inputs.ManualChecklist = json.RawMessage(checklistData)
	}

	// Load validation.json
	validationFile := filepath.Join(evidenceDir, "validation.json")
	if validationData, err := os.ReadFile(validationFile); err == nil {
		inputs.Validation = json.RawMessage(validationData)
	}

	// Load acceptance.json
	acceptanceFile := filepath.Join(evidenceDir, "acceptance.json")
	if acceptanceData, err := os.ReadFile(acceptanceFile); err == nil {
		inputs.Acceptance = json.RawMessage(acceptanceData)
	}

	// Load review.json (machine-review)
	reviewFile := filepath.Join(evidenceDir, "review.json")
	if reviewData, err := os.ReadFile(reviewFile); err == nil {
		inputs.MachineReview = json.RawMessage(reviewData)
	}

	// Load task-results.json if available
	taskResultsFile := filepath.Join(evidenceDir, "task-results.json")
	if taskResultsData, err := os.ReadFile(taskResultsFile); err == nil {
		inputs.TaskResults = json.RawMessage(taskResultsData)
	}

	// Load run metadata (serialized subset of run state)
	runMetadata := map[string]interface{}{
		"status":          run.Status,
		"cycle":           run.Cycle,
		"blocker_summary": run.BlockerSummary,
		"replan_context":  run.ReplanContext,
		"failure_history": run.FailureHistory,
		"task_lineage":    run.Tasks,
	}
	metadataJSON, _ := json.Marshal(runMetadata)
	inputs.RunMetadata = json.RawMessage(metadataJSON)

	// Invoke distiller
	result, err := reviewdistiller.Distill(inputs, completer, tier)
	if err != nil {
		return fmt.Errorf("distillation failed: %w", err)
	}

	// Write distillation-proposals.json
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	proposalsJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal proposals: %w", err)
	}
	if err := os.WriteFile(proposalsPath, proposalsJSON, 0o644); err != nil {
		return fmt.Errorf("write distillation-proposals.json: %w", err)
	}

	// Write distillation-proposals.md
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	markdown := renderDistillationMarkdown(result)
	if err := os.WriteFile(markdownPath, []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("write distillation-proposals.md: %w", err)
	}

	return nil
}

// renderDistillationMarkdown renders a DistillationResult as markdown.
func renderDistillationMarkdown(result *reviewdistiller.DistillationResult) string {
	tmpl, err := template.New("distill").Parse(`# Distillation Proposals

**Run:** {{ .RunID }} | **Outcome:** {{ .Outcome }} | **Model Tier:** {{ .ModelTier }}

**Generated:** {{ .CreatedAt.Format "2006-01-02 15:04:05 UTC" }}

## Summary

Distilled {{ .Outcome }} review outcome into {{ len .Proposals }} improvement proposals.

{{ range .Proposals }}
---

## {{ .Title }}

**Type:** {{ .Type }}
**Confidence:** {{ .Confidence }}

### What Happened

{{ .WhatHappened }}

### What Was Missing

{{ .WhatWasMissing }}

### Proposed Change

{{ .ProposedChange }}

### Rationale

{{ .Rationale }}

**Confidence Rationale:** {{ .ConfidenceRationale }}

{{ if .EvidenceReferences }}
### Evidence References

{{ range .EvidenceReferences }}- {{ . }}
{{ end }}{{ end }}
{{ end }}
`)

	if err != nil {
		log.Printf("warning: template parse error: %v", err)
		return "# Distillation Proposals\n\nError rendering markdown\n"
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, result); err != nil {
		log.Printf("warning: template execute error: %v", err)
		return "# Distillation Proposals\n\nError rendering markdown\n"
	}

	return buf.String()
}

// invokerAdapter wraps llmadapter.Invoker to satisfy LLMCompleter.
type invokerAdapter struct {
	invoker llmadapter.Invoker
}

// NewInvokerAdapter creates an invokerAdapter from an llmadapter.Invoker.
func NewInvokerAdapter(invoker llmadapter.Invoker) reviewdistiller.LLMCompleter {
	return &invokerAdapter{invoker: invoker}
}

// Complete implements LLMCompleter by calling the invoker and extracting Output.
func (ia *invokerAdapter) Complete(ctx interface{}, prompt string) (string, error) {
	// Cast ctx to context.Context (or use background context if not provided)
	var actx context.Context
	if c, ok := ctx.(context.Context); ok {
		actx = c
	} else {
		// Fallback to background context if ctx is not a context.Context
		actx = context.Background()
	}

	result, err := ia.invoker.Invoke(actx, prompt)
	if err != nil {
		return "", err
	}

	return result.Output, nil
}

var _ reviewdistiller.LLMCompleter = (*invokerAdapter)(nil)

// stubLLMCompleter is a temporary stub LLMCompleter that returns mock proposals.
// This will be replaced with actual LLM invocation when integrated with llmadapter.
type stubLLMCompleter struct{}

func (s *stubLLMCompleter) Complete(ctx interface{}, prompt string) (string, error) {
	// Return a mock JSON response with sample proposals
	// This stub ensures the command structure works while waiting for LLM integration
	mockProposals := `{
  "proposals": [
    {
      "type": "doctrine_rule",
      "title": "Establish assertion isolation principle",
      "what_happened": "The test suite passed validation but assertions were tightly coupled to implementation details",
      "what_was_missing": "A documented doctrine rule on assertion isolation and brittle test detection",
      "proposed_change": "Add a doctrine rule: assertions should target behavior, not implementation; brittle tests should fail CI",
      "rationale": "Prevents future test fragility and maintenance burden",
      "confidence": "high",
      "confidence_rationale": "This pattern recurs across multiple failed runs",
      "evidence_references": ["validation.json", "acceptance.json"]
    },
    {
      "type": "validation_gap",
      "title": "Add mutation testing to validation suite",
      "what_happened": "Tests passed but implementation had subtle logic errors that mutation testing would catch",
      "what_was_missing": "Mutation testing as a validation stage",
      "proposed_change": "Integrate mutation testing (stryker-like tool) into post-acceptance validation",
      "rationale": "Catches logic errors that survive conventional test coverage",
      "confidence": "medium",
      "confidence_rationale": "Beneficial but requires toolchain investment",
      "evidence_references": ["validation.json"]
    }
  ]
}`
	return mockProposals, nil
}

// tierToModel converts a tier designation to a concrete model name.
// This function serves as the boundary between the abstract tier system
// and provider-specific model resolution.
func tierToModel(tier reviewdistiller.Tier) string {
	switch tier {
	case reviewdistiller.TierLow:
		return "claude-haiku-4-5-20251001"
	case reviewdistiller.TierMedium:
		return "claude-sonnet-4-20250514"
	case reviewdistiller.TierHigh:
		return "claude-opus-4-20250805"
	default:
		return "claude-sonnet-4-20250514" // default fallback
	}
}
