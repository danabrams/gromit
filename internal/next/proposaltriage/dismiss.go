package proposaltriage

import (
	"errors"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// DismissSiblings creates dismissed decisions for all proposals in a group
// except the accepted one. Decisions are saved to each sibling's run evidence directory.
// Returns the slice of created decisions.
func DismissSiblings(acceptedProposalID string, group ProposalGroup, store *runstore.Store) ([]Decision, error) {
	// Validate that acceptedProposalID exists in the group
	found := false
	for _, proposal := range group.Proposals {
		if proposal.Proposal != nil && proposal.Proposal.ID == acceptedProposalID {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("acceptedProposalID not found in group")
	}

	// Step 1: Collect all decisions grouped by run evidence directory
	decisionsByRunDir := make(map[string][]Decision)
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

		// Group by run evidence directory
		evidenceDir := store.RunEvidenceDir(sibling.RunID)
		decisionsByRunDir[evidenceDir] = append(decisionsByRunDir[evidenceDir], decision)
		decisions = append(decisions, decision)
	}

	// Step 2: Save all collected decisions to their run directories.
	// Each batch of decisions is saved using upsert semantics: loading existing decisions,
	// deduplicating on ProposalID, and saving the combined result. This load-merge-save
	// pattern makes DismissSiblings idempotent—retrying the dismiss-group operation after
	// a partial failure will not duplicate dismissed decisions.
	var saveErrors []error
	for evidenceDir, runDecisions := range decisionsByRunDir {
		if err := SaveDecisions(evidenceDir, runDecisions); err != nil {
			saveErrors = append(saveErrors, err)
		}
	}

	return decisions, errors.Join(saveErrors...)
}
