package proposaltriage

import (
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
)

// Reject creates a Decision with action=rejected for a PendingProposal.
// Records the rejection reason and current timestamp.
// Returns an error if the proposal is in a terminal state (dismissed).
func Reject(pp *PendingProposal, reason string, existingDecision *Decision) (*Decision, error) {
	if pp == nil || pp.Proposal == nil {
		return nil, fmt.Errorf("pending proposal is nil")
	}

	// Validate terminal state
	if existingDecision != nil && IsTerminalDecision(*existingDecision) {
		return nil, fmt.Errorf("proposal %q cannot be re-decided: it has been dismissed", pp.Proposal.ID)
	}

	decision := &Decision{
		ProposalID: pp.Proposal.ID,
		Action:     "rejected",
		Reason:     reason,
		DecidedAt:  time.Now(),
	}

	return decision, nil
}

// RejectAfterAccept supersedes a previously accepted entry and records the rejection.
// It looks up the entry by the materialized ID from the accepted decision,
// marks it as superseded with the rejection proposal ID, and saves both stores.
// Returns an error if the accepted proposal is in a terminal state (dismissed).
func RejectAfterAccept(
	acceptedDecision *Decision,
	rejectionDecision *Decision,
	decisions []Decision,
	doctrineStore doctrine.Store,
	playbookStore *playbook.Store,
) error {
	if acceptedDecision == nil || rejectionDecision == nil {
		return fmt.Errorf("decisions cannot be nil")
	}

	// Validate terminal state of the accepted proposal
	if err := ValidateTerminalState(acceptedDecision.ProposalID, decisions); err != nil {
		return err
	}

	materializedID := acceptedDecision.MaterializedID
	if materializedID == "" {
		return fmt.Errorf("accepted decision has no materialized ID")
	}

	// Determine which store to update based on materialized ID prefix
	if strings.HasPrefix(materializedID, "promoted-") {
		// Update doctrine store
		if doctrineStore == nil {
			return fmt.Errorf("doctrine store required for promoted entry")
		}

		existingDoctrine, err := doctrineStore.Load()
		if err != nil {
			return fmt.Errorf("failed to load doctrine: %w", err)
		}

		// Find and update the rule
		found := false
		for i, rule := range existingDoctrine.Rules {
			if rule.ID == materializedID {
				existingDoctrine.Rules[i].Status = "superseded"
				existingDoctrine.Rules[i].SupersededBy = rejectionDecision.ProposalID
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("no doctrine rule found with ID %s", materializedID)
		}

		if err := doctrineStore.Save(existingDoctrine); err != nil {
			return fmt.Errorf("failed to save doctrine: %w", err)
		}
	} else {
		// Update playbook store
		if playbookStore == nil {
			return fmt.Errorf("playbook store required for playbook entry")
		}

		existingEntries, err := playbookStore.Load()
		if err != nil {
			return fmt.Errorf("failed to load playbook entries: %w", err)
		}

		// Find and update the entry
		found := false
		for i, entry := range existingEntries {
			if entry.ID == materializedID {
				existingEntries[i].Status = "superseded"
				existingEntries[i].SupersededBy = rejectionDecision.ProposalID
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("no playbook entry found with ID %s", materializedID)
		}

		if err := playbookStore.Save(existingEntries); err != nil {
			return fmt.Errorf("failed to save playbook entries: %w", err)
		}
	}

	return nil
}
