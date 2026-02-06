package retro

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ConsolidationProposal represents a proposal to merge related learnings
type ConsolidationProposal struct {
	LearningHashes     []string `json:"learning_hashes"`     // Hashes of learnings to merge
	ConsolidatedText   string   `json:"consolidated_text"`   // The merged learning text
	Rationale          string   `json:"rationale"`           // Why these should be merged
}

// PromotionProposal represents a proposal to promote a learning to a rule
type PromotionProposal struct {
	LearningHash  string `json:"learning_hash"`  // Hash of the learning to promote
	ProposedRule  string `json:"proposed_rule"`  // How it should appear in RULES.md
	Section       string `json:"section"`        // Target section (Code Style, Architecture, Safety, Process)
	Rationale     string `json:"rationale"`      // Why this should be a rule
}

// ArchiveProposal represents a proposal to archive a stale learning
type ArchiveProposal struct {
	LearningHash string `json:"learning_hash"` // Hash of the learning to archive
	Rationale    string `json:"rationale"`     // Why this is no longer relevant
}

// RuleChangeProposal represents a proposal to modify an existing rule
type RuleChangeProposal struct {
	CurrentRule   string `json:"current_rule"`   // Exact text from RULES.md
	ProposedRule  string `json:"proposed_rule"`  // New text
	Rationale     string `json:"rationale"`      // Why this change is needed
}

// Proposals represents all proposals from a retro analysis
type Proposals struct {
	Consolidations []ConsolidationProposal `json:"consolidations,omitempty"`
	Promotions     []PromotionProposal     `json:"promotions,omitempty"`
	Archives       []ArchiveProposal       `json:"archives,omitempty"`
	RuleChanges    []RuleChangeProposal    `json:"rule_changes,omitempty"`
}

// ParseProposals extracts structured proposals from Claude's analysis output.
// It looks for a JSON code block in the output and unmarshals it into Proposals.
func ParseProposals(output string) (*Proposals, error) {
	// Extract JSON from fenced code block
	jsonStr := extractJSONBlock(output)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON code block found in output")
	}

	var proposals Proposals
	if err := json.Unmarshal([]byte(jsonStr), &proposals); err != nil {
		return nil, fmt.Errorf("parsing JSON proposals: %w", err)
	}

	return &proposals, nil
}

// extractJSONBlock finds and extracts content from a ```json code block
func extractJSONBlock(output string) string {
	// Match ```json...``` or ```...``` blocks
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)\\n```")
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}
