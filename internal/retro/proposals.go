package retro

import (
	"fmt"
	"strings"

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

type Taxonomy struct {
	Technical        []string `json:"technical"`
	Architecture     []string `json:"architecture"`
	Process          []string `json:"process"`
	RepeatedPatterns []string `json:"repeated_patterns"`
}

type SystemAction struct {
	Finding          string `json:"finding"`
	Type             string `json:"type"`
	LocalFix         string `json:"local_fix"`
	SystemFix        string `json:"system_fix"`
	Owner            string `json:"owner"`
	DueDate          string `json:"due_date"`
	LeadingIndicator string `json:"leading_indicator"`
}

type WhyChainItem struct {
	Why     int    `json:"why"`
	Because string `json:"because"`
}

type FiveWhysProposal struct {
	Item           string         `json:"item"`
	Impact         string         `json:"impact"`
	WhyChain       []WhyChainItem `json:"why_chain"`
	RootCauseType  string         `json:"root_cause_type"`
	StoppingReason string         `json:"stopping_reason"`
}

// FrictionOption represents a remediation option for a workmanship finding.
type FrictionOption struct {
	Description string `json:"description"`
}

// WorkmanshipProposal captures workmanship-related findings from retro output.
type WorkmanshipProposal struct {
	Description     string           `json:"description"`
	FrictionOptions []FrictionOption `json:"friction_options,omitempty"`
}

// Proposals represents all proposals from a retro analysis
type Proposals struct {
	Consolidations []ConsolidationProposal `json:"consolidations,omitempty"`
	Promotions     []PromotionProposal     `json:"promotions,omitempty"`
	Archives       []ArchiveProposal       `json:"archives,omitempty"`
	RuleChanges    []RuleChangeProposal    `json:"rule_changes,omitempty"`
	Workmanship    []WorkmanshipProposal   `json:"workmanship,omitempty"`
	Taxonomy       Taxonomy                `json:"taxonomy,omitempty"`
	SystemActions  []SystemAction          `json:"system_actions,omitempty"`
	FiveWhys       []FiveWhysProposal      `json:"five_whys,omitempty"`
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
// This prevents issues with downstream code that marshals to JSON (nil → "null"
// vs [] → "[]") and ensures consistent behavior.
// See CLAUDE.md nil-field normalization visibility convention:
// Proposals lives in retro/, so the helper stays unexported.
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
	if p.Workmanship == nil {
		p.Workmanship = []WorkmanshipProposal{}
	}
	if p.SystemActions == nil {
		p.SystemActions = []SystemAction{}
	}
	if p.FiveWhys == nil {
		p.FiveWhys = []FiveWhysProposal{}
	}
	(&p.Taxonomy).normalizeNilFields()
	// Normalize nested slice fields in each ConsolidationProposal
	for i := range p.Consolidations {
		p.Consolidations[i].normalizeNilFields()
	}
	for i := range p.Workmanship {
		p.Workmanship[i].normalizeNilFields()
	}
	for i := range p.FiveWhys {
		p.FiveWhys[i].normalizeNilFields()
	}
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
// See CLAUDE.md nil-field normalization visibility convention:
// ConsolidationProposal lives in retro/, so the helper stays unexported.
func (c *ConsolidationProposal) normalizeNilFields() {
	if c == nil {
		return
	}
	if c.LearningHashes == nil {
		c.LearningHashes = []string{}
	}
}

func (w *WorkmanshipProposal) normalizeNilFields() {
	if w == nil {
		return
	}
	if w.FrictionOptions == nil {
		w.FrictionOptions = []FrictionOption{}
	}
}

func (t *Taxonomy) normalizeNilFields() {
	if t == nil {
		return
	}
	if t.Technical == nil {
		t.Technical = []string{}
	}
	if t.Architecture == nil {
		t.Architecture = []string{}
	}
	if t.Process == nil {
		t.Process = []string{}
	}
	if t.RepeatedPatterns == nil {
		t.RepeatedPatterns = []string{}
	}
}

func (f *FiveWhysProposal) normalizeNilFields() {
	if f == nil {
		return
	}
	if f.WhyChain == nil {
		f.WhyChain = []WhyChainItem{}
	}
}

// ParseProposals extracts structured proposals from Claude's analysis output.
// It looks for JSON in the output and unmarshals it into Proposals.
func ParseProposals(output string) (*Proposals, error) {
	var proposals Proposals

	// If a JSON code block is present, treat it as authoritative. An invalid
	// code block should fail fast instead of falling back to incidental JSON
	// fragments elsewhere in the output.
	if strings.Contains(output, "```") {
		if err := jsonutil.ExtractCodeBlock(output, &proposals); err != nil {
			return nil, fmt.Errorf("parsing proposals: %w", err)
		}
	} else {
		if err := jsonutil.ExtractJSON(output, &proposals); err != nil {
			return nil, fmt.Errorf("parsing proposals: %w", err)
		}
	}

	proposals.normalizeNilFields()

	return &proposals, nil
}
