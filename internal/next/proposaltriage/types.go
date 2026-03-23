package proposaltriage

import (
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

// PendingProposal wraps a reviewdistiller.Proposal with its run and spec context.
type PendingProposal struct {
	Proposal  *reviewdistiller.Proposal `json:"proposal"`
	RunID     string                    `json:"run_id"`
	SpecID    string                    `json:"spec_id"`
	CreatedAt time.Time                 `json:"created_at"`
	GroupID   string                    `json:"group_id"`
	Scope     string                    `json:"scope"`
}

// ProposalGroup groups related proposals together with context.
type ProposalGroup struct {
	GroupID     string            `json:"group_id"`
	Proposals   []PendingProposal `json:"proposals"`
	GroupReason string            `json:"group_reason"`
}

// Decision represents a triage decision on a proposal.
type Decision struct {
	ProposalID        string    `json:"proposal_id"`
	Action            string    `json:"action"` // accepted, rejected, or dismissed
	Reason            string    `json:"reason"`
	ApprovedTitle     string    `json:"approved_title"`
	ApprovedChange    string    `json:"approved_change"`
	ApprovedRationale string    `json:"approved_rationale"`
	MaterializedID    string    `json:"materialized_id"`
	DuplicateOf       string    `json:"duplicate_of"`
	DismissedBy       string    `json:"dismissed_by,omitempty"`
	DecidedAt         time.Time `json:"decided_at"`
}

// See CLAUDE.md nil-field normalization visibility convention: exported — cross-package boundary type
// NormalizeNilFields maps nil slices/maps to empty values for consistent JSON serialization.
func (d *Decision) NormalizeNilFields() {
	// Decision struct has no slice or map fields, so nothing to normalize.
	// This method exists for consistency with project conventions.
}

// See CLAUDE.md nil-field normalization visibility convention: exported — cross-package boundary type
// NormalizeNilFields maps nil slices/maps to empty values for consistent JSON serialization.
func (pp *PendingProposal) NormalizeNilFields() {
	if pp.Proposal != nil && pp.Proposal.EvidenceReferences == nil {
		pp.Proposal.EvidenceReferences = []string{}
	}
}

// See CLAUDE.md nil-field normalization visibility convention: exported — cross-package boundary type
// NormalizeNilFields maps nil slices/maps to empty values for consistent JSON serialization.
func (pg *ProposalGroup) NormalizeNilFields() {
	if pg.Proposals == nil {
		pg.Proposals = []PendingProposal{}
	}
}
