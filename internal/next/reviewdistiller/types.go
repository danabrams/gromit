// Package reviewdistiller provides types for distilling code review feedback into actionable proposals.
package reviewdistiller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Tier is a distinct string type for model tier levels.
// Using a named type prevents accidentally passing a resolved model name
// (e.g. "claude-3-5-sonnet-20241022") where a tier label is expected.
type Tier string

const (
	TierLow    Tier = "low"
	TierMedium Tier = "medium"
	TierHigh   Tier = "high"
)

// Proposal represents a single improvement proposal derived from review feedback.
type Proposal struct {
	ID                  string   `json:"id"`
	Type                string   `json:"type"` // doctrine_rule, validation_gap, planner_heuristic, refinement_guidance
	Title               string   `json:"title"`
	WhatHappened        string   `json:"what_happened"`
	WhatWasMissing      string   `json:"what_was_missing"`
	ProposedChange      string   `json:"proposed_change"`
	Rationale           string   `json:"rationale"`
	Confidence          string   `json:"confidence"` // high, medium, low
	ConfidenceRationale string   `json:"confidence_rationale"`
	EvidenceReferences  []string `json:"evidence_references"`
}

// DistillationResult contains the outcome of the distillation process.
type DistillationResult struct {
	RunID     string     `json:"run_id"`
	SpecID    string     `json:"spec_id"`
	Outcome   string     `json:"outcome"`
	ModelTier Tier       `json:"model_tier"`
	Proposals []Proposal `json:"proposals"`
	CreatedAt time.Time  `json:"created_at"`
}

// DistillerInputs bundles all artifacts the distiller needs.
// This is an in-memory data transfer object — it is not serialized.
// The caller loads files; the distiller never touches the filesystem.
type DistillerInputs struct {
	RunID             string
	SpecID            string
	SpecContent       string
	ReviewOutcome     json.RawMessage // review-outcome.json
	ProductReview     json.RawMessage // product-review.json
	ProcessReview     json.RawMessage // process-review.json
	ManualChecklist   json.RawMessage // manual-checklist.json (generated template)
	Validation        json.RawMessage // validation.json
	Acceptance        json.RawMessage // acceptance.json
	MachineReview     json.RawMessage // review.json
	TaskResults       json.RawMessage // task-results.json (if available)
	RunMetadata       json.RawMessage // serialized subset of run state
	RejectedProposals json.RawMessage // previously rejected proposals with rejection reasons
}

// LLMCompleter abstracts the model call so the distiller is testable.
// Production wraps llmadapter.Invoker, extracting the text from provider.Result.
// Tests use a stub returning canned JSON.
type LLMCompleter interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// validateOutcomeType checks that ReviewOutcome is a valid JSON object.
func validateOutcomeType(outcomeJSON json.RawMessage) error {
	if len(outcomeJSON) == 0 {
		return fmt.Errorf("outcome type is empty")
	}

	var outcomeObj map[string]interface{}
	if err := json.Unmarshal(outcomeJSON, &outcomeObj); err != nil {
		return fmt.Errorf("invalid outcome JSON: %w", err)
	}

	return nil
}

// extractOutcomeType extracts the outcome string from the ReviewOutcome JSON.
// Expects a JSON object with an "outcome" field containing the outcome type string.
func extractOutcomeType(outcomeJSON json.RawMessage) (string, error) {
	var outcomeObj struct {
		Outcome string `json:"outcome"`
	}

	if err := json.Unmarshal(outcomeJSON, &outcomeObj); err != nil {
		return "", fmt.Errorf("failed to unmarshal outcome: %w", err)
	}

	if outcomeObj.Outcome == "" {
		return "", fmt.Errorf("outcome field is empty")
	}

	return outcomeObj.Outcome, nil
}
