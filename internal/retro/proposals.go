package retro

import (
	"fmt"

	"github.com/danabrams/gromit/internal/jsonutil"
)

// ConsolidationProposal represents a proposal to merge related learnings
type ConsolidationProposal struct {
	LearningHashes   []string `json:"learning_hashes"`   // Hashes of learnings to merge
	ConsolidatedText string   `json:"consolidated_text"` // The merged learning text
	Rationale        string   `json:"rationale"`         // Why these should be merged
}

// PromotionProposal represents a proposal to promote a learning to a rule
type PromotionProposal struct {
	LearningHash string `json:"learning_hash"` // Hash of the learning to promote
	ProposedRule string `json:"proposed_rule"` // How it should appear in RULES.md
	Section      string `json:"section"`       // Target section (Code Style, Architecture, Safety, Process)
	Rationale    string `json:"rationale"`     // Why this should be a rule
}

// ArchiveProposal represents a proposal to archive a stale learning
type ArchiveProposal struct {
	LearningHash string `json:"learning_hash"` // Hash of the learning to archive
	Rationale    string `json:"rationale"`     // Why this is no longer relevant
}

// RuleChangeProposal represents a proposal to modify an existing rule
type RuleChangeProposal struct {
	CurrentRule  string `json:"current_rule"`  // Exact text from RULES.md
	ProposedRule string `json:"proposed_rule"` // New text
	Rationale    string `json:"rationale"`     // Why this change is needed
}

// Proposals represents all proposals from a retro analysis
type Proposals struct {
	Consolidations []ConsolidationProposal `json:"consolidations,omitempty"`
	Promotions     []PromotionProposal     `json:"promotions,omitempty"`
	Archives       []ArchiveProposal       `json:"archives,omitempty"`
	RuleChanges    []RuleChangeProposal    `json:"rule_changes,omitempty"`
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
// This prevents issues with downstream code that marshals to JSON (nil → "null"
// vs [] → "[]") and ensures consistent behavior.
func (p *Proposals) normalizeNilFields() {
	if p == nil {
		return
	}
	if p.Consolidations == nil {
		p.Consolidations = []ConsolidationProposal{}
	}
	if p.Promotions == nil {
		p.Promotions = []PromotionProposal{}
	}
	if p.Archives == nil {
		p.Archives = []ArchiveProposal{}
	}
	if p.RuleChanges == nil {
		p.RuleChanges = []RuleChangeProposal{}
	}
	// Normalize nested slice fields in each ConsolidationProposal
	for i := range p.Consolidations {
		p.Consolidations[i].normalizeNilFields()
	}
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
func (c *ConsolidationProposal) normalizeNilFields() {
	if c == nil {
		return
	}
	if c.LearningHashes == nil {
		c.LearningHashes = []string{}
	}
}

// ParseProposals extracts structured proposals from Claude's analysis output.
// It looks for JSON in the output and unmarshals it into Proposals.
func ParseProposals(output string) (*Proposals, error) {
	var proposals Proposals
	if err := jsonutil.ExtractJSON(output, &proposals); err != nil {
		return nil, fmt.Errorf("parsing proposals: %w", err)
	}

	proposals.normalizeNilFields()

	return &proposals, nil
}
