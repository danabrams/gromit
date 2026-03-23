package proposaltriage

import (
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// DismissSiblings creates dismissed decisions for all proposals in a group
// except the accepted one. Decisions are saved to each sibling's run evidence directory.
// Returns the slice of created decisions.
func DismissSiblings(acceptedProposalID string, group ProposalGroup, store *runstore.Store) ([]Decision, error) {
	var decisions []Decision

	for _, sibling := range group.Proposals {
		// Skip siblings with nil Proposal
		if sibling.Proposal == nil {
			continue
		}

		// Skip the accepted proposal
		if sibling.Proposal.ID == acceptedProposalID {
			continue
		}

		// Create decision for this sibling
		decision := Decision{
			ProposalID:  sibling.Proposal.ID,
			Action:      "dismissed",
			DismissedBy: acceptedProposalID,
			DecidedAt:   time.Now(),
		}

		// Save to the sibling's run evidence directory
		evidenceDir := store.RunEvidenceDir(sibling.RunID)
		if err := SaveDecision(evidenceDir, decision); err != nil {
			return nil, err
		}

		decisions = append(decisions, decision)
	}

	return decisions, nil
}
