package reviewdistiller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// isValidOutcome checks if the outcome is one of the three recognized types.
func isValidOutcome(outcome string) bool {
	switch outcome {
	case "accepted", "rework_implementation_gap", "rework_vision_change":
		return true
	default:
		return false
	}
}

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
	if inputs == nil {
		return nil, fmt.Errorf("inputs cannot be nil")
	}

	// Step 1: Validate outcome type by checking what it expects
	if err := validateOutcomeType(inputs.ReviewOutcome); err != nil {
		return nil, err
	}

	// Extract outcome string from ReviewOutcome JSON
	outcome, err := extractOutcomeType(inputs.ReviewOutcome)
	if err != nil {
		return nil, fmt.Errorf("failed to extract outcome type: %w", err)
	}

	// Validate outcome is one of the three recognized types
	if !isValidOutcome(outcome) {
		return nil, fmt.Errorf("unsupported outcome type: %q (must be 'accepted', 'rework_implementation_gap', or 'rework_vision_change')", outcome)
	}

	// Step 2: Build the prompt
	prompt := BuildPrompt(inputs, outcome)

	// Step 3: Invoke LLMCompleter
	response, err := llm.Complete(context.Background(), prompt)
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
		proposals[i].ID = GenerateProposalID(inputs.RunID, proposals[i])
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

// extractJSON extracts the first complete JSON array or object from s,
// stripping markdown code fences and ignoring surrounding prose.
func extractJSON(s string) string {
	// Strip markdown code fences (```json ... ``` or ``` ... ```)
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "```"); idx != -1 {
		s = s[idx+3:]
		if strings.HasPrefix(s, "json") {
			s = s[4:]
		}
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	}
	s = strings.TrimSpace(s)

	// Find the first '[' or '{' and the matching closing bracket.
	start := strings.IndexAny(s, "[{")
	if start == -1 {
		return s
	}
	open := rune(s[start])
	close := map[rune]rune{'[': ']', '{': '}'}[open]
	depth := 0
	inStr := false
	escape := false
	for i, ch := range s[start:] {
		switch {
		case escape:
			escape = false
		case ch == '\\' && inStr:
			escape = true
		case ch == '"':
			inStr = !inStr
		case !inStr && ch == open:
			depth++
		case !inStr && ch == close:
			depth--
			if depth == 0 {
				return s[start : start+i+1]
			}
		}
	}
	return s[start:]
}

// parseProposalsFromJSON parses a JSON response from the LLM into a slice of Proposal structs.
// Handles markdown code fences and prose surrounding the JSON, and accepts either a
// JSON array or an object with a "proposals" field.
func parseProposalsFromJSON(jsonStr string) ([]Proposal, error) {
	jsonStr = extractJSON(jsonStr)

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
		return nil, fmt.Errorf("failed to parse proposals from JSON: %w; extracted: %s", err, truncateForError(jsonStr, 200))
	}

	return proposalObj.Proposals, nil
}

// truncateForError truncates s to at most n bytes, appending "..." if truncated.
// Used to bound error message size when embedding extracted content.
func truncateForError(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// GenerateProposalID generates a stable ID for a proposal based on its content.
// Concatenates whitespace-trimmed type, title, and proposed_change separated by \x00,
// computes SHA-256, takes the first 8 hex characters, and formats as '<run_id>-proposal-<short-hash>'.
// This ensures the same proposal content always generates the same ID.
func GenerateProposalID(runID string, p Proposal) string {
	content := strings.Join([]string{
		strings.TrimSpace(p.Type),
		strings.TrimSpace(p.Title),
		strings.TrimSpace(p.ProposedChange),
	}, "\x00")

	hash := sha256.Sum256([]byte(content))
	shortHash := hex.EncodeToString(hash[:])[:8]

	return fmt.Sprintf("%s-proposal-%s", runID, shortHash)
}
