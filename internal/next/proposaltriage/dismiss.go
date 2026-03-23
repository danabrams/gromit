package proposaltriage

import (
	"path/filepath"
	"time"
)

// DismissSiblings creates dismissed decisions for all proposals in a group
// except the accepted one. Decisions are saved to each sibling's run evidence directory.
// Returns the slice of created decisions.
func DismissSiblings(acceptedProposalID string, group ProposalGroup, storeDir string) ([]Decision, error) {
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
		evidenceDir := filepath.Join(storeDir, "runs", sibling.RunID, "evidence")
		if err := SaveDecision(evidenceDir, decision); err != nil {
			return nil, err
		}

		decisions = append(decisions, decision)
	}

	return decisions, nil
}
