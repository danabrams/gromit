package proposaltriage

import (
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

// PendingProposal wraps a reviewdistiller.Proposal with its run and spec context.
type PendingProposal struct {
	Proposal *reviewdistiller.Proposal `json:"proposal"`
	RunID    string                    `json:"run_id"`
	SpecID   string                    `json:"spec_id"`
}

// AllProposal wraps a reviewdistiller.Proposal with its run, spec context, and optional decision.
type AllProposal struct {
	Proposal *reviewdistiller.Proposal `json:"proposal"`
	RunID    string                    `json:"run_id"`
	SpecID   string                    `json:"spec_id"`
	Decision *Decision                 `json:"decision,omitempty"`
}

// NormalizeNilFields maps nil slices/maps to empty values for consistent JSON serialization.
func (pp *PendingProposal) NormalizeNilFields() {
	if pp.Proposal != nil && pp.Proposal.EvidenceReferences == nil {
		pp.Proposal.EvidenceReferences = []string{}
	}
}

// Decision represents a triage decision on a proposal.
type Decision struct {
	ProposalID       string    `json:"proposal_id"`
	Action           string    `json:"action"` // accepted or rejected
	Reason           string    `json:"reason"`
	ApprovedTitle    string    `json:"approved_title"`
	ApprovedChange   string    `json:"approved_change"`
	ApprovedRationale string   `json:"approved_rationale"`
	MaterializedID   string    `json:"materialized_id"`
	DuplicateOf      string    `json:"duplicate_of"`
	DecidedAt        time.Time `json:"decided_at"`
}

// NormalizeNilFields maps nil slices/maps to empty values for consistent JSON serialization.
func (d *Decision) NormalizeNilFields() {
	// Decision struct has no slice or map fields, so nothing to normalize.
	// This method exists for consistency with project conventions.
}

// ProposalWithStatus wraps a reviewdistiller.Proposal with explicit status, run, and spec context.
type ProposalWithStatus struct {
	Proposal *reviewdistiller.Proposal `json:"proposal"`
	RunID    string                    `json:"run_id"`
	SpecID   string                    `json:"spec_id"`
	Status   string                    `json:"status"` // "pending", "accepted", or "rejected"
}

// DiscoverFilter wraps optional filter criteria for discovering proposals.
type DiscoverFilter struct {
	ProposalTypes *[]string `json:"proposal_types,omitempty"`
	RunIDs        *[]string `json:"run_ids,omitempty"`
}
