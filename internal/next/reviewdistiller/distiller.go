package reviewdistiller

import (
	"encoding/json"
	"fmt"
	"time"
)

// Distill orchestrates the complete distillation pipeline:
// 1. Validate the outcome type
// 2. Build a prompt from inputs
// 3. Invoke the LLMCompleter to get a JSON response
// 4. Parse the JSON response into proposals
// 5. Truncate proposals to max 5
// 6. Generate proposal IDs
// 7. Run outcome-specific validation
// 8. Assemble and return the DistillationResult
func Distill(inputs *DistillerInputs, llm LLMCompleter, tier Tier) (*DistillationResult, error) {
	// Step 1: Validate outcome type by checking what it expects
	if err := validateOutcomeType(inputs.ReviewOutcome); err != nil {
		return nil, err
	}

	// Extract outcome string from ReviewOutcome JSON
	outcome, err := extractOutcomeType(inputs.ReviewOutcome)
	if err != nil {
		return nil, fmt.Errorf("failed to extract outcome type: %w", err)
	}

	// Step 2: Build the prompt
	prompt := BuildPrompt(inputs, outcome)

	// Step 3: Invoke LLMCompleter
	response, err := llm.Complete(nil, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Step 4: Parse JSON response into proposals
	proposals, err := parseProposalsFromJSON(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proposals from LLM response: %w", err)
	}

	// Step 5: Truncate to max 5 proposals
	if len(proposals) > 5 {
		proposals = proposals[:5]
	}

	// Step 6: Generate proposal IDs
	for i := range proposals {
		proposals[i].ID = GenerateProposalID(i)
	}

	// Step 7: Run outcome-specific validation
	if err := ValidateProposals(outcome, proposals); err != nil {
		return nil, fmt.Errorf("proposal validation failed: %w", err)
	}

	// Step 8: Assemble and return DistillationResult
	result := &DistillationResult{
		RunID:     inputs.RunID,
		SpecID:    inputs.SpecID,
		Outcome:   outcome,
		ModelTier: tier,
		Proposals: proposals,
		CreatedAt: time.Now(),
	}

	return result, nil
}

// validateOutcomeType checks that ReviewOutcome is a valid JSON object.
// The actual outcome validation happens in ValidateProposals after proposals are extracted.
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

// parseProposalsFromJSON parses a JSON response from the LLM into a slice of Proposal structs.
// Expects a JSON array or an object with a "proposals" field containing an array.
func parseProposalsFromJSON(jsonStr string) ([]Proposal, error) {
	var proposals []Proposal

	// First, try to parse as a direct array
	if err := json.Unmarshal([]byte(jsonStr), &proposals); err == nil {
		return proposals, nil
	}

	// If that fails, try to parse as an object with a "proposals" field
	var proposalObj struct {
		Proposals []Proposal `json:"proposals"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &proposalObj); err != nil {
		return nil, fmt.Errorf("failed to parse proposals from JSON: %w", err)
	}

	return proposalObj.Proposals, nil
}

// GenerateProposalID generates a unique ID for a proposal based on its index.
// Uses a simple "p<index>" format where index starts at 1.
func GenerateProposalID(index int) string {
	return fmt.Sprintf("p%d", index+1)
}
