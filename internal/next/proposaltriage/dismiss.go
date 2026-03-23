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

	// Step 2: Save each batch of decisions for its run directory.
	// This operation has upsert semantics (deduplicates by ProposalID),
	// making retries safe if this function is called again.
	var saveErrors []error
	for evidenceDir, runDecisions := range decisionsByRunDir {
		if err := SaveDecisions(evidenceDir, runDecisions); err != nil {
			saveErrors = append(saveErrors, err)
		}
	}

	return decisions, errors.Join(saveErrors...)
}
